package usecase

import (
	"go.uber.org/fx"

	"github.com/poshagator/content-service/internal/domain/usecase/action"
	"github.com/poshagator/content-service/internal/domain/usecase/edu"
	"github.com/poshagator/content-service/internal/domain/usecase/filial"
	"github.com/poshagator/content-service/internal/domain/usecase/fuel"
	"github.com/poshagator/content-service/internal/domain/usecase/product"
)

func New() fx.Option {
	return fx.Module(
		"usecase",
		fx.Provide(
			edu.NewUsecase,
			product.NewUsecase,
			filial.NewUsecase,
			fuel.NewUsecase,
			action.NewUsecase,
		),
	)
}
