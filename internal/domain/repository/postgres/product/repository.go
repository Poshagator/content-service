package product

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"github.com/poshagator/content-service/config"
)

type Repository struct {
	ctx context.Context
	log *zap.Logger
	cfg *config.ConfigModel
	db  *pgxpool.Pool
}

func NewRepository(l *zap.Logger, cfg *config.ConfigModel, ctx context.Context, db *pgxpool.Pool) *Repository {
	return &Repository{
		ctx: ctx,
		log: l.Named("repo.product"),
		cfg: cfg,
		db:  db,
	}
}
