package edu

import (
	"context"
	"github.com/google/uuid"
	"github.com/poshagator/content-service/internal/domain/entities/edu"
)

func (u *Usecase) GetTimetable(ctx context.Context, groupID uuid.UUID) ([]edu.TimetableEntry, error) {
	return u.repo.GetTimetable(ctx, groupID)
}
