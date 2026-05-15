package grpc

import (
	grpcproduct "github.com/poshagator/content-service/internal/domain/delivery/grpc/product"
	"github.com/poshagator/content-service/internal/domain/delivery/grpc/schedule"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module("grpc",
		fx.Provide(
			grpcproduct.NewHandler,
			schedule.NewHandler,
			NewServer,
		),
		fx.Invoke(
			func(lc fx.Lifecycle, s *Server) {
				lc.Append(fx.Hook{
					OnStart: s.OnStart,
					OnStop:  s.OnStop,
				})
			},
		),
	)
}
