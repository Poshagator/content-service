package edu

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poshagator/content-service/internal/domain/entities/edu"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

func (h *Handler) GetEduGroup(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		h.logger.Error("failed to parse id", zap.String("id", c.Query("id")), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	EduGroup, err := h.usecase.GetEduGroup(c, id)
	if err != nil {
		h.logger.Error("failed to get EduGroup", zap.String("id", id.String()), zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, EduGroup)
}

func (h *Handler) CreateEduGroup(c *gin.Context) {
	var req edu.EduGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := h.usecase.CreateEduGroup(c, &req)
	if err != nil {
		h.logger.Error("failed to create eduGroup", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": group.ID})
}

func (h *Handler) UpdateEduGroup(c *gin.Context) {
	var req edu.EduGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := h.usecase.UpdateEduGroup(c, &req)
	if err != nil {
		h.logger.Error("failed to update eduGroup", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, group)
}

func (h *Handler) DeleteEduGroup(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		h.logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.usecase.DeleteEduGroup(c, id)
	if err != nil {
		h.logger.Error("failed to delete eduGroup", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

func (h *Handler) ListEduGroups(c *gin.Context) {
	filialID, err := uuid.Parse(c.Query("filialID"))
	if err != nil {
		h.logger.Error("failed to parse filialID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get filialID " + err.Error()})
		return
	}

	if c.Query("grouped") == "true" {
		eduGroups, err := h.usecase.GetEduGroupsGrouped(c, filialID)
		if err != nil {
			h.logger.Error("failed to get grouped eduGroups", zap.Error(err))
			h.renderError(c, err)
			return
		}

		c.JSON(http.StatusOK, eduGroups)
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

	eduGroups, err := h.usecase.GetEduGroups(c, filialID, size, page)
	if err != nil {
		h.logger.Error("failed to get eduGroups", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, eduGroups)
}

func (h *Handler) ListEduGroupsTree(c *gin.Context) {
	filialID, err := uuid.Parse(c.Query("filialID"))
	if err != nil {
		h.logger.Error("failed to parse filialID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get filialID " + err.Error()})
		return
	}

	eduGroups, err := h.usecase.GetEduGroupsGrouped(c, filialID)
	if err != nil {
		h.logger.Error("failed to get grouped eduGroups", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, eduGroups)
}

func (h *Handler) SearchEduGroups(c *gin.Context) {
	filialID, err := uuid.Parse(c.Query("filialID"))
	if err != nil {
		h.logger.Error("failed to parse filialID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filialID"})
		return
	}

	name := c.Query("name")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	eduGroups, err := h.usecase.SearchEduGroups(c, filialID, name, limit)
	if err != nil {
		h.logger.Error("failed to search eduGroups", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, eduGroups)
}
