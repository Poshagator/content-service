package action

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/internal/domain/entities/action"
)

const qGetExternalAction = `
SELECT id, filial_id, title, url, created_at, updated_at
FROM public.external_action
WHERE id = $1;
`

func (r *Repository) GetExternalAction(ctx context.Context, id uuid.UUID) (*action.ExternalAction, error) {
	rows, err := r.db.Query(ctx, qGetExternalAction, id)
	if err != nil {
		r.log.Error("failed to get external action", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	dao, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[action.ExternalActionDAO])
	if err != nil {
		r.log.Error("failed to collect external action", zap.Error(err))
		return nil, err
	}
	a := dao.ToExternalAction()

	return a, nil
}

const qGetExternalActions = `
SELECT id, filial_id, title, url, created_at, updated_at
FROM public.external_action
WHERE filial_id = $1
ORDER BY title;
`

func (r *Repository) GetExternalActions(ctx context.Context, filialID uuid.UUID) ([]action.ExternalAction, error) {
	rows, err := r.db.Query(ctx, qGetExternalActions, filialID)
	if err != nil {
		r.log.Error("failed to get external actions", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	daos, err := pgx.CollectRows(rows, pgx.RowToStructByName[action.ExternalActionDAO])
	if err != nil {
		r.log.Error("failed to collect external actions", zap.Error(err))
		return nil, err
	}

	var actions []action.ExternalAction
	for _, dao := range daos {
		actions = append(actions, *dao.ToExternalAction())
	}

	return actions, nil
}

const qCreateExternalAction = `
INSERT INTO public.external_action (
    filial_id, title, url, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id;
`

func (r *Repository) CreateExternalAction(ctx context.Context, a *action.ExternalAction) error {
	err := r.db.QueryRow(
		ctx, qCreateExternalAction,
		a.FilialID, a.Title, a.URL, a.CreatedAt, a.UpdatedAt,
	).Scan(&a.ID)

	if err != nil {
		r.log.Error("failed to create external action", zap.Error(err))
		return err
	}

	return nil
}

const qUpdateExternalAction = `
UPDATE public.external_action
SET filial_id=$1,
    title=$2,
    url=$3,
    updated_at=$4
WHERE id=$5;
`

func (r *Repository) UpdateExternalAction(ctx context.Context, a *action.ExternalAction) error {
	_, err := r.db.Exec(ctx, qUpdateExternalAction,
		a.FilialID, a.Title, a.URL, a.UpdatedAt, a.ID,
	)
	if err != nil {
		r.log.Error("failed to update external action", zap.Error(err))
		return err
	}

	return nil
}

const qDeleteExternalAction = `DELETE FROM public.external_action WHERE id = $1;`

func (r *Repository) DeleteExternalAction(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, qDeleteExternalAction, id)
	if err != nil {
		r.log.Error("failed to delete external action", zap.Error(err))
		return err
	}

	return nil
}
