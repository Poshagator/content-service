package app

import (
	"context"
	"github.com/poshagator/content-service/config"
	"github.com/poshagator/content-service/internal/domain/delivery/grpc"
	"github.com/poshagator/content-service/internal/domain/delivery/http"
	"github.com/poshagator/content-service/internal/domain/repository/postgres"
	"github.com/poshagator/content-service/internal/domain/usecase"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

func New() *fx.App {
	return fx.New(
		fx.Options(
			postgres.New(),
			usecase.New(),
			http.New(),
			grpc.New(),
		),
		fx.Provide(
			context.Background,
			config.NewConfig,
			zap.NewDevelopment,
		),
		fx.WithLogger(
			func(log *zap.Logger) fxevent.Logger {
				return &fxevent.ZapLogger{Logger: log}
			},
		),
	)
}
