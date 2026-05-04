package http

import (
	"github.com/poshagator/content-service/internal/domain/entities"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

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
