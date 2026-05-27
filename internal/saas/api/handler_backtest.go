package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/store"
)

// BacktestHandler accepts on-demand backtest requests from the Lab UI.
//
// This handler persists the request and returns the row. The actual run is
// driven asynchronously by a worker (out of scope for the routing phase) —
// the row's Status begins as "pending" and is updated by the worker.
type BacktestHandler struct {
	db *store.DB
}

// NewBacktestHandler constructs a BacktestHandler.
func NewBacktestHandler(db *store.DB) *BacktestHandler {
	return &BacktestHandler{db: db}
}

// RegisterRoutes mounts /backtests on r (the lab-protected group).
func (h *BacktestHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/backtests")
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
}

type createBacktestRequest struct {
	TemplateID uint            `json:"template_id" binding:"required"`
	Symbol     string          `json:"symbol" binding:"required"`
	ParamPack  json.RawMessage `json:"param_pack" binding:"required"`
}

// Create godoc
// POST /api/v1/backtests
// Persists the backtest request with status="pending"; an async worker picks
// it up and writes Result + transitions Status.
func (h *BacktestHandler) Create(c *gin.Context) {
	var req createBacktestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row := store.Backtest{
		UserID:     userIDFromCtx(c),
		TemplateID: req.TemplateID,
		Symbol:     req.Symbol,
		ParamPack:  req.ParamPack,
		Status:     "pending",
	}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, row)
}

// Get godoc
// GET /api/v1/backtests/:id
func (h *BacktestHandler) Get(c *gin.Context) {
	id, err := parseUintParam(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var row store.Backtest
	if err := h.db.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "backtest not found"})
		return
	}
	c.JSON(http.StatusOK, row)
}
