package product

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/internal/domain/entities/product"
)

func (h *Handler) ListFilialModifiers(c *gin.Context) {
	filialID, err := uuid.Parse(c.Query("filialID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filialID: " + err.Error()})
		return
	}

	allowedKinds := parseKindQuery(c.Query("kind"))

	mods, err := h.usecase.GetFilialModifiers(c, filialID)
	if err != nil {
		h.renderError(c, err)
		return
	}

	if allowedKinds != nil {
		mods = filterModifiersByKind(mods, allowedKinds)
	}

	c.JSON(http.StatusOK, mods)
}

func (h *Handler) AttachModifierToProduct(c *gin.Context) {
	var req struct {
		FilialID  string    `json:"filial_id"  binding:"required"`
		ProductID uuid.UUID `json:"product_id"  binding:"required"`
		GroupID   uuid.UUID `json:"group_id"    binding:"required"`
		SortOrder int       `json:"sort_order"`
		Required  bool      `json:"required"`
		MinSelect int       `json:"min_select"`
		MaxSelect int       `json:"max_select"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: err.Error()})
		return
	}

	fid, err := uuid.Parse(req.FilialID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: "invalid filial_id: " + err.Error()})
		return
	}

	link := &product.ProductModifierGroupLink{
		ProductID: req.ProductID,
		GroupID:   req.GroupID,
		SortOrder: req.SortOrder,
		Required:  req.Required,
		MinSelect: req.MinSelect,
		MaxSelect: req.MaxSelect,
	}

	if err := h.usecase.AttachModifierGroup(c, fid, link); err != nil {
		h.logger.Error("attach modifier", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, struct{}{})
}

func (h *Handler) CreateModifierGroup(c *gin.Context) {
	var req struct {
		FilialID  string                          `json:"filial_id"  binding:"required"`
		Name      string                          `json:"name"       binding:"required"`
		KindCode  string                          `json:"kind_code"`
		GroupType string                          `json:"group_type" binding:"required"` // single|multi|counter|toggle
		Options   []product.ProductModifierOption `json:"options"`                       // может быть пусто
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: err.Error()})
		return
	}

	filialID, err := uuid.Parse(req.FilialID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: "invalid filial_id: " + err.Error()})
		return
	}

	g := &product.ProductModifierGroup{
		FilialID: filialID,
		Name:     req.Name,
		Kind:     req.KindCode,
		Type:     req.GroupType,
		Options:  req.Options,
	}

	if err := h.usecase.CreateModifierGroup(c, g); err != nil {
		h.renderError(c, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

func (h *Handler) UpdateModifierGroup(c *gin.Context) {
	var req struct {
		FilialID  string                          `json:"filial_id"  binding:"required"`
		ID        uuid.UUID                       `json:"id"         binding:"required"`
		Name      string                          `json:"name"       binding:"required"`
		KindCode  string                          `json:"kind_code"`
		GroupType string                          `json:"group_type"`
		Options   []product.ProductModifierOption `json:"options"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: err.Error()})
		return
	}

	fid, err := uuid.Parse(req.FilialID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: "invalid filial_id: " + err.Error()})
		return
	}

	g := &product.ProductModifierGroup{
		ID:       req.ID,
		FilialID: fid,
		Name:     req.Name,
		Kind:     req.KindCode,
		Type:     req.GroupType,
		Options:  req.Options,
	}

	if err := h.usecase.UpdateModifierGroup(c, g); err != nil {
		h.logger.Error("update modifier group", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, g)
}

func (h *Handler) DeleteModifierGroup(c *gin.Context) {
	var req struct {
		FilialID string    `json:"filial_id" binding:"required"`
		ID       uuid.UUID `json:"id"        binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: err.Error()})
		return
	}

	fid, err := uuid.Parse(req.FilialID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: "invalid filial_id: " + err.Error()})
		return
	}

	if err := h.usecase.DeleteModifierGroup(c, fid, req.ID); err != nil {
		h.logger.Error("delete modifier group", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, struct{}{})
}

func parseKindQuery(raw string) map[string]struct{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := make(map[string]struct{})
	for _, k := range strings.Split(raw, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			out[strings.ToLower(k)] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func filterModifiersByKind(in *product.ModifierGroupsByType, allowed map[string]struct{}) *product.ModifierGroupsByType {
	f := func(src []product.ProductModifierGroup) []product.ProductModifierGroup {
		if len(src) == 0 {
			return src
		}
		dst := make([]product.ProductModifierGroup, 0, len(src))
		for _, g := range src {
			if g.Kind == "" {
				continue
			}
			if _, ok := allowed[strings.ToLower(g.Kind)]; ok {
				dst = append(dst, g)
			}
		}
		return dst
	}
	return &product.ModifierGroupsByType{
		Single:  f(in.Single),
		Toggle:  f(in.Toggle),
		Counter: f(in.Counter),
		Multi:   f(in.Multi),
	}
}
