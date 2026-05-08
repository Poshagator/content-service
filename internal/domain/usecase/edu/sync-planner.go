package edu

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/poshagator/content-service/pkg/proto/planner/gen"
	"time"
)

const StudyActivityID = "409f07cc-4a6a-41dd-b611-cba20d496ed4"

func (u *Usecase) SyncGroupScheduleToPlanner(ctx context.Context, userID string, groupID uuid.UUID, termID uuid.UUID, activityID string) error {
	// 1. Get Term info
	term, err := u.repo.GetAcademicTerm(ctx, termID)
	if err != nil {
		return fmt.Errorf("get academic term: %w", err)
	}

	// 2. Get Timetable
	timetable, err := u.repo.GetTimetable(ctx, groupID)
	if err != nil {
		return fmt.Errorf("get timetable: %w", err)
	}

	// 3. Prepare tasks
	var tasks []*planner.ExternalTask
	if activityID == "" {
		activityID = StudyActivityID
	}

	start := term.StartsOn
	now := time.Now()
	if start.Before(now) {
		// Start from today's beginning to catch today's lessons
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
	end := term.EndsOn

	for d := start; d.Before(end) || d.Equal(end); d = d.AddDate(0, 0, 1) {
		weekday := int(d.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday
		}

		// If term.WeekStart is 1, then the first week (diff=0) is ODD.
		// (0 + 1) % 2 = 1 (ODD)
		// (1 + 1) % 2 = 0 (EVEN)
		isOdd := (isoWeekDiff(term.StartsOn, d)+term.WeekStart)%2 == 1
		var currentWeekType string
		if isOdd {
			currentWeekType = "ODD"
		} else {
			currentWeekType = "EVEN"
		}

		for _, entry := range timetable {
			if entry.DayOfWeek != weekday {
				continue
			}
			// ALL means it happens every week
			if entry.WeekType != "ALL" && entry.WeekType != currentWeekType {
				continue
			}

			// Format HH:MM:SS -> HH:MM
			startTime := entry.StartsAt
			if len(startTime) > 5 {
				startTime = startTime[:5]
			}
			endTime := entry.EndsAt
			if len(endTime) > 5 {
				endTime = endTime[:5]
			}

			tasks = append(tasks, &planner.ExternalTask{
				ExternalId:  fmt.Sprintf("%s_%s", entry.ID, d.Format("2006-01-02")),
				Date:        d.Format("2006-01-02"),
				StartTime:   startTime,
				EndTime:     endTime,
				Title:       entry.SubjectName,
				Description: fmt.Sprintf("Преподаватель: %s\nАудитория: %s", entry.TeacherName, entry.RoomName),
				ActivityId:  activityID,
				Action:      planner.SyncAction_SYNC_ACTION_UPSERT,
			})
		}

		// Batch send if too many tasks to avoid gRPC message size limits
		if len(tasks) >= 100 {
			_, err = u.plannerClient.SyncTasks(ctx, &planner.SyncTasksRequest{
				UserId: userID,
				Source: "university",
				Tasks:  tasks,
			})
			if err != nil {
				return fmt.Errorf("sync tasks batch: %w", err)
			}
			tasks = nil
		}
	}

	// Final batch
	if len(tasks) > 0 {
		_, err = u.plannerClient.SyncTasks(ctx, &planner.SyncTasksRequest{
			UserId: userID,
			Source: "university",
			Tasks:  tasks,
		})
		if err != nil {
			return fmt.Errorf("sync tasks final batch: %w", err)
		}
	}

	return nil
}

func (u *Usecase) UnsubscribeFromPlanner(ctx context.Context, userID string, groupID uuid.UUID, termID uuid.UUID) error {
	// 1. Get Term info
	term, err := u.repo.GetAcademicTerm(ctx, termID)
	if err != nil {
		return fmt.Errorf("get academic term: %w", err)
	}

	// 2. Get Timetable
	timetable, err := u.repo.GetTimetable(ctx, groupID)
	if err != nil {
		return fmt.Errorf("get timetable: %w", err)
	}

	// 3. Prepare tasks for deletion
	var tasks []*planner.ExternalTask

	// Delete every task that could have been created for this subscription.
	for d := term.StartsOn; d.Before(term.EndsOn) || d.Equal(term.EndsOn); d = d.AddDate(0, 0, 1) {
		weekday := int(d.Weekday())
		if weekday == 0 {
			weekday = 7
		}

		isOdd := (isoWeekDiff(term.StartsOn, d)+term.WeekStart)%2 == 1
		currentWeekType := "EVEN"
		if isOdd {
			currentWeekType = "ODD"
		}

		for _, entry := range timetable {
			if entry.DayOfWeek != weekday {
				continue
			}
			if entry.WeekType != "ALL" && entry.WeekType != currentWeekType {
				continue
			}

			tasks = append(tasks, &planner.ExternalTask{
				ExternalId: fmt.Sprintf("%s_%s", entry.ID, d.Format("2006-01-02")),
				Action:     planner.SyncAction_SYNC_ACTION_DELETE,
			})
		}

		if len(tasks) >= 100 {
			_, err = u.plannerClient.SyncTasks(ctx, &planner.SyncTasksRequest{
				UserId: userID,
				Source: "university",
				Tasks:  tasks,
			})
			if err != nil {
				return fmt.Errorf("unsubscribe tasks batch: %w", err)
			}
			tasks = nil
		}
	}

	if len(tasks) > 0 {
		_, err = u.plannerClient.SyncTasks(ctx, &planner.SyncTasksRequest{
			UserId: userID,
			Source: "university",
			Tasks:  tasks,
		})
		if err != nil {
			return fmt.Errorf("unsubscribe tasks final batch: %w", err)
		}
	}

	return nil
}

func isoWeekDiff(from, to time.Time) int {
	fromWeekStart := isoWeekStart(from)
	toWeekStart := isoWeekStart(to)

	return int(toWeekStart.Sub(fromWeekStart).Hours() / 24 / 7)
}

func isoWeekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return dayStart.AddDate(0, 0, 1-weekday)
}
