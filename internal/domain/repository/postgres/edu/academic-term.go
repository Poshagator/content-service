package postgres

import (
	"context"
	"github.com/poshagator/content-service/internal/domain/entities"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const qGetAcademicTerm = `
SELECT 
    id, institution_id as filial_id, name, starts_on, ends_on, week_start
from
    public.academic_term
where
    id = $1
`

func (r *Repository) GetAcademicTerm(ctx context.Context, id uuid.UUID) (*entities.AcademicTerm, error) {
	rows, err := r.db.Query(ctx, qGetAcademicTerm, id)
	if err != nil {
		r.log.Error("failed to get AcademicTerms", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	AcademicTerms, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.AcademicTermDao])
	if err != nil {
		r.log.Error("failed to collect AcademicTerms", zap.Error(err))
		return nil, err
	}

	return AcademicTerms.ToAcademicTerm(), nil
}

const qCreateAcademicTerms = `
insert into 
    public.academic_term (institution_id, name, starts_on, ends_on, week_start)
values
	($1, $2, $3, $4, $5)
returning id
`

func (r *Repository) CreateAcademicTerms(ctx context.Context, AcademicTerms *entities.AcademicTerm) error {
	err := r.db.QueryRow(ctx, qCreateAcademicTerms, AcademicTerms.FilialID, AcademicTerms.Name,
		AcademicTerms.StartsOn, AcademicTerms.EndsOn, AcademicTerms.WeekStart).Scan(&AcademicTerms.ID)
	if err != nil {
		r.log.Error("failed to create AcademicTerms", zap.Error(err))
		return err
	}

	return nil
}

const qUpdateAcademicTerms = `
UPDATE public.academic_term
SET
    institution_id = $2,
--     CASE
--         WHEN permission = $4 THEN $2
--         ELSE institution_id
--     END,
    name = $3,
--     CASE
--         WHEN permission = $4 THEN $3
--         ELSE name
--     END
    starts_on = $4,
    ends_on = $5,
    week_start = $6
WHERE id = $1`

func (r *Repository) UpdateAcademicTerms(ctx context.Context, AcademicTerms *entities.AcademicTerm) error {
	_, err := r.db.Exec(ctx, qUpdateAcademicTerms,
		AcademicTerms.ID,
		AcademicTerms.FilialID, AcademicTerms.Name, AcademicTerms.StartsOn,
		AcademicTerms.EndsOn, AcademicTerms.WeekStart)
	if err != nil {
		r.log.Error("failed to update AcademicTerms", zap.Error(err))
		return err
	}

	return nil
}

const qDeleteAcademicTerms = `delete from public.academic_term where id = $1 --and permission = $2`

func (r *Repository) DeleteAcademicTerms(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, qDeleteAcademicTerms, id)
	if err != nil {
		r.log.Error("failed to delete AcademicTerms", zap.Error(err))
		return err
	}

	return nil
}

const qGetAcademicTerms = `
select 
    id, institution_id as filial_id, name, starts_on, ends_on, week_start
from
    public.academic_term
where 
    institution_id = $1
`

func (r *Repository) GetAcademicTerms(ctx context.Context, filialID uuid.UUID, sortQuery string) ([]entities.AcademicTerm, error) {
	fmt.Println(filialID, sortQuery)
	rows, err := r.db.Query(ctx, qGetAcademicTerms+sortQuery, filialID)
	if err != nil {
		r.log.Error("failed to get AcademicTermss", zap.Error(err))
		return nil, err
	}

	AcademicTermsDAOs, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.AcademicTermDao])
	if err != nil {
		r.log.Error("failed to collect AcademicTermss", zap.Error(err))
		return nil, err
	}

	return entities.AcademicTermsDao(AcademicTermsDAOs).ToAcademicTerms(), nil
}
