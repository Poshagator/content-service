package product

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/internal/domain/entities/product"
)

const qGetProduct = `
SELECT
    p.id,
    p.filial_id,
    p.category_id,
    c.name       AS category_name,
    c.photo_url  AS category_image,
    p.title,
    p.body,
    p.status,
    p.base_price,
    p.currency,
    p.weight,
    p.created_by,
    p.created_at,
    p.updated_at,
    COALESCE(array_agg(mf.url) FILTER (WHERE mf.url IS NOT NULL), '{}') AS media_urls
FROM public.product p
JOIN public.product_category c ON c.id = p.category_id
LEFT JOIN public.product_media pm ON pm.product_id = p.id
LEFT JOIN public.media_file     mf ON mf.id       = pm.file_id
WHERE p.id = $1
GROUP BY p.id, c.name, c.photo_url;
`

func (r *Repository) GetProduct(ctx context.Context, id uuid.UUID) (*product.Product, error) {
	rows, err := r.db.Query(ctx, qGetProduct, id)
	if err != nil {
		r.log.Error("failed to get product", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	dao, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[product.ProductDAO])
	if err != nil {
		r.log.Error("failed to collect product", zap.Error(err))
		return nil, err
	}
	p := dao.ToProduct()

	// Подтягиваем модификаторы
	if err := r.attachModifiers(ctx, []product.Product{*p}); err != nil {
		r.log.Warn("modifiers not attached", zap.Error(err))
	}

	return p, nil
}

const qCreateProduct = `
INSERT INTO public.product (
    filial_id, category_id, title, body,
    status, base_price, currency, weight,
    created_by, created_at, updated_at
) VALUES (
    $1,$2,$3,$4,
    $5,$6,$7,$8,
    $9,$10,$11
)
RETURNING id;
`

func (r *Repository) CreateProduct(ctx context.Context, p *product.Product) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, qEnsureFilial, p.FilialID); err != nil {
		r.log.Error("ensure filial", zap.Error(err))
		return err
	}

	if err = tx.QueryRow(
		ctx, qCreateProduct,
		p.FilialID, p.CategoryID, p.Title, p.Body,
		p.Status, p.BasePrice, p.Currency, p.Weight,
		p.CreatedBy, p.CreatedAt, p.UpdatedAt,
	).Scan(&p.ID); err != nil {
		r.log.Error("failed to create product", zap.Error(err))
		return err
	}

	return tx.Commit(ctx)
}

const qUpdateProduct = `
UPDATE public.product
SET filial_id=$1,
    category_id=$2,
    title=$3,
    body=$4,
    status=$5,
    base_price=$6,
    currency=$7,
    weight=$8,
    created_by=$9
WHERE id=$10;
`

func (r *Repository) UpdateProduct(ctx context.Context, p *product.Product) error {
	_, err := r.db.Exec(ctx, qUpdateProduct,
		p.FilialID, p.CategoryID, p.Title, p.Body,
		p.Status, p.BasePrice, p.Currency, p.Weight,
		p.CreatedBy, p.ID,
	)
	if err != nil {
		r.log.Error("failed to update product", zap.Error(err))
		return err
	}

	return nil
}

const qDeleteProduct = `DELETE FROM public.product WHERE id = $1;`

func (r *Repository) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, qDeleteProduct, id)
	if err != nil {
		r.log.Error("failed to delete product", zap.Error(err))
		return err
	}

	return nil
}

const qGetProducts = `
SELECT
    p.id,
    p.filial_id,
    p.category_id,
    c.name       AS category_name,
    c.photo_url  AS category_image,
    p.title,
    p.body,
    p.status,
    p.base_price,
    p.currency,
    p.weight,
    p.created_by,
    p.created_at,
    p.updated_at,
    COALESCE(array_agg(mf.url) FILTER (WHERE mf.url IS NOT NULL), '{}') AS media_urls
FROM public.product p
JOIN public.product_category c ON c.id = p.category_id
LEFT JOIN public.product_media pm ON pm.product_id = p.id
LEFT JOIN public.media_file     mf ON mf.id       = pm.file_id
WHERE p.filial_id = $1
GROUP BY p.id, c.name, c.photo_url
ORDER BY c.name, p.title
`

func (r *Repository) GetProducts(
	ctx context.Context,
	filialID uuid.UUID,
	sortQuery string,
) ([]product.Product, error) {

	rows, err := r.db.Query(ctx, qGetProducts+sortQuery, filialID)
	if err != nil {
		r.log.Error("failed to get products", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	productDAOs, err := pgx.CollectRows(rows, pgx.RowToStructByName[product.ProductDAO])
	if err != nil {
		r.log.Error("failed to collect products", zap.Error(err))
		return nil, err
	}

	products := product.ProductsDAO(productDAOs).ToProducts()

	if err := r.attachModifiers(ctx, products); err != nil {
		r.log.Warn("modifiers not attached", zap.Error(err))
	}

	return products, nil
}

const qEnsureFilial = `
INSERT INTO public.filial (id)
VALUES ($1)
ON CONFLICT (id) DO NOTHING;
`

func (r *Repository) ensureFilial(ctx context.Context, filialID uuid.UUID) error {
	_, err := r.db.Exec(ctx, qEnsureFilial, filialID)
	if err != nil {
		r.log.Error("failed to ensure filial", zap.Error(err))
	}
	return err
}

const qGetProductsByIDs = `
SELECT
    p.id,
    p.filial_id,
    p.category_id,
    c.name       AS category_name,
    c.photo_url  AS category_image,
    p.title,
    p.body,
    p.status,
    p.base_price,
    p.currency,
    p.weight,
    p.created_by,
    p.created_at,
    p.updated_at,
    COALESCE(array_agg(mf.url) FILTER (WHERE mf.url IS NOT NULL), '{}') AS media_urls
FROM public.product p
JOIN public.product_category c ON c.id = p.category_id
LEFT JOIN public.product_media pm ON pm.product_id = p.id
LEFT JOIN public.media_file     mf ON mf.id       = pm.file_id
WHERE p.id = ANY($1)
GROUP BY p.id, c.name, c.photo_url
ORDER BY c.name, p.title;
`

func (r *Repository) GetProductsByIDs(
	ctx context.Context,
	ids []uuid.UUID,
) ([]product.Product, error) {

	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, qGetProductsByIDs, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	daos, err := pgx.CollectRows(rows, pgx.RowToStructByName[product.ProductDAO])
	if err != nil {
		return nil, err
	}
	products := product.ProductsDAO(daos).ToProducts()
	if err := r.attachModifiers(ctx, products); err != nil {
		r.log.Warn("attachModifiers", zap.Error(err))
	}
	return products, nil
}
