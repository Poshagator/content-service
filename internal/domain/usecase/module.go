package usecase

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func New() fx.Option {
	return fx.Module(
		"usecase",
		fx.Provide(
			NewUsecase,
		),
		fx.Decorate(func(log *zap.Logger) *zap.Logger {
			return log.Named("usecase")
		}),
	)
}
