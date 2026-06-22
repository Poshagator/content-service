package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	entity "github.com/poshagator/content-service/internal/domain/entities/product"
)

func (u *Usecase) UpsertProduct(ctx context.Context, input entity.UpsertProductInput) (uuid.UUID, bool, error) {
	input, err := normalizeUpsertProductInput(input)
	if err != nil {
		return uuid.Nil, false, err
	}
	return u.repo.UpsertProduct(ctx, input)
}

func (u *Usecase) BatchUpsertProducts(ctx context.Context, input entity.BatchUpsertProductsInput) ([]uuid.UUID, error) {
	for i := range input.Products {
		norm, err := normalizeUpsertProductInput(input.Products[i])
		if err != nil {
			return nil, err
		}
		input.Products[i] = norm
	}
	return u.repo.BatchUpsertProducts(ctx, input)
}

func (u *Usecase) UpsertCategory(ctx context.Context, input entity.UpsertCategoryInput) (uuid.UUID, bool, error) {
	input, err := normalizeUpsertCategoryInput(input)
	if err != nil {
		return uuid.Nil, false, err
	}
	return u.repo.UpsertCategory(ctx, input)
}

func (u *Usecase) BatchUpsertCategories(ctx context.Context, input entity.BatchUpsertCategoriesInput) ([]uuid.UUID, error) {
	for i := range input.Categories {
		norm, err := normalizeUpsertCategoryInput(input.Categories[i])
		if err != nil {
			return nil, err
		}
		input.Categories[i] = norm
	}
	return u.repo.BatchUpsertCategories(ctx, input)
}

func normalizeUpsertProductInput(input entity.UpsertProductInput) (entity.UpsertProductInput, error) {
	input.Source = strings.TrimSpace(input.Source)
	if input.Source == "" {
		return input, fmt.Errorf("source required")
	}
	if input.FilialID == uuid.Nil {
		return input, fmt.Errorf("filial_id required")
	}
	
	item := &input.Product
	item.Title = strings.TrimSpace(item.Title)
	item.ExternalID = strings.TrimSpace(item.ExternalID)
	item.Body = strings.TrimSpace(item.Body)
	item.Currency = strings.TrimSpace(item.Currency)
	item.Weight = firstNonEmpty(strings.TrimSpace(item.Weight), strings.TrimSpace(item.Volume))
	
	if item.ExternalID == "" {
		catExt := ""
		if input.CategoryExternalID != nil {
			catExt = *input.CategoryExternalID
		}
		item.ExternalID = stableExternalID(input.Source, "product", catExt, item.Title, fmt.Sprint(item.BasePrice), item.Weight)
	}

	return input, nil
}

func normalizeUpsertCategoryInput(input entity.UpsertCategoryInput) (entity.UpsertCategoryInput, error) {
	input.Source = strings.TrimSpace(input.Source)
	if input.Source == "" {
		return input, fmt.Errorf("source required")
	}
	if input.FilialID == uuid.Nil {
		return input, fmt.Errorf("filial_id required")
	}

	cat := &input.Category
	cat.Name = strings.TrimSpace(cat.Name)
	cat.ExternalID = strings.TrimSpace(cat.ExternalID)

	if cat.ExternalID == "" {
		catType := ""
		if cat.Type != nil {
			catType = *cat.Type
		}
		cat.ExternalID = stableExternalID(input.Source, "category", catType, cat.Name)
	}

	return input, nil
}

func stableExternalID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])[:32]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
