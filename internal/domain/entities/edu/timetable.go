package edu

import (
	"github.com/google/uuid"
)

type TimetableEntry struct {
	ID          uuid.UUID `json:"id"`
	DayOfWeek   int       `json:"day_of_week"`
	OccursOn    string    `json:"occurs_on"`
	StartsAt    string    `json:"starts_at"`
	EndsAt      string    `json:"ends_at"`
	WeekType    string    `json:"week_type"`
	SubjectName string    `json:"subject_name"`
	TeacherName string    `json:"teacher_name"`
	RoomName    string    `json:"room_name"`
	IsExam      bool      `json:"is_exam"`
	Comment     string    `json:"comment"`
}

type TimetableEntries []TimetableEntry
