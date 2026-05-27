package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/store"
)

// StrategyHandler serves the read-only strategy template endpoints.
// Templates are administered out-of-band — there is no create/update API.
type StrategyHandler struct {
	db *store.DB
}

// NewStrategyHandler constructs a StrategyHandler.
func NewStrategyHandler(db *store.DB) *StrategyHandler {
	return &StrategyHandler{db: db}
}

// RegisterRoutes mounts /strategies on r (the authenticated /api/v1 group).
func (h *StrategyHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/strategies")
	g.GET("", h.List)
	g.GET("/:id", h.Get)
}

// List godoc
// GET /api/v1/strategies
func (h *StrategyHandler) List(c *gin.Context) {
	var rows []store.StrategyTemplate
	if err := h.db.Order("id ASC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"strategies": rows})
}

// Get godoc
// GET /api/v1/strategies/:id
func (h *StrategyHandler) Get(c *gin.Context) {
	id, err := parseUintParam(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var row store.StrategyTemplate
	if err := h.db.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "strategy not found"})
		return
	}
	c.JSON(http.StatusOK, row)
}
