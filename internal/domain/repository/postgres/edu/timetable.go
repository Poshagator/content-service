package edu

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"github.com/poshagator/content-service/internal/domain/entities/edu"
)

const qGetTimetable = `
SELECT 
    te.id,
    te.day_of_week,
    te.starts_at::text,
    te.ends_at::text,
    te.week_type,
    s.name as subject_name,
    trim(concat_ws(' ', p.last_name, p.first_name, p.middle_name)) as teacher_name,
    COALESCE(r.room_number, r.name) as room_name
FROM 
    public.timetable_entry te
JOIN 
    public.edu_subject s ON te.subject_id = s.id
LEFT JOIN 
    public.staff st ON te.teacher_id = st.id
LEFT JOIN 
    public.person p ON st.person_id = p.id
LEFT JOIN 
    public.room r ON te.classroom_id = r.id
WHERE 
    te.group_id = $1
ORDER BY 
    te.day_of_week, te.starts_at
`

func (r *Repository) GetTimetable(ctx context.Context, groupID uuid.UUID) ([]edu.TimetableEntry, error) {
	rows, err := r.db.Query(ctx, qGetTimetable, groupID)
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
