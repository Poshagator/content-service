package filial

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/internal/domain/usecase/action"
	"github.com/poshagator/content-service/internal/domain/usecase/filial"
	"github.com/poshagator/content-service/internal/domain/usecase/fuel"
)

type Handler struct {
	logger   *zap.Logger
	usecase  *filial.Usecase
	fuelUC   *fuel.Usecase
	actionUC *action.Usecase
}

func NewHandler(logger *zap.Logger, uc *filial.Usecase, fuelUC *fuel.Usecase, actionUC *action.Usecase) *Handler {
	return &Handler{
		logger:   logger,
		usecase:  uc,
		fuelUC:   fuelUC,
		actionUC: actionUC,
	}
}

func (h *Handler) GetFilialFeatures(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	filialID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id format"})
		return
	}

	features, err := h.usecase.GetFilialFeatures(c.Request.Context(), filialID)
	if err != nil {
		h.logger.Error("failed to get filial features", zap.Error(err), zap.String("filial_id", idStr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, features)
}

func (h *Handler) GetFuels(c *gin.Context) {
	idStr := c.Query("filial_id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filial_id is required"})
		return
	}

	filialID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filial_id format"})
		return
	}

	fuels, err := h.fuelUC.GetFuels(c.Request.Context(), filialID)
	if err != nil {
		h.logger.Error("failed to get fuels", zap.Error(err), zap.String("filial_id", idStr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, fuels)
}

func (h *Handler) GetExternalActions(c *gin.Context) {
	idStr := c.Query("filial_id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filial_id is required"})
		return
	}

	filialID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filial_id format"})
		return
	}

	actions, err := h.actionUC.GetExternalActions(c.Request.Context(), filialID)
	if err != nil {
		h.logger.Error("failed to get external actions", zap.Error(err), zap.String("filial_id", idStr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, actions)
}
