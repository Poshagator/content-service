BEGIN;

ALTER TABLE public.timetable_entry
    ADD COLUMN IF NOT EXISTS occurs_on date,
    ADD COLUMN IF NOT EXISTS effective_from date,
    ADD COLUMN IF NOT EXISTS effective_to date,
    ADD COLUMN IF NOT EXISTS source varchar,
    ADD COLUMN IF NOT EXISTS source_event_id bigint,
    ADD COLUMN IF NOT EXISTS is_exam boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS lesson_type varchar,
    ADD COLUMN IF NOT EXISTS comment varchar;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'timetable_entry_source_event_unique'
    ) THEN
        ALTER TABLE public.timetable_entry
            ADD CONSTRAINT timetable_entry_source_event_unique
            UNIQUE (term_id, group_id, source, source_event_id);
    END IF;
END $$;

COMMIT;
