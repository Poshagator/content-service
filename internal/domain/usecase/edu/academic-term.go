package edu

import (
	"context"
	"github.com/poshagator/content-service/internal/domain/entities/edu"
	"github.com/poshagator/content-service/pkg"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (u *Usecase) GetAcademicTerm(ctx context.Context, id uuid.UUID) (*edu.AcademicTerm, error) {
	AcademicTerm, err := u.repo.GetAcademicTerm(ctx, id)
	if err != nil {
		u.log.Error("failed to get AcademicTerm", zap.Error(err))
		return nil, err
	}

	return AcademicTerm, nil
}

func (u *Usecase) CreateAcademicTerm(ctx context.Context, AcademicTerm *edu.AcademicTerm) (*edu.AcademicTerm, error) {
	err := u.repo.CreateAcademicTerms(ctx, AcademicTerm)
	if err != nil {
		u.log.Error("failed to create AcademicTerm", zap.Error(err))
		return nil, err
	}

	return AcademicTerm, nil
}

func (u *Usecase) UpdateAcademicTerm(ctx context.Context, AcademicTerm *edu.AcademicTerm) (*edu.AcademicTerm, error) {
	err := u.repo.UpdateAcademicTerms(ctx, AcademicTerm)
	if err != nil {
		u.log.Error("failed to update AcademicTerm", zap.Error(err))
		return nil, err
	}

	return AcademicTerm, nil
}

func (u *Usecase) DeleteAcademicTerm(ctx context.Context, id uuid.UUID) error {
	err := u.repo.DeleteAcademicTerms(ctx, id)
	if err != nil {
		u.log.Error("failed to delete AcademicTerm", zap.Error(err))
		return err
	}

	return nil
}

func (u *Usecase) GetAcademicTerms(ctx context.Context, filialID uuid.UUID, size, page int) ([]edu.AcademicTerm, error) {
	AcademicTerms, err := u.repo.GetAcademicTerms(ctx, filialID, pkg.PaginationQuery(page, size))
	if err != nil {
		u.log.Error("failed to get AcademicTerms", zap.Error(err))
		return nil, err
	}

	return AcademicTerms, nil
}
