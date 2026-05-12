package suruz

import (
	"context"
	"encoding/json"
	"fmt"
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
	DefaultAPIBase  = "http://api.apps.inforino.ru/company/55193/suruz"
	DefaultFilialID = "4888f1e4-5916-45ef-a485-0d0381872968"
	DefaultDSN      = "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"
	DefaultTermName = "Suruz MGIMO test"
)

type Config struct {
	APIBase      string
	DSN          string
	FilialID     string
	SourceIDs    string
	SourceParam  string
	LimitSources int
	Timeout      time.Duration
	TermName     string
	Logger       *zap.Logger
}

type Stats struct {
	Groups          int
	Teachers        int
	Schedules       int
	FetchErrors     int
	Events          int
	EntriesCreated  int
	EntriesExisting int
}

func Sync(ctx context.Context, cfg Config) (Stats, error) {
	cfg = cfg.withDefaults()

	filialID, err := uuid.Parse(cfg.FilialID)
	if err != nil {
		return Stats{}, fmt.Errorf("parse filial id: %w", err)
	}

	client := &http.Client{Timeout: cfg.Timeout}
	meta, err := fetchGroups(ctx, client, cfg.APIBase)
	if err != nil {
		return Stats{}, fmt.Errorf("fetch groups: %w", err)
	}

	sources := buildSources(meta, cfg.SourceIDs, cfg.SourceParam)
	if cfg.LimitSources > 0 && len(sources) > cfg.LimitSources {
		sources = sources[:cfg.LimitSources]
	}
	if len(sources) == 0 {
		return Stats{}, fmt.Errorf("build schedule sources: no schedule sources found in Suruz groups response")
	}

	db, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return Stats{}, fmt.Errorf("connect postgres: %w", err)
	}
	defer db.Close()

	var stats Stats
	imp := &importer{
		db:       db,
		filialID: filialID,
		termName: cfg.TermName,
		subjects: make(map[string]uuid.UUID),
		rooms:    make(map[string]uuid.UUID),
		groups:   make(map[string]uuid.UUID),
		teachers: make(map[string]uuid.UUID),
	}

	// 1. Import Metadata (Groups & Teachers) first so they are visible immediately
	teacherByID, groupByID, subgroupToGroupID, err := imp.importMetadata(ctx, meta, &stats)
	if err != nil {
		return stats, fmt.Errorf("import metadata: %w", err)
	}

	// 2. Fetch and Import schedules one by one
	total := len(sources)
	for idx, source := range sources {
		if cfg.Logger != nil && (idx == 0 || (idx+1)%100 == 0 || idx+1 == total) {
			cfg.Logger.Info("fetching and importing schedule", zap.Int("current", idx+1), zap.Int("total", total), zap.String("source", source.Name))
		}

		schedule, err := fetchSchedule(ctx, client, cfg.APIBase, source)
		if err != nil {
			stats.FetchErrors++
			continue
		}
		stats.Schedules++

		// Import this single schedule in its own transaction
		if err := imp.importSingleSchedule(ctx, source, schedule, &stats, groupByID, teacherByID, subgroupToGroupID); err != nil {
			if cfg.Logger != nil {
				cfg.Logger.Warn("failed to import single schedule", zap.String("source", source.Name), zap.Error(err))
			}
			continue
		}
	}

	return stats, nil
}

func (c Config) withDefaults() Config {
	if strings.TrimSpace(c.APIBase) == "" {
		c.APIBase = DefaultAPIBase
	}
	if strings.TrimSpace(c.DSN) == "" {
		c.DSN = DefaultDSN
	}
	if strings.TrimSpace(c.FilialID) == "" {
		c.FilialID = DefaultFilialID
	}
	if strings.TrimSpace(c.SourceParam) == "" {
		c.SourceParam = "subgroup_id"
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	if strings.TrimSpace(c.TermName) == "" {
		c.TermName = DefaultTermName
	}
	return c
}

type apiResponse[T any] struct {
	Status  string `json:"status"`
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Data    T      `json:"data"`
}

type groupsData struct {
	Groups    []suruzGroup    `json:"groups"`
	Subgroups []suruzSubgroup `json:"subgroups"`
	Teachers  []suruzTeacher  `json:"teachers"`
}

type suruzGroup struct {
	ID        int             `json:"id"`
	Title     string          `json:"title"`
	Name      string          `json:"name"`
	Subgroups []suruzSubgroup `json:"subgroups"`
}

func (g suruzGroup) displayName() string {
	if strings.TrimSpace(g.Title) != "" {
		return strings.TrimSpace(g.Title)
	}
	return strings.TrimSpace(g.Name)
}

type suruzSubgroup struct {
	ID      int    `json:"id"`
	GroupID int    `json:"group_id"`
	Title   string `json:"title"`
	Name    string `json:"name"`
}

func (s suruzSubgroup) displayName() string {
	if strings.TrimSpace(s.Title) != "" {
		return strings.TrimSpace(s.Title)
	}
	return strings.TrimSpace(s.Name)
}

type suruzTeacher struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Position string `json:"position"`
}

type scheduleData struct {
	Events         []suruzEvent         `json:"events"`
	EventGroups    []suruzEventGroup    `json:"event_groups"`
	EventSubgroups []suruzEventSubgroup `json:"event_subgroups"`
	EventTeachers  []suruzEventTeacher  `json:"event_teachers"`
}

type fetchedSchedule struct {
	Source scheduleSource
	Data   scheduleData
}

type suruzEvent struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	StartDateISO string `json:"start_date_iso"`
	EndDateISO   string `json:"end_date_iso"`
	Lesson       string `json:"lesson"`
	LessonEnd    string `json:"lesson_end"`
	Recurrence   string `json:"recurrence"`
	Room         string `json:"room"`
	Weekday      string `json:"weekday"`
	CustomTime   string `json:"custom_time"`
	TeacherName  string `json:"teacher_name"`
	Comment      string `json:"comment"`
	Place        string `json:"place"`
	Exam         bool   `json:"exam"`
}

type suruzEventGroup struct {
	EventID int64 `json:"event_id"`
	GroupID int   `json:"group_id"`
}

type suruzEventSubgroup struct {
	EventID    int64 `json:"event_id"`
	SubgroupID int   `json:"subgroup_id"`
}

type suruzEventTeacher struct {
	EventID   int64 `json:"event_id"`
	TeacherID int   `json:"teacher_id"`
}

type scheduleSource struct {
	Param string
	ID    int
	Name  string
}

type importer struct {
	db       *pgxpool.Pool
	filialID uuid.UUID
	termName string

	subjects map[string]uuid.UUID
	rooms    map[string]uuid.UUID
	groups   map[string]uuid.UUID
	teachers map[string]uuid.UUID
}

func fetchGroups(ctx context.Context, client *http.Client, apiBase string) (groupsData, error) {
	var response apiResponse[groupsData]
	err := getJSON(ctx, client, strings.TrimRight(apiBase, "/")+"/v3/groups", &response)
	return response.Data, err
}

func fetchSchedule(ctx context.Context, client *http.Client, apiBase string, source scheduleSource) (scheduleData, error) {
	endpoint, err := url.Parse(strings.TrimRight(apiBase, "/") + "/v2/schedule")
	if err != nil {
		return scheduleData{}, err
	}
	q := endpoint.Query()
	q.Set(source.Param, strconv.Itoa(source.ID))
	endpoint.RawQuery = q.Encode()

	var response apiResponse[scheduleData]
	if err := getJSON(ctx, client, endpoint.String(), &response); err != nil {
		return scheduleData{}, err
	}
	return response.Data, nil
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("GET %s: status %s", endpoint, res.Status)
	}
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", endpoint, err)
	}
	return nil
}

func buildSources(meta groupsData, explicitIDs, sourceParam string) []scheduleSource {
	if strings.TrimSpace(explicitIDs) != "" {
		var sources []scheduleSource
		for _, raw := range strings.Split(explicitIDs, ",") {
			id, err := strconv.Atoi(strings.TrimSpace(raw))
			if err == nil && id > 0 {
				sources = append(sources, scheduleSource{Param: sourceParam, ID: id})
			}
		}
		return sources
	}

	seen := make(map[int]struct{})
	var sources []scheduleSource
	addSubgroup := func(subgroup suruzSubgroup) {
		if subgroup.ID == 0 {
			return
		}
		if _, ok := seen[subgroup.ID]; ok {
			return
		}
		seen[subgroup.ID] = struct{}{}
		sources = append(sources, scheduleSource{Param: "subgroup_id", ID: subgroup.ID, Name: subgroup.displayName()})
	}

	for _, subgroup := range meta.Subgroups {
		addSubgroup(subgroup)
	}
	for _, group := range meta.Groups {
		for _, subgroup := range group.Subgroups {
			addSubgroup(subgroup)
		}
	}
	if len(sources) > 0 {
		return sources
	}

	for _, group := range meta.Groups {
		if group.ID > 0 {
			sources = append(sources, scheduleSource{Param: "group_id", ID: group.ID, Name: group.displayName()})
		}
	}
	return sources
}

func (i *importer) importMetadata(ctx context.Context, meta groupsData, stats *Stats) (map[int]string, map[int]string, map[int]int, error) {
	tx, err := i.db.Begin(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `INSERT INTO public.filial (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, i.filialID); err != nil {
		return nil, nil, nil, err
	}

	for _, group := range meta.Groups {
		name := group.displayName()
		if name == "" || isExamGroupName(name) {
			continue
		}
		if _, err := i.ensureGroup(ctx, tx, name); err != nil {
			return nil, nil, nil, err
		}
		stats.Groups++
	}

	teacherByID := make(map[int]string, len(meta.Teachers))
	for _, teacher := range meta.Teachers {
		name := strings.TrimSpace(teacher.Name)
		if name == "" {
			continue
		}
		teacherByID[teacher.ID] = name
		if _, err := i.ensureTeacher(ctx, tx, name, teacher.Position); err != nil {
			return nil, nil, nil, err
		}
		stats.Teachers++
	}

	groupByID := make(map[int]string, len(meta.Groups))
	for _, group := range meta.Groups {
		groupByID[group.ID] = group.displayName()
	}

	subgroupToGroupID := make(map[int]int)
	for _, subgroup := range meta.Subgroups {
		if subgroup.ID > 0 && subgroup.GroupID > 0 {
			subgroupToGroupID[subgroup.ID] = subgroup.GroupID
		}
	}
	for _, group := range meta.Groups {
		for _, subgroup := range group.Subgroups {
			if subgroup.ID > 0 {
				if subgroup.GroupID > 0 {
					subgroupToGroupID[subgroup.ID] = subgroup.GroupID
				} else {
					subgroupToGroupID[subgroup.ID] = group.ID
				}
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, nil, err
	}

	return teacherByID, groupByID, subgroupToGroupID, nil
}

func (i *importer) importSingleSchedule(ctx context.Context, source scheduleSource, data scheduleData, stats *Stats, groupByID map[int]string, teacherByID map[int]string, subgroupToGroupID map[int]int) error {
	tx, err := i.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	minDate, maxDate := singleScheduleDateRange(data)
	termID, err := i.ensureTerm(ctx, tx, minDate, maxDate)
	if err != nil {
		return err
	}

	eventGroups := make(map[int64][]int)
	for _, link := range data.EventGroups {
		eventGroups[link.EventID] = append(eventGroups[link.EventID], link.GroupID)
	}
	eventSubgroups := make(map[int64][]int)
	for _, link := range data.EventSubgroups {
		eventSubgroups[link.EventID] = append(eventSubgroups[link.EventID], link.SubgroupID)
	}
	eventTeachers := make(map[int64][]int)
	for _, link := range data.EventTeachers {
		eventTeachers[link.EventID] = append(eventTeachers[link.EventID], link.TeacherID)
	}

	for _, event := range data.Events {
		groups := eventGroups[event.ID]
		for _, subgroupID := range eventSubgroups[event.ID] {
			if groupID := subgroupToGroupID[subgroupID]; groupID > 0 {
				groups = append(groups, groupID)
			}
		}
		if len(groups) == 0 {
			if source.Param == "group_id" {
				groups = []int{source.ID}
			} else if groupID := subgroupToGroupID[source.ID]; groupID > 0 {
				groups = []int{groupID}
			}
		}
		groups = uniqueInts(groups)

		created, existing, err := i.importEvent(ctx, tx, termID, event, groups, eventTeachers[event.ID], groupByID, teacherByID)
		if err != nil {
			return err
		}
		stats.Events++
		stats.EntriesCreated += created
		stats.EntriesExisting += existing
	}

	return tx.Commit(ctx)
}

func (i *importer) importEvent(ctx context.Context, tx pgx.Tx, termID uuid.UUID, event suruzEvent, groupIDs []int, teacherIDs []int, groupByID map[int]string, teacherByID map[int]string) (int, int, error) {
	subjectName := strings.TrimSpace(event.Title)
	if subjectName == "" {
		return 0, 0, nil
	}

	startDate, err := parseAPIDate(event.StartDateISO)
	if err != nil {
		return 0, 0, nil
	}
	startsAt, endsAt, err := parseLessonTime(event.CustomTime, event.Lesson, event.LessonEnd)
	if err != nil {
		return 0, 0, nil
	}
	weekType := eventWeekType(event, startDate)
	var occursOn *time.Time
	if isExactEvent(event) {
		day := dateOnly(startDate)
		occursOn = &day
		weekType = "ONCE"
	}

	subjectID, err := i.ensureSubject(ctx, tx, subjectName)
	if err != nil {
		return 0, 0, err
	}

	var roomID *uuid.UUID
	if roomName := strings.TrimSpace(event.Room); roomName != "" {
		id, err := i.ensureRoom(ctx, tx, roomName)
		if err != nil {
			return 0, 0, err
		}
		roomID = &id
	}

	var teacherID *uuid.UUID
	if len(teacherIDs) > 0 {
		if name := teacherByID[teacherIDs[0]]; name != "" {
			id, err := i.ensureTeacher(ctx, tx, name, "")
			if err != nil {
				return 0, 0, err
			}
			teacherID = &id
		}
	} else if strings.TrimSpace(event.TeacherName) != "" {
		id, err := i.ensureTeacher(ctx, tx, strings.TrimSpace(event.TeacherName), "")
		if err != nil {
			return 0, 0, err
		}
		teacherID = &id
	}

	created := 0
	existing := 0
	for _, apiGroupID := range groupIDs {
		groupName := groupByID[apiGroupID]
		if groupName == "" {
			continue
		}

		// Try to extract real group name from exam group name
		if isExamGroupName(groupName) {
			realName := extractRealGroupName(groupName)
			if realName != "" {
				groupName = realName
			}
		}

		groupID, err := i.ensureGroup(ctx, tx, groupName)
		if err != nil {
			return 0, 0, err
		}
		wasCreated, err := i.ensureTimetableEntry(ctx, tx, termID, groupID, subjectID, teacherID, roomID, event.ID, postgresDayOfWeek(startDate), occursOn, startsAt, endsAt, weekType, event.Exam, eventComment(event))
		if err != nil {
			return 0, 0, err
		}
		if wasCreated {
			created++
		} else {
			existing++
		}
	}

	return created, existing, nil
}

func isExamGroupName(name string) bool {
	return strings.Contains(name, "(Зач.)") || strings.Contains(name, "(Экз.)")
}

func extractRealGroupName(name string) string {
	// Pattern: "Subject(Exam)-Semester (GroupName)" -> GroupName
	lastOpen := strings.LastIndex(name, "(")
	lastClose := strings.LastIndex(name, ")")
	if lastOpen != -1 && lastClose != -1 && lastClose > lastOpen+1 {
		candidate := name[lastOpen+1 : lastClose]
		// If candidate is just a number or contains special chars, it might not be a real group
		// But in Suruz it's often the group name
		if !isExamGroupName(candidate) {
			return candidate
		}
	}
	return ""
}

func (i *importer) ensureTerm(ctx context.Context, tx pgx.Tx, startsOn, endsOn time.Time) (uuid.UUID, error) {
	if startsOn.IsZero() {
		now := time.Now()
		startsOn = now.AddDate(0, -1, 0)
		endsOn = now.AddDate(0, 6, 0)
	}
	if endsOn.Before(startsOn) {
		endsOn = startsOn.AddDate(0, 6, 0)
	}

	var id uuid.UUID
	err := tx.QueryRow(ctx, `
SELECT id FROM public.academic_term WHERE filial_id = $1 AND name = $2 LIMIT 1
`, i.filialID, i.termName).Scan(&id)
	if err == nil {
		_, err = tx.Exec(ctx, `
UPDATE public.academic_term
SET starts_on = LEAST(starts_on, $2), ends_on = GREATEST(ends_on, $3), updated_at = now()
WHERE id = $1
`, id, startsOn, endsOn)
		return id, err
	}
	if err != pgx.ErrNoRows {
		return uuid.Nil, err
	}

	err = tx.QueryRow(ctx, `
INSERT INTO public.academic_term (filial_id, name, starts_on, ends_on, week_start)
VALUES ($1, $2, $3, $4, 1)
RETURNING id
`, i.filialID, i.termName, startsOn, endsOn).Scan(&id)
	return id, err
}

func (i *importer) ensureGroup(ctx context.Context, tx pgx.Tx, name string) (uuid.UUID, error) {
	if id, ok := i.groups[name]; ok {
		return id, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM public.edu_group WHERE filial_id = $1 AND name = $2 LIMIT 1`, i.filialID, name).Scan(&id)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `INSERT INTO public.edu_group (filial_id, name) VALUES ($1, $2) RETURNING id`, i.filialID, name).Scan(&id)
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

func (i *importer) ensureRoom(ctx context.Context, tx pgx.Tx, name string) (uuid.UUID, error) {
	if id, ok := i.rooms[name]; ok {
		return id, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
SELECT id FROM public.room WHERE filial_id = $1 AND COALESCE(room_number, name) = $2 LIMIT 1
`, i.filialID, name).Scan(&id)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `
INSERT INTO public.room (filial_id, room_type, name, room_number)
VALUES ($1, 'CLASSROOM', $2, $2)
RETURNING id
`, i.filialID, name).Scan(&id)
	}
	if err != nil {
		return uuid.Nil, err
	}
	i.rooms[name] = id
	return id, nil
}

func (i *importer) ensureTeacher(ctx context.Context, tx pgx.Tx, fullName, position string) (uuid.UUID, error) {
	fullName = strings.TrimSpace(fullName)
	if id, ok := i.teachers[fullName]; ok {
		return id, nil
	}

	lastName, firstName, middleName := splitTeacherName(fullName)
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
		return uuid.Nil, err
	}

	var staffID uuid.UUID
	err = tx.QueryRow(ctx, `
SELECT id FROM public.staff WHERE person_id = $1 AND filial_id = $2 AND staff_type = 'TEACHER' LIMIT 1
`, personID, i.filialID).Scan(&staffID)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `
INSERT INTO public.staff (person_id, filial_id, staff_type, position_title, active)
VALUES ($1, $2, 'TEACHER', $3, true)
RETURNING id
`, personID, i.filialID, nullIfEmpty(position)).Scan(&staffID)
	}
	if err != nil {
		return uuid.Nil, err
	}

	i.teachers[fullName] = staffID
	return staffID, nil
}

func (i *importer) ensureTimetableEntry(ctx context.Context, tx pgx.Tx, termID, groupID, subjectID uuid.UUID, teacherID, roomID *uuid.UUID, sourceEventID int64, dayOfWeek int, occursOn *time.Time, startsAt, endsAt, weekType string, isExam bool, comment string) (bool, error) {
	var created bool
	err := tx.QueryRow(ctx, `
INSERT INTO public.timetable_entry (
	term_id, group_id, subject_id, teacher_id, classroom_id, day_of_week, occurs_on, starts_at, ends_at, week_type, source, source_event_id, is_exam, comment
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8::time, $9::time, $10, 'suruz', $11, $12, $13
)
ON CONFLICT ON CONSTRAINT timetable_entry_source_event_unique DO UPDATE SET
	subject_id = EXCLUDED.subject_id,
	teacher_id = EXCLUDED.teacher_id,
	classroom_id = EXCLUDED.classroom_id,
	day_of_week = EXCLUDED.day_of_week,
	occurs_on = EXCLUDED.occurs_on,
	starts_at = EXCLUDED.starts_at,
	ends_at = EXCLUDED.ends_at,
	week_type = EXCLUDED.week_type,
	is_exam = EXCLUDED.is_exam,
	comment = EXCLUDED.comment,
	updated_at = now()
RETURNING xmax = 0
`, termID, groupID, subjectID, teacherID, roomID, dayOfWeek, occursOn, startsAt, endsAt, weekType, sourceEventID, isExam, nullIfEmpty(comment)).Scan(&created)
	return created, err
}

func singleScheduleDateRange(data scheduleData) (time.Time, time.Time) {
	var minDate time.Time
	var maxDate time.Time
	for _, event := range data.Events {
		date, err := parseAPIDate(event.StartDateISO)
		if err != nil {
			continue
		}
		if minDate.IsZero() || date.Before(minDate) {
			minDate = date
		}
		if maxDate.IsZero() || date.After(maxDate) {
			maxDate = date
		}
	}
	return minDate, maxDate
}

func parseAPIDate(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	date, err := time.Parse(time.RFC3339, raw)
	if err == nil {
		return date, nil
	}
	return time.Parse("2006-01-02", raw)
}

func parseLessonTime(customTime, lesson, lessonEnd string) (string, string, error) {
	parts := strings.Fields(strings.ReplaceAll(customTime, "\\n", "\n"))
	if len(parts) >= 2 && isClock(parts[0]) && isClock(parts[1]) {
		return normalizeClock(parts[0]), normalizeClock(parts[1]), nil
	}

	start, ok := lessonStart(strings.TrimSpace(lesson))
	if !ok {
		return "", "", fmt.Errorf("no lesson time")
	}
	endLesson := strings.TrimSpace(lessonEnd)
	if endLesson == "" {
		endLesson = strings.TrimSpace(lesson)
	}
	end, ok := lessonEndTime(endLesson)
	if !ok {
		return "", "", fmt.Errorf("no lesson end time")
	}
	return start, end, nil
}

func lessonStart(lesson string) (string, bool) {
	switch lesson {
	case "1":
		return "09:00", true
	case "2":
		return "10:35", true
	case "3":
		return "12:10", true
	case "4":
		return "14:30", true
	case "5":
		return "16:05", true
	case "6":
		return "17:40", true
	case "7":
		return "19:10", true
	case "8":
		return "20:40", true
	default:
		return "", false
	}
}

func lessonEndTime(lesson string) (string, bool) {
	switch lesson {
	case "1":
		return "10:20", true
	case "2":
		return "11:55", true
	case "3":
		return "13:30", true
	case "4":
		return "15:50", true
	case "5":
		return "17:25", true
	case "6":
		return "19:00", true
	case "7":
		return "20:30", true
	case "8":
		return "22:00", true
	default:
		return "", false
	}
}

func postgresDayOfWeek(date time.Time) int {
	if date.Weekday() == time.Sunday {
		return 7
	}
	return int(date.Weekday())
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func isExactEvent(event suruzEvent) bool {
	recurrence := strings.ToUpper(strings.TrimSpace(event.Recurrence))
	return event.Exam || recurrence == "ONCE" || recurrence == "NONE" || recurrence == "SINGLE"
}

func eventWeekType(event suruzEvent, startDate time.Time) string {
	recurrence := strings.ToUpper(strings.TrimSpace(event.Recurrence))
	switch {
	case recurrence == "", recurrence == "ALL", recurrence == "EVERY_WEEK", recurrence == "WEEKLY":
		return "ALL"
	case strings.Contains(recurrence, "ODD"):
		return "ODD"
	case strings.Contains(recurrence, "EVEN"):
		return "EVEN"
	case strings.Contains(recurrence, "TWO") || strings.Contains(recurrence, "2"):
		_, week := startDate.ISOWeek()
		if week%2 == 1 {
			return "ODD"
		}
		return "EVEN"
	default:
		return "ALL"
	}
}

func eventComment(event suruzEvent) string {
	parts := make([]string, 0, 2)
	if comment := strings.TrimSpace(event.Comment); comment != "" {
		parts = append(parts, comment)
	}
	if place := strings.TrimSpace(event.Place); place != "" {
		parts = append(parts, place)
	}
	return strings.Join(parts, "\n")
}

func uniqueInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func isClock(raw string) bool {
	_, err := time.Parse("15:04", normalizeClock(raw))
	return err == nil
}

func normalizeClock(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) == len("15:04:05") {
		return raw[:5]
	}
	return raw
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
		initials := strings.Split(strings.ReplaceAll(fields[1], ".", ". "), " ")
		if len(initials) > 0 {
			firstName = strings.TrimSpace(initials[0])
		}
		if len(initials) > 1 {
			middleName = strings.TrimSpace(initials[1])
		}
	}
	if len(fields) > 2 {
		firstName = fields[1]
		middleName = strings.Join(fields[2:], " ")
	}
	return strings.TrimSuffix(lastName, "."), strings.TrimSuffix(firstName, "."), strings.TrimSuffix(middleName, ".")
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
