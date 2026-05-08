package grpc

import (
	"fmt"
	"github.com/poshagator/content-service/config"
	"github.com/poshagator/content-service/pkg/proto/planner/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewPlannerClient(cfg *config.ConfigModel) (planner.PlannerServiceClient, error) {
	addr := fmt.Sprintf("%s:%s", cfg.PlannerGRPC.Host, cfg.PlannerGRPC.Port)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial planner service: %w", err)
	}

	return planner.NewPlannerServiceClient(conn), nil
}
