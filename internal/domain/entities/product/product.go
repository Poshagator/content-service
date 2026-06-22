package product

import (
	"database/sql"
	"github.com/google/uuid"
	"time"
)

func nullUUID(ns sql.NullString) uuid.UUID {
	if ns.Valid && ns.String != "" {
		if u, err := uuid.Parse(ns.String); err == nil {
			return u
		}
	}
	return uuid.Nil
}

type ProductModifierOption struct {
	ID   uuid.UUID `json:"id"`
	Type string    `json:"type"`

	Name          string   `json:"name"`
	PriceDelta    *float64 `json:"price_delta,omitempty"`
	WeightDelta   *float64 `json:"weight_delta,omitempty"`
	MaxQty        *int     `json:"max_qty,omitempty"`
	PricePerUnit  *float64 `json:"price_per_unit,omitempty"`
	WeightPerUnit *float64 `json:"weight_per_unit,omitempty"`
	Step          *float64 `json:"step,omitempty"`
	MinQty        *int     `json:"min_qty,omitempty"`
	DefaultState  *bool    `json:"default_state,omitempty"`

	SortOrder int `json:"sort_order"`
}

type ProductModifierKind struct {
	ID        uuid.UUID `json:"id"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProductModifierGroup struct {
	ID        uuid.UUID               `json:"id"`
	FilialID  uuid.UUID               `json:"filial_id"`
	Name      string                  `json:"name"`
	Type      string                  `json:"type"`
	KindID    *uuid.UUID              `json:"-"`
	Kind      string                  `json:"kind"`
	Required  bool                    `json:"required"`
	MinSelect int                     `json:"min_select"`
	MaxSelect int                     `json:"max_select"`
	Options   []ProductModifierOption `json:"options"`
}

type CategoryGroup struct {
	CategoryID    uuid.UUID `json:"category_id"`
	CategoryName  string    `json:"category_name"`
	CategoryImage string    `json:"category_image"`
	Products      []Product `json:"products"`
}

type ProductCategory struct {
	ID        uuid.UUID `db:"id" json:"id"`
	FilialID  uuid.UUID `db:"filial_id" json:"filial_id"`
	Name      string    `db:"name" json:"name"`
	PhotoURL  string    `db:"photo_url" json:"photo_url"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type Product struct {
	ID            uuid.UUID              `db:"id" json:"id"`
	FilialID      uuid.UUID              `db:"filial_id" json:"filial_id"`
	CategoryID    uuid.UUID              `db:"category_id" json:"category_id"`
	CategoryName  string                 `db:"category_name" json:"category"`
	CategoryImage string                 `db:"category_image" json:"category_image"`
	Title         string                 `db:"title" json:"title"`
	Body          string                 `db:"body" json:"body"`
	Status        bool                   `db:"status" json:"status"`
	BasePrice     float64                `db:"base_price" json:"base_price"`
	Currency      string                 `db:"currency" json:"currency"`
	Weight        string                 `db:"weight" json:"weight"`
	CreatedBy     uuid.UUID              `db:"created_by" json:"created_by"`
	Modifiers     []ProductModifierGroup `json:"modifiers"`
	CreatedAt     time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time              `db:"updated_at" json:"updated_at"`
	MediaUrls     []string               `db:"media_urls" json:"media_urls"`
}

type UpsertProductInput struct {
	FilialID           uuid.UUID
	Source             string
	Product            ProductInput
	CategoryExternalID *string
	CategoryName       *string
	CategoryID         *uuid.UUID
}

type ProductInput struct {
	ExternalID string   `json:"external_id"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	BasePrice  float64  `json:"base_price"`
	Currency   string   `json:"currency"`
	Weight     string   `json:"weight"`
	Volume     string   `json:"volume"`
	Status     *bool    `json:"status"`
	MediaUrls  []string `json:"media_urls"`
}

type UpsertCategoryInput struct {
	FilialID uuid.UUID
	Source   string
	Category CategoryInput
}

type CategoryInput struct {
	ExternalID string  `json:"external_id"`
	Name       string  `json:"name"`
	PhotoURL   *string `json:"photo_url"`
	Type       *string `json:"type"`
}

type BatchUpsertProductsInput struct {
	Products []UpsertProductInput
}

type BatchUpsertCategoriesInput struct {
	Categories []UpsertCategoryInput
}

type ProductSyncStats struct {
	CategoriesUpserted int64 `json:"categories_upserted"`
	ProductsUpserted   int64 `json:"products_upserted"`
	ProductsDisabled   int64 `json:"products_disabled"`
}

type ProductsDAO []ProductDAO
type ProductDAO struct {
	ID            sql.NullString  `db:"id" json:"id"`
	FilialID      sql.NullString  `db:"filial_id" json:"filial_id"`
	CategoryID    sql.NullString  `db:"category_id" json:"category_id"`
	CategoryName  sql.NullString  `db:"category_name"`
	CategoryImage sql.NullString  `db:"category_image"`
	Title         sql.NullString  `db:"title" json:"title"`
	Body          sql.NullString  `db:"body" json:"body"`
	Status        sql.NullBool    `db:"status" json:"status"`
	BasePrice     sql.NullFloat64 `db:"base_price" json:"base_price"`
	Currency      sql.NullString  `db:"currency" json:"currency"`
	Weight        sql.NullString  `db:"weight" json:"weight"`
	CreatedBy     sql.NullString  `db:"created_by" json:"created_by"`
	CreatedAt     sql.NullTime    `db:"created_at" json:"created_at"`
	UpdatedAt     sql.NullTime    `db:"updated_at" json:"updated_at"`
	MediaUrls     []string        `db:"media_urls" json:"media_urls"`
}

func (p *ProductDAO) ToProduct() *Product {
	return &Product{
		ID:            uuid.MustParse(p.ID.String),
		FilialID:      uuid.MustParse(p.FilialID.String),
		CategoryID:    uuid.MustParse(p.CategoryID.String),
		CategoryName:  p.CategoryName.String,
		CategoryImage: p.CategoryImage.String,
		Title:         p.Title.String,
		Body:          p.Body.String,
		Status:        p.Status.Bool,
		BasePrice:     p.BasePrice.Float64,
		Currency:      p.Currency.String,
		Weight:        p.Weight.String,
		CreatedBy:     nullUUID(p.CreatedBy),
		CreatedAt:     p.CreatedAt.Time,
		UpdatedAt:     p.UpdatedAt.Time,
		MediaUrls:     p.MediaUrls,
	}
}

func (p ProductsDAO) ToProducts() []Product {
	products := make([]Product, len(p))
	for i, product := range p {
		products[i] = *product.ToProduct()
	}

	return products
}

type ProductModifierGroupLink struct {
	ProductID uuid.UUID `db:"product_id" json:"product_id"`
	GroupID   uuid.UUID `db:"group_id" json:"group_id"`
	SortOrder int       `db:"sort_order" json:"sort_order"`
	Required  bool      `db:"required" json:"required"`
	MinSelect int       `db:"min_select" json:"min_select"`
	MaxSelect int       `db:"max_select" json:"max_select"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type ProductModifierOptionLink struct {
	ProductID   uuid.UUID `db:"product_id" json:"product_id"`
	OptionID    uuid.UUID `db:"option_id" json:"option_id"`
	PriceDelta  float64   `db:"price_delta" json:"price_delta"`
	WeightDelta float64   `db:"weight_delta" json:"weight_delta"`
	MaxQty      int       `db:"max_qty" json:"max_qty"`
}

type ProductMedia struct {
	ID        uuid.UUID `db:"id" json:"id"`
	ProductID uuid.UUID `db:"product_id" json:"product_id"`
	FileID    uuid.UUID `db:"file_id" json:"file_id"`
}

type ModifierGroupsByType struct {
	Single  []ProductModifierGroup `json:"single"`
	Toggle  []ProductModifierGroup `json:"toggle"`
	Counter []ProductModifierGroup `json:"counter"`
	Multi   []ProductModifierGroup `json:"multi"`
}

type OrderItemInput struct {
	ProductID     uuid.UUID `json:"product_id"`
	Qty           int       `json:"qty"`
	ModifiersJSON string    `json:"modifiers_json,omitempty"`
}

type Order struct {
	ID         uuid.UUID `json:"id"`
	FilialID   uuid.UUID `json:"filial_id"`
	Status     string    `json:"status"`
	TotalPrice float64   `json:"total_price,omitempty"`
	Currency   string    `json:"currency,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type DeliveryInfoInput struct {
	AddressID  uuid.UUID `json:"address_id"`
	DeliveryAt time.Time `json:"delivery_time"` // RFC3339 в JSON
}
type TakeawayInfoInput struct {
	PickupAt time.Time `json:"pickup_time"` // RFC3339
}
type TableInfoInput struct {
	TableNumber string `json:"table_number"`
}

type OrderInput struct {
	FilialID uuid.UUID        `json:"filial_id"`
	UserID   uuid.UUID        `json:"-"`
	TypeCode string           `json:"type_code"` // delivery|takeaway|table
	Items    []OrderItemInput `json:"items"`
	Comment  string           `json:"comment,omitempty"`

	Delivery *DeliveryInfoInput `json:"delivery,omitempty"`
	Takeaway *TakeawayInfoInput `json:"takeaway,omitempty"`
	Table    *TableInfoInput    `json:"table,omitempty"`
}

type OrderItemDescription struct {
	ProductID uuid.UUID `json:"product_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	Modifiers []string  `json:"modifiers,omitempty"`
}
