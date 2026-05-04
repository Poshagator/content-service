package product

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/internal/domain/entities/product"
	repo "github.com/poshagator/content-service/internal/domain/repository/postgres/product"
	"github.com/poshagator/content-service/pkg"
)

type Usecase struct {
	log  *zap.Logger
	repo *repo.Repository
}

func NewUsecase(
	log *zap.Logger,
	repo *repo.Repository,
) (*Usecase, error) {
	return &Usecase{
		log:  log.Named("usecase.product"),
		repo: repo,
	}, nil
}

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

func (u *Usecase) GetFilialModifiers(
	ctx context.Context,
	filialID uuid.UUID,
) (*product.ModifierGroupsByType, error) {

	groups, err := u.repo.GetFilialModifiers(ctx, filialID)
	if err != nil {
		u.log.Error("failed to get filial modifiers", zap.Error(err))
		return nil, err
	}

	out := &product.ModifierGroupsByType{}
	for _, g := range groups {
		switch g.Type {
		case "single":
			out.Single = append(out.Single, g)
		case "toggle":
			out.Toggle = append(out.Toggle, g)
		case "counter":
			out.Counter = append(out.Counter, g)
		case "multi":
			out.Multi = append(out.Multi, g)
		}
	}

	sort.Slice(out.Single, func(i, j int) bool { return lessGroup(out.Single[i], out.Single[j]) })
	sort.Slice(out.Toggle, func(i, j int) bool { return lessGroup(out.Toggle[i], out.Toggle[j]) })
	sort.Slice(out.Counter, func(i, j int) bool { return lessGroup(out.Counter[i], out.Counter[j]) })
	sort.Slice(out.Multi, func(i, j int) bool { return lessGroup(out.Multi[i], out.Multi[j]) })

	return out, nil
}

func lessGroup(a, b product.ProductModifierGroup) bool {
	ak := a.Kind
	bk := b.Kind
	if ak == "" && bk != "" {
		return false
	}
	if bk == "" && ak != "" {
		return true
	}
	if ak != bk {
		return ak < bk
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return a.ID.String() < b.ID.String()
}

func (u *Usecase) AttachModifierGroup(
	ctx context.Context,
	filialID uuid.UUID,
	link *product.ProductModifierGroupLink,
	headerPerms map[string]struct{},
) error {
	if err := u.repo.AttachModifierGroup(ctx, filialID, link, headerPerms); err != nil {
		u.log.Error("attach modifier", zap.Error(err))
		return err
	}
	return nil
}

func (u *Usecase) CreateModifierGroup(
	ctx context.Context,
	g *product.ProductModifierGroup,
	headerPerms map[string]struct{},
) error {

	if err := normalizeModifierGroup(g, true); err != nil {
		u.log.Warn("CreateModifierGroup invalid payload", zap.Error(err))
		return err
	}
	if err := u.repo.CreateModifierGroup(ctx, g, headerPerms); err != nil {
		u.log.Error("CreateModifierGroup repo", zap.Error(err))
		return err
	}
	return nil
}

func (u *Usecase) UpdateModifierGroup(
	ctx context.Context,
	g *product.ProductModifierGroup,
	headerPerms map[string]struct{},
) error {
	if err := normalizeModifierGroup(g, false); err != nil {
		u.log.Warn("UpdateModifierGroup invalid payload", zap.Error(err))
		return err
	}
	if err := u.repo.UpdateModifierGroup(ctx, g, headerPerms); err != nil {
		u.log.Error("UpdateModifierGroup repo", zap.Error(err))
		return err
	}
	return nil
}

func (u *Usecase) DeleteModifierGroup(
	ctx context.Context,
	filialID uuid.UUID,
	id uuid.UUID,
	headerPerms map[string]struct{},
) error {
	if err := u.repo.DeleteModifierGroup(ctx, filialID, id, headerPerms); err != nil {
		u.log.Error("DeleteModifierGroup repo", zap.Error(err))
		return err
	}
	return nil
}

func normalizeModifierGroup(g *product.ProductModifierGroup, requireOptions bool) error {
	if g == nil {
		return fmt.Errorf("nil group")
	}
	g.Type = strings.ToLower(strings.TrimSpace(g.Type))
	switch g.Type {
	case "single", "multi", "counter", "toggle":
	default:
		return fmt.Errorf("invalid group_type: %q", g.Type)
	}

	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return fmt.Errorf("name required")
	}

	if len(g.Options) == 0 && requireOptions {
		return fmt.Errorf("at least 1 option required")
	}

	for i := range g.Options {
		if g.Options[i].SortOrder == 0 {
			g.Options[i].SortOrder = i + 1
		}
		switch g.Type {
		case "single":
			g.Options[i].MaxQty = nil
			g.Options[i].PricePerUnit = nil
			g.Options[i].WeightPerUnit = nil
			g.Options[i].Step = nil
			g.Options[i].MinQty = nil
			g.Options[i].DefaultState = nil
		case "multi":
			g.Options[i].PricePerUnit = nil
			g.Options[i].WeightPerUnit = nil
			g.Options[i].Step = nil
			g.Options[i].MinQty = nil
			g.Options[i].DefaultState = nil
		case "counter":
			g.Options[i].PriceDelta = nil
			g.Options[i].WeightDelta = nil
			g.Options[i].DefaultState = nil
		case "toggle":
			g.Options[i].MaxQty = nil
			g.Options[i].PricePerUnit = nil
			g.Options[i].WeightPerUnit = nil
			g.Options[i].Step = nil
			g.Options[i].MinQty = nil
		}
		g.Options[i].Type = g.Type
		g.Options[i].Name = strings.TrimSpace(g.Options[i].Name)
		if g.Options[i].Name == "" {
			return fmt.Errorf("empty option name at pos %d", i)
		}
	}

	sort.SliceStable(g.Options, func(i, j int) bool {
		if g.Options[i].SortOrder != g.Options[j].SortOrder {
			return g.Options[i].SortOrder < g.Options[j].SortOrder
		}
		return g.Options[i].Name < g.Options[j].Name
	})

	return nil
}
