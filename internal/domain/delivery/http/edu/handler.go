package edu

import (
	"go.uber.org/zap"
	"github.com/poshagator/content-service/internal/domain/usecase/edu"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"net/http"
)

type Handler struct {
	logger  *zap.Logger
	usecase *edu.Usecase
}

func NewHandler(logger *zap.Logger, uc *edu.Usecase) *Handler {
	return &Handler{
		logger:  logger,
		usecase: uc,
	}
}

type ErrorResponse struct {
	Message string `json:"message"`
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
