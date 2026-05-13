package edu

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"net/http"
)

type SyncToPlannerRequest struct {
	UserID     string    `json:"userID"`
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

	if userID := plannerUserID(c); userID != "" {
		req.UserID = userID
	}

	if req.UserID == "" || req.GroupID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userID and groupID are required"})
		return
	}

	err := h.usecase.SyncGroupScheduleToPlanner(c.Request.Context(), req.UserID, req.GroupID, req.TermID, req.ActivityID)
	if err != nil {
		h.logger.Error("failed to sync schedule to planner",
			zap.String("userID", req.UserID),
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

	if userID := plannerUserID(c); userID != "" {
		req.UserID = userID
	}

	if req.UserID == "" || req.GroupID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userID and groupID are required"})
		return
	}

	err := h.usecase.UnsubscribeFromPlanner(c.Request.Context(), req.UserID, req.GroupID, req.TermID)
	if err != nil {
		h.logger.Error("failed to unsubscribe from planner",
			zap.String("userID", req.UserID),
			zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Unsubscribed successfully"})
}

func (h *Handler) GetPlannerSubscriptions(c *gin.Context) {
	userID := plannerUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userID is required"})
		return
	}

	subscriptions, err := h.usecase.GetPlannerSubscriptions(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get planner subscriptions", zap.String("userID", userID), zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"subscriptions": subscriptions})
}

func plannerUserID(c *gin.Context) string {
	if userID := c.GetHeader("X-User-Id"); userID != "" {
		return userID
	}
	if userID := c.GetHeader("X-App-Id"); userID != "" {
		return userID
	}
	return c.GetHeader("X-UserId")
}
