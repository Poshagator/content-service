package http

import (
	"github.com/poshagator/content-service/internal/domain/entities"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

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
