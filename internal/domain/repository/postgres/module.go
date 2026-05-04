package postgres

import (
	"context"

	"github.com/poshagator/content-service/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func New() fx.Option {
	return fx.Module("repo",
		fx.Provide(
			func(l *zap.Logger, cfg *config.ConfigModel, ctx context.Context) (*Repository, error) {
				return NewRepository(l, cfg, ctx)
			},
		),
		fx.Invoke(func(lc fx.Lifecycle, repo *Repository) {
			lc.Append(fx.Hook{
				OnStart: repo.OnStart,
				OnStop:  repo.OnStop,
			})
		}),
	)
}
