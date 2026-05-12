package filial

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	eduRepo "github.com/poshagator/content-service/internal/domain/repository/postgres/edu"
	productRepo "github.com/poshagator/content-service/internal/domain/repository/postgres/product"
)

type Usecase struct {
	log         *zap.Logger
	eduRepo     *eduRepo.Repository
	productRepo *productRepo.Repository
}

func NewUsecase(
	log *zap.Logger,
	eduRepo *eduRepo.Repository,
	productRepo *productRepo.Repository,
) (*Usecase, error) {
	return &Usecase{
		log:         log.Named("usecase.filial"),
		eduRepo:     eduRepo,
		productRepo: productRepo,
	}, nil
}

type FilialFeatures struct {
	HasMenu     bool `json:"has_menu"`
	HasSchedule bool `json:"has_schedule"`
}

func (u *Usecase) GetFilialFeatures(ctx context.Context, filialID uuid.UUID) (*FilialFeatures, error) {
	hasMenu, err := u.productRepo.HasProducts(ctx, filialID)
	if err != nil {
		return nil, err
	}

	hasSchedule, err := u.eduRepo.HasEduGroups(ctx, filialID)
	if err != nil {
		return nil, err
	}

	return &FilialFeatures{
		HasMenu:     hasMenu,
		HasSchedule: hasSchedule,
	}, nil
}
