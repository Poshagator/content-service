package edu

import (
	"go.uber.org/zap"
	"github.com/poshagator/content-service/internal/domain/repository/postgres/edu"
)

type Usecase struct {
	log  *zap.Logger
	repo *edu.Repository
}

func NewUsecase(
	log *zap.Logger,
	repo *edu.Repository,
) (*Usecase, error) {
	return &Usecase{
		log:  log.Named("usecase.edu"),
		repo: repo,
	}, nil
}
