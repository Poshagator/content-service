package edu

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"github.com/poshagator/content-service/internal/domain/entities/edu"
)

const qGetEduGroup = `
SELECT 
    id, filial_id, name
from
    public.edu_group
where
    id = $1
`

func (r *Repository) GetEduGroup(ctx context.Context, id uuid.UUID) (*edu.EduGroup, error) {
	rows, err := r.db.Query(ctx, qGetEduGroup, id)
	if err != nil {
		r.log.Error("failed to get EduGroups", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	EduGroups, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[edu.EduGroupDao])
	if err != nil {
		r.log.Error("failed to collect EduGroups", zap.Error(err))
		return nil, err
	}

	return EduGroups.ToEduGroup(), nil
}

const qCreateEduGroups = `
insert into 
    public.edu_group (filial_id, name)
values
	($1, $2)
returning id
`

func (r *Repository) CreateEduGroups(ctx context.Context, EduGroups *edu.EduGroup) error {
	err := r.db.QueryRow(ctx, qCreateEduGroups, EduGroups.FilialID, EduGroups.Name).Scan(&EduGroups.ID)
	if err != nil {
		r.log.Error("failed to create EduGroups", zap.Error(err))
		return err
	}

	return nil
}

const qUpdateEduGroups = `
UPDATE public.edu_group
SET
    filial_id = $2,
    name = $3
 WHERE id = $1`

func (r *Repository) UpdateEduGroups(ctx context.Context, EduGroups *edu.EduGroup) error {
	_, err := r.db.Exec(ctx, qUpdateEduGroups,
		EduGroups.ID, EduGroups.FilialID, EduGroups.Name)
	if err != nil {
		r.log.Error("failed to update EduGroups", zap.Error(err))
		return err
	}

	return nil
}

const qDeleteEduGroups = `delete from public.edu_group where id = $1`

func (r *Repository) DeleteEduGroups(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, qDeleteEduGroups, id)
	if err != nil {
		r.log.Error("failed to delete EduGroups", zap.Error(err))
		return err
	}

	return nil
}

const qGetEduGroups = `
select 
    id, filial_id, name
from
    public.edu_group
where 
    filial_id = $1
`

func (r *Repository) GetEduGroups(ctx context.Context, filialID uuid.UUID, sortQuery string) ([]edu.EduGroup, error) {
	fmt.Println(filialID, sortQuery)
	rows, err := r.db.Query(ctx, qGetEduGroups+sortQuery, filialID)
	if err != nil {
		r.log.Error("failed to get EduGroupss", zap.Error(err))
		return nil, err
	}

	EduGroupsDAOs, err := pgx.CollectRows(rows, pgx.RowToStructByName[edu.EduGroupDao])
	if err != nil {
		r.log.Error("failed to collect EduGroupss", zap.Error(err))
		return nil, err
	}

	return edu.EduGroupsDao(EduGroupsDAOs).ToEduGroups(), nil
}

const qSearchEduGroups = `
select 
    id, filial_id, name
from
    public.edu_group
where 
    filial_id = $1 AND name ILIKE $2
order by name
limit $3
`

func (r *Repository) SearchEduGroups(ctx context.Context, filialID uuid.UUID, name string, limit int) ([]edu.EduGroup, error) {
	rows, err := r.db.Query(ctx, qSearchEduGroups, filialID, "%"+name+"%", limit)
	if err != nil {
		r.log.Error("failed to search edu groups", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	daos, err := pgx.CollectRows(rows, pgx.RowToStructByName[edu.EduGroupDao])
	if err != nil {
		r.log.Error("failed to collect searched edu groups", zap.Error(err))
		return nil, err
	}

	return edu.EduGroupsDao(daos).ToEduGroups(), nil
}
