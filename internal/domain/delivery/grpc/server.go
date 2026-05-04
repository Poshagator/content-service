package grpc

import (
	"product-service/config"
	"product-service/internal/domain/usecase"
	//protos "product-service/pkg/proto/auth/gen/go"
	"context"
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
)

const statusOK = `OK`

type Server struct {
	logger  *zap.Logger
	cfg     *config.ConfigModel
	RPC     *grpc.Server
	Usecase *usecase.Usecase
	//protos.UnimplementedAuthServiceServer
}

func NewServer(logger *zap.Logger, cfg *config.ConfigModel, uc *usecase.Usecase) (*Server, error) {
	return &Server{
		logger:  logger,
		cfg:     cfg,
		RPC:     grpc.NewServer(),
		Usecase: uc,
	}, nil
}

func (s *Server) OnStart(_ context.Context) error {
	lis, err := net.Listen("tcp", s.cfg.GRPC.Host+":"+s.cfg.GRPC.Port)
	if err != nil {
		s.logger.Error("failed to listen: ", zap.Error(err))
		return fmt.Errorf("failed to listen:  %w", err)
	}
	//protos.RegisterAuthServiceServer(s.RPC, s)
	reflection.Register(s.RPC) //по сети теперь видно все методы сети
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

//пример
//func (s *Server) GetUserToken(ctx context.Context, request *protos.GetUserTokenRequest) (*protos.GetUserTokenResponse, error) {
//	token, err := s.Usecase.GetUserToken(
//		ctx,
//		convertToUserEntity(
//			"",
//			request.GetLogin(),
//			nil,
//			0,
//			"",
//		),
//		request.GetPassword(),
//	)
//	if err != nil {
//		return nil, err
//	}
//	return &protos.GetUserTokenResponse{
//		Token:  token,
//	}, nil
//}
