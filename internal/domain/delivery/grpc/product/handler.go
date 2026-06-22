package product

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	entity "github.com/poshagator/content-service/internal/domain/entities/product"
	ucaction "github.com/poshagator/content-service/internal/domain/usecase/action"
	ucfuel "github.com/poshagator/content-service/internal/domain/usecase/fuel"
	ucproduct "github.com/poshagator/content-service/internal/domain/usecase/product"
	productpb "github.com/poshagator/content-service/pkg/proto/product/gen"
)

type Handler struct {
	productpb.UnimplementedProductServiceServer
	log      *zap.Logger
	usecase  *ucproduct.Usecase
	actionUC *ucaction.Usecase
	fuelUC   *ucfuel.Usecase
}

func NewHandler(log *zap.Logger, usecase *ucproduct.Usecase, actionUC *ucaction.Usecase, fuelUC *ucfuel.Usecase) *Handler {
	return &Handler{
		log:      log.Named("grpc.product"),
		usecase:  usecase,
		actionUC: actionUC,
		fuelUC:   fuelUC,
	}
}

// ======================= Products =======================

func (h *Handler) UpsertProduct(ctx context.Context, req *productpb.UpsertProductRequest) (*productpb.UpsertProductResponse, error) {
	if req == nil || req.Product == nil {
		return nil, status.Error(codes.InvalidArgument, "request and product cannot be nil")
	}

	in, err := mapUpsertProductRequest(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	id, created, err := h.usecase.UpsertProduct(ctx, in)
	if err != nil {
		h.log.Error("UpsertProduct failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "UpsertProduct failed: %v", err)
	}

	return &productpb.UpsertProductResponse{
		ProductId: id.String(),
		Created:   created,
	}, nil
}

func (h *Handler) BatchUpsertProducts(ctx context.Context, req *productpb.BatchUpsertProductsRequest) (*productpb.BatchUpsertProductsResponse, error) {
	if req == nil || len(req.Products) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty products list")
	}

	var inputs []entity.UpsertProductInput
	for _, p := range req.Products {
		in, err := mapUpsertProductRequest(p)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid product %v: %v", p.Product.ExternalId, err)
		}
		inputs = append(inputs, in)
	}

	ids, err := h.usecase.BatchUpsertProducts(ctx, entity.BatchUpsertProductsInput{Products: inputs})
	if err != nil {
		h.log.Error("BatchUpsertProducts failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "BatchUpsertProducts failed: %v", err)
	}

	resp := &productpb.BatchUpsertProductsResponse{}
	for _, id := range ids {
		resp.Products = append(resp.Products, &productpb.UpsertProductResponse{
			ProductId: id.String(),
			// In batch we don't return 'created' accurately right now for simplicity, or we could if usecase returns it.
			Created:   false,
		})
	}
	return resp, nil
}

// ======================= Categories =======================

func (h *Handler) UpsertCategory(ctx context.Context, req *productpb.UpsertCategoryRequest) (*productpb.UpsertCategoryResponse, error) {
	if req == nil || req.Category == nil {
		return nil, status.Error(codes.InvalidArgument, "request and category cannot be nil")
	}

	in, err := mapUpsertCategoryRequest(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	id, created, err := h.usecase.UpsertCategory(ctx, in)
	if err != nil {
		h.log.Error("UpsertCategory failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "UpsertCategory failed: %v", err)
	}

	return &productpb.UpsertCategoryResponse{
		CategoryId: id.String(),
		Created:    created,
	}, nil
}

func (h *Handler) BatchUpsertCategories(ctx context.Context, req *productpb.BatchUpsertCategoriesRequest) (*productpb.BatchUpsertCategoriesResponse, error) {
	if req == nil || len(req.Categories) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty categories list")
	}

	var inputs []entity.UpsertCategoryInput
	for _, c := range req.Categories {
		in, err := mapUpsertCategoryRequest(c)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid category: %v", err)
		}
		inputs = append(inputs, in)
	}

	ids, err := h.usecase.BatchUpsertCategories(ctx, entity.BatchUpsertCategoriesInput{Categories: inputs})
	if err != nil {
		h.log.Error("BatchUpsertCategories failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "BatchUpsertCategories failed: %v", err)
	}

	resp := &productpb.BatchUpsertCategoriesResponse{}
	for _, id := range ids {
		resp.Categories = append(resp.Categories, &productpb.UpsertCategoryResponse{
			CategoryId: id.String(),
			Created:    false,
		})
	}
	return resp, nil
}

// ======================= Fuels & Actions =======================

func (h *Handler) UpsertFuel(ctx context.Context, req *productpb.UpsertFuelRequest) (*productpb.UpsertFuelResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (h *Handler) BatchUpsertFuels(ctx context.Context, req *productpb.BatchUpsertFuelsRequest) (*productpb.BatchUpsertFuelsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (h *Handler) UpsertExternalAction(ctx context.Context, req *productpb.UpsertExternalActionRequest) (*productpb.UpsertExternalActionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (h *Handler) BatchUpsertExternalActions(ctx context.Context, req *productpb.BatchUpsertExternalActionsRequest) (*productpb.BatchUpsertExternalActionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

// ======================= Mappers =======================

func mapUpsertProductRequest(req *productpb.UpsertProductRequest) (entity.UpsertProductInput, error) {
	filialID, err := uuid.Parse(req.FilialId)
	if err != nil {
		return entity.UpsertProductInput{}, fmt.Errorf("invalid filial_id: %v", err)
	}
	
	var catID *uuid.UUID
	if req.CategoryId != nil && *req.CategoryId != "" {
		id, err := uuid.Parse(*req.CategoryId)
		if err == nil {
			catID = &id
		}
	}
	
	var statusVal *bool
	if req.Product.Status != nil {
		b := *req.Product.Status
		statusVal = &b
	}

	return entity.UpsertProductInput{
		FilialID: filialID,
		Source:   req.Source,
		Product: entity.ProductInput{
			ExternalID: req.Product.ExternalId,
			Title:      req.Product.Title,
			Body:       req.Product.Body,
			BasePrice:  req.Product.BasePrice,
			Currency:   req.Product.Currency,
			Weight:     req.Product.Weight,
			Volume:     req.Product.Volume,
			Status:     statusVal,
			MediaUrls:  req.Product.MediaUrls,
		},
		CategoryExternalID: req.CategoryExternalId,
		CategoryName:       req.CategoryName,
		CategoryID:         catID,
	}, nil
}

func mapUpsertCategoryRequest(req *productpb.UpsertCategoryRequest) (entity.UpsertCategoryInput, error) {
	filialID, err := uuid.Parse(req.FilialId)
	if err != nil {
		return entity.UpsertCategoryInput{}, fmt.Errorf("invalid filial_id: %v", err)
	}

	return entity.UpsertCategoryInput{
		FilialID: filialID,
		Source:   req.Source,
		Category: entity.CategoryInput{
			ExternalID: req.Category.ExternalId,
			Name:       req.Category.Name,
			PhotoURL:   req.Category.PhotoUrl,
			Type:       req.Category.Type,
		},
	}, nil
}
