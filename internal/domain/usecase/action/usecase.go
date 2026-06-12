package action

import (
	"context"

	"go.uber.org/zap"

	"github.com/poshagator/content-service/config"
	actionRepo "github.com/poshagator/content-service/internal/domain/repository/postgres/action"
)

type Usecase struct {
	log  *zap.Logger
	cfg  *config.ConfigModel
	ctx  context.Context
	repo *actionRepo.Repository
}

func NewUsecase(
	l *zap.Logger,
	cfg *config.ConfigModel,
	ctx context.Context,
	repo *actionRepo.Repository,
) *Usecase {
	return &Usecase{
		log:  l.Named("usecase.action"),
		cfg:  cfg,
		ctx:  ctx,
		repo: repo,
	}
}
