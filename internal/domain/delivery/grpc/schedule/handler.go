package schedule

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

	stats, err := suruz.Sync(ctx, suruz.Config{
		APIBase:      payload.APIBase,
		DSN:          h.postgresDSN(),
		FilialID:     filialID,
		SourceIDs:    sourceIDs,
		SourceParam:  payload.SourceParam,
		LimitSources: payload.LimitSources,
		Timeout:      60 * time.Second,
		TermName:     payload.TermName,
	})
	if err != nil {
		h.log.Error("failed to sync Suruz schedule", zap.Error(err))
		return &schedule.SyncScheduleResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &schedule.SyncScheduleResponse{
		Success: true,
		Message: fmt.Sprintf(
			"Suruz sync complete: groups=%d teachers=%d schedules=%d fetch_errors=%d events=%d entries_created=%d entries_existing=%d",
			stats.Groups,
			stats.Teachers,
			stats.Schedules,
			stats.FetchErrors,
			stats.Events,
			stats.EntriesCreated,
			stats.EntriesExisting,
		),
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
