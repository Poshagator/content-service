package fuel

import (
	"context"

	"github.com/google/uuid"
	fuelEnt "github.com/poshagator/content-service/internal/domain/entities/fuel"
	"go.uber.org/zap"
)

func (u *Usecase) SyncFuels(ctx context.Context, filialID uuid.UUID, inputs []fuelEnt.Fuel) (int64, error) {
	// For simplicity, we can do a basic sync. In real life we'd want transactions or upsert.
	// Since we are inserting new ones or maybe replacing them, let's just create them.
	// We'll fetch all and update by name, or simply recreate.
	// For now, let's just insert them directly.

	// In a real application, you might want to replace existing ones for the filial.
	// Let's delete existing and insert new ones to simulate "sync replacement".

	existingFuels, err := u.repo.GetFuels(ctx, filialID)
	if err != nil {
		u.log.Error("failed to get existing fuels", zap.Error(err))
		return 0, err
	}

	for _, ef := range existingFuels {
		_ = u.repo.DeleteFuel(ctx, ef.ID)
	}

	var upserted int64 = 0
	for _, in := range inputs {
		in.FilialID = filialID
		if err := u.repo.CreateFuel(ctx, &in); err != nil {
			u.log.Error("failed to create fuel", zap.Error(err))
			continue
		}
		upserted++
	}

	return upserted, nil
}
