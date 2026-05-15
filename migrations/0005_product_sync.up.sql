BEGIN;

ALTER TABLE public.product_category
    ADD COLUMN IF NOT EXISTS source varchar,
    ADD COLUMN IF NOT EXISTS external_id varchar;

ALTER TABLE public.product
    ADD COLUMN IF NOT EXISTS source varchar,
    ADD COLUMN IF NOT EXISTS external_id varchar;

CREATE UNIQUE INDEX IF NOT EXISTS ux_product_category_filial_source_external
    ON public.product_category (filial_id, source, external_id)
    WHERE source IS NOT NULL AND external_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_product_filial_source_external
    ON public.product (filial_id, source, external_id)
    WHERE source IS NOT NULL AND external_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS public.product_media (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id uuid NOT NULL REFERENCES public.product(id) ON DELETE CASCADE,
    file_id    uuid NOT NULL REFERENCES public.media_file(id) ON DELETE CASCADE,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    CONSTRAINT product_media_product_file_unique UNIQUE (product_id, file_id)
);

COMMIT;
