BEGIN;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

------------------------------------------------------------------
-- 1. БАЗОВЫЕ СУЩНОСТИ
------------------------------------------------------------------
CREATE TABLE filial (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4()
);

CREATE TABLE media_file (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    url         varchar NOT NULL,
    file_type   varchar NOT NULL DEFAULT 'IMAGE',
    mime_type   varchar,
    width_px    int,
    height_px   int,
    size_bytes  bigint,
    created_at  timestamp NOT NULL DEFAULT now(),
    updated_at  timestamp NOT NULL DEFAULT now()
);

------------------------------------------------------------------
-- 2. PRODUCTS (из product-service)
------------------------------------------------------------------
CREATE TABLE product_category (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    filial_id   uuid NOT NULL REFERENCES filial(id) ON DELETE CASCADE,
    name        varchar  NOT NULL,
    photo_url   varchar,
    created_at  timestamp NOT NULL DEFAULT now(),
    updated_at  timestamp NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_product_category_filial_name ON product_category (filial_id, name);

CREATE TABLE product (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    filial_id   uuid NOT NULL REFERENCES filial(id) ON DELETE CASCADE,
    category_id uuid NOT NULL REFERENCES product_category(id) ON DELETE CASCADE,
    title       varchar  NOT NULL,
    body        text,
    status      boolean  DEFAULT true,
    base_price  numeric  NOT NULL,
    currency    char(3)  NOT NULL DEFAULT 'RUB',
    weight      varchar,
    created_by  uuid,
    created_at  timestamp NOT NULL DEFAULT now(),
    updated_at  timestamp NOT NULL DEFAULT now()
);

------------------------------------------------------------------
-- 3. EDU & SCHEDULES (из edu-service)
------------------------------------------------------------------
CREATE TABLE edu_subject (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    code       varchar,
    name       varchar NOT NULL,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now()
);

CREATE TABLE room (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    filial_id   uuid NOT NULL REFERENCES filial(id) ON DELETE CASCADE,
    room_type   varchar NOT NULL DEFAULT 'CLASSROOM',
    name        varchar,
    room_number varchar,
    capacity    int,
    layout_json jsonb,
    floor       int,
    note        text,
    created_at  timestamp NOT NULL DEFAULT now(),
    updated_at  timestamp NOT NULL DEFAULT now()
);

CREATE TABLE person (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    first_name  varchar,
    last_name   varchar,
    middle_name varchar,
    bio         text,
    birth_date  date,
    photo_url   varchar,
    created_at  timestamp NOT NULL DEFAULT now(),
    updated_at  timestamp NOT NULL DEFAULT now()
);

CREATE TABLE staff (
    id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    person_id      uuid NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    filial_id      uuid NOT NULL REFERENCES filial(id) ON DELETE CASCADE,
    staff_type     varchar NOT NULL,
    position_title varchar,
    active         boolean NOT NULL DEFAULT TRUE,
    created_at     timestamp NOT NULL DEFAULT now(),
    updated_at     timestamp NOT NULL DEFAULT now()
);

CREATE TABLE academic_term (
    id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    filial_id      uuid NOT NULL REFERENCES filial(id) ON DELETE CASCADE,
    name           varchar NOT NULL,
    starts_on      date NOT NULL,
    ends_on        date NOT NULL,
    week_start     int NOT NULL DEFAULT 1,
    created_at     timestamp NOT NULL DEFAULT now(),
    updated_at     timestamp NOT NULL DEFAULT now()
);

CREATE TABLE edu_group (
    id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    filial_id      uuid NOT NULL REFERENCES filial(id) ON DELETE CASCADE,
    parent_group_id uuid REFERENCES edu_group(id) ON DELETE CASCADE,
    name           varchar NOT NULL,
    source         varchar,
    source_group_id int,
    source_subgroup_id int,
    is_subgroup    boolean NOT NULL DEFAULT false,
    faculty_id     int,
    faculty_name   varchar,
    course_id      int,
    course_name    varchar,
    study_form_id  int,
    study_form_name varchar,
    education_level varchar,
    is_magistracy  boolean NOT NULL DEFAULT false,
    created_at     timestamp NOT NULL DEFAULT now(),
    updated_at     timestamp NOT NULL DEFAULT now(),
    CONSTRAINT edu_group_source_unique UNIQUE (filial_id, source, source_group_id),
    CONSTRAINT edu_group_source_subgroup_unique UNIQUE (filial_id, source, source_subgroup_id)
);

CREATE TABLE timetable_entry (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    term_id       uuid NOT NULL REFERENCES academic_term(id) ON DELETE CASCADE,
    group_id      uuid NOT NULL REFERENCES edu_group(id) ON DELETE CASCADE,
    subject_id    uuid NOT NULL REFERENCES edu_subject(id) ON DELETE RESTRICT,
    teacher_id    uuid REFERENCES staff(id) ON DELETE SET NULL,
    classroom_id  uuid REFERENCES room(id) ON DELETE SET NULL,
    day_of_week   int  NOT NULL,
    occurs_on     date,
    effective_from date,
    effective_to   date,
    starts_at     time NOT NULL,
    ends_at       time NOT NULL,
    week_type     varchar NOT NULL DEFAULT 'ALL',
    source         varchar,
    source_event_id bigint,
    is_exam       boolean NOT NULL DEFAULT false,
    lesson_type   varchar,
    comment       varchar,
    created_at    timestamp NOT NULL DEFAULT now(),
    updated_at    timestamp NOT NULL DEFAULT now(),
    CONSTRAINT timetable_entry_source_event_unique UNIQUE (term_id, group_id, source, source_event_id)
);

CREATE TABLE timetable_exception (
    id               uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    entry_id         uuid NOT NULL REFERENCES timetable_entry(id) ON DELETE CASCADE,
    date             date NOT NULL,
    action           varchar NOT NULL,
    new_teacher_id   uuid REFERENCES staff(id) ON DELETE SET NULL,
    new_classroom_id uuid REFERENCES room(id) ON DELETE SET NULL,
    new_starts_at    time,
    new_ends_at      time,
    created_at       timestamp NOT NULL DEFAULT now(),
    updated_at       timestamp NOT NULL DEFAULT now()
);

COMMIT;
