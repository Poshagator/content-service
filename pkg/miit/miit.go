package miit

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	DefaultAPIBase  = "https://www.miit.ru"
	DefaultFilialID = "92ad36d7-84c0-41fb-a9d6-4f1ceb56adb0" // Placeholder
	DefaultTermName = "Расписание МИИТ"
)

type Config struct {
	APIBase   string
	DSN       string
	FilialID  string
	GroupName string
	Institute string
	Timeout   time.Duration
	TermName  string
	Logger    *zap.Logger
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

	var groups []miitGroup
	if cfg.GroupName != "" {
		// Single group sync
		timetableLink, err := findTimetableLink(ctx, client, cfg.APIBase, cfg.GroupName, cfg.Institute)
		if err != nil {
			return Stats{}, fmt.Errorf("find timetable link: %w", err)
		}
		groups = append(groups, miitGroup{
			Name:          cfg.GroupName,
			TimetableLink: timetableLink,
			InstituteName: cfg.Institute,
		})
	} else {
		// Full sync - fetch catalog
		catalog, err := FetchGroups(ctx, client, cfg.APIBase)
		if err != nil {
			return Stats{}, fmt.Errorf("fetch catalog: %w", err)
		}
		groups = catalog
	}

	if len(groups) == 0 {
		return Stats{}, fmt.Errorf("no groups found to sync")
	}

	db, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return Stats{}, fmt.Errorf("connect postgres: %w", err)
	}
	defer db.Close()
	if err := ensureFilialExistsOrCreate(ctx, db, filialID); err != nil {
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

	total := len(groups)
	startedAt := time.Now()
	for idx, group := range groups {
		if cfg.Logger != nil && (idx == 0 || (idx+1)%10 == 0 || idx+1 == total) {
			cfg.Logger.Info("fetching and importing MIIT schedule",
				zap.Int("current", idx+1),
				zap.Int("total", total),
				zap.String("group", group.Name),
				zap.Duration("elapsed", time.Since(startedAt)),
			)
		}

		// Sync regular schedule (Odd/Even weeks)
		err := imp.syncRegularSchedule(ctx, client, cfg.APIBase, group, &stats)
		if err != nil {
			stats.FetchErrors++
			if cfg.Logger != nil {
				cfg.Logger.Warn("failed to sync MIIT regular schedule", zap.String("group", group.Name), zap.Error(err))
			}
		} else {
			stats.Schedules++
		}

		// Sync session schedule (Exams)
		err = imp.syncSessionSchedule(ctx, client, cfg.APIBase, group, &stats)
		if err != nil {
			if cfg.Logger != nil {
				cfg.Logger.Warn("failed to sync MIIT session schedule", zap.String("group", group.Name), zap.Error(err))
			}
		}
		if cfg.Logger != nil && (idx == 0 || (idx+1)%25 == 0 || idx+1 == total) {
			cfg.Logger.Info("MIIT sync progress",
				zap.Int("current", idx+1),
				zap.Int("total", total),
				zap.Int("events", stats.Events),
				zap.Int("fetchErrors", stats.FetchErrors),
				zap.Int("affectedGroups", len(imp.affectedGroups)),
				zap.Duration("elapsed", time.Since(startedAt)),
			)
		}
	}

	stats.AffectedGroupIDs = imp.affectedGroupIDs()
	return stats, nil
}

type miitGroup struct {
	Name          string
	TimetableLink string
	InstituteName string
	InstituteID   string
	Course        int
}

func FetchGroups(ctx context.Context, client *http.Client, apiBase string) ([]miitGroup, error) {
	res, err := client.Get(strings.TrimRight(apiBase, "/") + "/timetable")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	var groups []miitGroup
	doc.Find("div.info-block[id]").Each(func(i int, s *goquery.Selection) {
		instituteID, _ := s.Attr("id")
		instituteName := cleanText(s.Find("span.info-block__header-text").Text())

		s.Find("li.text-form__item").Each(func(j int, li *goquery.Selection) {
			courseStr := li.Find("span.text-form__item-name").Text()
			course := parseCourse(courseStr)

			li.Find("div.timetable-url").Each(func(k int, div *goquery.Selection) {
				// Direct links
				div.Find("a:not(.dropdown-toggle)").Each(func(l int, a *goquery.Selection) {
					href, _ := a.Attr("href")
					name := cleanText(a.Text())
					if href != "" && name != "" {
						groups = append(groups, miitGroup{
							Name:          name,
							TimetableLink: href,
							InstituteName: instituteName,
							InstituteID:   instituteID,
							Course:        course,
						})
					}
				})

				// Dropdowns
				div.Find("div.dropdown-menu a.dropdown-item").Each(func(l int, a *goquery.Selection) {
					href, _ := a.Attr("href")
					name := cleanText(a.Text())
					if href != "" && name != "" {
						groups = append(groups, miitGroup{
							Name:          name,
							TimetableLink: href,
							InstituteName: instituteName,
							InstituteID:   instituteID,
							Course:        course,
						})
					}
				})
			})
		})
	})

	return groups, nil
}

func parseCourse(s string) int {
	s = strings.ToLower(s)
	if strings.Contains(s, "1") {
		return 1
	}
	if strings.Contains(s, "2") {
		return 2
	}
	if strings.Contains(s, "3") {
		return 3
	}
	if strings.Contains(s, "4") {
		return 4
	}
	if strings.Contains(s, "5") {
		return 5
	}
	if strings.Contains(s, "6") {
		return 6
	}
	return 0
}

func (i *importer) syncRegularSchedule(ctx context.Context, client *http.Client, apiBase string, group miitGroup, stats *Stats) error {
	res, err := client.Get(apiBase + group.TimetableLink + "?type=1")
	if err != nil {
		return err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return err
	}

	for week := 1; week <= 2; week++ {
		for dayIdx := 3; dayIdx <= 15; dayIdx += 2 {
			dayOfWeek := (dayIdx - 1) / 2
			for lessonIdx := 2; lessonIdx <= 16; lessonIdx += 2 {
				lessonNum := lessonIdx / 2
				lessonName := getLesson(doc, week, lessonIdx, dayIdx)
				if lessonName == "" {
					continue
				}
				teacherName := getTeacher(doc, week, lessonIdx, dayIdx)
				roomName := getRoom(doc, week, lessonIdx, dayIdx)

				tx, err := i.db.Begin(ctx)
				if err != nil {
					return err
				}
				err = i.importMiitEvent(ctx, tx, group.Name, lessonName, teacherName, roomName, week, dayOfWeek, lessonNum, nil, false, "", stats)
				if err != nil {
					tx.Rollback(ctx)
					continue
				}
				if err := tx.Commit(ctx); err != nil {
					return err
				}
				stats.Events++
			}
		}
	}
	return nil
}

func (i *importer) syncSessionSchedule(ctx context.Context, client *http.Client, apiBase string, group miitGroup, stats *Stats) error {
	res, err := client.Get(apiBase + group.TimetableLink + "?type=2")
	if err != nil {
		return err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return err
	}

	doc.Find("div.info-block[id]").Each(func(idx int, s *goquery.Selection) {
		dateStr := cleanText(s.Find("span.info-block__header-text").Text())
		date, err := parseSessionDate(dateStr)
		if err != nil {
			return
		}

		s.Find("div.info-block__content .row").Each(func(j int, row *goquery.Selection) {
			timeStr := cleanText(row.Find(".col-md-2").First().Text())
			typeStr := cleanText(row.Find(".col-md-2").Eq(1).Text())
			subjectName := cleanText(row.Find(".col-md-4").Text())
			teacherRoom := cleanText(row.Find(".col-md-3").Text())

			if subjectName == "" {
				return
			}

			teacherName, roomName := splitTeacherRoom(teacherRoom)

			tx, err := i.db.Begin(ctx)
			if err != nil {
				return
			}

			dayOfWeek := int(date.Weekday())
			if dayOfWeek == 0 {
				dayOfWeek = 7
			}
			err = i.importMiitEvent(ctx, tx, group.Name, subjectName, teacherName, roomName, 0, dayOfWeek, 0, &date, true, typeStr, stats, timeStr)
			if err != nil {
				tx.Rollback(ctx)
				return
			}
			if err := tx.Commit(ctx); err != nil {
				return
			}
			stats.Events++
		})
	})

	return nil
}

func parseSessionDate(s string) (time.Time, error) {
	// Example: "15 мая 2026, Пятница"
	parts := strings.Split(s, ",")
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("invalid date format")
	}
	datePart := strings.TrimSpace(parts[0])

	months := map[string]string{
		"января": "01", "февраля": "02", "марта": "03", "апреля": "04",
		"мая": "05", "июня": "06", "июля": "07", "августа": "08",
		"сентября": "09", "октября": "10", "ноября": "11", "декабря": "12",
	}

	fields := strings.Fields(datePart)
	if len(fields) < 3 {
		return time.Time{}, fmt.Errorf("invalid date parts")
	}

	day := fields[0]
	if len(day) == 1 {
		day = "0" + day
	}
	month := months[strings.ToLower(fields[1])]
	year := fields[2]

	return time.Parse("2006-01-02", fmt.Sprintf("%s-%s-%s", year, month, day))
}

func splitTeacherRoom(s string) (string, string) {
	// Example: "Иванов И.И. (ауд. 123)"
	if idx := strings.Index(s, "("); idx != -1 {
		teacher := strings.TrimSpace(s[:idx])
		room := strings.TrimSpace(s[idx:])
		room = strings.TrimPrefix(room, "(")
		room = strings.TrimSuffix(room, ")")
		room = strings.TrimPrefix(room, "ауд.")
		return teacher, strings.TrimSpace(room)
	}
	return s, ""
}

func (c Config) withDefaults() Config {
	if strings.TrimSpace(c.APIBase) == "" {
		c.APIBase = DefaultAPIBase
	}
	if strings.TrimSpace(c.FilialID) == "" {
		c.FilialID = DefaultFilialID
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	if strings.TrimSpace(c.TermName) == "" {
		c.TermName = DefaultTermName
	}
	return c
}

func findTimetableLink(ctx context.Context, client *http.Client, apiBase, groupName, institute string) (string, error) {
	searchURL := fmt.Sprintf("%s/timetable?query=%s", apiBase, url.QueryEscape(groupName))
	res, err := client.Get(searchURL)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", err
	}

	// Selector from Dart: #$nameInstitute > div.info-block__content.info-block__content_top-padding > ul > li > span.text-form__item-description > div > a
	selector := fmt.Sprintf("#%s div.info-block__content_top-padding ul li span.text-form__item-description div a", institute)
	link, ok := doc.Find(selector).Attr("href")
	if !ok {
		// Try a broader selector if specific one fails
		link, ok = doc.Find("a").FilterFunction(func(i int, s *goquery.Selection) bool {
			return strings.Contains(s.Text(), groupName)
		}).Attr("href")
	}

	if !ok {
		return "", fmt.Errorf("timetable link not found for group %s in institute %s", groupName, institute)
	}

	return link, nil
}

func getLesson(doc *goquery.Document, week, lessonIdx, dayIdx int) string {
	selector := fmt.Sprintf("#week-%d div.d-none.d-md-block table tbody tr:nth-child(%d) td:nth-child(%d) div.timetable__grid-day-lesson", week, lessonIdx, dayIdx)
	return cleanText(doc.Find(selector).Text())
}

func getTeacher(doc *goquery.Document, week, lessonIdx, dayIdx int) string {
	selector := fmt.Sprintf("#week-%d div.d-none.d-md-block table tbody tr:nth-child(%d) td:nth-child(%d) div:nth-child(3)", week, lessonIdx, dayIdx)
	return cleanText(doc.Find(selector).Text())
}

func getRoom(doc *goquery.Document, week, lessonIdx, dayIdx int) string {
	selector := fmt.Sprintf("#week-%d div.d-none.d-md-block table tbody tr:nth-child(%d) td:nth-child(%d) div:nth-child(5)", week, lessonIdx, dayIdx)
	return cleanText(doc.Find(selector).Text())
}

func cleanText(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

type importer struct {
	db             *pgxpool.Pool
	filialID       uuid.UUID
	termName       string
	subjects       map[string]uuid.UUID
	rooms          map[string]uuid.UUID
	groups         map[string]uuid.UUID
	teachers       map[string]uuid.UUID
	affectedGroups map[uuid.UUID]struct{}
}

func (i *importer) importMiitEvent(ctx context.Context, tx pgx.Tx, groupName, lessonName, teacherName, roomName string, week, dayOfWeek, lessonNum int, occursOn *time.Time, isExam bool, lessonType string, stats *Stats, customTime ...string) error {
	termID, err := i.ensureTerm(ctx, tx)
	if err != nil {
		return err
	}

	groupID, err := i.ensureGroup(ctx, tx, groupName)
	if err != nil {
		return err
	}
	i.affectedGroups[groupID] = struct{}{}

	subjectID, err := i.ensureSubject(ctx, tx, lessonName)
	if err != nil {
		return err
	}

	var teacherID *uuid.UUID
	if teacherName != "" {
		id, err := i.ensureTeacher(ctx, tx, teacherName)
		if err != nil {
			return err
		}
		teacherID = &id
	}

	var roomID *uuid.UUID
	if roomName != "" {
		id, err := i.ensureRoom(ctx, tx, roomName)
		if err != nil {
			return err
		}
		roomID = &id
	}

	var startsAt, endsAt string
	if lessonNum > 0 {
		startsAt, endsAt = lessonTime(lessonNum)
	} else if len(customTime) > 0 && customTime[0] != "" {
		startsAt = customTime[0]
	}

	weekType := "ALL"
	if week == 1 {
		weekType = "ODD"
	} else if week == 2 {
		weekType = "EVEN"
	}

	sourceEventID := sourceEventIDInt64(groupName, week, dayOfWeek, lessonNum, lessonName, occursOn, isExam)
	if isExam {
		weekType = "ONCE"
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO public.timetable_entry (
			term_id, group_id, subject_id, teacher_id, classroom_id, day_of_week, week_type, occurs_on, starts_at, ends_at, source, source_event_id, is_exam, lesson_type, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9::time, $10::time, 'miit', $11, $12, $13, now()
		)
		ON CONFLICT ON CONSTRAINT timetable_entry_source_event_unique DO UPDATE SET
			subject_id = EXCLUDED.subject_id,
			teacher_id = EXCLUDED.teacher_id,
			classroom_id = EXCLUDED.classroom_id,
			day_of_week = EXCLUDED.day_of_week,
			week_type = EXCLUDED.week_type,
			occurs_on = EXCLUDED.occurs_on,
			starts_at = EXCLUDED.starts_at,
			ends_at = EXCLUDED.ends_at,
			is_exam = EXCLUDED.is_exam,
			lesson_type = EXCLUDED.lesson_type,
			updated_at = now()
	`, termID, groupID, subjectID, teacherID, roomID, dayOfWeek, weekType, occursOn, nullIfEmptyString(startsAt), nullIfEmptyString(endsAt), sourceEventID, isExam, nullIfEmptyString(lessonType))

	return err
}

func sourceEventIDInt64(groupName string, week, dayOfWeek, lessonNum int, lessonName string, occursOn *time.Time, isExam bool) int64 {
	var key string
	if isExam && occursOn != nil {
		key = fmt.Sprintf("miit-exam|%s|%s|%s", groupName, occursOn.Format("2006-01-02"), lessonName)
	} else {
		key = fmt.Sprintf("miit-regular|%s|%d|%d|%d|%s", groupName, week, dayOfWeek, lessonNum, lessonName)
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return int64(h.Sum64() & 0x7fffffffffffffff)
}

func nullIfEmptyString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (i *importer) ensureTerm(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM public.academic_term WHERE filial_id = $1 AND name = $2 LIMIT 1`, i.filialID, i.termName).Scan(&id)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `
			INSERT INTO public.academic_term (filial_id, name, starts_on, ends_on)
			VALUES ($1, $2, now(), now() + interval '6 months')
			RETURNING id
		`, i.filialID, i.termName).Scan(&id)
	}
	return id, err
}

func (i *importer) ensureGroup(ctx context.Context, tx pgx.Tx, name string) (uuid.UUID, error) {
	if id, ok := i.groups[name]; ok {
		return id, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM public.edu_group WHERE filial_id = $1 AND name = $2 LIMIT 1`, i.filialID, name).Scan(&id)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `INSERT INTO public.edu_group (filial_id, name, source) VALUES ($1, $2, 'miit') RETURNING id`, i.filialID, name).Scan(&id)
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

func (i *importer) ensureTeacher(ctx context.Context, tx pgx.Tx, fullName string) (uuid.UUID, error) {
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
INSERT INTO public.staff (person_id, filial_id, staff_type, active)
VALUES ($1, $2, 'TEACHER', true)
RETURNING id
`, personID, i.filialID).Scan(&staffID)
	}
	if err != nil {
		return uuid.Nil, err
	}

	i.teachers[fullName] = staffID
	return staffID, nil
}

func (i *importer) ensureRoom(ctx context.Context, tx pgx.Tx, name string) (uuid.UUID, error) {
	if id, ok := i.rooms[name]; ok {
		return id, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM public.room WHERE filial_id = $1 AND name = $2 LIMIT 1`, i.filialID, name).Scan(&id)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `INSERT INTO public.room (filial_id, room_type, name) VALUES ($1, 'CLASSROOM', $2) RETURNING id`, i.filialID, name).Scan(&id)
	}
	if err != nil {
		return uuid.Nil, err
	}
	i.rooms[name] = id
	return id, nil
}

func (i *importer) affectedGroupIDs() []string {
	ids := make([]string, 0, len(i.affectedGroups))
	for id := range i.affectedGroups {
		ids = append(ids, id.String())
	}
	return ids
}

func lessonTime(num int) (string, string) {
	switch num {
	case 1:
		return "08:30", "09:50"
	case 2:
		return "10:05", "11:25"
	case 3:
		return "11:40", "13:00"
	case 4:
		return "13:45", "15:05"
	case 5:
		return "15:20", "16:40"
	case 6:
		return "16:55", "18:15"
	case 7:
		return "18:30", "19:50"
	default:
		return "00:00", "00:00"
	}
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

func ensureFilialExistsOrCreate(ctx context.Context, db *pgxpool.Pool, filialID uuid.UUID) error {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.filial WHERE id = $1)`, filialID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check filial exists: %w", err)
	}
	if exists {
		return nil
	}
	if _, err := db.Exec(ctx, `INSERT INTO public.filial (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, filialID); err != nil {
		return fmt.Errorf("create filial: %w", err)
	}
	return nil
}
