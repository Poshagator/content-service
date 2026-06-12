package fuel

import (
	"context"

	"go.uber.org/zap"

	"github.com/poshagator/content-service/config"
	fuelRepo "github.com/poshagator/content-service/internal/domain/repository/postgres/fuel"
)

type Usecase struct {
	log  *zap.Logger
	cfg  *config.ConfigModel
	ctx  context.Context
	repo *fuelRepo.Repository
}

func NewUsecase(
	l *zap.Logger,
	cfg *config.ConfigModel,
	ctx context.Context,
	repo *fuelRepo.Repository,
) *Usecase {
	return &Usecase{
		log:  l.Named("usecase.fuel"),
		cfg:  cfg,
		ctx:  ctx,
		repo: repo,
	}
}
