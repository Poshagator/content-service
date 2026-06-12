package postgres

import (
	"context"

	"github.com/poshagator/content-service/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/internal/domain/repository/postgres/action"
	"github.com/poshagator/content-service/internal/domain/repository/postgres/edu"
	"github.com/poshagator/content-service/internal/domain/repository/postgres/fuel"
	"github.com/poshagator/content-service/internal/domain/repository/postgres/product"
)

func New() fx.Option {
	return fx.Module("repo",
		fx.Provide(
			NewPool,
			func(db *pgxpool.Pool, l *zap.Logger, cfg *config.ConfigModel, ctx context.Context) *edu.Repository {
				return edu.NewRepository(l, cfg, ctx, db)
			},
			func(db *pgxpool.Pool, l *zap.Logger, cfg *config.ConfigModel, ctx context.Context) *product.Repository {
				return product.NewRepository(l, cfg, ctx, db)
			},
			func(db *pgxpool.Pool, l *zap.Logger, cfg *config.ConfigModel, ctx context.Context) *fuel.Repository {
				return fuel.NewRepository(l, cfg, ctx, db)
			},
			func(db *pgxpool.Pool, l *zap.Logger, cfg *config.ConfigModel, ctx context.Context) *action.Repository {
				return action.NewRepository(l, cfg, ctx, db)
			},
		),
	)
}
