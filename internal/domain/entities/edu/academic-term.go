package edu

import (
	"database/sql"
	"github.com/google/uuid"
	"time"
)

type AcademicTerm struct {
	ID        uuid.UUID `json:"id"`
	FilialID  uuid.UUID `json:"filial_id"`
	Name      string    `json:"name"`
	StartsOn  time.Time `json:"starts_on"`
	EndsOn    time.Time `json:"ends_on"`
	WeekStart int       `json:"week_start"`
}

type AcademicTermsDao []AcademicTermDao
type AcademicTermDao struct {
	//Permission sql.NullString `db:"permission" json:"permission"`
	ID        sql.NullString `db:"id" json:"id"`
	FilialID  sql.NullString `db:"filial_id" json:"filial_id"`
	Name      sql.NullString `db:"name" json:"name"`
	StartsOn  sql.NullTime   `db:"starts_on" json:"starts_on"`
	EndsOn    sql.NullTime   `db:"ends_on" json:"ends_on"`
	WeekStart sql.NullInt32  `db:"week_start" json:"week_start"`
}

func (e *AcademicTermDao) ToAcademicTerm() *AcademicTerm {
	eg := &AcademicTerm{
		//ID: e.ID.String,
		//FilialID:  e.FilialID.String,
		//Permission: e.Permission.String,
		Name:      e.Name.String,
		StartsOn:  e.StartsOn.Time,
		EndsOn:    e.EndsOn.Time,
		WeekStart: int(e.WeekStart.Int32),
	}

	eg.FilialID, _ = uuid.Parse(e.FilialID.String)
	eg.ID, _ = uuid.Parse(e.ID.String)

	return eg
}

func (e AcademicTermsDao) ToAcademicTerms() []AcademicTerm {
	egs := make([]AcademicTerm, len(e))
	for i, eg := range e {
		egs[i] = *eg.ToAcademicTerm()
	}

	return egs
}
