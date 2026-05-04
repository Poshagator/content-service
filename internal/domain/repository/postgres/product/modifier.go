package product

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/poshagator/content-service/internal/domain/entities/product"
)

const qProductModifiers = `
WITH base AS (
    SELECT pmgl.product_id,
           pmg.id              AS group_id,
           pmg.filial_id       AS group_filial_id,
           pmg.name            AS group_name,
           'single'            AS group_type,
           pmk.code            AS kind_code,
           pmgl.required,
           pmgl.min_select,
           pmgl.max_select,

           pso.id              AS option_id,
           pso.name            AS option_name,
           'single'            AS option_type,
           pso.price_delta,
           pso.weight_delta,
           NULL::int           AS max_qty,
           NULL::numeric       AS price_per_unit,
           NULL::numeric       AS weight_per_unit,
           NULL::numeric       AS step,
           NULL::int           AS min_qty,
           NULL::bool          AS default_state
    FROM product_modifier_group_link pmgl
    JOIN product_modifier_group pmg          ON pmg.id = pmgl.group_id
    LEFT JOIN product_modifier_kind pmk      ON pmk.id = pmg.kind_id
    JOIN product_modifier_single_option pso  ON pso.group_id = pmgl.group_id

UNION ALL
    SELECT pmgl.product_id,
           pmg.id, pmg.filial_id, pmg.name, 'multi',
           pmk.code,
           pmgl.required, pmgl.min_select, pmgl.max_select,
           pmo.id, pmo.name, 'multi',
           pmo.price_delta, pmo.weight_delta,
           pmo.max_qty,
           NULL, NULL, NULL, NULL, NULL
    FROM product_modifier_group_link pmgl
    JOIN product_modifier_group pmg         ON pmg.id = pmgl.group_id
    LEFT JOIN product_modifier_kind pmk     ON pmk.id = pmg.kind_id
    JOIN product_modifier_multi_option pmo  ON pmo.group_id = pmgl.group_id

UNION ALL
    SELECT pmgl.product_id,
           pmg.id, pmg.filial_id, pmg.name, 'counter',
           pmk.code,
           pmgl.required, pmgl.min_select, pmgl.max_select,
           pco.id, pco.name, 'counter',
           NULL, NULL,
           pco.max_qty,
           pco.price_per_unit, pco.weight_per_unit,
           pco.step, pco.min_qty,
           NULL
    FROM product_modifier_group_link pmgl
    JOIN product_modifier_group pmg           ON pmg.id = pmgl.group_id
    LEFT JOIN product_modifier_kind pmk       ON pmk.id = pmg.kind_id
    JOIN product_modifier_counter_option pco  ON pco.group_id = pmgl.group_id

UNION ALL
    SELECT pmgl.product_id,
           pmg.id, pmg.filial_id, pmg.name, 'toggle',
           pmk.code,
           pmgl.required, pmgl.min_select, pmgl.max_select,
           pto.id, pto.name, 'toggle',
           pto.price_delta, pto.weight_delta,
           NULL::int,
           NULL::numeric, NULL::numeric, NULL::numeric, NULL::int,
           pto.default_state
    FROM product_modifier_group_link pmgl
    JOIN product_modifier_group pmg         ON pmg.id = pmgl.group_id
    LEFT JOIN product_modifier_kind pmk     ON pmk.id = pmg.kind_id
    JOIN product_modifier_toggle_option pto ON pto.group_id = pmgl.group_id
)
SELECT *
FROM base
WHERE product_id = ANY($1)
ORDER BY product_id, group_name, option_name;
`

func (r *Repository) attachModifiers(ctx context.Context, products []product.Product) error {
	if len(products) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(products))
	for i, p := range products {
		ids[i] = p.ID
	}
	rows, err := r.db.Query(ctx, qProductModifiers, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		ProductID     uuid.UUID
		GroupID       uuid.UUID
		GroupFilialID uuid.UUID
		GroupName     string
		GroupType     string
		KindCode      sql.NullString
		Required      bool
		MinSelect     int32
		MaxSelect     int32
		OptionID      uuid.UUID
		OptionName    string
		OptionType    string
		PriceDelta    sql.NullFloat64
		WeightDelta   sql.NullFloat64
		MaxQty        sql.NullInt32
		PricePerUnit  sql.NullFloat64
		WeightPerUnit sql.NullFloat64
		Step          sql.NullFloat64
		MinQty        sql.NullInt32
		DefaultState  sql.NullBool
	}
	cache := make(map[uuid.UUID]map[uuid.UUID]*product.ProductModifierGroup)

	for rows.Next() {
		var rrow row
		if err := rows.Scan(
			&rrow.ProductID,
			&rrow.GroupID,
			&rrow.GroupFilialID,
			&rrow.GroupName,
			&rrow.GroupType,
			&rrow.KindCode,
			&rrow.Required,
			&rrow.MinSelect,
			&rrow.MaxSelect,
			&rrow.OptionID,
			&rrow.OptionName,
			&rrow.OptionType,
			&rrow.PriceDelta,
			&rrow.WeightDelta,
			&rrow.MaxQty,
			&rrow.PricePerUnit,
			&rrow.WeightPerUnit,
			&rrow.Step,
			&rrow.MinQty,
			&rrow.DefaultState,
		); err != nil {
			return err
		}

		if _, ok := cache[rrow.ProductID]; !ok {
			cache[rrow.ProductID] = make(map[uuid.UUID]*product.ProductModifierGroup)
		}
		g, ok := cache[rrow.ProductID][rrow.GroupID]
		if !ok {
			g = &product.ProductModifierGroup{
				ID:        rrow.GroupID,
				FilialID:  rrow.GroupFilialID,
				Name:      rrow.GroupName,
				Type:      rrow.GroupType, // UI rendering type
				Kind:      rrow.KindCode.String,
				Required:  rrow.Required,
				MinSelect: int(rrow.MinSelect),
				MaxSelect: int(rrow.MaxSelect),
			}
			cache[rrow.ProductID][rrow.GroupID] = g
		}

		opt := product.ProductModifierOption{
			ID:            rrow.OptionID,
			Type:          rrow.OptionType,
			Name:          rrow.OptionName,
			PriceDelta:    nullFloatPtr(rrow.PriceDelta),
			WeightDelta:   nullFloatPtr(rrow.WeightDelta),
			MaxQty:        nullIntPtr(rrow.MaxQty),
			PricePerUnit:  nullFloatPtr(rrow.PricePerUnit),
			WeightPerUnit: nullFloatPtr(rrow.WeightPerUnit),
			Step:          nullFloatPtr(rrow.Step),
			MinQty:        nullIntPtr(rrow.MinQty),
			DefaultState:  nullBoolPtr(rrow.DefaultState),
		}
		g.Options = append(g.Options, opt)
	}

	// Переливаем в продукты
	for i, p := range products {
		if groups, ok := cache[p.ID]; ok {
			p.Modifiers = make([]product.ProductModifierGroup, 0, len(groups))
			for _, g := range groups {
				p.Modifiers = append(p.Modifiers, *g)
			}
			products[i] = p
		}
	}
	return nil
}

const qFilialModifiers = `
WITH base AS (
    -- single
    SELECT
        pmg.id              AS group_id,
        pmg.filial_id       AS group_filial_id,
        pmg.name            AS group_name,
        'single'            AS group_type,
        pmk.code            AS kind_code,
        pso.id              AS option_id,
        pso.name            AS option_name,
        'single'            AS option_type,
        pso.price_delta,
        pso.weight_delta,
        NULL::int           AS max_qty,
        NULL::numeric       AS price_per_unit,
        NULL::numeric       AS weight_per_unit,
        NULL::numeric       AS step,
        NULL::int           AS min_qty,
        NULL::bool          AS default_state
    FROM product_modifier_group pmg
    LEFT JOIN product_modifier_kind pmk      ON pmk.id = pmg.kind_id
    JOIN product_modifier_single_option pso  ON pso.group_id = pmg.id
    WHERE pmg.filial_id = $1

UNION ALL
    -- multi
    SELECT
        pmg.id, pmg.filial_id, pmg.name, 'multi',
        pmk.code,
        pmo.id, pmo.name, 'multi',
        pmo.price_delta, pmo.weight_delta,
        pmo.max_qty,
        NULL, NULL, NULL, NULL, NULL
    FROM product_modifier_group pmg
    LEFT JOIN product_modifier_kind pmk     ON pmk.id = pmg.kind_id
    JOIN product_modifier_multi_option pmo  ON pmo.group_id = pmg.id
    WHERE pmg.filial_id = $1

UNION ALL
    -- counter
    SELECT
        pmg.id, pmg.filial_id, pmg.name, 'counter',
        pmk.code,
        pco.id, pco.name, 'counter',
        NULL::numeric, NULL::numeric,
        pco.max_qty,
        pco.price_per_unit, pco.weight_per_unit,
        pco.step, pco.min_qty,
        NULL
    FROM product_modifier_group pmg
    LEFT JOIN product_modifier_kind pmk       ON pmk.id = pmg.kind_id
    JOIN product_modifier_counter_option pco  ON pco.group_id = pmg.id
    WHERE pmg.filial_id = $1

UNION ALL
    -- toggle
    SELECT
        pmg.id, pmg.filial_id, pmg.name, 'toggle',
        pmk.code,
        pto.id, pto.name, 'toggle',
        pto.price_delta, pto.weight_delta,
        NULL::int,
        NULL::numeric, NULL::numeric, NULL::numeric, NULL::int,
        pto.default_state
    FROM product_modifier_group pmg
    LEFT JOIN product_modifier_kind pmk     ON pmk.id = pmg.kind_id
    JOIN product_modifier_toggle_option pto ON pto.group_id = pmg.id
    WHERE pmg.filial_id = $1
)
SELECT *
FROM base
ORDER BY group_name, option_name;
`

func (r *Repository) GetFilialModifiers(
	ctx context.Context,
	filialID uuid.UUID,
) ([]product.ProductModifierGroup, error) {

	rows, err := r.db.Query(ctx, qFilialModifiers, filialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type row struct {
		GroupID, GroupFilialID, OptionID                           uuid.UUID
		GroupName, GroupType, KindCode, OptionName, OptionType     string
		PriceDelta, WeightDelta, PricePerUnit, WeightPerUnit, Step sql.NullFloat64
		MaxQty, MinQty                                             sql.NullInt32
		DefaultState                                               sql.NullBool
	}

	cache := make(map[uuid.UUID]*product.ProductModifierGroup)

	for rows.Next() {
		var rrow row
		if err := rows.Scan(
			&rrow.GroupID,
			&rrow.GroupFilialID,
			&rrow.GroupName,
			&rrow.GroupType,
			&rrow.KindCode,
			&rrow.OptionID,
			&rrow.OptionName,
			&rrow.OptionType,
			&rrow.PriceDelta,
			&rrow.WeightDelta,
			&rrow.MaxQty,
			&rrow.PricePerUnit,
			&rrow.WeightPerUnit,
			&rrow.Step,
			&rrow.MinQty,
			&rrow.DefaultState,
		); err != nil {
			return nil, err
		}

		g, ok := cache[rrow.GroupID]
		if !ok {
			g = &product.ProductModifierGroup{
				ID:       rrow.GroupID,
				FilialID: rrow.GroupFilialID,
				Name:     rrow.GroupName,
				Type:     rrow.GroupType,
				Kind:     rrow.KindCode,
			}
			cache[rrow.GroupID] = g
		}
		g.Options = append(g.Options, product.ProductModifierOption{
			ID:            rrow.OptionID,
			Type:          rrow.OptionType,
			Name:          rrow.OptionName,
			PriceDelta:    nullFloatPtr(rrow.PriceDelta),
			WeightDelta:   nullFloatPtr(rrow.WeightDelta),
			MaxQty:        nullIntPtr(rrow.MaxQty),
			PricePerUnit:  nullFloatPtr(rrow.PricePerUnit),
			WeightPerUnit: nullFloatPtr(rrow.WeightPerUnit),
			Step:          nullFloatPtr(rrow.Step),
			MinQty:        nullIntPtr(rrow.MinQty),
			DefaultState:  nullBoolPtr(rrow.DefaultState),
		})
	}

	out := make([]product.ProductModifierGroup, 0, len(cache))
	for _, g := range cache {
		out = append(out, *g)
	}
	return out, nil
}

const qAttachProductModifierGroup = `
INSERT INTO public.product_modifier_group_link AS t (
    product_id,
    group_id,
    sort_order,
    required,
    min_select,
    max_select
) VALUES (
    $1,$2,$3,$4,$5,$6
)
ON CONFLICT (product_id, group_id) DO UPDATE
SET sort_order = EXCLUDED.sort_order,
    required   = EXCLUDED.required,
    min_select = EXCLUDED.min_select,
    max_select = EXCLUDED.max_select,
    updated_at = NOW();
`

func (r *Repository) checkProductAndGroupFilial(
	ctx context.Context,
	productID, groupID uuid.UUID,
) (prodF, groupF *uuid.UUID, err error) {
	const q = `
SELECT
	(SELECT filial_id FROM public.product WHERE id=$1) AS product_filial,
	(SELECT filial_id FROM public.product_modifier_group WHERE id=$2) AS group_filial;
`
	var pfNS, gfNS sql.NullString
	if err = r.db.QueryRow(ctx, q, productID, groupID).Scan(&pfNS, &gfNS); err != nil {
		return nil, nil, err
	}
	if pfNS.Valid && pfNS.String != "" {
		if u, e := uuid.Parse(pfNS.String); e == nil {
			prodF = &u
		}
	}
	if gfNS.Valid && gfNS.String != "" {
		if u, e := uuid.Parse(gfNS.String); e == nil {
			groupF = &u
		}
	}
	return prodF, groupF, nil
}

func (r *Repository) AttachModifierGroup(
	ctx context.Context,
	filialID uuid.UUID,
	link *product.ProductModifierGroupLink,
) error {

	pf, gf, err := r.checkProductAndGroupFilial(ctx, link.ProductID, link.GroupID)
	if err != nil {
		r.log.Error("AttachModifierGroup check filials", zap.Error(err))
		return err
	}
	if pf == nil {
		return status.Errorf(codes.NotFound, "product %s not found", link.ProductID)
	}
	if gf == nil {
		return status.Errorf(codes.NotFound, "modifier group %s not found", link.GroupID)
	}
	if *pf != *gf {
		return status.Errorf(
			codes.InvalidArgument,
			"product (%s) and group (%s) belong to different filials (%s <> %s)",
			link.ProductID, link.GroupID, pf.String(), gf.String(),
		)
	}
	if filialID != *pf {
		return status.Errorf(
			codes.InvalidArgument,
			"payload filial_id %s does not match entity filial_id %s",
			filialID, pf.String(),
		)
	}

	_, err = r.db.Exec(ctx, qAttachProductModifierGroup,
		link.ProductID,
		link.GroupID,
		link.SortOrder,
		link.Required,
		link.MinSelect,
		link.MaxSelect,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return status.Errorf(codes.InvalidArgument, "filial mismatch: %s", pgErr.Message)
		}
		r.log.Error("attach modifier", zap.Error(err))
		return err
	}
	return nil
}

func (r *Repository) lookupKindID(ctx context.Context, code string) (*uuid.UUID, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil
	}
	const q = `SELECT id FROM public.product_modifier_kind WHERE LOWER(code)=LOWER($1) LIMIT 1`
	var id uuid.UUID
	if err := r.db.QueryRow(ctx, q, code).Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

const qInsertModifierGroup = `
INSERT INTO public.product_modifier_group (filial_id, name, kind_id)
VALUES ($1, $2, $3)
RETURNING id, filial_id, name, kind_id, created_at, updated_at;
`

const qUpdateModifierGroup = `
UPDATE public.product_modifier_group
SET name=$2,
    kind_id=$3,
    updated_at=NOW()
WHERE id=$1
RETURNING id, filial_id, name, kind_id, created_at, updated_at;
`

func (r *Repository) CreateModifierGroup(
	ctx context.Context,
	g *product.ProductModifierGroup,
) error {

	if err := r.ensureFilial(ctx, g.FilialID); err != nil {
		return err
	}

	kindID, err := r.lookupKindID(ctx, g.Kind)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err = tx.QueryRow(ctx, qInsertModifierGroup, g.FilialID, g.Name, kindID).
		Scan(&g.ID, &g.FilialID, &g.Name, new(uuid.UUID), new(time.Time), new(time.Time)); err != nil {
		return err
	}

	if len(g.Options) > 0 {
		if err = r.insertGroupOptions(ctx, tx, g.ID, g.Type, g.Options); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) UpdateModifierGroup(
	ctx context.Context,
	g *product.ProductModifierGroup,
) error {

	kindID, err := r.lookupKindID(ctx, g.Kind)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err = tx.QueryRow(ctx, qUpdateModifierGroup, g.ID, g.Name, kindID).
		Scan(new(uuid.UUID), &g.FilialID, &g.Name, new(uuid.UUID), new(time.Time), new(time.Time)); err != nil {
		return err
	}

	table := map[string]string{
		"single":  "product_modifier_single_option",
		"multi":   "product_modifier_multi_option",
		"counter": "product_modifier_counter_option",
		"toggle":  "product_modifier_toggle_option",
	}[g.Type]
	if table == "" {
		return fmt.Errorf("unknown group_type %q", g.Type)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM `+table+` WHERE group_id=$1`, g.ID); err != nil {
		return err
	}
	if len(g.Options) > 0 {
		if err = r.insertGroupOptions(ctx, tx, g.ID, g.Type, g.Options); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) getGroupFilial(ctx context.Context, id uuid.UUID) (*uuid.UUID, error) {
	const q = `SELECT filial_id FROM public.product_modifier_group WHERE id=$1 LIMIT 1`
	var ns sql.NullString
	if err := r.db.QueryRow(ctx, q, id).Scan(&ns); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if ns.Valid && ns.String != "" {
		if u, err := uuid.Parse(ns.String); err == nil {
			return &u, nil
		}
	}
	return nil, nil
}

const qDeleteModifierGroup = `
DELETE FROM public.product_modifier_group
WHERE id=$1 AND filial_id=$2;
`

func (r *Repository) DeleteModifierGroup(
	ctx context.Context,
	filialID uuid.UUID,
	id uuid.UUID,
) error {
	gFilial, err := r.getGroupFilial(ctx, id)
	if err != nil {
		return err
	}
	if gFilial == nil {
		return status.Errorf(codes.NotFound, "modifier group %s not found", id)
	}
	if *gFilial != filialID {
		return status.Errorf(codes.NotFound, "modifier group %s not found in filial %s", id, filialID)
	}

	tag, err := r.db.Exec(ctx, qDeleteModifierGroup, id, filialID)
	if err != nil {
		r.log.Error("DeleteModifierGroup delete", zap.Error(err))
		return err
	}
	if tag.RowsAffected() == 0 {
		return status.Errorf(codes.NotFound, "modifier group %s not found in filial %s", id, filialID)
	}
	return nil
}

func (r *Repository) insertGroupOptions(
	ctx context.Context, tx pgx.Tx,
	groupID uuid.UUID, groupType string,
	opts []product.ProductModifierOption,
) error {

	for i, o := range opts {
		order := o.SortOrder
		if order == 0 {
			order = i + 1
		}

		var q string
		var args []any

		switch groupType {
		case "single":
			q = `INSERT INTO product_modifier_single_option
                 (group_id,name,price_delta,weight_delta,sort_order)
                 VALUES ($1,$2,$3,$4,$5) RETURNING id`
			args = []any{groupID, o.Name, o.PriceDelta, o.WeightDelta, order}

		case "multi":
			q = `INSERT INTO product_modifier_multi_option
                 (group_id,name,price_delta,weight_delta,max_qty,sort_order)
                 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`
			args = []any{groupID, o.Name, o.PriceDelta, o.WeightDelta, o.MaxQty, order}

		case "counter":
			q = `INSERT INTO product_modifier_counter_option
                 (group_id,name,price_per_unit,weight_per_unit,step,min_qty,max_qty,sort_order)
                 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`
			args = []any{groupID, o.Name, o.PricePerUnit, o.WeightPerUnit, o.Step,
				o.MinQty, o.MaxQty, order}

		case "toggle":
			q = `INSERT INTO product_modifier_toggle_option
                 (group_id,name,default_state,price_delta,weight_delta,sort_order)
                 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`
			args = []any{groupID, o.Name, o.DefaultState, o.PriceDelta, o.WeightDelta, order}

		default:
			return fmt.Errorf("unknown group_type %q", groupType)
		}

		if err := tx.QueryRow(ctx, q, args...).Scan(&o.ID); err != nil {
			return err
		}
		opts[i].ID = o.ID
	}
	return nil
}

func nullFloatPtr(v sql.NullFloat64) *float64 {
	if v.Valid {
		return &v.Float64
	}
	return nil
}
func nullIntPtr(v sql.NullInt32) *int {
	if v.Valid {
		i := int(v.Int32)
		return &i
	}
	return nil
}
func nullBoolPtr(v sql.NullBool) *bool {
	if v.Valid {
		return &v.Bool
	}
	return nil
}
