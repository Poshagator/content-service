package campus

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	DefaultCampusAPIBase = "https://api.campus.dewish.ru/v3"
	DefaultKFUAPIBase    = "https://auth.kpfu.tyuop.ru/api/v1"
	DefaultOrganization  = "5f8692136da69409bce8f28f"
	DefaultTermName      = "Расписание Campus"
)

type Config struct {
	CampusAPIBase string
	KFUAPIBase    string
	DSN           string
	FilialID      string
	Organization  string
	SourceIDs     string
	SourceParam   string
	LimitSources  int
	Timeout       time.Duration
	TermName      string
	Logger        *zap.Logger
}

type Stats struct {
	Groups           int
	Teachers         int
	Schedules        int
	FetchErrors      int
	Events           int
	EntriesCreated   int
	EntriesExisting  int
	AffectedGroupIDs []string
}

type campusTeacher struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

type employeeSearchResponse struct {
	Success   bool          `json:"success"`
	Employees []kfuEmployee `json:"employees"`
}

type kfuEmployee struct {
	EmployeeID  int    `json:"employee_id"`
	Lastname    string `json:"lastname"`
	Firstname   string `json:"firstname"`
	Middlename  string `json:"middlename"`
	IsTeacher   bool   `json:"is_teacher"`
	Post        string `json:"post"`
	Subdivision string `json:"subdivision"`
}

type scheduleResponse struct {
	Success  bool         `json:"success"`
	Subjects []kfuSubject `json:"subjects"`
}

type kfuSubject struct {
	ID                    string `json:"id"`
	Semester              int    `json:"semester"`
	Year                  int    `json:"year"`
	SubjectName           string `json:"subject_name"`
	SubjectID             int    `json:"subject_id"`
	StartDaySchedule      string `json:"start_day_schedule"`
	FinishDaySchedule     string `json:"finish_day_schedule"`
	DayWeekSchedule       int    `json:"day_week_schedule"`
	TypeWeekSchedule      int    `json:"type_week_schedule"`
	NoteSchedule          string `json:"note_schedule"`
	TotalTimeSchedule     string `json:"total_time_schedule"`
	BeginTimeSchedule     string `json:"begin_time_schedule"`
	EndTimeSchedule       string `json:"end_time_schedule"`
	TeacherID             int    `json:"teacher_id"`
	TeacherLastname       string `json:"teacher_lastname"`
	TeacherFirstname      string `json:"teacher_firstname"`
	TeacherMiddlename     string `json:"teacher_middlename"`
	NumAuditoriumSchedule string `json:"num_auditorium_schedule"`
	BuildingName          string `json:"building_name"`
	BuildingID            string `json:"building_id"`
	GroupList             string `json:"group_list"`
	SubjectKindName       string `json:"subject_kind_name"`
}

type scheduleSource struct {
	EmployeeID int
	Name       string
	Position   string
}

func Sync(ctx context.Context, cfg Config) (Stats, error) {
	cfg = cfg.withDefaults()
	if strings.TrimSpace(cfg.DSN) == "" {
		return Stats{}, fmt.Errorf("dsn is required")
	}
	filialID, err := uuid.Parse(cfg.FilialID)
	if err != nil {
		return Stats{}, fmt.Errorf("parse filial id: %w", err)
	}

	client := &http.Client{Timeout: cfg.Timeout}
	sources, err := buildSources(ctx, client, cfg)
	if err != nil {
		return Stats{}, err
	}
	if cfg.LimitSources > 0 && len(sources) > cfg.LimitSources {
		sources = sources[:cfg.LimitSources]
	}
	if len(sources) == 0 {
		return Stats{}, fmt.Errorf("no Campus/KFU schedule sources found")
	}

	db, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return Stats{}, fmt.Errorf("connect postgres: %w", err)
	}
	defer db.Close()
	if err := ensureFilial(ctx, db, filialID); err != nil {
		return Stats{}, err
	}

	var stats Stats
	imp := &importer{
		db:             db,
		filialID:       filialID,
		termName:       cfg.TermName,
		subjects:       make(map[string]uuid.UUID),
		rooms:          make(map[string]uuid.UUID),
		groups:         make(map[string]uuid.UUID),
		teachers:       make(map[string]uuid.UUID),
		affectedGroups: make(map[uuid.UUID]struct{}),
	}

	total := len(sources)
	for idx, source := range sources {
		if cfg.Logger != nil && (idx == 0 || (idx+1)%25 == 0 || idx+1 == total) {
			cfg.Logger.Info("fetching and importing Campus schedule", zap.Int("current", idx+1), zap.Int("total", total), zap.Int("employeeID", source.EmployeeID), zap.String("teacher", source.Name))
		}
		schedule, err := fetchSchedule(ctx, client, cfg.KFUAPIBase, source.EmployeeID)
		if err != nil {
			stats.FetchErrors++
			if cfg.Logger != nil {
				cfg.Logger.Warn("failed to fetch Campus/KFU schedule", zap.Int("employeeID", source.EmployeeID), zap.Error(err))
			}
			continue
		}
		stats.Schedules++
		if err := imp.importSchedule(ctx, source, schedule.Subjects, &stats); err != nil {
			stats.FetchErrors++
			if cfg.Logger != nil {
				cfg.Logger.Warn("failed to import Campus/KFU schedule", zap.Int("employeeID", source.EmployeeID), zap.Error(err))
			}
			continue
		}
	}

	stats.AffectedGroupIDs = imp.affectedGroupIDs()
	return stats, nil
}

func (c Config) withDefaults() Config {
	if strings.TrimSpace(c.CampusAPIBase) == "" {
		c.CampusAPIBase = DefaultCampusAPIBase
	}
	if strings.TrimSpace(c.KFUAPIBase) == "" {
		c.KFUAPIBase = DefaultKFUAPIBase
	}
	if strings.TrimSpace(c.Organization) == "" {
		c.Organization = DefaultOrganization
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	if strings.TrimSpace(c.TermName) == "" {
		c.TermName = DefaultTermName
	}
	return c
}

func buildSources(ctx context.Context, client *http.Client, cfg Config) ([]scheduleSource, error) {
	if strings.TrimSpace(cfg.SourceIDs) != "" {
		var sources []scheduleSource
		for _, raw := range splitList(cfg.SourceIDs) {
			id, err := strconv.Atoi(raw)
			if err != nil || id <= 0 {
				continue
			}
			sources = append(sources, scheduleSource{EmployeeID: id})
		}
		return sources, nil
	}

	if query := strings.TrimSpace(cfg.SourceParam); query != "" {
		employees, err := searchEmployees(ctx, client, cfg.KFUAPIBase, query)
		if err != nil {
			return nil, err
		}
		return employeesToSources(employees), nil
	}

	teachers, err := fetchCampusTeachers(ctx, client, cfg.CampusAPIBase, cfg.Organization)
	if err != nil {
		return nil, err
	}
	if cfg.LimitSources > 0 && len(teachers) > cfg.LimitSources {
		teachers = teachers[:cfg.LimitSources]
	}

	sources := make([]scheduleSource, 0, len(teachers))
	seen := make(map[int]struct{})
	for _, teacher := range teachers {
		name := strings.TrimSpace(teacher.Name)
		if name == "" || strings.HasPrefix(name, "_") {
			continue
		}
		employees, err := searchEmployees(ctx, client, cfg.KFUAPIBase, name)
		if err != nil {
			continue
		}
		for _, source := range employeesToSources(employees) {
			if _, ok := seen[source.EmployeeID]; ok {
				continue
			}
			seen[source.EmployeeID] = struct{}{}
			sources = append(sources, source)
			break
		}
	}
	return sources, nil
}

func employeesToSources(employees []kfuEmployee) []scheduleSource {
	sources := make([]scheduleSource, 0, len(employees))
	for _, employee := range employees {
		if !employee.IsTeacher || employee.EmployeeID <= 0 {
			continue
		}
		sources = append(sources, scheduleSource{
			EmployeeID: employee.EmployeeID,
			Name:       fullName(employee.Lastname, employee.Firstname, employee.Middlename),
			Position:   employee.Post,
		})
	}
	return sources
}

func fetchCampusTeachers(ctx context.Context, client *http.Client, apiBase, organization string) ([]campusTeacher, error) {
	rawURL := fmt.Sprintf("%s/organizations/%s/entities?type=Teacher", strings.TrimRight(apiBase, "/"), url.PathEscape(organization))
	var teachers []campusTeacher
	if err := getJSON(ctx, client, rawURL, &teachers); err != nil {
		return nil, fmt.Errorf("fetch Campus teachers: %w", err)
	}
	return teachers, nil
}

func searchEmployees(ctx context.Context, client *http.Client, apiBase, query string) ([]kfuEmployee, error) {
	rawURL := fmt.Sprintf("%s/employees?q=%s", strings.TrimRight(apiBase, "/"), url.QueryEscape(query))
	var res employeeSearchResponse
	if err := getJSON(ctx, client, rawURL, &res); err != nil {
		return nil, fmt.Errorf("search KFU employees: %w", err)
	}
	if !res.Success {
		return nil, fmt.Errorf("KFU employee search returned success=false")
	}
	return res.Employees, nil
}

func fetchSchedule(ctx context.Context, client *http.Client, apiBase string, employeeID int) (scheduleResponse, error) {
	rawURL := fmt.Sprintf("%s/employees/%d/schedule", strings.TrimRight(apiBase, "/"), employeeID)
	var res scheduleResponse
	if err := getJSON(ctx, client, rawURL, &res); err != nil {
		return res, err
	}
	if !res.Success {
		return res, fmt.Errorf("KFU schedule returned success=false")
	}
	return res, nil
}

func getJSON(ctx context.Context, client *http.Client, rawURL string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "ru")
	req.Header.Set("User-Agent", "Campus/4.15.0 (ru.dewish.campus; build:99; iOS iOS 18.3.1)")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("GET %s: status %s", rawURL, res.Status)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return err
	}
	return nil
}

type importer struct {
	db             *pgxpool.Pool
	filialID       uuid.UUID
	termName       string
	termID         uuid.UUID
	subjects       map[string]uuid.UUID
	rooms          map[string]uuid.UUID
	groups         map[string]uuid.UUID
	teachers       map[string]uuid.UUID
	affectedGroups map[uuid.UUID]struct{}
}

func (i *importer) importSchedule(ctx context.Context, source scheduleSource, subjects []kfuSubject, stats *Stats) error {
	if len(subjects) == 0 {
		return nil
	}
	tx, err := i.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	termID, err := i.ensureTerm(ctx, tx)
	if err != nil {
		return err
	}
	teacherID, err := i.ensureTeacher(ctx, tx, source, subjects[0])
	if err != nil {
		return err
	}
	stats.Teachers++

	for _, subject := range subjects {
		groups := splitGroups(subject.GroupList)
		if len(groups) == 0 {
			continue
		}
		for _, groupName := range groups {
			groupID, err := i.ensureGroup(ctx, tx, groupName)
			if err != nil {
				return err
			}
			if _, seen := i.affectedGroups[groupID]; !seen {
				stats.Groups++
			}
			i.affectedGroups[groupID] = struct{}{}
			subjectID, err := i.ensureSubject(ctx, tx, subject.SubjectName)
			if err != nil {
				return err
			}
			roomID, err := i.ensureRoom(ctx, tx, campusRoomName(subject))
			if err != nil {
				return err
			}
			created, err := i.ensureTimetableEntry(ctx, tx, termID, groupID, subjectID, teacherID, roomID, sourceEventID(subject, groupName), subject)
			if err != nil {
				return err
			}
			if created {
				stats.EntriesCreated++
			} else {
				stats.EntriesExisting++
			}
			stats.Events++
		}
	}
	return tx.Commit(ctx)
}

func (i *importer) ensureTerm(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
	if i.termID != uuid.Nil {
		return i.termID, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM public.academic_term WHERE filial_id = $1 AND name = $2 LIMIT 1`, i.filialID, i.termName).Scan(&id)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `
INSERT INTO public.academic_term (filial_id, name, starts_on, ends_on)
VALUES ($1, $2, now(), now() + interval '6 months')
RETURNING id
`, i.filialID, i.termName).Scan(&id)
	}
	if err != nil {
		return uuid.Nil, err
	}
	i.termID = id
	return id, nil
}

func (i *importer) ensureGroup(ctx context.Context, tx pgx.Tx, name string) (uuid.UUID, error) {
	if id, ok := i.groups[name]; ok {
		return id, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM public.edu_group WHERE filial_id = $1 AND name = $2 LIMIT 1`, i.filialID, name).Scan(&id)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `INSERT INTO public.edu_group (filial_id, name, source) VALUES ($1, $2, 'campus') RETURNING id`, i.filialID, name).Scan(&id)
	}
	if err != nil {
		return uuid.Nil, err
	}
	i.groups[name] = id
	return id, nil
}

func (i *importer) ensureSubject(ctx context.Context, tx pgx.Tx, name string) (uuid.UUID, error) {
	if id, ok := i.subjects[name]; ok {
		return id, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM public.edu_subject WHERE name = $1 LIMIT 1`, name).Scan(&id)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `INSERT INTO public.edu_subject (name) VALUES ($1) RETURNING id`, name).Scan(&id)
	}
	if err != nil {
		return uuid.Nil, err
	}
	i.subjects[name] = id
	return id, nil
}

func (i *importer) ensureRoom(ctx context.Context, tx pgx.Tx, name string) (*uuid.UUID, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	if id, ok := i.rooms[name]; ok {
		return &id, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM public.room WHERE filial_id = $1 AND name = $2 LIMIT 1`, i.filialID, name).Scan(&id)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `INSERT INTO public.room (filial_id, room_type, name) VALUES ($1, 'CLASSROOM', $2) RETURNING id`, i.filialID, name).Scan(&id)
	}
	if err != nil {
		return nil, err
	}
	i.rooms[name] = id
	return &id, nil
}

func (i *importer) ensureTeacher(ctx context.Context, tx pgx.Tx, source scheduleSource, sample kfuSubject) (*uuid.UUID, error) {
	name := strings.TrimSpace(source.Name)
	if name == "" {
		name = fullName(sample.TeacherLastname, sample.TeacherFirstname, sample.TeacherMiddlename)
	}
	if name == "" {
		return nil, nil
	}
	if id, ok := i.teachers[name]; ok {
		return &id, nil
	}
	lastName, firstName, middleName := splitTeacherName(name)
	var personID uuid.UUID
	err := tx.QueryRow(ctx, `
SELECT id FROM public.person
WHERE COALESCE(last_name, '') = $1 AND COALESCE(first_name, '') = $2 AND COALESCE(middle_name, '') = $3
LIMIT 1
`, lastName, firstName, middleName).Scan(&personID)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `
INSERT INTO public.person (last_name, first_name, middle_name)
VALUES ($1, $2, $3)
RETURNING id
`, nullIfEmpty(lastName), nullIfEmpty(firstName), nullIfEmpty(middleName)).Scan(&personID)
	}
	if err != nil {
		return nil, err
	}

	var staffID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM public.staff WHERE person_id = $1 AND filial_id = $2 AND staff_type = 'TEACHER' LIMIT 1`, personID, i.filialID).Scan(&staffID)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `
INSERT INTO public.staff (person_id, filial_id, staff_type, position_title, active)
VALUES ($1, $2, 'TEACHER', $3, true)
RETURNING id
`, personID, i.filialID, nullIfEmpty(source.Position)).Scan(&staffID)
	}
	if err != nil {
		return nil, err
	}
	i.teachers[name] = staffID
	return &staffID, nil
}

func (i *importer) ensureTimetableEntry(ctx context.Context, tx pgx.Tx, termID, groupID, subjectID uuid.UUID, teacherID, roomID *uuid.UUID, sourceEventID int64, subject kfuSubject) (bool, error) {
	var created bool
	startDate, _ := parseAPIDate(subject.StartDaySchedule)
	endDate, _ := parseAPIDate(subject.FinishDaySchedule)
	weekType := campusWeekType(subject.TypeWeekSchedule)
	err := tx.QueryRow(ctx, `
INSERT INTO public.timetable_entry (
	term_id, group_id, subject_id, teacher_id, classroom_id, day_of_week, effective_from, effective_to, starts_at, ends_at, week_type, source, source_event_id, is_exam, lesson_type, comment
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9::time, $10::time, $11, 'campus', $12, false, $13, $14
)
ON CONFLICT ON CONSTRAINT timetable_entry_source_event_unique DO UPDATE SET
	subject_id = EXCLUDED.subject_id,
	teacher_id = EXCLUDED.teacher_id,
	classroom_id = EXCLUDED.classroom_id,
	day_of_week = EXCLUDED.day_of_week,
	effective_from = EXCLUDED.effective_from,
	effective_to = EXCLUDED.effective_to,
	starts_at = EXCLUDED.starts_at,
	ends_at = EXCLUDED.ends_at,
	week_type = EXCLUDED.week_type,
	lesson_type = EXCLUDED.lesson_type,
	comment = EXCLUDED.comment,
	updated_at = now()
RETURNING xmax = 0
`, termID, groupID, subjectID, teacherID, roomID, subject.DayWeekSchedule, nullIfZeroTime(startDate), nullIfZeroTime(endDate), subject.BeginTimeSchedule, subject.EndTimeSchedule, weekType, sourceEventID, nullIfEmpty(subject.SubjectKindName), nullIfEmpty(subject.NoteSchedule)).Scan(&created)
	return created, err
}

func parseAPIDate(value string) (time.Time, error) {
	return time.Parse("02.01.06", strings.TrimSpace(value))
}

func campusWeekType(value int) string {
	switch value {
	case 1:
		return "ODD"
	case 2:
		return "EVEN"
	default:
		return "ALL"
	}
}

func sourceEventID(subject kfuSubject, groupName string) int64 {
	key := fmt.Sprintf("campus|%s|%s|%d|%s|%s|%s", subject.ID, groupName, subject.TeacherID, subject.StartDaySchedule, subject.BeginTimeSchedule, subject.EndTimeSchedule)
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return int64(h.Sum64() & 0x7fffffffffffffff)
}

func splitGroups(value string) []string {
	value = strings.NewReplacer("\n", ",", ";", ",").Replace(value)
	return splitList(value)
}

func splitList(value string) []string {
	var parts []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func fullName(lastName, firstName, middleName string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(strings.TrimSpace(lastName)+" "+strings.TrimSpace(firstName)+" "+strings.TrimSpace(middleName)), " "))
}

func campusRoomName(subject kfuSubject) string {
	room := strings.TrimSpace(subject.NumAuditoriumSchedule)
	building := strings.TrimSpace(subject.BuildingName)
	if room == "" {
		return building
	}
	if building == "" {
		return room
	}
	return building + ", " + room
}

func splitTeacherName(fullName string) (string, string, string) {
	fields := strings.Fields(fullName)
	if len(fields) == 0 {
		return "", "", ""
	}
	lastName := fields[0]
	firstName := ""
	middleName := ""
	if len(fields) > 1 {
		firstName = fields[1]
	}
	if len(fields) > 2 {
		middleName = strings.Join(fields[2:], " ")
	}
	return lastName, firstName, middleName
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullIfZeroTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func (i *importer) affectedGroupIDs() []string {
	ids := make([]string, 0, len(i.affectedGroups))
	for id := range i.affectedGroups {
		ids = append(ids, id.String())
	}
	return ids
}

func ensureFilial(ctx context.Context, db *pgxpool.Pool, filialID uuid.UUID) error {
	_, err := db.Exec(ctx, `INSERT INTO public.filial (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, filialID)
	return err
}
