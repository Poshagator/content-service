BEGIN;

DROP TABLE IF EXISTS public.product_media CASCADE;

DROP INDEX IF EXISTS public.ux_product_filial_source_external;
DROP INDEX IF EXISTS public.ux_product_category_filial_source_external;

ALTER TABLE public.product
    DROP COLUMN IF EXISTS external_id,
    DROP COLUMN IF EXISTS source;

ALTER TABLE public.product_category
    DROP COLUMN IF EXISTS external_id,
    DROP COLUMN IF EXISTS source;

COMMIT;
