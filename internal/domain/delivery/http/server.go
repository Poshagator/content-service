package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/config"
	"github.com/poshagator/content-service/internal/domain/delivery/http/edu"
	"github.com/poshagator/content-service/internal/domain/delivery/http/product"
	"github.com/poshagator/content-service/internal/domain/delivery/http/filial"
)

type Server struct {
	logger         *zap.Logger
	cfg            *config.ConfigModel
	serv           *gin.Engine
	productHandler *product.Handler
	eduHandler     *edu.Handler
	filialHandler  *filial.Handler
}

func NewServer(
	logger *zap.Logger,
	cfg *config.ConfigModel,
	ph *product.Handler,
	eh *edu.Handler,
	fh *filial.Handler,
) (*Server, error) {
	return &Server{
		logger:         logger,
		cfg:            cfg,
		serv:           gin.Default(),
		productHandler: ph,
		eduHandler:     eh,
		filialHandler:  fh,
	}, nil
}

func (s *Server) OnStart(_ context.Context) error {
	s.createController()
	s.createFilialController()

	go func() {
		addr := s.cfg.HTTP.Host + ":" + s.cfg.HTTP.Port
		s.logger.Info("HTTP server started", zap.String("addr", addr))
		if err := s.serv.Run(addr); err != nil {
			s.logger.Error("HTTP server exited", zap.Error(err))
		}
	}()
	return nil
}

func (s *Server) OnStop(_ context.Context) error {
	s.logger.Info("HTTP server stopped")
	return nil
}
