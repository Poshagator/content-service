package grpc

import (
	"context"
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/poshagator/content-service/config"
	"github.com/poshagator/content-service/internal/domain/delivery/grpc/schedule"
	"github.com/poshagator/content-service/internal/domain/usecase/edu"
	"github.com/poshagator/content-service/internal/domain/usecase/product"
	pkgSchedule "github.com/poshagator/content-service/pkg/proto/schedule/gen"
)

type Server struct {
	logger          *zap.Logger
	cfg             *config.ConfigModel
	RPC             *grpc.Server
	productUsecase  *product.Usecase
	eduUsecase      *edu.Usecase
	scheduleHandler *schedule.Handler
}

func NewServer(
	logger *zap.Logger,
	cfg *config.ConfigModel,
	pu *product.Usecase,
	eu *edu.Usecase,
	sh *schedule.Handler,
) (*Server, error) {
	s := &Server{
		logger:          logger,
		cfg:             cfg,
		RPC:             grpc.NewServer(),
		productUsecase:  pu,
		eduUsecase:      eu,
		scheduleHandler: sh,
	}

	pkgSchedule.RegisterScheduleServiceServer(s.RPC, s.scheduleHandler)

	return s, nil
}

func (s *Server) OnStart(_ context.Context) error {
	lis, err := net.Listen("tcp", s.cfg.GRPC.Host+":"+s.cfg.GRPC.Port)
	if err != nil {
		s.logger.Error("failed to listen: ", zap.Error(err))
		return fmt.Errorf("failed to listen:  %w", err)
	}
	reflection.Register(s.RPC)
	go func() {
		s.logger.Debug("grps serv started")
		if err = s.RPC.Serve(lis); err != nil {
			s.logger.Error("failed to serve: " + err.Error())
		}
		return
	}()
	return nil
}

func (s *Server) OnStop(_ context.Context) error {
	s.logger.Debug("stop grps")
	s.RPC.GracefulStop()
	return nil
}
