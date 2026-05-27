// Package api contains the Gin HTTP handlers for the SaaS API.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/epoch"
	"github.com/quantsaas/platform/internal/saas/store"
	"gorm.io/gorm"
)

const (
	championCacheTTL = time.Hour
	championKeyFmt   = "champion:%d" // keyed by strategyID
)

// EvolutionHandler handles the GA evolution and genome champion endpoints.
type EvolutionHandler struct {
	svc   *epoch.EpochService
	db    *store.DB
	redis *store.RedisClient
}

// NewEvolutionHandler constructs an EvolutionHandler.
func NewEvolutionHandler(svc *epoch.EpochService, db *store.DB, redis *store.RedisClient) *EvolutionHandler {
	return &EvolutionHandler{svc: svc, db: db, redis: redis}
}

// RegisterRoutes registers evolution and genome routes on r.
func (h *EvolutionHandler) RegisterRoutes(r *gin.RouterGroup) {
	ev := r.Group("/evolution")
	ev.POST("/tasks", h.CreateTask)
	ev.GET("/tasks", h.ListTasks)
	ev.POST("/tasks/:taskID/promote", h.PromoteChallenger)

	r.GET("/genome/champion", h.GetChampion)
	r.GET("/genome/challengers", h.ListChallengers)
}

// ListChallengers godoc
// GET /api/v1/genome/challengers?strategy_id=N&limit=M
// Returns the most recent challenger gene packs for a strategy.
func (h *EvolutionHandler) ListChallengers(c *gin.Context) {
	strategyID, err := parseUintParam(c.Query("strategy_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "strategy_id must be a positive integer"})
		return
	}
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := parseUintParam(v); err == nil && n > 0 && n <= 200 {
			limit = int(n)
		}
	}
	var rows []store.GeneRecord
	if err := h.db.Where("strategy_id = ? AND role = ?", strategyID, store.GeneRoleChallenger).
		Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]ChallengerSummary, len(rows))
	for i, r := range rows {
		out[i] = ChallengerSummary{
			ID:          r.ID,
			ScoreTotal:  r.ScoreTotal,
			MaxDrawdown: r.MaxDrawdown,
			CreatedAt:   r.CreatedAt,
			ParamPack:   r.ParamPack,
		}
	}
	c.JSON(http.StatusOK, gin.H{"challengers": out})
}

// CreateTask godoc
// POST /api/v1/evolution/tasks
// Creates and asynchronously starts a GA evolution task.
func (h *EvolutionHandler) CreateTask(c *gin.Context) {
	var req epoch.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.svc.CreateAndRunTask(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

// ListTasksResponse is the response for GET /api/v1/evolution/tasks.
type ListTasksResponse struct {
	Tasks       []store.EvolutionTask `json:"tasks"`
	Challengers []ChallengerSummary   `json:"challengers"`
}

// ChallengerSummary summarises one challenger GeneRecord for the review UI.
type ChallengerSummary struct {
	ID          uint            `json:"id"`
	ScoreTotal  float64         `json:"score_total"`
	MaxDrawdown float64         `json:"max_drawdown"`
	CreatedAt   time.Time       `json:"created_at"`
	ParamPack   json.RawMessage `json:"param_pack,omitempty"`
}

// ListTasks godoc
// GET /api/v1/evolution/tasks?strategy_id=N
func (h *EvolutionHandler) ListTasks(c *gin.Context) {
	strategyID, err := parseUintParam(c.Query("strategy_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "strategy_id must be a positive integer"})
		return
	}

	var tasks []store.EvolutionTask
	h.db.Where("strategy_id = ?", strategyID).Order("created_at DESC").Limit(50).Find(&tasks)

	var challengers []store.GeneRecord
	h.db.Where("strategy_id = ? AND role = ?", strategyID, store.GeneRoleChallenger).
		Order("created_at DESC").Limit(20).Find(&challengers)

	summaries := make([]ChallengerSummary, len(challengers))
	for i, ch := range challengers {
		summaries[i] = ChallengerSummary{
			ID:          ch.ID,
			ScoreTotal:  ch.ScoreTotal,
			MaxDrawdown: ch.MaxDrawdown,
			CreatedAt:   ch.CreatedAt,
			ParamPack:   ch.ParamPack,
		}
	}

	c.JSON(http.StatusOK, ListTasksResponse{Tasks: tasks, Challengers: summaries})
}

// promoteRequest is the optional body for the promote endpoint.
type promoteRequest struct {
	GeneRecordID uint `json:"gene_record_id"` // if 0, promotes the most recent challenger
}

// PromoteChallenger godoc
// POST /api/v1/evolution/tasks/:taskID/promote
// Promotes the selected challenger to champion in a single DB transaction,
// then invalidates the Redis champion cache.
func (h *EvolutionHandler) PromoteChallenger(c *gin.Context) {
	taskID, err := parseUintParam(c.Param("taskID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid taskID"})
		return
	}

	// Load the task to get the strategyID
	var task store.EvolutionTask
	if err := h.db.First(&task, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	var req promoteRequest
	_ = c.ShouldBindJSON(&req) // body is optional

	// Find the challenger to promote
	var challenger store.GeneRecord
	q := h.db.Where("strategy_id = ? AND role = ?", task.StrategyID, store.GeneRoleChallenger)
	if req.GeneRecordID > 0 {
		q = q.Where("id = ?", req.GeneRecordID)
	} else {
		q = q.Order("created_at DESC")
	}
	if err := q.First(&challenger).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no challenger found for this task"})
		return
	}

	// Transactional promote: champion → retired, challenger → champion
	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&store.GeneRecord{}).
			Where("strategy_id = ? AND role = ?", task.StrategyID, store.GeneRoleChampion).
			Update("role", store.GeneRoleRetired).Error; err != nil {
			return err
		}
		return tx.Model(&challenger).Update("role", store.GeneRoleChampion).Error
	})
	if txErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": txErr.Error()})
		return
	}

	// Invalidate Redis champion cache
	cacheKey := championKey(task.StrategyID)
	_ = h.redis.Del(context.Background(), cacheKey)

	c.JSON(http.StatusOK, gin.H{"promoted_id": challenger.ID})
}

// GetChampion godoc
// GET /api/v1/genome/champion?strategy_id=N
// Returns the current champion gene pack. Redis-cached with 1h TTL.
func (h *EvolutionHandler) GetChampion(c *gin.Context) {
	strategyID, err := parseUintParam(c.Query("strategy_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "strategy_id must be a positive integer"})
		return
	}

	cacheKey := championKey(strategyID)
	ctx := context.Background()

	if cached, err := h.redis.Get(ctx, cacheKey); err == nil && cached != "" {
		c.Data(http.StatusOK, "application/json", []byte(cached))
		return
	}

	var champ store.GeneRecord
	if err := h.db.Where("strategy_id = ? AND role = ?", strategyID, store.GeneRoleChampion).
		First(&champ).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no champion found"})
		return
	}

	payload, _ := json.Marshal(champ)
	_ = h.redis.Set(ctx, cacheKey, string(payload), championCacheTTL)

	c.Data(http.StatusOK, "application/json", payload)
}

// -- helpers --

func parseUintParam(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil || v == 0 {
		return 0, err
	}
	return uint(v), nil
}

func championKey(strategyID uint) string {
	return "champion:" + strconv.FormatUint(uint64(strategyID), 10)
}
