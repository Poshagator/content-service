package action

import (
	"context"

	"github.com/google/uuid"
	actionEnt "github.com/poshagator/content-service/internal/domain/entities/action"
	"go.uber.org/zap"
)

func (u *Usecase) SyncExternalActions(ctx context.Context, filialID uuid.UUID, inputs []actionEnt.ExternalAction) (int64, error) {
	existing, err := u.repo.GetExternalActions(ctx, filialID)
	if err != nil {
		u.log.Error("failed to get existing external actions", zap.Error(err))
		return 0, err
	}

	for _, e := range existing {
		_ = u.repo.DeleteExternalAction(ctx, e.ID)
	}

	var upserted int64 = 0
	for _, in := range inputs {
		in.FilialID = filialID
		if err := u.repo.CreateExternalAction(ctx, &in); err != nil {
			u.log.Error("failed to create external action", zap.Error(err))
			continue
		}
		upserted++
	}

	return upserted, nil
}
