package product

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/poshagator/content-service/internal/domain/entities/product"
	uc "github.com/poshagator/content-service/internal/domain/usecase/product"
)

const (
	headerPermissionsFilial = "x-permissions-filial"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

type Handler struct {
	logger  *zap.Logger
	usecase *uc.Usecase
}

func NewHandler(logger *zap.Logger, uc *uc.Usecase) *Handler {
	return &Handler{
		logger:  logger,
		usecase: uc,
	}
}

func (h *Handler) GetProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		h.logger.Error("failed to parse id", zap.String("id", c.Query("id")), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.usecase.GetProduct(c, id)
	if err != nil {
		h.logger.Error("failed to get product", zap.String("id", id.String()), zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *Handler) CreateProducts(c *gin.Context) {
	req := new(product.Product)
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.usecase.CreateProduct(c, req)
	if err != nil {
		h.logger.Error("failed to create product", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *Handler) DeleteProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		h.logger.Error("failed to parse id", zap.String("id", c.Query("id")), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.usecase.DeleteProduct(c, id); err != nil {
		h.logger.Error("failed to delete product", zap.String("id", id.String()), zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, struct{}{})
}

func (h *Handler) UpdateProduct(c *gin.Context) {
	req := new(product.Product)
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.usecase.UpdateProduct(c, req)
	if err != nil {
		h.logger.Error("failed to update product", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *Handler) GetProducts(c *gin.Context) {
	filialID, err := uuid.Parse(c.Query("filialID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get filialID " + err.Error()})
		return
	}

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get page " + err.Error()})
		return
	}
	size, err := strconv.Atoi(c.Query("perPage"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get size " + err.Error()})
		return
	}

	groups, err := h.usecase.GetProductsGrouped(c, filialID, size, page)
	if err != nil {
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, groups)
}

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
	headerPerms, ok := h.getHeaderPermsOr403(c)
	if !ok {
		return
	}

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

	if err := h.usecase.AttachModifierGroup(c, fid, link, headerPerms); err != nil {
		h.logger.Error("attach modifier", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, struct{}{})
}

func (h *Handler) CreateModifierGroup(c *gin.Context) {
	headerPerms, ok := h.getHeaderPermsOr403(c)
	if !ok {
		return
	}

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

	if err := h.usecase.CreateModifierGroup(c, g, headerPerms); err != nil {
		h.renderError(c, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

func (h *Handler) UpdateModifierGroup(c *gin.Context) {
	headerPerms, ok := h.getHeaderPermsOr403(c)
	if !ok {
		return
	}

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

	if err := h.usecase.UpdateModifierGroup(c, g, headerPerms); err != nil {
		h.logger.Error("update modifier group", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, g)
}

func (h *Handler) DeleteModifierGroup(c *gin.Context) {
	headerPerms, ok := h.getHeaderPermsOr403(c)
	if !ok {
		return
	}

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

	if err := h.usecase.DeleteModifierGroup(c, fid, req.ID, headerPerms); err != nil {
		h.logger.Error("delete modifier group", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, struct{}{})
}

func (h *Handler) renderError(c *gin.Context, err error) {
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.InvalidArgument:
			c.JSON(http.StatusBadRequest, ErrorResponse{Message: st.Message()})
		case codes.NotFound:
			c.JSON(http.StatusNotFound, ErrorResponse{Message: st.Message()})
		case codes.PermissionDenied, codes.Unauthenticated:
			c.JSON(http.StatusForbidden, ErrorResponse{Message: st.Message()})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Message: st.Message()})
		}
		return
	}
	c.JSON(http.StatusInternalServerError, ErrorResponse{Message: err.Error()})
}

func (h *Handler) getHeaderPermsOr403(c *gin.Context) (map[string]struct{}, bool) {
	permsHeader := c.GetHeader(headerPermissionsFilial)
	if permsHeader == "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Message: headerPermissionsFilial + " header required"})
		return nil, false
	}
	return parsePermissionsHeader(permsHeader), true
}

func parsePermissionsHeader(h string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, p := range strings.Split(h, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out[p] = struct{}{}
		}
	}
	return out
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
