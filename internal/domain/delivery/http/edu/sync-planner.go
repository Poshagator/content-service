package edu

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

type SyncToPlannerRequest struct {
	UserID     int64     `json:"userID"`
	GroupID    uuid.UUID `json:"groupID"`
	TermID     uuid.UUID `json:"termID"`
	ActivityID string    `json:"activityID"` // Optional, but provided by user
}

func (h *Handler) SyncToPlanner(c *gin.Context) {
	var req SyncToPlannerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("failed to bind sync request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Read userID from X-User-Id header
	userIDStr := c.GetHeader("X-User-Id")
	if userIDStr != "" {
		uid, _ := strconv.ParseInt(userIDStr, 10, 64)
		req.UserID = uid
	}

	if req.UserID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-User-Id header is required"})
		return
	}

	err := h.usecase.SyncGroupScheduleToPlanner(c.Request.Context(), req.UserID, req.GroupID, req.TermID)
	if err != nil {
		h.logger.Error("failed to sync schedule to planner", 
			zap.Int64("userID", req.UserID),
			zap.String("groupID", req.GroupID.String()),
			zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sync completed successfully"})
}

func (h *Handler) UnsubscribeFromPlanner(c *gin.Context) {
	var req SyncToPlannerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("failed to bind unsubscribe request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	userIDStr := c.GetHeader("X-User-Id")
	if userIDStr != "" {
		uid, _ := strconv.ParseInt(userIDStr, 10, 64)
		req.UserID = uid
	}

	if req.UserID == 0 || req.GroupID == uuid.Nil || req.TermID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userID, groupID and termID are required"})
		return
	}

	err := h.usecase.UnsubscribeFromPlanner(c.Request.Context(), req.UserID, req.GroupID, req.TermID)
	if err != nil {
		h.logger.Error("failed to unsubscribe from planner", 
			zap.Int64("userID", req.UserID),
			zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Unsubscribed successfully"})
}
