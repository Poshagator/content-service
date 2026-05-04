package edu

import (
	"github.com/poshagator/content-service/internal/domain/entities/edu"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

func (h *Handler) GetAcademicTerm(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		h.logger.Error("failed to parse id", zap.String("id", c.Query("id")), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	AcademicTerm, err := h.usecase.GetAcademicTerm(c, id)
	if err != nil {
		h.logger.Error("failed to get AcademicTerm", zap.String("id", id.String()), zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, AcademicTerm)
}

func (h *Handler) CreateAcademicTerm(c *gin.Context) {
	var req edu.AcademicTerm
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	term, err := h.usecase.CreateAcademicTerm(c, &req)
	if err != nil {
		h.logger.Error("failed to create academicTerm", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": term.ID})
}

func (h *Handler) UpdateAcademicTerm(c *gin.Context) {
	var req edu.AcademicTerm
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	term, err := h.usecase.UpdateAcademicTerm(c, &req)
	if err != nil {
		h.logger.Error("failed to update academicTerm", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, term)
}

func (h *Handler) DeleteAcademicTerm(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		h.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.usecase.DeleteAcademicTerm(c, id)
	if err != nil {
		h.logger.Error("failed to delete academicTerm", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

func (h *Handler) ListAcademicTerms(c *gin.Context) {
	filialID, err := uuid.Parse(c.Query("filialID"))
	if err != nil {
		h.logger.Error("failed to parse filialID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get filialID " + err.Error()})
		return
	}

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		h.logger.Error("failed to convert page to int", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get page " + err.Error()})
		return
	}
	size, err := strconv.Atoi(c.Query("perPage"))
	if err != nil {
		h.logger.Error("failed to get perPage " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get size " + err.Error()})
		return
	}

	academicTerms, err := h.usecase.GetAcademicTerms(c, filialID, size, page)
	if err != nil {
		h.logger.Error("failed to get academicTerms", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, academicTerms)
}
