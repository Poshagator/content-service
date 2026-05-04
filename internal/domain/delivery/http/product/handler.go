package product

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	uc "github.com/poshagator/content-service/internal/domain/usecase/product"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

type Handler struct {
	logger  *zap.Logger
	usecase *uc.Usecase
}

func NewHandler(logger *zap.Logger, uc *uc.Usecase) *Handler {
	return &Handler{
		logger:  logger,
		usecase: uc,
	}
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
