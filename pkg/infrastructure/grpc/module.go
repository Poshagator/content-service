package grpc

import "go.uber.org/fx"

func New() fx.Option {
	return fx.Provide(
		NewPlannerClient,
	)
}
