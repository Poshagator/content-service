package postgres

import (
	"context"

	"github.com/poshagator/content-service/config"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/internal/domain/repository/postgres/edu"
	"github.com/poshagator/content-service/internal/domain/repository/postgres/product"
)

func New() fx.Option {
	return fx.Module("repo",
		fx.Provide(
			NewPoolManager,
			func(pm *PoolManager, l *zap.Logger, cfg *config.ConfigModel, ctx context.Context) *edu.Repository {
				return edu.NewRepository(l, cfg, ctx, pm.GetPool())
			},
			func(pm *PoolManager, l *zap.Logger, cfg *config.ConfigModel, ctx context.Context) *product.Repository {
				return product.NewRepository(l, cfg, ctx, pm.GetPool())
			},
		),
		fx.Invoke(func(lc fx.Lifecycle, pm *PoolManager) {
			lc.Append(fx.Hook{
				OnStart: pm.OnStart,
				OnStop:  pm.OnStop,
			})
		}),
	)
}
