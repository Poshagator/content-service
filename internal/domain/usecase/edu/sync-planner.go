package edu

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	entityedu "github.com/poshagator/content-service/internal/domain/entities/edu"
	"github.com/poshagator/content-service/pkg/proto/planner/gen"
	"go.uber.org/zap"
	"time"
)

const StudyActivityID = "409f07cc-4a6a-41dd-b611-cba20d496ed4"
const plannerSourceTypeEduGroup = "edu_group"

func (u *Usecase) SyncGroupScheduleToPlanner(ctx context.Context, userID string, groupID uuid.UUID, termID uuid.UUID, activityID string) error {
	group, err := u.repo.GetEduGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("get edu group: %w", err)
	}
	metadata, _ := json.Marshal(map[string]any{
		"group_id":           group.ID.String(),
		"name":               group.Name,
		"faculty_name":       group.FacultyName,
		"course_name":        group.CourseName,
		"education_level":    group.EducationLevel,
		"is_subgroup":        group.IsSubgroup,
		"parent_group_id":    group.ParentGroupID.String(),
		"source_group_id":    group.SourceGroupID,
		"source_subgroup_id": group.SourceSubgroupID,
	})

	if _, err := u.plannerClient.Subscribe(ctx, &planner.SubscribeRequest{
		UserId:       userID,
		SourceType:   plannerSourceTypeEduGroup,
		SourceId:     groupID.String(),
		SourceName:   group.Name,
		MetadataJson: string(metadata),
	}); err != nil {
		return fmt.Errorf("subscribe planner source: %w", err)
	}

	tasks, err := u.GenerateGroupScheduleTasks(ctx, groupID, termID, activityID)
	if err != nil {
		return err
	}

	resp, err := u.plannerClient.SyncSourceTasks(ctx, &planner.SyncSourceTasksRequest{
		SourceType: plannerSourceTypeEduGroup,
		SourceId:   groupID.String(),
		Tasks:      tasks,
	})
	if err != nil {
		return fmt.Errorf("sync planner source tasks: %w", err)
	}

	u.log.Info("planner subscription synced",
		zap.String("userID", userID),
		zap.String("groupID", groupID.String()),
		zap.Int("tasks", len(tasks)),
		zap.Int32("subscriptions", resp.GetSubscriptionsCount()),
		zap.Int32("syncedTasks", resp.GetSyncedTasksCount()),
		zap.Int32("deletedTasks", resp.GetDeletedTasksCount()),
	)

	return nil
}

func (u *Usecase) GenerateGroupScheduleTasks(ctx context.Context, groupID uuid.UUID, termID uuid.UUID, activityID string) ([]*planner.ExternalTask, error) {
	term, err := u.resolveScheduleTerm(ctx, groupID, termID)
	if err != nil {
		return nil, fmt.Errorf("get academic term: %w", err)
	}
	timetable, err := u.repo.GetTimetableByTerm(ctx, groupID, term.ID)
	if err != nil {
		return nil, fmt.Errorf("get timetable: %w", err)
	}
	if activityID == "" {
		activityID = StudyActivityID
	}

	tasks := make([]*planner.ExternalTask, 0)
	start := term.StartsOn
	now := time.Now()
	if start.Before(now) {
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
	end := term.EndsOn

	for d := start; d.Before(end) || d.Equal(end); d = d.AddDate(0, 0, 1) {
		weekday := int(d.Weekday())
		if weekday == 0 {
			weekday = 7
		}

		isOdd := (isoWeekDiff(term.StartsOn, d)+term.WeekStart)%2 == 1
		var currentWeekType string
		if isOdd {
			currentWeekType = "ODD"
		} else {
			currentWeekType = "EVEN"
		}

		for _, entry := range timetable {
			date := d.Format("2006-01-02")
			if entry.OccursOn != "" && entry.OccursOn != date {
				continue
			}
			if entry.EffectiveFrom != "" && date < entry.EffectiveFrom {
				continue
			}
			if entry.EffectiveTo != "" && date > entry.EffectiveTo {
				continue
			}
			if entry.DayOfWeek != weekday {
				continue
			}
			// Exact Suruz events (exams, credits, one-off changes) are bound to OccursOn.
			if entry.OccursOn == "" && entry.WeekType != "ALL" && entry.WeekType != currentWeekType {
				continue
			}

			startTime := entry.StartsAt
			if len(startTime) > 5 {
				startTime = startTime[:5]
			}
			endTime := entry.EndsAt
			if len(endTime) > 5 {
				endTime = endTime[:5]
			}

			description := fmt.Sprintf("Преподаватель: %s\nАудитория: %s", entry.TeacherName, entry.RoomName)
			if entry.IsExam {
				description = "Экзамен\n" + description
			}
			if entry.Comment != "" {
				description += "\n" + entry.Comment
			}

			tasks = append(tasks, &planner.ExternalTask{
				ExternalId:  fmt.Sprintf("%s_%s", entry.ID, date),
				Date:        date,
				StartTime:   startTime,
				EndTime:     endTime,
				Title:       entry.SubjectName,
				Description: description,
				ActivityId:  activityID,
				Action:      planner.SyncAction_SYNC_ACTION_UPSERT,
			})
		}
	}

	return tasks, nil
}

func (u *Usecase) UnsubscribeFromPlanner(ctx context.Context, userID string, groupID uuid.UUID, termID uuid.UUID) error {
	_, err := u.plannerClient.Unsubscribe(ctx, &planner.UnsubscribeRequest{
		UserId:     userID,
		SourceType: plannerSourceTypeEduGroup,
		SourceId:   groupID.String(),
	})
	if err != nil {
		return fmt.Errorf("unsubscribe planner source: %w", err)
	}
	return nil
}

func (u *Usecase) GetPlannerSubscriptions(ctx context.Context, userID string) ([]*planner.Subscription, error) {
	resp, err := u.plannerClient.GetUserSubscriptions(ctx, &planner.GetUserSubscriptionsRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("get planner subscriptions: %w", err)
	}
	return resp.GetSubscriptions(), nil
}

func (u *Usecase) SyncPlannerSourceForGroup(ctx context.Context, groupID uuid.UUID) error {
	tasks, err := u.GenerateGroupScheduleTasks(ctx, groupID, uuid.Nil, "")
	if err != nil {
		return err
	}
	resp, err := u.plannerClient.SyncSourceTasks(ctx, &planner.SyncSourceTasksRequest{
		SourceType: plannerSourceTypeEduGroup,
		SourceId:   groupID.String(),
		Tasks:      tasks,
	})
	if err != nil {
		return fmt.Errorf("sync planner source tasks: %w", err)
	}
	u.log.Info("planner source synced",
		zap.String("groupID", groupID.String()),
		zap.Int("tasks", len(tasks)),
		zap.Int32("subscriptions", resp.GetSubscriptionsCount()),
		zap.Int32("syncedTasks", resp.GetSyncedTasksCount()),
		zap.Int32("deletedTasks", resp.GetDeletedTasksCount()),
	)
	return nil
}

func (u *Usecase) resolveScheduleTerm(ctx context.Context, groupID uuid.UUID, termID uuid.UUID) (*entityedu.AcademicTerm, error) {
	if termID != uuid.Nil {
		return u.repo.GetAcademicTerm(ctx, termID)
	}
	return u.repo.GetCurrentAcademicTermForGroup(ctx, groupID)
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
