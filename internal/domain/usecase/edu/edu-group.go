package edu

import (
	"context"
	"github.com/google/uuid"
	"github.com/poshagator/content-service/internal/domain/entities/edu"
	"github.com/poshagator/content-service/pkg"
	"go.uber.org/zap"
)

func (u *Usecase) GetEduGroup(ctx context.Context, id uuid.UUID) (*edu.EduGroup, error) {
	EduGroup, err := u.repo.GetEduGroup(ctx, id)
	if err != nil {
		u.log.Error("failed to get EduGroup", zap.Error(err))
		return nil, err
	}

	return EduGroup, nil
}

func (u *Usecase) CreateEduGroup(ctx context.Context, EduGroup *edu.EduGroup) (*edu.EduGroup, error) {
	err := u.repo.CreateEduGroups(ctx, EduGroup)
	if err != nil {
		u.log.Error("failed to create EduGroup", zap.Error(err))
		return nil, err
	}

	return EduGroup, nil
}

func (u *Usecase) UpdateEduGroup(ctx context.Context, EduGroup *edu.EduGroup) (*edu.EduGroup, error) {
	err := u.repo.UpdateEduGroups(ctx, EduGroup)
	if err != nil {
		u.log.Error("failed to update EduGroup", zap.Error(err))
		return nil, err
	}

	return EduGroup, nil
}

func (u *Usecase) DeleteEduGroup(ctx context.Context, id uuid.UUID) error {
	err := u.repo.DeleteEduGroups(ctx, id)
	if err != nil {
		u.log.Error("failed to delete EduGroup", zap.Error(err))
		return err
	}

	return nil
}

func (u *Usecase) GetEduGroups(ctx context.Context, filialID uuid.UUID, size, page int) ([]edu.EduGroup, error) {
	EduGroups, err := u.repo.GetEduGroups(ctx, filialID, pkg.PaginationQuery(page, size))
	if err != nil {
		u.log.Error("failed to get EduGroups", zap.Error(err))
		return nil, err
	}

	return EduGroups, nil
}

func (u *Usecase) GetEduGroupsGrouped(ctx context.Context, filialID uuid.UUID) (edu.EduGroupsByFaculty, error) {
	groups, err := u.repo.GetEduGroups(ctx, filialID, " ORDER BY faculty_name NULLS LAST, education_level, course_name NULLS LAST, name")
	if err != nil {
		u.log.Error("failed to get grouped EduGroups", zap.Error(err))
		return nil, err
	}

	return groupEduGroups(groups), nil
}

func (u *Usecase) GetEduGroupsGroupedWithSubscriptions(ctx context.Context, filialID uuid.UUID, userID string) (edu.EduGroupsByFaculty, error) {
	tree, err := u.GetEduGroupsGrouped(ctx, filialID)
	if err != nil {
		return nil, err
	}
	if userID == "" {
		return tree, nil
	}
	subscriptions, err := u.GetPlannerSubscriptions(ctx, userID)
	if err != nil {
		return nil, err
	}
	subscribed := make(map[string]struct{}, len(subscriptions))
	for _, sub := range subscriptions {
		if sub.GetSourceType() == plannerSourceTypeEduGroup {
			subscribed[sub.GetSourceId()] = struct{}{}
		}
	}
	for fi := range tree {
		for ci := range tree[fi].Courses {
			for gi := range tree[fi].Courses[ci].Groups {
				markGroupSubscription(&tree[fi].Courses[ci].Groups[gi], subscribed)
			}
		}
	}
	return tree, nil
}

func (u *Usecase) SearchEduGroups(ctx context.Context, filialID uuid.UUID, name string, limit int) ([]edu.EduGroup, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return u.repo.SearchEduGroups(ctx, filialID, name, limit)
}

func markGroupSubscription(group *edu.EduGroup, subscribed map[string]struct{}) {
	if _, ok := subscribed[group.ID.String()]; ok {
		group.IsSubscribed = true
	}
	for i := range group.Subgroups {
		markGroupSubscription(&group.Subgroups[i], subscribed)
	}
}

func groupEduGroups(groups []edu.EduGroup) edu.EduGroupsByFaculty {
	byID := make(map[uuid.UUID]*edu.EduGroup, len(groups))
	parentGroups := make([]edu.EduGroup, 0, len(groups))
	for i := range groups {
		group := groups[i]
		group.Subgroups = nil
		if !group.IsSubgroup {
			parentGroups = append(parentGroups, group)
		}
	}
	for i := range parentGroups {
		byID[parentGroups[i].ID] = &parentGroups[i]
	}
	for _, group := range groups {
		if !group.IsSubgroup || group.ParentGroupID == uuid.Nil {
			continue
		}
		if parent, ok := byID[group.ParentGroupID]; ok {
			parent.Subgroups = append(parent.Subgroups, group)
		}
	}

	facultyIndex := make(map[string]int)
	courseIndex := make(map[string]map[string]int)
	result := make(edu.EduGroupsByFaculty, 0)

	for _, group := range parentGroups {
		facultyKey := group.FacultyName
		if facultyKey == "" {
			facultyKey = "Без факультета"
		}
		facultyKey = facultyKey + "|" + group.EducationLevel

		fi, ok := facultyIndex[facultyKey]
		if !ok {
			fi = len(result)
			facultyIndex[facultyKey] = fi
			courseIndex[facultyKey] = make(map[string]int)
			result = append(result, edu.EduFacultyGroup{
				FacultyID:      group.FacultyID,
				FacultyName:    group.FacultyName,
				EducationLevel: group.EducationLevel,
				IsMagistracy:   group.IsMagistracy,
				Courses:        make([]edu.EduCourseGroup, 0),
			})
		}

		courseKey := group.CourseName
		if courseKey == "" {
			courseKey = "Без курса"
		}
		ci, ok := courseIndex[facultyKey][courseKey]
		if !ok {
			ci = len(result[fi].Courses)
			courseIndex[facultyKey][courseKey] = ci
			result[fi].Courses = append(result[fi].Courses, edu.EduCourseGroup{
				CourseID:   group.CourseID,
				CourseName: group.CourseName,
				Groups:     make([]edu.EduGroup, 0),
			})
		}

		result[fi].Courses[ci].Groups = append(result[fi].Courses[ci].Groups, group)
	}

	return result
}
