package edu

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poshagator/content-service/internal/domain/entities/edu"
	"go.uber.org/zap"
)

const qGetTimetable = `
SELECT 
    te.id,
    te.day_of_week,
    COALESCE(te.occurs_on::text, '') AS occurs_on,
    COALESCE(te.effective_from::text, '') AS effective_from,
    COALESCE(te.effective_to::text, '') AS effective_to,
    te.starts_at::text,
    te.ends_at::text,
    te.week_type,
    s.name as subject_name,
    trim(concat_ws(' ', p.last_name, p.first_name, p.middle_name)) as teacher_name,
    COALESCE(r.room_number, r.name, '') as room_name,
    te.is_exam,
    COALESCE(te.comment, '') AS comment
FROM 
    public.timetable_entry te
JOIN
    public.edu_group selected_group ON selected_group.id = $1
JOIN 
    public.edu_subject s ON te.subject_id = s.id
LEFT JOIN 
    public.staff st ON te.teacher_id = st.id
LEFT JOIN 
    public.person p ON st.person_id = p.id
LEFT JOIN 
    public.room r ON te.classroom_id = r.id
WHERE 
    (te.group_id = $1 OR te.group_id = selected_group.parent_group_id)
    AND ($2::uuid IS NULL OR te.term_id = $2)
ORDER BY 
    COALESCE(te.occurs_on, '9999-12-31'::date), te.day_of_week, te.starts_at
`

func (r *Repository) GetTimetable(ctx context.Context, groupID uuid.UUID) ([]edu.TimetableEntry, error) {
	return r.getTimetable(ctx, groupID, nil)
}

func (r *Repository) GetTimetableByTerm(ctx context.Context, groupID uuid.UUID, termID uuid.UUID) ([]edu.TimetableEntry, error) {
	return r.getTimetable(ctx, groupID, termID)
}

func (r *Repository) getTimetable(ctx context.Context, groupID uuid.UUID, termID any) ([]edu.TimetableEntry, error) {
	rows, err := r.db.Query(ctx, qGetTimetable, groupID, termID)
	if err != nil {
		r.log.Error("failed to get timetable", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	entries, err := pgx.CollectRows(rows, pgx.RowToStructByName[edu.TimetableEntry])
	if err != nil {
		r.log.Error("failed to collect timetable entries", zap.Error(err))
		return nil, err
	}

	return entries, nil
}
