package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/instance"
	"github.com/quantsaas/platform/internal/saas/store"
)

// planQuota maps the subscription plan to the maximum number of *non-deleted*
// strategy instances a user is permitted to own.
var planQuota = map[string]int{
	"free":  1,
	"pro":   5,
	"elite": 50,
}

// InstanceHandler owns the CRUD + lifecycle endpoints for StrategyInstance.
type InstanceHandler struct {
	db  *store.DB
	mgr *instance.Manager
}

// NewInstanceHandler constructs an InstanceHandler.
func NewInstanceHandler(db *store.DB, mgr *instance.Manager) *InstanceHandler {
	return &InstanceHandler{db: db, mgr: mgr}
}

// RegisterRoutes mounts /instances on r (the authenticated /api/v1 group).
func (h *InstanceHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/instances")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.POST("/:id/start", h.Start)
	g.POST("/:id/stop", h.Stop)
	g.DELETE("/:id", h.Delete)
	g.GET("/:id/lots", h.Lots)
	g.GET("/:id/trades", h.Trades)
}

// List godoc
// GET /api/v1/instances — returns instances belonging to the caller.
func (h *InstanceHandler) List(c *gin.Context) {
	uid := userIDFromCtx(c)
	var rows []store.StrategyInstance
	if err := h.db.Where("user_id = ?", uid).
		Preload("Template").
		Order("id DESC").
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"instances": rows})
}

type createInstanceRequest struct {
	TemplateID uint `json:"template_id" binding:"required"`
}

// Create godoc
// POST /api/v1/instances — quota-checked instance creation. Defaults to STOPPED.
func (h *InstanceHandler) Create(c *gin.Context) {
	uid := userIDFromCtx(c)

	var req createInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Confirm template exists before we burn a quota slot.
	var tmpl store.StrategyTemplate
	if err := h.db.First(&tmpl, req.TemplateID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown template_id"})
		return
	}

	var user store.User
	if err := h.db.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	quota, ok := planQuota[user.Plan]
	if !ok {
		quota = planQuota["free"]
	}
	var existing int64
	if err := h.db.Model(&store.StrategyInstance{}).
		Where("user_id = ?", uid).
		Count(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if int(existing) >= quota {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "subscription quota exhausted",
			"plan":  user.Plan,
			"quota": quota,
		})
		return
	}

	inst := store.StrategyInstance{
		UserID:     uid,
		TemplateID: req.TemplateID,
		Status:     store.InstanceStatusStopped,
	}
	if err := h.db.Create(&inst).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, inst)
}

// Start godoc
// POST /api/v1/instances/:id/start
func (h *InstanceHandler) Start(c *gin.Context) {
	id, ok := h.requireOwnedInstance(c)
	if !ok {
		return
	}
	if err := h.mgr.Start(context.Background(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "status": store.InstanceStatusRunning})
}

// Stop godoc
// POST /api/v1/instances/:id/stop
func (h *InstanceHandler) Stop(c *gin.Context) {
	id, ok := h.requireOwnedInstance(c)
	if !ok {
		return
	}
	if err := h.mgr.Stop(context.Background(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "status": store.InstanceStatusStopped})
}

// Delete godoc
// DELETE /api/v1/instances/:id
func (h *InstanceHandler) Delete(c *gin.Context) {
	id, ok := h.requireOwnedInstance(c)
	if !ok {
		return
	}
	if err := h.mgr.Delete(context.Background(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "deleted": true})
}

// Lots godoc
// GET /api/v1/instances/:id/lots
func (h *InstanceHandler) Lots(c *gin.Context) {
	id, ok := h.requireOwnedInstance(c)
	if !ok {
		return
	}
	var lots []store.SpotLot
	if err := h.db.Where("instance_id = ?", id).Order("id ASC").Find(&lots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"lots": lots})
}

// Trades godoc
// GET /api/v1/instances/:id/trades?limit=N
func (h *InstanceHandler) Trades(c *gin.Context) {
	id, ok := h.requireOwnedInstance(c)
	if !ok {
		return
	}
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := parseUintParam(v); err == nil && n > 0 && n <= 1000 {
			limit = int(n)
		}
	}
	var trades []store.TradeRecord
	if err := h.db.Where("instance_id = ?", id).
		Order("created_at DESC").
		Limit(limit).
		Find(&trades).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"trades": trades})
}

// requireOwnedInstance pulls the :id param and verifies the caller owns the
// instance. Writes an error response and returns ok=false on any failure.
func (h *InstanceHandler) requireOwnedInstance(c *gin.Context) (uint, bool) {
	id, err := parseUintParam(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	var inst store.StrategyInstance
	if err := h.db.Select("id", "user_id").First(&inst, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return 0, false
	}
	if inst.UserID != userIDFromCtx(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your instance"})
		return 0, false
	}
	return id, true
}
