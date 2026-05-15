package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	entity "github.com/poshagator/content-service/internal/domain/entities/product"
)

func (u *Usecase) SyncProducts(ctx context.Context, input entity.ProductSyncInput) (entity.ProductSyncStats, error) {
	normalized, err := normalizeProductSyncInput(input)
	if err != nil {
		return entity.ProductSyncStats{}, err
	}
	return u.repo.SyncProducts(ctx, normalized)
}

func normalizeProductSyncInput(input entity.ProductSyncInput) (entity.ProductSyncInput, error) {
	input.Source = strings.TrimSpace(input.Source)
	if input.Source == "" {
		return input, fmt.Errorf("source required")
	}
	if input.FilialID.String() == "00000000-0000-0000-0000-000000000000" {
		return input, fmt.Errorf("filial_id required")
	}
	if input.Mode == "" {
		input.Mode = entity.ProductSyncModeUpsertOnly
	}
	if input.Mode != entity.ProductSyncModeUpsertOnly && input.Mode != entity.ProductSyncModeReplaceSource {
		return input, fmt.Errorf("unsupported sync mode: %s", input.Mode)
	}

	categories := make([]entity.ProductSyncCategory, 0, len(input.Categories))
	for _, category := range input.Categories {
		category.Name = strings.TrimSpace(category.Name)
		category.Type = strings.TrimSpace(category.Type)
		category.PhotoURL = strings.TrimSpace(category.PhotoURL)
		category.ExternalID = strings.TrimSpace(category.ExternalID)
		if category.Name == "" {
			continue
		}
		if category.ExternalID == "" {
			category.ExternalID = stableExternalID(input.Source, "category", category.Type, category.Name)
		}

		products := make([]entity.ProductSyncItem, 0, len(category.Products))
		for _, item := range category.Products {
			item.Title = strings.TrimSpace(item.Title)
			if item.Title == "" {
				continue
			}
			item.ExternalID = strings.TrimSpace(item.ExternalID)
			item.Body = strings.TrimSpace(item.Body)
			item.Currency = strings.TrimSpace(item.Currency)
			item.Weight = firstNonEmpty(strings.TrimSpace(item.Weight), strings.TrimSpace(item.Volume))
			item.PhotoURL = firstNonEmpty(strings.TrimSpace(item.PhotoURL), strings.TrimSpace(item.PhotoLink))
			if item.BasePrice == 0 && strings.TrimSpace(item.Price) != "" {
				item.BasePrice = parsePrice(item.Price)
			}
			if item.ExternalID == "" {
				item.ExternalID = stableExternalID(input.Source, "product", category.ExternalID, item.Title, item.Price, item.Weight, item.PhotoURL)
			}
			item.MediaUrls = normalizeMediaURLs(item.PhotoURL, item.MediaUrls)
			products = append(products, item)
		}
		category.Products = products
		categories = append(categories, category)
	}
	input.Categories = categories
	return input, nil
}

func stableExternalID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])[:32]
}

func parsePrice(raw string) float64 {
	var b strings.Builder
	seenDecimal := false
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case (r == '.' || r == ',') && !seenDecimal:
			b.WriteByte('.')
			seenDecimal = true
		}
	}
	if b.Len() == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(b.String(), 64)
	return v
}

func normalizeMediaURLs(photoURL string, urls []string) []string {
	seen := make(map[string]struct{}, len(urls)+1)
	out := make([]string, 0, len(urls)+1)
	for _, url := range append([]string{photoURL}, urls...) {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		out = append(out, url)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
