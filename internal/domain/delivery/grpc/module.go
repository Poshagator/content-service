package grpc

import (
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module("grpc",
		fx.Provide(
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
