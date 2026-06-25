package action

import (
	"context"

	"github.com/google/uuid"
	actionEnt "github.com/poshagator/content-service/internal/domain/entities/action"
)

func (u *Usecase) GetExternalActions(ctx context.Context, filialID uuid.UUID) ([]actionEnt.ExternalAction, error) {
	return u.repo.GetExternalActions(ctx, filialID)
}
