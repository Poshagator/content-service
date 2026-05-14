package miit

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	APIBase     string
	DSN         string
	FilialID    string
	GroupName   string
	Institute   string
	Timeout     time.Duration
	Concurrency int
	TermName    string
	Logger      *zap.Logger
}

type Stats struct {
	Groups           int64
	Teachers         int64
	Schedules        int64
	FetchErrors      int64
	Events           int64
	EntriesCreated   int64
	EntriesExisting  int64
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

	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	total := len(groups)
	startedAt := time.Now()
	for idx, group := range groups {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, group miitGroup) {
			defer wg.Done()
			defer func() { <-sem }()

			groupStartedAt := time.Now()
			if cfg.Logger != nil && (idx == 0 || (idx+1)%25 == 0 || idx+1 == total) {
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
				atomic.AddInt64(&stats.FetchErrors, 1)
				if cfg.Logger != nil {
					cfg.Logger.Warn("failed to sync MIIT regular schedule",
						zap.String("group", group.Name),
						zap.Error(err),
						zap.Duration("groupElapsed", time.Since(groupStartedAt)),
					)
				}
			} else {
				atomic.AddInt64(&stats.Schedules, 1)
			}

			// Sync session schedule (Exams)
			err = imp.syncSessionSchedule(ctx, client, cfg.APIBase, group, &stats)
			if err != nil {
				if cfg.Logger != nil {
					cfg.Logger.Warn("failed to sync MIIT session schedule",
						zap.String("group", group.Name),
						zap.Error(err),
						zap.Duration("groupElapsed", time.Since(groupStartedAt)),
					)
				}
			}

			curEvents := atomic.LoadInt64(&stats.Events)
			curErrors := atomic.LoadInt64(&stats.FetchErrors)

			if cfg.Logger != nil {
				cfg.Logger.Info("MIIT group processed",
					zap.Int("current", idx+1),
					zap.Int("total", total),
					zap.String("group", group.Name),
					zap.Duration("groupElapsed", time.Since(groupStartedAt)),
					zap.Int64("eventsTotal", curEvents),
					zap.Int64("fetchErrorsTotal", curErrors),
				)
			}

			if cfg.Logger != nil && (idx == 0 || (idx+1)%50 == 0 || idx+1 == total) {
				imp.mu.RLock()
				affectedCount := len(imp.affectedGroups)
				imp.mu.RUnlock()

				cfg.Logger.Info("MIIT sync progress",
					zap.Int("current", idx+1),
					zap.Int("total", total),
					zap.Int64("events", curEvents),
					zap.Int64("fetchErrors", curErrors),
					zap.Int("affectedGroups", affectedCount),
					zap.Duration("elapsed", time.Since(startedAt)),
				)
			}
		}(idx, group)
	}
	wg.Wait()

	stats.AffectedGroupIDs = imp.affectedGroupIDs()
	return stats, nil
}

type miitGroup struct {
	Name          string
	TimetableLink string
	SourceGroupID int
	InstituteName string
	InstituteID   string
	Course        int
}

func FetchGroups(ctx context.Context, client *http.Client, apiBase string) ([]miitGroup, error) {
	doc, err := fetchDocument(ctx, client, strings.TrimRight(apiBase, "/")+"/timetable")
	if err != nil {
		return nil, err
	}

	var groups []miitGroup
	seen := make(map[string]struct{})
	addGroup := func(g miitGroup) {
		key := strings.TrimSpace(g.TimetableLink)
		if key == "" {
			// Fallback key for malformed rows without link.
			key = "name:" + strings.TrimSpace(g.Name)
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		groups = append(groups, g)
	}
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
						addGroup(miitGroup{
							Name:          name,
							TimetableLink: href,
							SourceGroupID: parseTimetableID(href),
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
						addGroup(miitGroup{
							Name:          name,
							TimetableLink: href,
							SourceGroupID: parseTimetableID(href),
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

func parseTimetableID(href string) int {
	href = strings.TrimSpace(href)
	if href == "" {
		return 0
	}
	parts := strings.Split(strings.Trim(href, "/"), "/")
	if len(parts) == 0 {
		return 0
	}
	id, _ := strconv.Atoi(parts[len(parts)-1])
	return id
}

func (i *importer) syncRegularSchedule(ctx context.Context, client *http.Client, apiBase string, group miitGroup, stats *Stats) error {
	doc, err := fetchDocument(ctx, client, strings.TrimRight(apiBase, "/")+group.TimetableLink+"?type=1")
	if err != nil {
		return err
	}

	effectiveFrom, effectiveTo := parseScheduleEffectiveRange(doc)
	var events []regularEvent
	for week := 1; week <= 2; week++ {
		events = append(events, parseRegularWeekEvents(doc, week)...)
	}

	if err := i.clearGroupSchedule(ctx, group); err != nil {
		return err
	}

	tx, err := i.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, event := range events {
		err = i.importMiitEvent(ctx, tx, group, event.SubjectName, event.TeacherName, event.RoomName, event.Week, event.DayOfWeek, event.LessonNum, nil, effectiveFrom, effectiveTo, false, event.LessonType, event.Comment, stats)
		if err != nil {
			return err
		}
		atomic.AddInt64(&stats.Events, 1)
	}
	return tx.Commit(ctx)
}

func (i *importer) clearGroupSchedule(ctx context.Context, group miitGroup) error {
	tx, err := i.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	termID, err := i.ensureTerm(ctx, tx)
	if err != nil {
		return err
	}
	groupID, err := i.ensureGroup(ctx, tx, group)
	if err != nil {
		return err
	}
	i.mu.Lock()
	i.affectedGroups[groupID] = struct{}{}
	i.mu.Unlock()

	_, err = tx.Exec(ctx, `
DELETE FROM public.timetable_entry
WHERE term_id = $1
  AND group_id = $2
  AND source = 'miit'
`, termID, groupID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type regularEvent struct {
	SubjectName string
	TeacherName string
	RoomName    string
	LessonType  string
	Comment     string
	Week        int
	DayOfWeek   int
	LessonNum   int
}

func parseRegularWeekEvents(doc *goquery.Document, week int) []regularEvent {
	var events []regularEvent
	weekPane := doc.Find(fmt.Sprintf("#week-%d", week))
	weekPane.Find("div.d-none.d-md-block table.timetable__grid tr").Each(func(rowIdx int, row *goquery.Selection) {
		lessonNum := rowIdx
		if lessonNum <= 0 {
			return
		}

		row.Find("td.timetable__grid-day").Each(func(dayIdx int, cell *goquery.Selection) {
			dayOfWeek := dayIdx + 1
			parseRegularCellEvents(cell, week, dayOfWeek, lessonNum, &events)
		})
	})
	return events
}

func parseRegularCellEvents(cell *goquery.Selection, week, dayOfWeek, lessonNum int, events *[]regularEvent) {
	cell.Find("div.timetable__grid-day-lesson").Each(func(_ int, lesson *goquery.Selection) {
		lessonType := cleanText(lesson.Find(".timetable__grid-text_gray").First().Text())
		subjectName := cleanText(strings.TrimPrefix(cleanText(lesson.Text()), lessonType))
		if subjectName == "" {
			return
		}

		details := lesson.NextUntil("div.timetable__grid-day-lesson")
		teacherName, extraTeachers := extractTeacherNames(details)
		roomName := extractRoomNames(details)
		comment := extractCommunityText(details)
		if extraTeachers != "" {
			if comment != "" {
				comment += "; "
			}
			comment += "Преподаватели: " + extraTeachers
		}

		*events = append(*events, regularEvent{
			SubjectName: subjectName,
			TeacherName: teacherName,
			RoomName:    roomName,
			LessonType:  lessonType,
			Comment:     comment,
			Week:        week,
			DayOfWeek:   dayOfWeek,
			LessonNum:   lessonNum,
		})
	})
}

func extractTeacherNames(details *goquery.Selection) (string, string) {
	var names []string
	details.Find("a.icon-academic-cap").Each(func(_ int, s *goquery.Selection) {
		name := cleanText(s.AttrOr("title", ""))
		if idx := strings.Index(name, ","); idx >= 0 {
			name = strings.TrimSpace(name[:idx])
		}
		if name == "" {
			name = cleanText(s.Text())
		}
		if name != "" {
			names = append(names, name)
		}
	})
	if len(names) == 0 {
		return "", ""
	}
	if len(names) == 1 {
		return names[0], ""
	}
	return names[0], strings.Join(names, "; ")
}

func extractRoomNames(details *goquery.Selection) string {
	var rooms []string
	details.Find("a.icon-location").Each(func(_ int, s *goquery.Selection) {
		room := cleanText(s.AttrOr("title", ""))
		if room == "" {
			room = cleanText(s.Text())
		}
		room = strings.TrimPrefix(room, "Аудитория ")
		if room != "" {
			rooms = append(rooms, room)
		}
	})
	return strings.Join(rooms, "; ")
}

func extractCommunityText(details *goquery.Selection) string {
	var parts []string
	details.Find(".icon-community").Each(func(_ int, s *goquery.Selection) {
		text := cleanText(s.Text())
		if text != "" {
			parts = append(parts, text)
		}
	})
	return strings.Join(parts, "; ")
}

func (i *importer) syncSessionSchedule(ctx context.Context, client *http.Client, apiBase string, group miitGroup, stats *Stats) error {
	doc, err := fetchDocument(ctx, client, strings.TrimRight(apiBase, "/")+group.TimetableLink+"?type=4")
	if err != nil {
		return err
	}

	events := parseSessionEvents(doc)
	if len(events) == 0 {
		return nil
	}

	tx, err := i.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, event := range events {
		err = i.importMiitEvent(ctx, tx, group, event.SubjectName, event.TeacherName, event.RoomName, 0, event.DayOfWeek, 0, &event.OccursOn, nil, nil, true, event.LessonType, event.Comment, stats, event.StartsAt, event.EndsAt)
		if err != nil {
			return err
		}
		atomic.AddInt64(&stats.Events, 1)
	}
	return tx.Commit(ctx)
}

type sessionEvent struct {
	SubjectName string
	TeacherName string
	RoomName    string
	LessonType  string
	Comment     string
	OccursOn    time.Time
	DayOfWeek   int
	StartsAt    string
	EndsAt      string
}

func parseSessionEvents(doc *goquery.Document) []sessionEvent {
	var events []sessionEvent
	doc.Find("div.info-block[data-date]").Each(func(_ int, block *goquery.Selection) {
		date, err := time.Parse("2006-01-02", block.AttrOr("data-date", ""))
		if err != nil {
			return
		}
		dayOfWeek := int(date.Weekday())
		if dayOfWeek == 0 {
			dayOfWeek = 7
		}

		block.Find("div.timetable__list-timeslot").Each(func(_ int, slot *goquery.Selection) {
			startsAt, endsAt := parseTimeRange(cleanText(slot.Find("div.mb-1").First().Text()))
			body := slot.Find("div.pl-4").First()
			lessonType := cleanText(body.Find("span.timetable__grid-text_gray").First().Text())
			subjectName := cleanText(strings.TrimPrefix(cleanText(body.Clone().Children().Remove().End().Text()), lessonType))
			if subjectName == "" {
				subjectName = cleanText(strings.TrimPrefix(cleanText(body.Text()), lessonType))
			}
			if subjectName == "" {
				return
			}

			details := body.Find("div.timetable__grid-about").First()
			teacherName, extraTeachers := extractTeacherNames(details)
			roomName := extractRoomNames(details)
			comment := ""
			if extraTeachers != "" {
				comment = "Преподаватели: " + extraTeachers
			}

			events = append(events, sessionEvent{
				SubjectName: subjectName,
				TeacherName: teacherName,
				RoomName:    roomName,
				LessonType:  lessonType,
				Comment:     comment,
				OccursOn:    date,
				DayOfWeek:   dayOfWeek,
				StartsAt:    startsAt,
				EndsAt:      endsAt,
			})
		})
	})
	return events
}

func parseTimeRange(value string) (string, string) {
	value = strings.ReplaceAll(value, "—", "-")
	parts := strings.Split(value, "-")
	if len(parts) == 0 {
		return "", ""
	}
	startsAt := strings.TrimSpace(parts[0])
	endsAt := ""
	if len(parts) > 1 {
		endsAt = strings.TrimSpace(parts[1])
	}
	return startsAt, endsAt
}

func parseScheduleEffectiveRange(doc *goquery.Document) (*time.Time, *time.Time) {
	text := cleanText(doc.Text())
	re := regexp.MustCompile(`Расписание действует\s+с\s+(\d{2}\.\d{2}\.\d{4})\s+по\s+(\d{2}\.\d{2}\.\d{4})`)
	matches := re.FindStringSubmatch(text)
	if len(matches) != 3 {
		return nil, nil
	}

	startsOn, err := time.Parse("02.01.2006", matches[1])
	if err != nil {
		return nil, nil
	}
	endsOn, err := time.Parse("02.01.2006", matches[2])
	if err != nil {
		return nil, nil
	}
	return &startsOn, &endsOn
}

func fetchDocument(ctx context.Context, client *http.Client, rawURL string) (*goquery.Document, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")

		res, err := client.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			continue
		}
		defer res.Body.Close()

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			lastErr = fmt.Errorf("GET %s: status %s", rawURL, res.Status)
			if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return nil, lastErr
		}
		return goquery.NewDocumentFromReader(res.Body)
	}
	return nil, fmt.Errorf("after 3 attempts: %w", lastErr)
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
	if c.Concurrency <= 0 {
		c.Concurrency = 10
	}
	if strings.TrimSpace(c.TermName) == "" {
		c.TermName = DefaultTermName
	}
	return c
}

func findTimetableLink(ctx context.Context, client *http.Client, apiBase, groupName, institute string) (string, error) {
	searchURL := fmt.Sprintf("%s/timetable?query=%s", apiBase, url.QueryEscape(groupName))
	doc, err := fetchDocument(ctx, client, searchURL)
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

func cleanText(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
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
	mu             sync.RWMutex
}

func (i *importer) importMiitEvent(ctx context.Context, tx pgx.Tx, group miitGroup, lessonName, teacherName, roomName string, week, dayOfWeek, lessonNum int, occursOn, effectiveFrom, effectiveTo *time.Time, isExam bool, lessonType, comment string, stats *Stats, customTime ...string) error {
	termID, err := i.ensureTerm(ctx, tx)
	if err != nil {
		return err
	}

	groupID, err := i.ensureGroup(ctx, tx, group)
	if err != nil {
		return err
	}
	i.mu.Lock()
	i.affectedGroups[groupID] = struct{}{}
	i.mu.Unlock()

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
		if len(customTime) > 1 {
			endsAt = customTime[1]
		}
	}

	weekType := "ALL"
	if week == 1 {
		weekType = "ODD"
	} else if week == 2 {
		weekType = "EVEN"
	}

	sourceEventID := sourceEventIDInt64(group.Name, week, dayOfWeek, lessonNum, lessonName, teacherName, roomName, lessonType, comment, startsAt, endsAt, occursOn, isExam)
	if isExam {
		weekType = "ONCE"
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO public.timetable_entry (
			term_id, group_id, subject_id, teacher_id, classroom_id, day_of_week, week_type, occurs_on, effective_from, effective_to, starts_at, ends_at, source, source_event_id, is_exam, lesson_type, comment, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::time, $12::time, 'miit', $13, $14, $15, $16, now()
		)
		ON CONFLICT ON CONSTRAINT timetable_entry_source_event_unique DO UPDATE SET
			subject_id = EXCLUDED.subject_id,
			teacher_id = EXCLUDED.teacher_id,
			classroom_id = EXCLUDED.classroom_id,
			day_of_week = EXCLUDED.day_of_week,
			week_type = EXCLUDED.week_type,
			occurs_on = EXCLUDED.occurs_on,
			effective_from = EXCLUDED.effective_from,
			effective_to = EXCLUDED.effective_to,
			starts_at = EXCLUDED.starts_at,
			ends_at = EXCLUDED.ends_at,
			is_exam = EXCLUDED.is_exam,
			lesson_type = EXCLUDED.lesson_type,
			comment = EXCLUDED.comment,
			updated_at = now()
	`, termID, groupID, subjectID, teacherID, roomID, dayOfWeek, weekType, occursOn, effectiveFrom, effectiveTo, nullIfEmptyString(startsAt), nullIfEmptyString(endsAt), sourceEventID, isExam, nullIfEmptyString(lessonType), nullIfEmptyString(comment))

	return err
}

func sourceEventIDInt64(groupName string, week, dayOfWeek, lessonNum int, lessonName, teacherName, roomName, lessonType, comment, startsAt, endsAt string, occursOn *time.Time, isExam bool) int64 {
	var key string
	if isExam && occursOn != nil {
		key = fmt.Sprintf("miit-exam|%s|%s|%s|%s|%s|%s|%s|%s", groupName, occursOn.Format("2006-01-02"), startsAt, endsAt, lessonName, teacherName, roomName, lessonType)
	} else {
		key = fmt.Sprintf("miit-regular|%s|%d|%d|%d|%s|%s|%s|%s|%s", groupName, week, dayOfWeek, lessonNum, lessonName, teacherName, roomName, lessonType, comment)
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
	i.mu.RLock()
	if i.termID != uuid.Nil {
		defer i.mu.RUnlock()
		return i.termID, nil
	}
	i.mu.RUnlock()

	i.mu.Lock()
	defer i.mu.Unlock()
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

func (i *importer) ensureGroup(ctx context.Context, tx pgx.Tx, group miitGroup) (uuid.UUID, error) {
	i.mu.RLock()
	if id, ok := i.groups[group.Name]; ok {
		defer i.mu.RUnlock()
		return id, nil
	}
	i.mu.RUnlock()

	i.mu.Lock()
	defer i.mu.Unlock()
	if id, ok := i.groups[group.Name]; ok {
		return id, nil
	}

	courseName := miitCourseName(group.Course)
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
SELECT id
FROM public.edu_group
WHERE filial_id = $1 AND source = 'miit' AND source_group_id = NULLIF($2, 0)
LIMIT 1
`, i.filialID, group.SourceGroupID).Scan(&id)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `SELECT id FROM public.edu_group WHERE filial_id = $1 AND name = $2 LIMIT 1`, i.filialID, group.Name).Scan(&id)
		if err == pgx.ErrNoRows {
			err = tx.QueryRow(ctx, `
INSERT INTO public.edu_group (
	filial_id, name, source, source_group_id, faculty_name, course_id, course_name, education_level, is_magistracy
) VALUES (
	$1, $2, 'miit', NULLIF($3, 0), NULLIF($4, ''), NULLIF($5, 0), NULLIF($6, ''), NULL, false
) RETURNING id
`, i.filialID, group.Name, group.SourceGroupID, group.InstituteName, group.Course, courseName).Scan(&id)
		} else if err == nil {
			_, err = tx.Exec(ctx, `
UPDATE public.edu_group
SET source = 'miit',
    source_group_id = NULLIF($2, 0),
    faculty_name = NULLIF($3, ''),
    course_id = NULLIF($4, 0),
    course_name = NULLIF($5, ''),
    education_level = NULL,
    is_magistracy = false,
    updated_at = now()
WHERE id = $1
`, id, group.SourceGroupID, group.InstituteName, group.Course, courseName)
		}
	} else if err == nil {
		_, err = tx.Exec(ctx, `
UPDATE public.edu_group
SET name = $2,
    faculty_name = NULLIF($3, ''),
    course_id = NULLIF($4, 0),
    course_name = NULLIF($5, ''),
    education_level = NULL,
    is_magistracy = false,
    updated_at = now()
WHERE id = $1
`, id, group.Name, group.InstituteName, group.Course, courseName)
	}
	if err != nil {
		return uuid.Nil, err
	}
	i.groups[group.Name] = id
	return id, nil
}

func miitCourseName(course int) string {
	if course <= 0 {
		return ""
	}
	return strconv.Itoa(course)
}

func (i *importer) ensureSubject(ctx context.Context, tx pgx.Tx, name string) (uuid.UUID, error) {
	i.mu.RLock()
	if id, ok := i.subjects[name]; ok {
		defer i.mu.RUnlock()
		return id, nil
	}
	i.mu.RUnlock()

	i.mu.Lock()
	defer i.mu.Unlock()
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
	i.mu.RLock()
	if id, ok := i.teachers[fullName]; ok {
		defer i.mu.RUnlock()
		return id, nil
	}
	i.mu.RUnlock()

	i.mu.Lock()
	defer i.mu.Unlock()
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
	i.mu.RLock()
	if id, ok := i.rooms[name]; ok {
		defer i.mu.RUnlock()
		return id, nil
	}
	i.mu.RUnlock()

	i.mu.Lock()
	defer i.mu.Unlock()
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
	i.mu.RLock()
	defer i.mu.RUnlock()
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
