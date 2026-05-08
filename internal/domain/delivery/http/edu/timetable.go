package edu

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"net/http"
)

func (h *Handler) GetTimetable(c *gin.Context) {
	groupID, err := uuid.Parse(c.Query("groupID"))
	if err != nil {
		h.logger.Error("failed to parse groupID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid groupID: " + err.Error()})
		return
	}

	entries, err := h.usecase.GetTimetable(c, groupID)
	if err != nil {
		h.logger.Error("failed to get timetable", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, entries)
}
