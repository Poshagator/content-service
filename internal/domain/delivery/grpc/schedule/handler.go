package schedule

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/poshagator/content-service/config"
	"github.com/poshagator/content-service/internal/domain/usecase/edu"
	"github.com/poshagator/content-service/pkg/proto/schedule/gen"
	"github.com/poshagator/content-service/pkg/suruz"
	"go.uber.org/zap"
)

type Handler struct {
	schedule.UnimplementedScheduleServiceServer
	log        *zap.Logger
	cfg        *config.ConfigModel
	eduUsecase *edu.Usecase
}

func NewHandler(log *zap.Logger, cfg *config.ConfigModel, eduUsecase *edu.Usecase) *Handler {
	return &Handler{
		log:        log.Named("grpc.schedule"),
		cfg:        cfg,
		eduUsecase: eduUsecase,
	}
}

type syncPayload struct {
	APIBase      string `json:"api_base"`
	FilialID     string `json:"filial_id"`
	SourceIDs    string `json:"source_ids"`
	SourceParam  string `json:"source_param"`
	LimitSources int    `json:"limit_sources"`
	TermName     string `json:"term_name"`
}

func (h *Handler) SyncSchedule(ctx context.Context, req *schedule.SyncScheduleRequest) (*schedule.SyncScheduleResponse, error) {
	h.log.Info("sync schedule request received",
		zap.String("university_id", req.UniversityId),
		zap.String("group_id", req.GroupId),
		zap.String("parser_id", req.ParserId),
	)

	if req.ParserId != "" && req.ParserId != "suruz" {
		return &schedule.SyncScheduleResponse{
			Success: false,
			Message: "unsupported parser_id: " + req.ParserId,
		}, nil
	}

	payload := syncPayload{}
	if req.PayloadJson != "" {
		if err := json.Unmarshal([]byte(req.PayloadJson), &payload); err != nil {
			return &schedule.SyncScheduleResponse{
				Success: false,
				Message: "invalid payload_json: " + err.Error(),
			}, nil
		}
	}

	filialID := suruz.DefaultFilialID
	if req.UniversityId != "" {
		filialID = req.UniversityId
	}
	if payload.FilialID != "" {
		filialID = payload.FilialID
	}

	sourceIDs := req.GroupId
	if payload.SourceIDs != "" {
		sourceIDs = payload.SourceIDs
	}

	// Run sync in background to avoid gRPC timeouts
	go func() {
		// Use background context since the request context will be canceled
		bgCtx := context.Background()
		stats, err := suruz.Sync(bgCtx, suruz.Config{
			APIBase:      payload.APIBase,
			DSN:          h.postgresDSN(),
			FilialID:     filialID,
			SourceIDs:    sourceIDs,
			SourceParam:  payload.SourceParam,
			LimitSources: payload.LimitSources,
			Timeout:      60 * time.Second,
			TermName:     payload.TermName,
			Logger:       h.log,
		})
		if err != nil {
			h.log.Error("background Suruz sync failed", zap.Error(err))
			return
		}

		h.log.Info("background Suruz sync complete",
			zap.Int("groups", stats.Groups),
			zap.Int("teachers", stats.Teachers),
			zap.Int("schedules", stats.Schedules),
			zap.Int("events", stats.Events),
			zap.Int("affectedGroups", len(stats.AffectedGroupIDs)),
		)

		for _, rawGroupID := range stats.AffectedGroupIDs {
			groupID, err := uuid.Parse(rawGroupID)
			if err != nil {
				h.log.Warn("skip planner source sync for invalid group id", zap.String("groupID", rawGroupID), zap.Error(err))
				continue
			}
			if err := h.eduUsecase.SyncPlannerSourceForGroup(bgCtx, groupID); err != nil {
				h.log.Warn("failed to refresh planner source after Suruz sync", zap.String("groupID", rawGroupID), zap.Error(err))
			}
		}
	}()

	return &schedule.SyncScheduleResponse{
		Success: true,
		Message: "Suruz sync started in background. Check service logs for progress.",
	}, nil
}

func (h *Handler) postgresDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		h.cfg.Postgres.User,
		h.cfg.Postgres.Password,
		h.cfg.Postgres.Host,
		h.cfg.Postgres.Port,
		h.cfg.Postgres.DBName,
		h.cfg.Postgres.SSLMode,
	)
}
