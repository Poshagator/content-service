package repository

import (
	"github.com/poshagator/content-service/internal/domain/repository/postgres"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module("repository",
		fx.Provide(
			postgres.NewRepository,
		),
		fx.Invoke(
			func(lc fx.Lifecycle, a *postgres.Repository) {
				lc.Append(fx.Hook{
					OnStart: a.OnStart,
					OnStop:  a.OnStop,
				})
			},
		),
	)
}
