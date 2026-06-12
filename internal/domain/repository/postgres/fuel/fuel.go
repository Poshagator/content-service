package fuel

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/internal/domain/entities/fuel"
)

const qGetFuel = `
SELECT id, filial_id, name, price, created_at, updated_at
FROM public.fuel
WHERE id = $1;
`

func (r *Repository) GetFuel(ctx context.Context, id uuid.UUID) (*fuel.Fuel, error) {
	rows, err := r.db.Query(ctx, qGetFuel, id)
	if err != nil {
		r.log.Error("failed to get fuel", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	dao, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[fuel.FuelDAO])
	if err != nil {
		r.log.Error("failed to collect fuel", zap.Error(err))
		return nil, err
	}
	f := dao.ToFuel()

	return f, nil
}

const qGetFuels = `
SELECT id, filial_id, name, price, created_at, updated_at
FROM public.fuel
WHERE filial_id = $1
ORDER BY name;
`

func (r *Repository) GetFuels(ctx context.Context, filialID uuid.UUID) ([]fuel.Fuel, error) {
	rows, err := r.db.Query(ctx, qGetFuels, filialID)
	if err != nil {
		r.log.Error("failed to get fuels", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	daos, err := pgx.CollectRows(rows, pgx.RowToStructByName[fuel.FuelDAO])
	if err != nil {
		r.log.Error("failed to collect fuels", zap.Error(err))
		return nil, err
	}

	var fuels []fuel.Fuel
	for _, dao := range daos {
		fuels = append(fuels, *dao.ToFuel())
	}

	return fuels, nil
}

const qCreateFuel = `
INSERT INTO public.fuel (
    filial_id, name, price, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id;
`

func (r *Repository) CreateFuel(ctx context.Context, f *fuel.Fuel) error {
	err := r.db.QueryRow(
		ctx, qCreateFuel,
		f.FilialID, f.Name, f.Price, f.CreatedAt, f.UpdatedAt,
	).Scan(&f.ID)

	if err != nil {
		r.log.Error("failed to create fuel", zap.Error(err))
		return err
	}

	return nil
}

const qUpdateFuel = `
UPDATE public.fuel
SET filial_id=$1,
    name=$2,
    price=$3,
    updated_at=$4
WHERE id=$5;
`

func (r *Repository) UpdateFuel(ctx context.Context, f *fuel.Fuel) error {
	_, err := r.db.Exec(ctx, qUpdateFuel,
		f.FilialID, f.Name, f.Price, f.UpdatedAt, f.ID,
	)
	if err != nil {
		r.log.Error("failed to update fuel", zap.Error(err))
		return err
	}

	return nil
}

const qDeleteFuel = `DELETE FROM public.fuel WHERE id = $1;`

func (r *Repository) DeleteFuel(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, qDeleteFuel, id)
	if err != nil {
		r.log.Error("failed to delete fuel", zap.Error(err))
		return err
	}

	return nil
}
