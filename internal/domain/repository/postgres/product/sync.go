package product

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	entity "github.com/poshagator/content-service/internal/domain/entities/product"
)

const qUpsertSyncProduct = `
INSERT INTO public.product (
    filial_id, category_id, title, body, status, base_price, currency, weight,
    source, external_id, updated_at
) VALUES (
    $1, $2, $3, NULLIF($4, ''), $5, $6, $7, NULLIF($8, ''),
    $9, $10, now()
)
ON CONFLICT (filial_id, source, external_id)
WHERE source IS NOT NULL AND external_id IS NOT NULL
DO UPDATE SET
    category_id = EXCLUDED.category_id,
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    status = EXCLUDED.status,
    base_price = EXCLUDED.base_price,
    currency = EXCLUDED.currency,
    weight = EXCLUDED.weight,
    updated_at = now()
RETURNING id;
`

func (r *Repository) SyncProducts(ctx context.Context, input entity.ProductSyncInput) (entity.ProductSyncStats, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return entity.ProductSyncStats{}, err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, qEnsureFilial, input.FilialID); err != nil {
		return entity.ProductSyncStats{}, err
	}

	stats := entity.ProductSyncStats{}
	seenProductExternalIDs := make([]string, 0)

	for _, category := range input.Categories {
		categoryID, err := r.upsertSyncCategory(ctx, tx, input.FilialID, input.Source, category)
		if err != nil {
			return entity.ProductSyncStats{}, err
		}
		stats.CategoriesUpserted++

		for _, item := range category.Products {
			status := true
			if item.Status != nil {
				status = *item.Status
			}
			currency := strings.ToUpper(strings.TrimSpace(item.Currency))
			if currency == "" {
				currency = "RUB"
			}

			var productID uuid.UUID
			if err = tx.QueryRow(
				ctx,
				qUpsertSyncProduct,
				input.FilialID,
				categoryID,
				item.Title,
				item.Body,
				status,
				item.BasePrice,
				currency,
				item.Weight,
				input.Source,
				item.ExternalID,
			).Scan(&productID); err != nil {
				return entity.ProductSyncStats{}, err
			}
			stats.ProductsUpserted++
			seenProductExternalIDs = append(seenProductExternalIDs, item.ExternalID)

			if err = r.syncProductMedia(ctx, tx, productID, item.MediaUrls); err != nil {
				return entity.ProductSyncStats{}, err
			}
		}
	}

	if input.Mode == entity.ProductSyncModeReplaceSource {
		tag, err := tx.Exec(ctx, `
UPDATE public.product
SET status = false, updated_at = now()
WHERE filial_id = $1
  AND source = $2
  AND external_id IS NOT NULL
  AND NOT (external_id = ANY($3));
`, input.FilialID, input.Source, seenProductExternalIDs)
		if err != nil {
			return entity.ProductSyncStats{}, err
		}
		stats.ProductsDisabled = tag.RowsAffected()
	}

	if err = tx.Commit(ctx); err != nil {
		return entity.ProductSyncStats{}, err
	}
	return stats, nil
}

func (r *Repository) upsertSyncCategory(
	ctx context.Context,
	tx pgx.Tx,
	filialID uuid.UUID,
	source string,
	category entity.ProductSyncCategory,
) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
SELECT id
FROM public.product_category
WHERE filial_id = $1
  AND ((source = $2 AND external_id = $3) OR name = $4)
ORDER BY CASE WHEN source = $2 AND external_id = $3 THEN 0 ELSE 1 END
LIMIT 1;
`, filialID, source, category.ExternalID, category.Name).Scan(&id)
	if err != nil && err != pgx.ErrNoRows {
		return uuid.Nil, err
	}
	if err == nil {
		_, err = tx.Exec(ctx, `
UPDATE public.product_category
SET name = $2,
    photo_url = NULLIF($3, ''),
    source = COALESCE(source, $4),
    external_id = COALESCE(external_id, $5),
    updated_at = now()
WHERE id = $1;
`, id, category.Name, category.PhotoURL, source, category.ExternalID)
		return id, err
	}

	err = tx.QueryRow(ctx, `
INSERT INTO public.product_category (filial_id, name, photo_url, source, external_id)
VALUES ($1, $2, NULLIF($3, ''), $4, $5)
RETURNING id;
`, filialID, category.Name, category.PhotoURL, source, category.ExternalID).Scan(&id)
	return id, err
}

func (r *Repository) syncProductMedia(ctx context.Context, tx pgx.Tx, productID uuid.UUID, urls []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM public.product_media WHERE product_id = $1`, productID); err != nil {
		return err
	}
	for _, rawURL := range urls {
		url := strings.TrimSpace(rawURL)
		if url == "" {
			continue
		}
		fileID, err := r.ensureMediaFile(ctx, tx, url)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `
INSERT INTO public.product_media (product_id, file_id)
VALUES ($1, $2)
ON CONFLICT ON CONSTRAINT product_media_product_file_unique DO NOTHING;
`, productID, fileID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ensureMediaFile(ctx context.Context, tx pgx.Tx, url string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM public.media_file WHERE url = $1 LIMIT 1`, url).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.Nil, err
	}
	err = tx.QueryRow(ctx, `
INSERT INTO public.media_file (url, file_type)
VALUES ($1, 'IMAGE')
RETURNING id;
`, url).Scan(&id)
	return id, err
}
