package schedule

import (
	"context"

	"github.com/poshagator/content-service/internal/domain/usecase/edu"
	"github.com/poshagator/content-service/pkg/api/grpc/schedule"
	"go.uber.org/zap"
)

type Handler struct {
	schedule.UnimplementedScheduleServiceServer
	log        *zap.Logger
	eduUsecase *edu.Usecase
}

func NewHandler(log *zap.Logger, eduUsecase *edu.Usecase) *Handler {
	return &Handler{
		log:        log.Named("grpc.schedule"),
		eduUsecase: eduUsecase,
	}
}

func (h *Handler) SyncSchedule(ctx context.Context, req *schedule.SyncScheduleRequest) (*schedule.SyncScheduleResponse, error) {
	h.log.Info("sync schedule request received",
		zap.String("university_id", req.UniversityId),
		zap.String("group_id", req.GroupId),
		zap.String("parser_id", req.ParserId),
	)

	// TODO: Implement actual synchronization logic in eduUsecase

	return &schedule.SyncScheduleResponse{
		Success: true,
		Message: "Schedule sync initiated",
	}, nil
}
