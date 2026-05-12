package edu

import (
	"database/sql"
	"github.com/google/uuid"
)

type EduGroup struct {
	ID             uuid.UUID `json:"id" db:"id"`
	FilialID       uuid.UUID `json:"filial_id" db:"filial_id"`
	Name           string    `json:"name" db:"name"`
	Source         string    `json:"source" db:"source"`
	SourceGroupID  int       `json:"source_group_id" db:"source_group_id"`
	FacultyID      int       `json:"faculty_id" db:"faculty_id"`
	FacultyName    string    `json:"faculty_name" db:"faculty_name"`
	CourseID       int       `json:"course_id" db:"course_id"`
	CourseName     string    `json:"course_name" db:"course_name"`
	StudyFormID    int       `json:"study_form_id" db:"study_form_id"`
	StudyFormName  string    `json:"study_form_name" db:"study_form_name"`
	EducationLevel string    `json:"education_level" db:"education_level"`
	IsMagistracy   bool      `json:"is_magistracy" db:"is_magistracy"`
}

type EduGroupsDao []EduGroupDao

type EduGroupsByFaculty []EduFacultyGroup

type EduFacultyGroup struct {
	FacultyID      int              `json:"faculty_id"`
	FacultyName    string           `json:"faculty_name"`
	EducationLevel string           `json:"education_level"`
	IsMagistracy   bool             `json:"is_magistracy"`
	Courses        []EduCourseGroup `json:"courses"`
}

type EduCourseGroup struct {
	CourseID   int        `json:"course_id"`
	CourseName string     `json:"course_name"`
	Groups     []EduGroup `json:"groups"`
}

type EduGroupDao struct {
	ID             sql.NullString `db:"id" json:"id"`
	FilialID       sql.NullString `db:"filial_id" json:"filial_id"`
	Name           sql.NullString `db:"name" json:"name"`
	Source         sql.NullString `db:"source" json:"source"`
	SourceGroupID  sql.NullInt32  `db:"source_group_id" json:"source_group_id"`
	FacultyID      sql.NullInt32  `db:"faculty_id" json:"faculty_id"`
	FacultyName    sql.NullString `db:"faculty_name" json:"faculty_name"`
	CourseID       sql.NullInt32  `db:"course_id" json:"course_id"`
	CourseName     sql.NullString `db:"course_name" json:"course_name"`
	StudyFormID    sql.NullInt32  `db:"study_form_id" json:"study_form_id"`
	StudyFormName  sql.NullString `db:"study_form_name" json:"study_form_name"`
	EducationLevel sql.NullString `db:"education_level" json:"education_level"`
	IsMagistracy   sql.NullBool   `db:"is_magistracy" json:"is_magistracy"`
}

func (e *EduGroupDao) ToEduGroup() *EduGroup {
	eg := &EduGroup{
		Name:           e.Name.String,
		Source:         e.Source.String,
		SourceGroupID:  int(e.SourceGroupID.Int32),
		FacultyID:      int(e.FacultyID.Int32),
		FacultyName:    e.FacultyName.String,
		CourseID:       int(e.CourseID.Int32),
		CourseName:     e.CourseName.String,
		StudyFormID:    int(e.StudyFormID.Int32),
		StudyFormName:  e.StudyFormName.String,
		EducationLevel: e.EducationLevel.String,
		IsMagistracy:   e.IsMagistracy.Bool,
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
