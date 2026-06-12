package fuel

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Fuel struct {
	ID        uuid.UUID `db:"id" json:"id"`
	FilialID  uuid.UUID `db:"filial_id" json:"filial_id"`
	Name      string    `db:"name" json:"name"`
	Price     float64   `db:"price" json:"price"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type FuelDAO struct {
	ID        sql.NullString  `db:"id"`
	FilialID  sql.NullString  `db:"filial_id"`
	Name      sql.NullString  `db:"name"`
	Price     sql.NullFloat64 `db:"price"`
	CreatedAt sql.NullTime    `db:"created_at"`
	UpdatedAt sql.NullTime    `db:"updated_at"`
}

func (f *FuelDAO) ToFuel() *Fuel {
	return &Fuel{
		ID:        uuid.MustParse(f.ID.String),
		FilialID:  uuid.MustParse(f.FilialID.String),
		Name:      f.Name.String,
		Price:     f.Price.Float64,
		CreatedAt: f.CreatedAt.Time,
		UpdatedAt: f.UpdatedAt.Time,
	}
}
