package product

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/internal/domain/entities/product"
	"github.com/poshagator/content-service/pkg"
)

func (u *Usecase) GetProduct(ctx context.Context, id uuid.UUID) (*product.Product, error) {
	p, err := u.repo.GetProduct(ctx, id)
	if err != nil {
		u.log.Error("failed to get product", zap.Error(err))
		return nil, err
	}
	return p, nil
}

func (u *Usecase) CreateProduct(ctx context.Context, p *product.Product) (*product.Product, error) {
	if err := u.repo.CreateProduct(ctx, p); err != nil {
		u.log.Error("failed to create product", zap.Error(err))
		return nil, err
	}
	return p, nil
}

func (u *Usecase) UpdateProduct(ctx context.Context, p *product.Product) (*product.Product, error) {
	if err := u.repo.UpdateProduct(ctx, p); err != nil {
		u.log.Error("failed to update product", zap.Error(err))
		return nil, err
	}
	return p, nil
}

func (u *Usecase) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	if err := u.repo.DeleteProduct(ctx, id); err != nil {
		u.log.Error("failed to delete product", zap.Error(err))
		return err
	}
	return nil
}

func (u *Usecase) GetProducts(ctx context.Context, filialID uuid.UUID, size, page int) ([]product.Product, error) {
	products, err := u.repo.GetProducts(ctx, filialID, pkg.PaginationQuery(page, size))
	if err != nil {
		u.log.Error("failed to get products", zap.Error(err))
		return nil, err
	}
	return products, nil
}

func (u *Usecase) GetProductsGrouped(
	ctx context.Context,
	filialID uuid.UUID,
	limit, page int,
) ([]product.CategoryGroup, error) {

	products, err := u.repo.GetProducts(ctx, filialID, pkg.PaginationQuery(page, limit))
	if err != nil {
		return nil, err
	}

	groups := make(map[uuid.UUID]*product.CategoryGroup)
	for _, p := range products {
		g, ok := groups[p.CategoryID]
		if !ok {
			g = &product.CategoryGroup{
				CategoryID:    p.CategoryID,
				CategoryName:  p.CategoryName,
				CategoryImage: p.CategoryImage,
			}
			groups[p.CategoryID] = g
		}
		g.Products = append(g.Products, p)
	}

	out := make([]product.CategoryGroup, 0, len(groups))
	for _, p := range products {
		if g, ok := groups[p.CategoryID]; ok {
			out = appendUnique(out, *g)
			delete(groups, p.CategoryID)
		}
	}
	return out, nil
}

func appendUnique(dst []product.CategoryGroup, g product.CategoryGroup) []product.CategoryGroup {
	for _, v := range dst {
		if v.CategoryID == g.CategoryID {
			return dst
		}
	}
	return append(dst, g)
}
