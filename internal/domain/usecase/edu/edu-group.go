package edu

import (
	"context"
	"github.com/poshagator/content-service/internal/domain/entities/edu"
	"github.com/poshagator/content-service/pkg"
	"github.com/google/uuid"
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

func (u *Usecase) SearchEduGroups(ctx context.Context, filialID uuid.UUID, name string, limit int) ([]edu.EduGroup, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return u.repo.SearchEduGroups(ctx, filialID, name, limit)
}
