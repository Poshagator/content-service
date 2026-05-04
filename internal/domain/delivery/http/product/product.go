package product

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/internal/domain/entities/product"
)

func (h *Handler) GetProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		h.logger.Error("failed to parse id", zap.String("id", c.Query("id")), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.usecase.GetProduct(c, id)
	if err != nil {
		h.logger.Error("failed to get product", zap.String("id", id.String()), zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *Handler) CreateProducts(c *gin.Context) {
	req := new(product.Product)
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.usecase.CreateProduct(c, req)
	if err != nil {
		h.logger.Error("failed to create product", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *Handler) DeleteProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Query("id"))
	if err != nil {
		h.logger.Error("failed to parse id", zap.String("id", c.Query("id")), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.usecase.DeleteProduct(c, id); err != nil {
		h.logger.Error("failed to delete product", zap.String("id", id.String()), zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, struct{}{})
}

func (h *Handler) UpdateProduct(c *gin.Context) {
	req := new(product.Product)
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.usecase.UpdateProduct(c, req)
	if err != nil {
		h.logger.Error("failed to update product", zap.Error(err))
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *Handler) GetProducts(c *gin.Context) {
	filialID, err := uuid.Parse(c.Query("filialID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get filialID " + err.Error()})
		return
	}

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get page " + err.Error()})
		return
	}
	size, err := strconv.Atoi(c.Query("perPage"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get size " + err.Error()})
		return
	}

	groups, err := h.usecase.GetProductsGrouped(c, filialID, size, page)
	if err != nil {
		h.renderError(c, err)
		return
	}

	c.JSON(http.StatusOK, groups)
}
