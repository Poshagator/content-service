BEGIN;

ALTER TABLE public.edu_group
    DROP CONSTRAINT IF EXISTS edu_group_source_unique,
    DROP COLUMN IF EXISTS is_magistracy,
    DROP COLUMN IF EXISTS education_level,
    DROP COLUMN IF EXISTS study_form_name,
    DROP COLUMN IF EXISTS study_form_id,
    DROP COLUMN IF EXISTS course_name,
    DROP COLUMN IF EXISTS course_id,
    DROP COLUMN IF EXISTS faculty_name,
    DROP COLUMN IF EXISTS faculty_id,
    DROP COLUMN IF EXISTS source_group_id,
    DROP COLUMN IF EXISTS source;

COMMIT;
