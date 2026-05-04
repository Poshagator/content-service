// Package http implements the public REST API facade over the business use-case
// layer.  All endpoints are grouped under the legacy prefix "/app" for mobile
// backward-compatibility.
package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"content-service/config"
	"content-service/internal/domain/entities"
	"content-service/internal/domain/usecase"
)

const (
	headerPermissionsFilial = "x-permissions-filial"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

type Server struct {
	logger  *zap.Logger
	cfg     *config.ConfigModel
	serv    *gin.Engine
	Usecase *usecase.Usecase
}

func NewServer(logger *zap.Logger, cfg *config.ConfigModel, uc *usecase.Usecase) (*Server, error) {
	return &Server{
		logger:  logger,
		cfg:     cfg,
		serv:    gin.Default(),
		Usecase: uc,
	}, nil
}

func (s *Server) OnStart(_ context.Context) error {
	s.createController()

	go func() {
		addr := s.cfg.HTTP.Host + ":" + s.cfg.HTTP.Port
		s.logger.Info("HTTP server started", zap.String("addr", addr))
		if err := s.serv.Run(addr); err != nil {
			s.logger.Error("HTTP server exited", zap.Error(err))
		}
	}()
	return nil
}

func (s *Server) OnStop(_ context.Context) error {
	s.logger.Info("HTTP server stopped")
	return nil
}

// parsePermissionsHeader разбивает строку вида "a,b,c" → set.
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

// parseKindQuery поддерживает ?kind=k1,k2. Пусто → nil (нет фильтра).
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

func (s *Server) GetProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		s.logger.Error("failed to parse id", zap.String("id", c.Query("id")), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product, err := s.Usecase.GetProduct(c, id)
	if err != nil {
		s.logger.Error("failed to get product", zap.String("id", id.String()), zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, product)
}

func (s *Server) CreateProducts(c *gin.Context) {
	req := new(entities.Product)
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product, err := s.Usecase.CreateProduct(c, req)
	if err != nil {
		s.logger.Error("failed to create product", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, product)
}

func (s *Server) DeleteProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		s.logger.Error("failed to parse id", zap.String("id", c.Query("id")), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err = s.Usecase.DeleteProduct(c, id); err != nil {
		s.logger.Error("failed to delete product", zap.String("id", id.String()), zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, struct{}{})
}

func (s *Server) UpdateProduct(c *gin.Context) {
	req := new(entities.Product)
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product, err := s.Usecase.UpdateProduct(c, req)
	if err != nil {
		s.logger.Error("failed to update product", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, product)
}

func (s *Server) GetProducts(c *gin.Context) {
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

	groups, err := s.Usecase.GetProductsGrouped(c, filialID, size, page)
	if err != nil {
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, groups)
}

func (s *Server) renderError(c *gin.Context, err error) {
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

// ListFilialModifiers
// GET /product/modifier/list-by-filial?filialID=...&kind=comma,separated,codes
// kind — необязателен; если не указан, возвращаются все группы.
func (s *Server) ListFilialModifiers(c *gin.Context) {
	filialID, err := uuid.Parse(c.Query("filialID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filialID: " + err.Error()})
		return
	}

	allowedKinds := parseKindQuery(c.Query("kind"))

	mods, err := s.Usecase.GetFilialModifiers(c, filialID)
	if err != nil {
		s.renderError(c, err)
		return
	}

	if allowedKinds != nil {
		mods = filterModifiersByKind(mods, allowedKinds)
	}

	c.JSON(http.StatusOK, mods)
}

// filterModifiersByKind применяет фильтр к каждой категории UI‑типа.
func filterModifiersByKind(in *entities.ModifierGroupsByType, allowed map[string]struct{}) *entities.ModifierGroupsByType {
	f := func(src []entities.ProductModifierGroup) []entities.ProductModifierGroup {
		if len(src) == 0 {
			return src
		}
		dst := make([]entities.ProductModifierGroup, 0, len(src))
		for _, g := range src {
			if g.Kind == "" {
				continue // без kind не показываем при фильтре
			}
			if _, ok := allowed[strings.ToLower(g.Kind)]; ok {
				dst = append(dst, g)
			}
		}
		return dst
	}
	return &entities.ModifierGroupsByType{
		Single:  f(in.Single),
		Toggle:  f(in.Toggle),
		Counter: f(in.Counter),
		Multi:   f(in.Multi),
	}
}

func (s *Server) AttachModifierToProduct(c *gin.Context) {
	// Проверяем обязательный permissions‑хедер.
	headerPerms, ok := s.getHeaderPermsOr403(c)
	if !ok {
		return
	}

	// filial_id теперь обязателен в body.
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

	link := &entities.ProductModifierGroupLink{
		ProductID: req.ProductID,
		GroupID:   req.GroupID,
		SortOrder: req.SortOrder,
		Required:  req.Required,
		MinSelect: req.MinSelect,
		MaxSelect: req.MaxSelect,
	}

	if err := s.Usecase.AttachModifierGroup(c, fid, link, headerPerms); err != nil {
		s.logger.Error("attach modifier", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, struct{}{})
}

func (s *Server) getHeaderPermsOr403(c *gin.Context) (map[string]struct{}, bool) {
	permsHeader := c.GetHeader(headerPermissionsFilial)
	if permsHeader == "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Message: headerPermissionsFilial + " header required"})
		return nil, false
	}
	return parsePermissionsHeader(permsHeader), true
}

func (s *Server) CreateModifierGroup(c *gin.Context) {
	headerPerms, ok := s.getHeaderPermsOr403(c)
	if !ok {
		return
	}

	var req struct {
		FilialID  string                           `json:"filial_id"  binding:"required"`
		Name      string                           `json:"name"       binding:"required"`
		KindCode  string                           `json:"kind_code"`
		GroupType string                           `json:"group_type" binding:"required"` // single|multi|counter|toggle
		Options   []entities.ProductModifierOption `json:"options"`                       // может быть пусто
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

	g := &entities.ProductModifierGroup{
		FilialID: filialID,
		Name:     req.Name,
		Kind:     req.KindCode,
		Type:     req.GroupType,
		Options:  req.Options,
	}

	if err := s.Usecase.CreateModifierGroup(c, g, headerPerms); err != nil {
		s.renderError(c, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

func (s *Server) UpdateModifierGroup(c *gin.Context) {
	headerPerms, ok := s.getHeaderPermsOr403(c)
	if !ok {
		return
	}

	var req struct {
		FilialID  string                           `json:"filial_id"  binding:"required"`
		ID        uuid.UUID                        `json:"id"         binding:"required"`
		Name      string                           `json:"name"       binding:"required"`
		KindCode  string                           `json:"kind_code"`
		GroupType string                           `json:"group_type"`
		Options   []entities.ProductModifierOption `json:"options"`
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

	g := &entities.ProductModifierGroup{
		ID:       req.ID,
		FilialID: fid,
		Name:     req.Name,
		Kind:     req.KindCode,
		Type:     req.GroupType,
		Options:  req.Options,
	}

	if err := s.Usecase.UpdateModifierGroup(c, g, headerPerms); err != nil {
		s.logger.Error("update modifier group", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, g)
}

func (s *Server) DeleteModifierGroup(c *gin.Context) {
	headerPerms, ok := s.getHeaderPermsOr403(c)
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

	if err := s.Usecase.DeleteModifierGroup(c, fid, req.ID, headerPerms); err != nil {
		s.logger.Error("delete modifier group", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, struct{}{})
}


type AcademicTermBody struct {
	FilialID string `json:"filial_id" binding:"required,uuid"`
}

func (s *Server) GetAcademicTerm(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		s.logger.Error("failed to parse id", zap.String("id", c.Query("id")), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	AcademicTerm, err := s.Usecase.GetAcademicTerm(c, id)
	if err != nil {
		s.logger.Error("failed to get AcademicTerm", zap.String("id", id.String()), zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, AcademicTerm)
}

// POST /filials/:filial_id/edu-groups
func (s *Server) CreateAcademicTerm(c *gin.Context) {
	var req entities.AcademicTerm
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := s.Usecase.CreateAcademicTerm(c, &req)
	if err != nil {
		s.logger.Error("failed to create AcademicTerm", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": group.ID})
}

// PATCH /edu-groups/:id
func (s *Server) UpdateAcademicTerm(c *gin.Context) {
	var req entities.AcademicTerm
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := s.Usecase.UpdateAcademicTerm(c, &req)
	if err != nil {
		s.logger.Error("failed to update AcademicTerm", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, group)
}

// DELETE /edu-groups/:id
func (s *Server) DeleteAcademicTerm(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		s.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = s.Usecase.DeleteAcademicTerm(c, id)
	if err != nil {
		s.logger.Error("failed to delete AcademicTerm", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

// GET /filials/:filial_id/edu-groups
func (s *Server) ListAcademicTerms(c *gin.Context) {
	filialID, err := uuid.Parse(c.Query("filialID"))
	if err != nil {
		s.logger.Error("failed to parse filialID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get filialID " + err.Error()})
		return
	}

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		s.logger.Error("failed to convert page to int", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get page " + err.Error()})
		return
	}
	size, err := strconv.Atoi(c.Query("perPage"))
	if err != nil {
		s.logger.Error("failed to get perPage " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get size " + err.Error()})
		return
	}

	AcademicTerms, err := s.Usecase.GetAcademicTerms(c, filialID, size, page)
	if err != nil {
		s.logger.Error("failed to get AcademicTerms", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, AcademicTerms)
}


type eduGroupBody struct {
	FilialID string `json:"filial_id" binding:"required,uuid"`
}

func (s *Server) GetEduGroup(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		s.logger.Error("failed to parse id", zap.String("id", c.Query("id")), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	EduGroup, err := s.Usecase.GetEduGroup(c, id)
	if err != nil {
		s.logger.Error("failed to get EduGroup", zap.String("id", id.String()), zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, EduGroup)
}

// POST /filials/:filial_id/edu-groups
func (s *Server) CreateEduGroup(c *gin.Context) {
	var req entities.EduGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := s.Usecase.CreateEduGroup(c, &req)
	if err != nil {
		s.logger.Error("failed to create eduGroup", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": group.ID})
}

// PATCH /edu-groups/:id
func (s *Server) UpdateEduGroup(c *gin.Context) {
	var req entities.EduGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := s.Usecase.UpdateEduGroup(c, &req)
	if err != nil {
		s.logger.Error("failed to update eduGroup", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, group)
}

// DELETE /edu-groups/:id
func (s *Server) DeleteEduGroup(c *gin.Context) {
	// TODO: Call s.Usecase.DeleteEduGroup
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		s.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = s.Usecase.DeleteEduGroup(c, id)
	if err != nil {
		s.logger.Error("failed to delete eduGroup", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

// GET /filials/:filial_id/edu-groups
func (s *Server) ListEduGroups(c *gin.Context) {
	filialID, err := uuid.Parse(c.Query("filialID"))
	if err != nil {
		s.logger.Error("failed to parse filialID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get filialID " + err.Error()})
		return
	}

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		s.logger.Error("failed to convert page to int", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get page " + err.Error()})
		return
	}
	size, err := strconv.Atoi(c.Query("perPage"))
	if err != nil {
		s.logger.Error("failed to get perPage " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get size " + err.Error()})
		return
	}

	eduGroups, err := s.Usecase.GetEduGroups(c, filialID, size, page)
	if err != nil {
		s.logger.Error("failed to get eduGroups", zap.Error(err))
		s.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, eduGroups)
}
