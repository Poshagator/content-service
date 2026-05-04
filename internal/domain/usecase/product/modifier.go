package product

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/internal/domain/entities/product"
)

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
) error {
	if err := u.repo.AttachModifierGroup(ctx, filialID, link); err != nil {
		u.log.Error("attach modifier", zap.Error(err))
		return err
	}
	return nil
}

func (u *Usecase) CreateModifierGroup(
	ctx context.Context,
	g *product.ProductModifierGroup,
) error {

	if err := normalizeModifierGroup(g, true); err != nil {
		u.log.Warn("CreateModifierGroup invalid payload", zap.Error(err))
		return err
	}
	if err := u.repo.CreateModifierGroup(ctx, g); err != nil {
		u.log.Error("CreateModifierGroup repo", zap.Error(err))
		return err
	}
	return nil
}

func (u *Usecase) UpdateModifierGroup(
	ctx context.Context,
	g *product.ProductModifierGroup,
) error {
	if err := normalizeModifierGroup(g, false); err != nil {
		u.log.Warn("UpdateModifierGroup invalid payload", zap.Error(err))
		return err
	}
	if err := u.repo.UpdateModifierGroup(ctx, g); err != nil {
		u.log.Error("UpdateModifierGroup repo", zap.Error(err))
		return err
	}
	return nil
}

func (u *Usecase) DeleteModifierGroup(
	ctx context.Context,
	filialID uuid.UUID,
	id uuid.UUID,
) error {
	if err := u.repo.DeleteModifierGroup(ctx, filialID, id); err != nil {
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
