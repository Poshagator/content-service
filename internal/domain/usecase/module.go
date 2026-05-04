package usecase

import (
	"go.uber.org/fx"

	"github.com/poshagator/content-service/internal/domain/usecase/edu"
	"github.com/poshagator/content-service/internal/domain/usecase/product"
)

func New() fx.Option {
	return fx.Module(
		"usecase",
		fx.Provide(
			edu.NewUsecase,
			product.NewUsecase,
		),
	)
}
