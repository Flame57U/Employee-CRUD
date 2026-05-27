package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/store"
)

// DashboardHandler aggregates per-user account snapshots for the UI overview.
type DashboardHandler struct {
	db *store.DB
}

// NewDashboardHandler constructs a DashboardHandler.
func NewDashboardHandler(db *store.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

// RegisterRoutes mounts /dashboard on r.
func (h *DashboardHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/dashboard", h.Get)
}

type instanceSummary struct {
	ID          uint    `json:"id"`
	TemplateID  uint    `json:"template_id"`
	Status      string  `json:"status"`
	CNYBalance  float64 `json:"cny_balance"`
	TotalEquity float64 `json:"total_equity"`
}

type dashboardResponse struct {
	TotalInstances int               `json:"total_instances"`
	RunningCount   int               `json:"running_count"`
	TotalCNY       float64           `json:"total_cny"`
	TotalEquity    float64           `json:"total_equity"`
	Instances      []instanceSummary `json:"instances"`
}

// Get godoc
// GET /api/v1/dashboard
// Returns a per-instance summary plus aggregated equity for the caller.
func (h *DashboardHandler) Get(c *gin.Context) {
	uid := userIDFromCtx(c)

	var insts []store.StrategyInstance
	if err := h.db.Where("user_id = ?", uid).Find(&insts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := dashboardResponse{
		TotalInstances: len(insts),
		Instances:      make([]instanceSummary, 0, len(insts)),
	}

	for _, inst := range insts {
		if inst.Status == store.InstanceStatusRunning {
			resp.RunningCount++
		}
		var ps store.PortfolioState
		_ = h.db.Where("instance_id = ?", inst.ID).First(&ps).Error
		resp.TotalCNY += ps.CNYBalance
		resp.TotalEquity += ps.TotalEquity
		resp.Instances = append(resp.Instances, instanceSummary{
			ID:          inst.ID,
			TemplateID:  inst.TemplateID,
			Status:      inst.Status,
			CNYBalance:  ps.CNYBalance,
			TotalEquity: ps.TotalEquity,
		})
	}
	c.JSON(http.StatusOK, resp)
}
