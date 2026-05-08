package grpc

import (
	"crypto/tls"
	"fmt"
	"github.com/poshagator/content-service/config"
	"github.com/poshagator/content-service/pkg/proto/planner/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"strings"
)

func NewPlannerClient(cfg *config.ConfigModel) (planner.PlannerServiceClient, error) {
	addr := cfg.PlannerGRPC.Host
	if cfg.PlannerGRPC.Port != "" {
		addr = fmt.Sprintf("%s:%s", cfg.PlannerGRPC.Host, cfg.PlannerGRPC.Port)
	}

	var opts []grpc.DialOption
	if strings.Contains(addr, ":443") {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial planner service: %w", err)
	}

	return planner.NewPlannerServiceClient(conn), nil
}
