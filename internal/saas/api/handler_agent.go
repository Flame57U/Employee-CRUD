package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/ws"
)

// AgentHandler exposes WebSocket connection state to the REST API.
type AgentHandler struct {
	hub *ws.Hub
}

// NewAgentHandler constructs an AgentHandler.
func NewAgentHandler(hub *ws.Hub) *AgentHandler {
	return &AgentHandler{hub: hub}
}

// RegisterRoutes mounts /agents/status on r.
func (h *AgentHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/agents/status", h.Status)
}

// Status godoc
// GET /api/v1/agents/status
// Reports whether THIS user's Agent is currently connected to the Hub.
func (h *AgentHandler) Status(c *gin.Context) {
	uid := userIDFromCtx(c)
	c.JSON(http.StatusOK, gin.H{
		"user_id":   uid,
		"connected": h.hub.IsConnected(uid),
	})
}
