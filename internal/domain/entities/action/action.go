package action

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type ExternalAction struct {
	ID        uuid.UUID `db:"id" json:"id"`
	FilialID  uuid.UUID `db:"filial_id" json:"filial_id"`
	Title     string    `db:"title" json:"title"`
	URL       string    `db:"url" json:"url"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type ExternalActionDAO struct {
	ID        sql.NullString `db:"id"`
	FilialID  sql.NullString `db:"filial_id"`
	Title     sql.NullString `db:"title"`
	URL       sql.NullString `db:"url"`
	CreatedAt sql.NullTime   `db:"created_at"`
	UpdatedAt sql.NullTime   `db:"updated_at"`
}

func (d *ExternalActionDAO) ToExternalAction() *ExternalAction {
	return &ExternalAction{
		ID:        uuid.MustParse(d.ID.String),
		FilialID:  uuid.MustParse(d.FilialID.String),
		Title:     d.Title.String,
		URL:       d.URL.String,
		CreatedAt: d.CreatedAt.Time,
		UpdatedAt: d.UpdatedAt.Time,
	}
}
