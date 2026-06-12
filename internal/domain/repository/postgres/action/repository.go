package action

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/config"
)

type Repository struct {
	log *zap.Logger
	cfg *config.ConfigModel
	ctx context.Context
	db  *pgxpool.Pool
}

func NewRepository(l *zap.Logger, cfg *config.ConfigModel, ctx context.Context, db *pgxpool.Pool) *Repository {
	return &Repository{
		log: l.Named("repo.action"),
		cfg: cfg,
		ctx: ctx,
		db:  db,
	}
}
