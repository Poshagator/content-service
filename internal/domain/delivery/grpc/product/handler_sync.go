package product

import (
	"context"

	actionEnt "github.com/poshagator/content-service/internal/domain/entities/action"
	fuelEnt "github.com/poshagator/content-service/internal/domain/entities/fuel"
	productpb "github.com/poshagator/content-service/pkg/proto/product/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) SyncFuels(ctx context.Context, req *productpb.SyncFuelsRequest) (*productpb.SyncFuelsResponse, error) {
	filialID, err := parseFilialID(req.GetFilialId(), "")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	inputs := make([]fuelEnt.Fuel, 0, len(req.GetFuels()))
	for _, f := range req.GetFuels() {
		inputs = append(inputs, fuelEnt.Fuel{
			Name:  f.GetName(),
			Price: f.GetPrice(),
		})
	}

	upserted, err := h.fuelUC.SyncFuels(ctx, filialID, inputs)
	if err != nil {
		h.log.Error("sync fuels failed", zap.String("filial_id", filialID.String()), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "sync fuels failed: %v", err)
	}

	return &productpb.SyncFuelsResponse{
		Success:       true,
		Message:       "fuels synced",
		FuelsUpserted: upserted,
	}, nil
}

func (h *Handler) SyncExternalActions(ctx context.Context, req *productpb.SyncExternalActionsRequest) (*productpb.SyncExternalActionsResponse, error) {
	filialID, err := parseFilialID(req.GetFilialId(), "")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	inputs := make([]actionEnt.ExternalAction, 0, len(req.GetActions()))
	for _, a := range req.GetActions() {
		inputs = append(inputs, actionEnt.ExternalAction{
			Title: a.GetTitle(),
			URL:   a.GetUrl(),
		})
	}

	upserted, err := h.actionUC.SyncExternalActions(ctx, filialID, inputs)
	if err != nil {
		h.log.Error("sync external actions failed", zap.String("filial_id", filialID.String()), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "sync external actions failed: %v", err)
	}

	return &productpb.SyncExternalActionsResponse{
		Success:         true,
		Message:         "actions synced",
		ActionsUpserted: upserted,
	}, nil
}
