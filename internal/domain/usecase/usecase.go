package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/protobuf/types/known/timestamppb"
	orderclient "product-service/internal/domain/clients/order/grpc"
	orderpb "product-service/pkg/proto/order/gen/go"
	"sort"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"product-service/internal/domain/entities"
	"product-service/internal/domain/repository/postgres"
	"product-service/pkg"
)

// -----------------------------------------------------------------------------
// Use-case layer (business-logic façade)
// -----------------------------------------------------------------------------

type Usecase struct {
	log      *zap.Logger
	repo     *postgres.Repository
	orderCli *orderclient.Client
}

func NewUsecase(
	log *zap.Logger,
	repo *postgres.Repository,
	oc *orderclient.Client,
) (*Usecase, error) {
	return &Usecase{
		log:      log.Named("usecase"),
		repo:     repo,
		orderCli: oc,
	}, nil
}

func (u *Usecase) GetProduct(ctx context.Context, id uuid.UUID) (*entities.Product, error) {
	product, err := u.repo.GetProduct(ctx, id)
	if err != nil {
		u.log.Error("failed to get product", zap.Error(err))
		return nil, err
	}
	return product, nil
}

func (u *Usecase) CreateProduct(ctx context.Context, product *entities.Product) (*entities.Product, error) {
	if err := u.repo.CreateProduct(ctx, product); err != nil {
		u.log.Error("failed to create product", zap.Error(err))
		return nil, err
	}
	return product, nil
}

func (u *Usecase) UpdateProduct(ctx context.Context, product *entities.Product) (*entities.Product, error) {
	if err := u.repo.UpdateProduct(ctx, product); err != nil {
		u.log.Error("failed to update product", zap.Error(err))
		return nil, err
	}
	return product, nil
}

func (u *Usecase) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	if err := u.repo.DeleteProduct(ctx, id); err != nil {
		u.log.Error("failed to delete product", zap.Error(err))
		return err
	}
	return nil
}

func (u *Usecase) GetProducts(ctx context.Context, filialID uuid.UUID, size, page int) ([]entities.Product, error) {
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
) ([]entities.CategoryGroup, error) {

	products, err := u.repo.GetProducts(ctx, filialID, pkg.PaginationQuery(page, limit))
	if err != nil {
		return nil, err
	}

	// группируем в map[catID] → *CategoryGroup
	groups := make(map[uuid.UUID]*entities.CategoryGroup)
	for _, p := range products {
		g, ok := groups[p.CategoryID]
		if !ok {
			g = &entities.CategoryGroup{
				CategoryID:    p.CategoryID,
				CategoryName:  p.CategoryName,
				CategoryImage: p.CategoryImage,
			}
			groups[p.CategoryID] = g
		}
		g.Products = append(g.Products, p)
	}

	// превращаем в срез, сохраняем порядок по products (они уже отсортированы SQL‑запросом)
	out := make([]entities.CategoryGroup, 0, len(groups))
	for _, p := range products {
		if g, ok := groups[p.CategoryID]; ok {
			out = appendUnique(out, *g)
			delete(groups, p.CategoryID) // чтобы не дублировать
		}
	}
	return out, nil
}

func appendUnique(dst []entities.CategoryGroup, g entities.CategoryGroup) []entities.CategoryGroup {
	for _, v := range dst {
		if v.CategoryID == g.CategoryID {
			return dst
		}
	}
	return append(dst, g)
}

// GetFilialModifiers возвращает группы модификаторов, используемые в продуктах филиала,
// разложенные по UI‑типам (single/multi/counter/toggle). Поле g.Kind содержит "семейство"
// (например size, volume и т.п.) — его можно использовать на фронте для слияния схожих групп.
func (u *Usecase) GetFilialModifiers(
	ctx context.Context,
	filialID uuid.UUID,
) (*entities.ModifierGroupsByType, error) {

	groups, err := u.repo.GetFilialModifiers(ctx, filialID)
	if err != nil {
		u.log.Error("failed to get filial modifiers", zap.Error(err))
		return nil, err
	}

	// Раскладываем по типам
	out := &entities.ModifierGroupsByType{}
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

	// Стабильный порядок внутри каждого среза: Kind -> Name.
	sort.Slice(out.Single, func(i, j int) bool { return lessGroup(out.Single[i], out.Single[j]) })
	sort.Slice(out.Toggle, func(i, j int) bool { return lessGroup(out.Toggle[i], out.Toggle[j]) })
	sort.Slice(out.Counter, func(i, j int) bool { return lessGroup(out.Counter[i], out.Counter[j]) })
	sort.Slice(out.Multi, func(i, j int) bool { return lessGroup(out.Multi[i], out.Multi[j]) })

	return out, nil
}

// lessGroup сортирует сперва по Kind (пустые Kind идут в конец), затем по Name, затем по ID.
func lessGroup(a, b entities.ProductModifierGroup) bool {
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

// GroupByKind удобен, если на фронте надо слепить несколько групп (например "Размер" и "Объём")
// в единый UI‑блок по Kind. Пока не используется в HTTP‑слое, но оставлен как вспомогательный.
func GroupByKind(groups []entities.ProductModifierGroup) map[string][]entities.ProductModifierGroup {
	out := make(map[string][]entities.ProductModifierGroup)
	for _, g := range groups {
		out[g.Kind] = append(out[g.Kind], g)
	}
	return out
}

func (u *Usecase) AttachModifierGroup(
	ctx context.Context,
	filialID uuid.UUID,
	link *entities.ProductModifierGroupLink,
	headerPerms map[string]struct{},
) error {
	if err := u.repo.AttachModifierGroup(ctx, filialID, link, headerPerms); err != nil {
		u.log.Error("attach modifier", zap.Error(err))
		return err
	}
	return nil
}

// ModifierOptionInput — упрощённый входной DTO (см. http/server.go).
type ModifierOptionInput struct {
	ID            *uuid.UUID
	Name          string
	PriceDelta    *float64
	WeightDelta   *float64
	MaxQty        *int
	PricePerUnit  *float64
	WeightPerUnit *float64
	Step          *float64
	MinQty        *int
	DefaultState  *bool
	SortOrder     *int
}

func (u *Usecase) CreateModifierGroup(
	ctx context.Context,
	g *entities.ProductModifierGroup,
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
	g *entities.ProductModifierGroup,
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

func normalizeModifierGroup(g *entities.ProductModifierGroup, requireOptions bool) error {
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

	// проставим sort_order если 0; при этом сохраним относительный порядок
	for i := range g.Options {
		if g.Options[i].SortOrder == 0 {
			g.Options[i].SortOrder = i + 1
		}
		// подчистим поля, не применимые к типу
		switch g.Type {
		case "single":
			g.Options[i].MaxQty = nil
			g.Options[i].PricePerUnit = nil
			g.Options[i].WeightPerUnit = nil
			g.Options[i].Step = nil
			g.Options[i].MinQty = nil
			// DefaultState не используется
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
		g.Options[i].Type = g.Type // выравниваем
		g.Options[i].Name = strings.TrimSpace(g.Options[i].Name)
		if g.Options[i].Name == "" {
			return fmt.Errorf("empty option name at pos %d", i)
		}
	}

	// стабильная сортировка по SortOrder, затем по Name
	sort.SliceStable(g.Options, func(i, j int) bool {
		if g.Options[i].SortOrder != g.Options[j].SortOrder {
			return g.Options[i].SortOrder < g.Options[j].SortOrder
		}
		return g.Options[i].Name < g.Options[j].Name
	})

	return nil
}

// fragment ‑‑ internal/domain/usecase/usecase.go
func (u *Usecase) CreateOrder(ctx context.Context, in *entities.OrderInput) (*entities.Order, error) {
	req := &orderpb.CreateOrderRequest{
		FilialId: in.FilialID.String(),
		UserId:   in.UserID.String(),
		TypeCode: in.TypeCode,
		Comment:  in.Comment,
		Items: func() []*orderpb.OrderItemInput {
			out := make([]*orderpb.OrderItemInput, len(in.Items))
			for i, it := range in.Items {
				out[i] = &orderpb.OrderItemInput{
					ProductId:     it.ProductID.String(),
					Qty:           int32(it.Qty),
					ModifiersJson: it.ModifiersJSON,
				}
			}
			return out
		}(),
	}

	// one‑of details
	switch in.TypeCode {
	case "delivery":
		req.Details = &orderpb.CreateOrderRequest_Delivery{
			Delivery: &orderpb.DeliveryInfo{
				AddressId:    in.Delivery.AddressID.String(),
				DeliveryTime: timestamppb.New(in.Delivery.DeliveryAt),
			},
		}
	case "takeaway":
		req.Details = &orderpb.CreateOrderRequest_Takeaway{
			Takeaway: &orderpb.TakeawayInfo{
				PickupTime: timestamppb.New(in.Takeaway.PickupAt),
			},
		}
	case "table":
		req.Details = &orderpb.CreateOrderRequest_Table{
			Table: &orderpb.TableInfo{
				TableNumber: in.Table.TableNumber,
			},
		}
	}

	resp, err := u.orderCli.Service.CreateOrder(ctx, req)
	if err != nil {
		u.log.Error("gRPC CreateOrder failed", zap.Error(err))
		return nil, err
	}

	od := resp.GetOrder()
	if od == nil {
		return nil, fmt.Errorf("order-service returned empty order")
	}

	return &entities.Order{
		ID:         uuid.MustParse(od.Id),
		FilialID:   uuid.MustParse(od.FilialId),
		Status:     od.StatusCode,
		TotalPrice: od.TotalPrice,
		Currency:   od.Currency,
		CreatedAt:  od.CreatedAt.AsTime(),
	}, nil
}

func (u *Usecase) DescribeOrder(
	ctx context.Context,
	items []entities.OrderItemInput,
) ([]entities.OrderItemDescription, error) {

	// собираем уникальные product_id
	ids := make([]uuid.UUID, 0, len(items))
	seen := map[uuid.UUID]struct{}{}
	for _, it := range items {
		if _, ok := seen[it.ProductID]; !ok {
			ids = append(ids, it.ProductID)
			seen[it.ProductID] = struct{}{}
		}
	}

	// тянем продукты + все их группы/опции
	products, err := u.repo.GetProductsByIDs(ctx, ids)
	if err != nil {
		u.log.Error("GetProductsByIDs", zap.Error(err))
		return nil, err
	}
	pmap := make(map[uuid.UUID]entities.Product, len(products))
	for _, p := range products {
		pmap[p.ID] = p
	}

	// формируем описание под каждый item
	out := make([]entities.OrderItemDescription, len(items))
	for i, it := range items {
		p, ok := pmap[it.ProductID]
		if !ok {
			return nil, fmt.Errorf("product %s not found", it.ProductID)
		}

		descr := entities.OrderItemDescription{
			ProductID: p.ID,
			Title:     p.Title,
			Body:      p.Body,
		}

		// если modifiers_json пустой — отдаём только продукт
		if it.ModifiersJSON != "" {
			modsText, err := u.pickSelectedModifiers(p, it.ModifiersJSON)
			if err != nil {
				return nil, err
			}
			descr.Modifiers = modsText
		}
		out[i] = descr
	}
	return out, nil
}

// pickSelectedModifiers фильтрует p.Modifiers по JSON‑выбору
// и возвращает массив строк «Группа: Опция …».
func (u *Usecase) pickSelectedModifiers(
	p entities.Product,
	raw string,
) ([]string, error) {
	// формат JSON в заказе: { "<group_id>": ["<option_id>", …] }
	var sel map[string][]string
	if err := json.Unmarshal([]byte(raw), &sel); err != nil {
		return nil, fmt.Errorf("bad modifiers_json: %w", err)
	}

	// индексируем группы/опции по id для быстрого доступа
	gIdx := map[uuid.UUID]entities.ProductModifierGroup{}
	oIdx := map[uuid.UUID]entities.ProductModifierOption{}
	for _, g := range p.Modifiers {
		gIdx[g.ID] = g
		for _, o := range g.Options {
			oIdx[o.ID] = o
		}
	}

	var out []string
	for gStr, opts := range sel {
		gid, err := uuid.Parse(gStr)
		if err != nil {
			continue // пропускаем мусор
		}
		g, ok := gIdx[gid]
		if !ok {
			continue
		}
		for _, oidStr := range opts {
			oid, err := uuid.Parse(oidStr)
			if err != nil {
				continue
			}
			if o, ok := oIdx[oid]; ok {
				out = append(out, fmt.Sprintf("%s: %s", g.Name, o.Name))
			}
		}
	}
	sort.Strings(out) // стабильный порядок
	return out, nil
}
