package edu

import (
	"database/sql"
	"github.com/google/uuid"
)

type EduGroup struct {
	ID         uuid.UUID `json:"id" db:"id"`
	Permission string    `json:"permission" db:"-"`
	FilialID   uuid.UUID `json:"filial_id" db:"filial_id"`
	Name       string    `json:"name" db:"name"`
}

type EduGroupsDao []EduGroupDao
type EduGroupDao struct {
	ID         sql.NullString `db:"id" json:"id"`
	Permission sql.NullString `db:"permission" json:"permission"`
	FilialID   sql.NullString `db:"filial_id" json:"filial_id"`
	Name       sql.NullString `db:"name" json:"name"`
}

func (e *EduGroupDao) ToEduGroup() *EduGroup {
	eg := &EduGroup{
		Permission: e.Permission.String,
		Name:       e.Name.String,
	}

	eg.FilialID, _ = uuid.Parse(e.FilialID.String)
	eg.ID, _ = uuid.Parse(e.ID.String)

	return eg
}

func (e EduGroupsDao) ToEduGroups() []EduGroup {
	egs := make([]EduGroup, len(e))
	for i, eg := range e {
		egs[i] = *eg.ToEduGroup()
	}

	return egs
}
