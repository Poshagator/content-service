BEGIN;

ALTER TABLE public.timetable_entry
    DROP CONSTRAINT IF EXISTS timetable_entry_source_event_unique,
    DROP COLUMN IF EXISTS comment,
    DROP COLUMN IF EXISTS lesson_type,
    DROP COLUMN IF EXISTS is_exam,
    DROP COLUMN IF EXISTS source_event_id,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS effective_to,
    DROP COLUMN IF EXISTS effective_from,
    DROP COLUMN IF EXISTS occurs_on;

COMMIT;
