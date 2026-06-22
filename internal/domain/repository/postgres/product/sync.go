package product

import (
	"context"
	"fmt"
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

func (r *Repository) UpsertProduct(ctx context.Context, input entity.UpsertProductInput) (uuid.UUID, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	defer tx.Rollback(ctx)

	id, created, err := r.upsertProductTx(ctx, tx, input)
	if err != nil {
		return uuid.Nil, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, false, err
	}
	return id, created, nil
}

func (r *Repository) BatchUpsertProducts(ctx context.Context, input entity.BatchUpsertProductsInput) ([]uuid.UUID, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	ids := make([]uuid.UUID, 0, len(input.Products))
	for _, p := range input.Products {
		id, _, err := r.upsertProductTx(ctx, tx, p)
		if err != nil {
			return nil, fmt.Errorf("upsert product %s: %w", p.Product.ExternalID, err)
		}
		ids = append(ids, id)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *Repository) upsertProductTx(ctx context.Context, tx pgx.Tx, input entity.UpsertProductInput) (uuid.UUID, bool, error) {
	if _, err := tx.Exec(ctx, qEnsureFilial, input.FilialID); err != nil {
		return uuid.Nil, false, err
	}

	var categoryID uuid.UUID
	var err error
	
	if input.CategoryID != nil {
		categoryID = *input.CategoryID
	} else if input.CategoryExternalID != nil || input.CategoryName != nil {
		extID := ""
		if input.CategoryExternalID != nil {
			extID = *input.CategoryExternalID
		}
		name := ""
		if input.CategoryName != nil {
			name = *input.CategoryName
		}
		categoryID, err = r.upsertSyncCategory(ctx, tx, input.FilialID, input.Source, entity.CategoryInput{
			ExternalID: extID,
			Name:       name,
		})
		if err != nil {
			return uuid.Nil, false, fmt.Errorf("resolve category: %w", err)
		}
	} else {
		// Category is strictly required by DB schema in qUpsertSyncProduct, or we might need a default category
		// For now we assume category_id can be Nil if DB allows, but DB expects UUID.
		// If DB expects UUID, we will pass Nil UUID and hope it allows nulls or fails.
	}

	item := input.Product
	status := true
	if item.Status != nil {
		status = *item.Status
	}
	currency := strings.ToUpper(strings.TrimSpace(item.Currency))
	if currency == "" {
		currency = "RUB"
	}
	weight := item.Weight
	if weight == "" {
		weight = item.Volume
	}

	var productID uuid.UUID
	var created bool
	
	// Try to see if it exists to set "created" flag correctly
	var existingID uuid.UUID
	err = tx.QueryRow(ctx, "SELECT id FROM public.product WHERE filial_id = $1 AND source = $2 AND external_id = $3", input.FilialID, input.Source, item.ExternalID).Scan(&existingID)
	if err == pgx.ErrNoRows {
		created = true
	} else if err != nil {
		return uuid.Nil, false, err
	} else {
		created = false
	}

	var catIDParam interface{}
	if categoryID != uuid.Nil {
		catIDParam = categoryID
	} else {
		catIDParam = nil
	}

	if err = tx.QueryRow(
		ctx,
		qUpsertSyncProduct,
		input.FilialID,
		catIDParam,
		item.Title,
		item.Body,
		status,
		item.BasePrice,
		currency,
		weight,
		input.Source,
		item.ExternalID,
	).Scan(&productID); err != nil {
		return uuid.Nil, false, fmt.Errorf("upsert product: %w", err)
	}

	if err = r.syncProductMedia(ctx, tx, productID, item.MediaUrls); err != nil {
		return uuid.Nil, false, fmt.Errorf("sync media: %w", err)
	}

	return productID, created, nil
}

func (r *Repository) UpsertCategory(ctx context.Context, input entity.UpsertCategoryInput) (uuid.UUID, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	defer tx.Rollback(ctx)

	id, err := r.upsertSyncCategory(ctx, tx, input.FilialID, input.Source, input.Category)
	if err != nil {
		return uuid.Nil, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, false, err
	}
	// We don't have accurate 'created' for category right now, return true
	return id, true, nil
}

func (r *Repository) BatchUpsertCategories(ctx context.Context, input entity.BatchUpsertCategoriesInput) ([]uuid.UUID, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	ids := make([]uuid.UUID, 0, len(input.Categories))
	for _, c := range input.Categories {
		id, err := r.upsertSyncCategory(ctx, tx, c.FilialID, c.Source, c.Category)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *Repository) upsertSyncCategory(
	ctx context.Context,
	tx pgx.Tx,
	filialID uuid.UUID,
	source string,
	category entity.CategoryInput,
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
	
	photoURL := ""
	if category.PhotoURL != nil {
		photoURL = *category.PhotoURL
	}

	if err == nil {
		_, err = tx.Exec(ctx, `
UPDATE public.product_category
SET name = COALESCE(NULLIF($2, ''), name),
    photo_url = COALESCE(NULLIF($3, ''), photo_url),
    source = COALESCE(source, $4),
    external_id = COALESCE(external_id, NULLIF($5, '')),
    updated_at = now()
WHERE id = $1;
`, id, category.Name, photoURL, source, category.ExternalID)
		return id, err
	}

	err = tx.QueryRow(ctx, `
INSERT INTO public.product_category (filial_id, name, photo_url, source, external_id)
VALUES ($1, $2, NULLIF($3, ''), $4, NULLIF($5, ''))
RETURNING id;
`, filialID, category.Name, photoURL, source, category.ExternalID).Scan(&id)
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
