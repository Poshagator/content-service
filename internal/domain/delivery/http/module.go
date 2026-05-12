package http

import (
	"go.uber.org/fx"
	"github.com/poshagator/content-service/internal/domain/delivery/http/edu"
	"github.com/poshagator/content-service/internal/domain/delivery/http/product"
	"github.com/poshagator/content-service/internal/domain/delivery/http/filial"
)

func New() fx.Option {
	return fx.Module("http",
		fx.Provide(
			edu.NewHandler,
			product.NewHandler,
			filial.NewHandler,
			NewServer,
		),
		fx.Invoke(func(lc fx.Lifecycle, s *Server) {
			lc.Append(fx.Hook{
				OnStart: s.OnStart,
				OnStop:  s.OnStop,
			})
		}),
	)
}
