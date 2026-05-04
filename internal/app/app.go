package app

import (
	"context"
	"content-service/config"
	"content-service/internal/domain/delivery/http"
	"content-service/internal/domain/repository"
	"content-service/internal/domain/usecase"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	orderclient "content-service/internal/domain/clients/order/grpc"
)

func New() *fx.App {
	return fx.New(
		fx.Options(
			repository.New(),
			usecase.New(),
			http.New(),
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
