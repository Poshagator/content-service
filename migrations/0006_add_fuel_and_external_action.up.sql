BEGIN;

-- 1. Create external_action table for actions
CREATE TABLE IF NOT EXISTS public.external_action (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    filial_id  uuid NOT NULL REFERENCES public.filial(id) ON DELETE CASCADE,
    title      varchar NOT NULL,
    url        varchar NOT NULL,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now()
);

-- 2. Create fuel table for petrol and fuel
CREATE TABLE IF NOT EXISTS public.fuel (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    filial_id  uuid NOT NULL REFERENCES public.filial(id) ON DELETE CASCADE,
    name       varchar NOT NULL,
    price      numeric NOT NULL,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now()
);

COMMIT;
