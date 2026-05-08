package edu

import (
	"go.uber.org/zap"
	"github.com/poshagator/content-service/internal/domain/repository/postgres/edu"
	"github.com/poshagator/content-service/pkg/proto/planner/gen"
)

type Usecase struct {
	log           *zap.Logger
	repo          *edu.Repository
	plannerClient planner.PlannerServiceClient
}

func NewUsecase(
	log *zap.Logger,
	repo *edu.Repository,
	plannerClient planner.PlannerServiceClient,
) (*Usecase, error) {
	return &Usecase{
		log:           log.Named("usecase.edu"),
		repo:          repo,
		plannerClient: plannerClient,
	}, nil
}
