package product

import (
	"go.uber.org/zap"

	repo "github.com/poshagator/content-service/internal/domain/repository/postgres/product"
)

type Usecase struct {
	log  *zap.Logger
	repo *repo.Repository
}

func NewUsecase(
	log *zap.Logger,
	repo *repo.Repository,
) (*Usecase, error) {
	return &Usecase{
		log:  log.Named("usecase.product"),
		repo: repo,
	}, nil
}
