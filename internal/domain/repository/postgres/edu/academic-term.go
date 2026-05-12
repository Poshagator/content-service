package edu

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poshagator/content-service/internal/domain/entities/edu"
	"go.uber.org/zap"
)

const qGetAcademicTerm = `
SELECT 
    id, filial_id, name, starts_on, ends_on, week_start
from
    public.academic_term
where
    id = $1
`

func (r *Repository) GetAcademicTerm(ctx context.Context, id uuid.UUID) (*edu.AcademicTerm, error) {
	rows, err := r.db.Query(ctx, qGetAcademicTerm, id)
	if err != nil {
		r.log.Error("failed to get AcademicTerms", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	AcademicTerms, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[edu.AcademicTermDao])
	if err != nil {
		r.log.Error("failed to collect AcademicTerms", zap.Error(err))
		return nil, err
	}

	return AcademicTerms.ToAcademicTerm(), nil
}

const qCreateAcademicTerms = `
insert into 
    public.academic_term (filial_id, name, starts_on, ends_on, week_start)
values
	($1, $2, $3, $4, $5)
returning id
`

func (r *Repository) CreateAcademicTerms(ctx context.Context, AcademicTerms *edu.AcademicTerm) error {
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
    filial_id = $2,
    name = $3,
    starts_on = $4,
    ends_on = $5,
    week_start = $6
 WHERE id = $1`

func (r *Repository) UpdateAcademicTerms(ctx context.Context, AcademicTerms *edu.AcademicTerm) error {
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

const qDeleteAcademicTerms = `delete from public.academic_term where id = $1`

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
    id, filial_id, name, starts_on, ends_on, week_start
from
    public.academic_term
where 
    filial_id = $1
`

func (r *Repository) GetAcademicTerms(ctx context.Context, filialID uuid.UUID, sortQuery string) ([]edu.AcademicTerm, error) {
	fmt.Println(filialID, sortQuery)
	rows, err := r.db.Query(ctx, qGetAcademicTerms+sortQuery, filialID)
	if err != nil {
		r.log.Error("failed to get AcademicTermss", zap.Error(err))
		return nil, err
	}

	AcademicTermsDAOs, err := pgx.CollectRows(rows, pgx.RowToStructByName[edu.AcademicTermDao])
	if err != nil {
		r.log.Error("failed to collect AcademicTermss", zap.Error(err))
		return nil, err
	}

	return edu.AcademicTermsDao(AcademicTermsDAOs).ToAcademicTerms(), nil
}

const qGetCurrentAcademicTermForGroup = `
SELECT
    at.id, at.filial_id, at.name, at.starts_on, at.ends_on, at.week_start
FROM
    public.academic_term at
JOIN
    public.edu_group selected_group ON selected_group.id = $1
WHERE
    EXISTS (
        SELECT 1
        FROM public.timetable_entry te
        WHERE te.term_id = at.id
          AND (te.group_id = $1 OR te.group_id = selected_group.parent_group_id)
    )
ORDER BY
    CASE
        WHEN at.starts_on <= CURRENT_DATE AND at.ends_on >= CURRENT_DATE THEN 0
        WHEN at.starts_on > CURRENT_DATE THEN 1
        ELSE 2
    END,
    CASE WHEN at.starts_on > CURRENT_DATE THEN at.starts_on END ASC,
    at.ends_on DESC
LIMIT 1
`

func (r *Repository) GetCurrentAcademicTermForGroup(ctx context.Context, groupID uuid.UUID) (*edu.AcademicTerm, error) {
	rows, err := r.db.Query(ctx, qGetCurrentAcademicTermForGroup, groupID)
	if err != nil {
		r.log.Error("failed to get current AcademicTerm for group", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	term, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[edu.AcademicTermDao])
	if err != nil {
		r.log.Error("failed to collect current AcademicTerm for group", zap.Error(err))
		return nil, err
	}

	return term.ToAcademicTerm(), nil
}
