package repository

import (
	"github.com/poshagator/content-service/internal/domain/repository/postgres"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module("repository",
		postgres.New(),
	)
}
