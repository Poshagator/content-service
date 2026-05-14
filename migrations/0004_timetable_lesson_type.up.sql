BEGIN;

ALTER TABLE public.timetable_entry
    ADD COLUMN IF NOT EXISTS lesson_type varchar;

COMMIT;
