package edu

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poshagator/content-service/internal/domain/entities/edu"
	"go.uber.org/zap"
)

const qGetEduGroup = `
SELECT 
    id, filial_id, parent_group_id, name, source, source_group_id, source_subgroup_id, is_subgroup, faculty_id, faculty_name,
    course_id, course_name, study_form_id, study_form_name, education_level, is_magistracy
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
    public.edu_group (
        filial_id, parent_group_id, name, source, source_group_id, source_subgroup_id, is_subgroup, faculty_id, faculty_name,
        course_id, course_name, study_form_id, study_form_name, education_level, is_magistracy
    )
values
	($1, NULLIF($2, '00000000-0000-0000-0000-000000000000'::uuid), $3, NULLIF($4, ''), NULLIF($5, 0), NULLIF($6, 0), $7, NULLIF($8, 0), NULLIF($9, ''),
     NULLIF($10, 0), NULLIF($11, ''), NULLIF($12, 0), NULLIF($13, ''), NULLIF($14, ''), $15)
returning id
`

func (r *Repository) CreateEduGroups(ctx context.Context, EduGroups *edu.EduGroup) error {
	err := r.db.QueryRow(ctx, qCreateEduGroups,
		EduGroups.FilialID, EduGroups.ParentGroupID, EduGroups.Name, EduGroups.Source, EduGroups.SourceGroupID, EduGroups.SourceSubgroupID, EduGroups.IsSubgroup,
		EduGroups.FacultyID, EduGroups.FacultyName, EduGroups.CourseID, EduGroups.CourseName,
		EduGroups.StudyFormID, EduGroups.StudyFormName, EduGroups.EducationLevel, EduGroups.IsMagistracy).Scan(&EduGroups.ID)
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
    parent_group_id = NULLIF($3, '00000000-0000-0000-0000-000000000000'::uuid),
    name = $4,
    source = NULLIF($5, ''),
    source_group_id = NULLIF($6, 0),
    source_subgroup_id = NULLIF($7, 0),
    is_subgroup = $8,
    faculty_id = NULLIF($9, 0),
    faculty_name = NULLIF($10, ''),
    course_id = NULLIF($11, 0),
    course_name = NULLIF($12, ''),
    study_form_id = NULLIF($13, 0),
    study_form_name = NULLIF($14, ''),
    education_level = NULLIF($15, ''),
    is_magistracy = $16,
    updated_at = now()
 WHERE id = $1`

func (r *Repository) UpdateEduGroups(ctx context.Context, EduGroups *edu.EduGroup) error {
	_, err := r.db.Exec(ctx, qUpdateEduGroups,
		EduGroups.ID, EduGroups.FilialID, EduGroups.ParentGroupID, EduGroups.Name, EduGroups.Source, EduGroups.SourceGroupID, EduGroups.SourceSubgroupID, EduGroups.IsSubgroup,
		EduGroups.FacultyID, EduGroups.FacultyName, EduGroups.CourseID, EduGroups.CourseName,
		EduGroups.StudyFormID, EduGroups.StudyFormName, EduGroups.EducationLevel, EduGroups.IsMagistracy)
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
    id, filial_id, parent_group_id, name, source, source_group_id, source_subgroup_id, is_subgroup, faculty_id, faculty_name,
    course_id, course_name, study_form_id, study_form_name, education_level, is_magistracy
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
    id, filial_id, parent_group_id, name, source, source_group_id, source_subgroup_id, is_subgroup, faculty_id, faculty_name,
    course_id, course_name, study_form_id, study_form_name, education_level, is_magistracy
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

func (r *Repository) HasEduGroups(ctx context.Context, filialID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM public.edu_group WHERE filial_id = $1)", filialID).Scan(&exists)
	if err != nil {
		r.log.Error("failed to check edu groups existence", zap.Error(err))
		return false, err
	}
	return exists, nil
}
