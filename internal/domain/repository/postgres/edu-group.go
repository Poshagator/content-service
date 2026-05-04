package postgres

import (
	"context"
	"edu-service/internal/domain/entities"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const qGetEduGroup = `
SELECT 
    id, institution_id as filial_id, name, permission
from
    public.edu_group
where
    id = $1
`

func (r *Repository) GetEduGroup(ctx context.Context, id uuid.UUID) (*entities.EduGroup, error) {
	rows, err := r.db.Query(ctx, qGetEduGroup, id)
	if err != nil {
		r.log.Error("failed to get EduGroups", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	EduGroups, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.EduGroupDao])
	if err != nil {
		r.log.Error("failed to collect EduGroups", zap.Error(err))
		return nil, err
	}

	return EduGroups.ToEduGroup(), nil
}

const qCreateEduGroups = `
insert into 
    public.edu_group (institution_id, name, permission)
values
	($1, $2, $3)
returning id
`

func (r *Repository) CreateEduGroups(ctx context.Context, EduGroups *entities.EduGroup) error {
	err := r.db.QueryRow(ctx, qCreateEduGroups, EduGroups.FilialID, EduGroups.Name, EduGroups.Permission).Scan(&EduGroups.ID)
	if err != nil {
		r.log.Error("failed to create EduGroups", zap.Error(err))
		return err
	}

	return nil
}

const qUpdateEduGroups = `
UPDATE public.edu_group
SET
    institution_id = $2,
--     CASE
--         WHEN permission = $4 THEN $2
--         ELSE institution_id
--     END,
    name = $3
--     CASE
--         WHEN permission = $4 THEN $3
--         ELSE name
--     END
WHERE id = $1`

func (r *Repository) UpdateEduGroups(ctx context.Context, EduGroups *entities.EduGroup) error {
	_, err := r.db.Exec(ctx, qUpdateEduGroups,
		EduGroups.ID, EduGroups.FilialID, EduGroups.Name)
	if err != nil {
		r.log.Error("failed to update EduGroups", zap.Error(err))
		return err
	}

	return nil
}

const qDeleteEduGroups = `delete from public.edu_group where id = $1 --and permission = $2`

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
    id, institution_id as filial_id, permission, name
from
    public.edu_group
where 
    institution_id = $1
`

func (r *Repository) GetEduGroups(ctx context.Context, filialID uuid.UUID, sortQuery string) ([]entities.EduGroup, error) {
	fmt.Println(filialID, sortQuery)
	rows, err := r.db.Query(ctx, qGetEduGroups+sortQuery, filialID)
	if err != nil {
		r.log.Error("failed to get EduGroupss", zap.Error(err))
		return nil, err
	}

	EduGroupsDAOs, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.EduGroupDao])
	if err != nil {
		r.log.Error("failed to collect EduGroupss", zap.Error(err))
		return nil, err
	}

	return entities.EduGroupsDao(EduGroupsDAOs).ToEduGroups(), nil
}
