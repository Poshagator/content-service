BEGIN;

ALTER TABLE public.edu_group
    ADD COLUMN IF NOT EXISTS source varchar,
    ADD COLUMN IF NOT EXISTS source_group_id int,
    ADD COLUMN IF NOT EXISTS faculty_id int,
    ADD COLUMN IF NOT EXISTS faculty_name varchar,
    ADD COLUMN IF NOT EXISTS course_id int,
    ADD COLUMN IF NOT EXISTS course_name varchar,
    ADD COLUMN IF NOT EXISTS study_form_id int,
    ADD COLUMN IF NOT EXISTS study_form_name varchar,
    ADD COLUMN IF NOT EXISTS education_level varchar,
    ADD COLUMN IF NOT EXISTS is_magistracy boolean NOT NULL DEFAULT false;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'edu_group_source_unique'
    ) THEN
        ALTER TABLE public.edu_group
            ADD CONSTRAINT edu_group_source_unique
            UNIQUE (filial_id, source, source_group_id);
    END IF;
END $$;

COMMIT;
