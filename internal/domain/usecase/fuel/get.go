package fuel

import (
	"context"

	"github.com/google/uuid"
	fuelEnt "github.com/poshagator/content-service/internal/domain/entities/fuel"
)

func (u *Usecase) GetFuels(ctx context.Context, filialID uuid.UUID) ([]fuelEnt.Fuel, error) {
	return u.repo.GetFuels(ctx, filialID)
}
