package product

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	entity "github.com/poshagator/content-service/internal/domain/entities/product"
	ucproduct "github.com/poshagator/content-service/internal/domain/usecase/product"
	productpb "github.com/poshagator/content-service/pkg/proto/product/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	productpb.UnimplementedProductServiceServer
	log     *zap.Logger
	usecase *ucproduct.Usecase
}

func NewHandler(log *zap.Logger, usecase *ucproduct.Usecase) *Handler {
	return &Handler{
		log:     log.Named("grpc.product"),
		usecase: usecase,
	}
}

func (h *Handler) SyncProducts(ctx context.Context, req *productpb.SyncProductsRequest) (*productpb.SyncProductsResponse, error) {
	filialID, err := parseFilialID(req.GetFilialId(), req.GetExternalFilialId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	input := entity.ProductSyncInput{
		FilialID: filialID,
		Source:   strings.TrimSpace(req.GetSource()),
		Mode:     syncMode(req.GetMode()),
	}
	if req.GetPayloadJson() != "" {
		if err = json.Unmarshal([]byte(req.GetPayloadJson()), &input); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid payload_json: %v", err)
		}
		input.FilialID = filialID
		if strings.TrimSpace(req.GetSource()) != "" {
			input.Source = strings.TrimSpace(req.GetSource())
		}
		if req.GetMode() != productpb.SyncMode_SYNC_MODE_UNSPECIFIED {
			input.Mode = syncMode(req.GetMode())
		}
		if err = applyPayloadAliases(req.GetPayloadJson(), &input); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid payload_json aliases: %v", err)
		}
	} else {
		input.Categories = categoriesFromProto(req.GetCategories())
	}

	stats, err := h.usecase.SyncProducts(ctx, input)
	if err != nil {
		h.log.Error("sync products failed", zap.String("filial_id", filialID.String()), zap.String("source", req.GetSource()), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "sync products failed: %v", err)
	}

	return &productpb.SyncProductsResponse{
		Success:            true,
		Message:            "products synced",
		CategoriesUpserted: stats.CategoriesUpserted,
		ProductsUpserted:   stats.ProductsUpserted,
		ProductsDisabled:   stats.ProductsDisabled,
	}, nil
}

func parseFilialID(filialID, externalFilialID string) (uuid.UUID, error) {
	raw := strings.TrimSpace(filialID)
	if raw == "" {
		raw = strings.TrimSpace(externalFilialID)
	}
	if raw == "" {
		return uuid.Nil, fmt.Errorf("filial_id required")
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("filial_id/external_filial_id must be UUID: %w", err)
	}
	return parsed, nil
}

func syncMode(mode productpb.SyncMode) entity.ProductSyncMode {
	if mode == productpb.SyncMode_SYNC_MODE_REPLACE_SOURCE {
		return entity.ProductSyncModeReplaceSource
	}
	return entity.ProductSyncModeUpsertOnly
}

func categoriesFromProto(categories []*productpb.ProductCategoryInput) []entity.ProductSyncCategory {
	out := make([]entity.ProductSyncCategory, 0, len(categories))
	for _, category := range categories {
		c := entity.ProductSyncCategory{
			ExternalID: category.GetExternalId(),
			Name:       category.GetName(),
			PhotoURL:   category.GetPhotoUrl(),
			Type:       category.GetType(),
			Products:   make([]entity.ProductSyncItem, 0, len(category.GetProducts())),
		}
		for _, product := range category.GetProducts() {
			statusValue := product.GetStatus()
			c.Products = append(c.Products, entity.ProductSyncItem{
				ExternalID: product.GetExternalId(),
				Title:      product.GetTitle(),
				Body:       product.GetBody(),
				BasePrice:  product.GetBasePrice(),
				Currency:   product.GetCurrency(),
				Weight:     product.GetWeight(),
				Status:     &statusValue,
				MediaUrls:  product.GetMediaUrls(),
			})
		}
		out = append(out, c)
	}
	return out
}

func applyPayloadAliases(raw string, input *entity.ProductSyncInput) error {
	var payload struct {
		Categories []struct {
			entity.ProductSyncCategory
			Items []entity.ProductSyncItem `json:"items"`
		} `json:"categories"`
		Items []struct {
			entity.ProductSyncCategory
			Items []entity.ProductSyncItem `json:"items"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return err
	}
	categories := payload.Categories
	if len(categories) == 0 {
		categories = payload.Items
	}
	if len(categories) == 0 {
		return nil
	}
	input.Categories = make([]entity.ProductSyncCategory, 0, len(categories))
	for _, category := range categories {
		c := category.ProductSyncCategory
		if len(c.Products) == 0 {
			c.Products = category.Items
		}
		input.Categories = append(input.Categories, c)
	}
	return nil
}
