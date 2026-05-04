package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/poshagator/content-service/config"
	"github.com/poshagator/content-service/internal/domain/entities"
)

// -----------------------------------------------------------------------------
// Repository
// -----------------------------------------------------------------------------

type Repository struct {
	ctx context.Context
	log *zap.Logger
	cfg *config.ConfigModel
	db  *pgxpool.Pool
}

// NewRepository returns a Repo instance ready to be plugged into an Fx graph.
func NewRepository(l *zap.Logger, cfg *config.ConfigModel, ctx context.Context) (*Repository, error) {
	return &Repository{ctx: ctx, log: l.Named("repo.pg"), cfg: cfg}, nil
}

// OnStart — Fx Lifecycle hook: opens a pgx connection-pool (with retries).
func (r *Repository) OnStart(_ context.Context) error {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		r.cfg.Postgres.Host,
		r.cfg.Postgres.Port,
		r.cfg.Postgres.User,
		r.cfg.Postgres.Password,
		r.cfg.Postgres.DBName,
		r.cfg.Postgres.SSLMode,
	)

	const tries = 5
	for i := 0; i < tries; i++ {
		if db, err := pgxpool.New(r.ctx, dsn); err == nil {
			r.db = db
			r.log.Info("PostgreSQL connected", zap.Int("try", i+1))
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("postgres: connection retries exceeded")
}

func (r *Repository) OnStop(_ context.Context) error {
	if r.db != nil {
		r.db.Close()
	}
	return nil
}

// -----------------------------------------------------------------------------
// SINGLE PRODUCT
// -----------------------------------------------------------------------------

// Вытаскиваем продукт вместе с category_name и media_urls, чтобы совпадало с ProductDAO.
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
    p.permission,
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

func (r *Repository) GetProduct(ctx context.Context, id uuid.UUID) (*entities.Product, error) {
	rows, err := r.db.Query(ctx, qGetProduct, id)
	if err != nil {
		r.log.Error("failed to get product", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	dao, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.ProductDAO])
	if err != nil {
		r.log.Error("failed to collect product", zap.Error(err))
		return nil, err
	}
	product := dao.ToProduct()

	// Подтягиваем модификаторы
	if err := r.attachModifiers(ctx, []entities.Product{*product}); err != nil {
		r.log.Warn("modifiers not attached", zap.Error(err))
	}

	return product, nil
}

// -----------------------------------------------------------------------------
// CREATE / UPDATE / DELETE PRODUCT
// -----------------------------------------------------------------------------

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

func (r *Repository) CreateProduct(ctx context.Context, p *entities.Product) error {
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

func (r *Repository) UpdateProduct(ctx context.Context, product *entities.Product) error {
	_, err := r.db.Exec(ctx, qUpdateProduct,
		product.FilialID, product.CategoryID, product.Title, product.Body,
		product.Status, product.BasePrice, product.Currency, product.Weight,
		product.CreatedBy, product.ID,
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

// -----------------------------------------------------------------------------
// LIST PRODUCTS BY FILIAL (с пагинацией)
// -----------------------------------------------------------------------------

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
    p.permission,
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
) ([]entities.Product, error) {

	rows, err := r.db.Query(ctx, qGetProducts+sortQuery, filialID)
	if err != nil {
		r.log.Error("failed to get products", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	productDAOs, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.ProductDAO])
	if err != nil {
		r.log.Error("failed to collect products", zap.Error(err))
		return nil, err
	}

	products := entities.ProductsDAO(productDAOs).ToProducts()

	if err := r.attachModifiers(ctx, products); err != nil {
		r.log.Warn("modifiers not attached", zap.Error(err))
	}

	return products, nil
}

// -----------------------------------------------------------------------------
// FILIAL ENSURE
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// PRODUCT MODIFIERS (по продуктам)
// -----------------------------------------------------------------------------
//
// Теперь группы филиальные: добавляем pmg.filial_id в выборку, чтобы заполнить
// entities.ProductModifierGroup.FilialID.
//

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

func (r *Repository) attachModifiers(ctx context.Context, products []entities.Product) error {
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
	cache := make(map[uuid.UUID]map[uuid.UUID]*entities.ProductModifierGroup)

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
			cache[rrow.ProductID] = make(map[uuid.UUID]*entities.ProductModifierGroup)
		}
		g, ok := cache[rrow.ProductID][rrow.GroupID]
		if !ok {
			g = &entities.ProductModifierGroup{
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

		opt := entities.ProductModifierOption{
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
			p.Modifiers = make([]entities.ProductModifierGroup, 0, len(groups))
			for _, g := range groups {
				p.Modifiers = append(p.Modifiers, *g)
			}
			products[i] = p
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// FILIAL MODIFIERS
// -----------------------------------------------------------------------------
//
// Возвращаем **все** модификаторные группы, принадлежащие филиалу (pmg.filial_id),
// с их опциями. Поля Required/Min/Max не заполняем (0 / false), потому что это
// филиальная карточка без продуктового контекста.
//

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
) ([]entities.ProductModifierGroup, error) {

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

	cache := make(map[uuid.UUID]*entities.ProductModifierGroup)

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
			g = &entities.ProductModifierGroup{
				ID:       rrow.GroupID,
				FilialID: rrow.GroupFilialID,
				Name:     rrow.GroupName,
				Type:     rrow.GroupType,
				Kind:     rrow.KindCode,
			}
			cache[rrow.GroupID] = g
		}
		g.Options = append(g.Options, entities.ProductModifierOption{
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

	out := make([]entities.ProductModifierGroup, 0, len(cache))
	for _, g := range cache {
		out = append(out, *g)
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// PERMISSIONS
// -----------------------------------------------------------------------------

func normalizeDefaultExpr(expr string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}
	// column_default обычно в одинарных кавычках, берём всё между первой и второй
	if strings.HasPrefix(expr, "'") {
		parts := strings.Split(expr, "'")
		if len(parts) >= 3 {
			return parts[1]
		}
	}
	// на всякий случай
	return strings.Trim(expr, "'")
}

// getDefaultPermission возвращает DEFAULT permission для table.permission.
func (r *Repository) getDefaultPermission(ctx context.Context, table string) (string, error) {
	const q = `
SELECT column_default
FROM information_schema.columns
WHERE table_schema='public' AND table_name=$1 AND column_name='permission'
LIMIT 1;`
	var defExpr *string
	if err := r.db.QueryRow(ctx, q, table).Scan(&defExpr); err != nil {
		return "", err
	}
	if defExpr == nil {
		return "", nil
	}
	return normalizeDefaultExpr(*defExpr), nil
}

// getRowPermission возвращает permission для конкретной строки или "" если не найдено.
func (r *Repository) getRowPermission(ctx context.Context, table, whereClause string, args ...any) (string, error) {
	query := fmt.Sprintf("SELECT permission FROM public.%s %s LIMIT 1", table, whereClause)
	var perm string
	err := r.db.QueryRow(ctx, query, args...).Scan(&perm)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return perm, nil
}

// EnsurePermission проверяет, что фактическое (или дефолтное при отсутствии строки)
// разрешение присутствует в headerPerms. Возвращает PermissionDeniedError при запрете.
func (r *Repository) EnsurePermission(
	ctx context.Context,
	table string,
	whereClause string,
	headerPerms map[string]struct{},
	args ...any,
) error {
	perm, err := r.getRowPermission(ctx, table, whereClause, args...)
	if err != nil {
		return err
	}
	if perm == "" {
		// строки нет: берём DEFAULT таблицы
		if perm, err = r.getDefaultPermission(ctx, table); err != nil {
			return err
		}
	}
	if perm == "" {
		// ни строки, ни дефолта — запрещаем
		return ErrPermissionDenied{Needed: fmt.Sprintf("%s(permission)", table)}
	}
	if _, ok := headerPerms[perm]; !ok {
		return ErrPermissionDenied{Needed: perm}
	}
	return nil
}

// ErrPermissionDenied используется для унифицированной обработки.
type ErrPermissionDenied struct {
	Needed string
}

func (e ErrPermissionDenied) Error() string {
	return fmt.Sprintf("permission denied: need %s", e.Needed)
}

// -----------------------------------------------------------------------------
// ATTACH MODIFIER GROUP TO PRODUCT
// -----------------------------------------------------------------------------

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

// checkProductAndGroupFilial возвращает филиалы продукта и группы; nil, если сущность не найдена.
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
	link *entities.ProductModifierGroupLink,
	headerPerms map[string]struct{},
) error {
	// Табличное permission на вставку/апдейт.
	if err := r.EnsureCreatePermission(ctx, "product_modifier_group_link", headerPerms); err != nil {
		return status.Error(codes.PermissionDenied, err.Error())
	}

	// Ранняя проверка консистентности филиалов (понятная ошибка до триггера).
	pf, gf, err := r.checkProductAndGroupFilial(ctx, link.ProductID, link.GroupID)
	if err != nil {
		r.log.Error("AttachModifierGroup check filials", zap.Error(err))
		return err
	}
	if pf == nil { // продукт не найден
		return status.Errorf(codes.NotFound, "product %s not found", link.ProductID)
	}
	if gf == nil { // группа не найдена
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

	// Upsert.
	_, err = r.db.Exec(ctx, qAttachProductModifierGroup,
		link.ProductID,
		link.GroupID,
		link.SortOrder,
		link.Required,
		link.MinSelect,
		link.MaxSelect,
	)
	if err != nil {
		// На всякий случай маппим pg триггер (check_violation 23514) в InvalidArgument.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return status.Errorf(codes.InvalidArgument, "filial mismatch: %s", pgErr.Message)
		}
		r.log.Error("attach modifier", zap.Error(err))
		return err
	}
	return nil
}

func (r *Repository) EnsureCreatePermission(
	ctx context.Context,
	table string,
	headerPerms map[string]struct{},
) error {
	perm, err := r.getDefaultPermission(ctx, table)
	if err != nil {
		return err
	}
	if perm == "" {
		// нет дефолта — трактуем как закрытую таблицу
		return ErrPermissionDenied{Needed: fmt.Sprintf("%s(permission:default)", table)}
	}
	if _, ok := headerPerms[perm]; !ok {
		return ErrPermissionDenied{Needed: perm}
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

// вставка филиальной группы
const qInsertModifierGroup = `
INSERT INTO public.product_modifier_group (filial_id, name, kind_id)
VALUES ($1, $2, $3)
RETURNING id, filial_id, name, kind_id, permission, created_at, updated_at;
`

// обновление (filial_id неизменяем)
const qUpdateModifierGroup = `
UPDATE public.product_modifier_group
SET name=$2,
    kind_id=$3,
    updated_at=NOW()
WHERE id=$1
RETURNING id, filial_id, name, kind_id, permission, created_at, updated_at;
`

func (r *Repository) CreateModifierGroup(
	ctx context.Context,
	g *entities.ProductModifierGroup,
	headerPerms map[string]struct{},
) error {
	if err := r.EnsureCreatePermission(ctx, "product_modifier_group", headerPerms); err != nil {
		return status.Error(codes.PermissionDenied, err.Error())
	}

	// убедимся, что филиал существует
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

	// 1. вставляем саму группу (филиальную)
	if err = tx.QueryRow(ctx, qInsertModifierGroup, g.FilialID, g.Name, kindID).
		Scan(&g.ID, &g.FilialID, &g.Name, new(uuid.UUID), new(string), new(time.Time), new(time.Time)); err != nil {
		return err
	}

	// 2. если есть опции -- вставляем
	if len(g.Options) > 0 {
		if err = r.insertGroupOptions(ctx, tx, g.ID, g.Type, g.Options); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) UpdateModifierGroup(
	ctx context.Context,
	g *entities.ProductModifierGroup,
	headerPerms map[string]struct{},
) error {
	if err := r.EnsurePermission(ctx, "product_modifier_group", "WHERE id=$1", headerPerms, g.ID); err != nil {
		return status.Error(codes.PermissionDenied, err.Error())
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

	// 1. обновляем поля группы (filial_id не трогаем; читаем обратно для консистентности)
	if err = tx.QueryRow(ctx, qUpdateModifierGroup, g.ID, g.Name, kindID).
		Scan(new(uuid.UUID), &g.FilialID, &g.Name, new(uuid.UUID), new(string), new(time.Time), new(time.Time)); err != nil {
		return err
	}

	// 2. пере-создаём опции (проще всего так)
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
	headerPerms map[string]struct{},
) error {
	// row-level permission (по id); сначала узнаём, есть ли строка.
	gFilial, err := r.getGroupFilial(ctx, id)
	if err != nil {
		return err
	}
	if gFilial == nil {
		return status.Errorf(codes.NotFound, "modifier group %s not found", id)
	}
	// контроль филиала из payload
	if *gFilial != filialID {
		return status.Errorf(codes.NotFound, "modifier group %s not found in filial %s", id, filialID)
	}

	// проверка permissions (по строке)
	if err := r.EnsurePermission(ctx, "product_modifier_group", "WHERE id=$1", headerPerms, id); err != nil {
		var perr ErrPermissionDenied
		if errors.As(err, &perr) {
			return status.Error(codes.PermissionDenied, perr.Error())
		}
		return err
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
	opts []entities.ProductModifierOption,
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
		opts[i].ID = o.ID // отдаём caller-у заполненные ID
	}
	return nil
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
    p.permission,
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
) ([]entities.Product, error) {

	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, qGetProductsByIDs, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	daos, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.ProductDAO])
	if err != nil {
		return nil, err
	}
	products := entities.ProductsDAO(daos).ToProducts()
	if err := r.attachModifiers(ctx, products); err != nil {
		r.log.Warn("attachModifiers", zap.Error(err))
	}
	return products, nil
}
