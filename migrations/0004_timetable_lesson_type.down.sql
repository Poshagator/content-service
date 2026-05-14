BEGIN;

ALTER TABLE public.timetable_entry
    DROP COLUMN IF EXISTS lesson_type;

COMMIT;
