// Package api wires up the REST router for the SaaS server.
//
// Layering:
//   /                         — health probe + WebSocket
//   /api/v1/auth/*            — public (register, login)
//   /api/v1/...               — JWT-protected user routes
//   /api/v1/evolution/*       — JWT + lab/dev role
//   /api/v1/backtests/*       — JWT + lab/dev role
//   /api/v1/genome/*          — JWT + lab/dev role
//   /ws/agent                 — Agent long-poll WebSocket (auth via first frame)
//
// Each handler owns its own RegisterRoutes; this file's only job is to
// assemble the middleware stacks and delegate.
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/auth"
	"github.com/quantsaas/platform/internal/saas/epoch"
	"github.com/quantsaas/platform/internal/saas/instance"
	"github.com/quantsaas/platform/internal/saas/store"
	"github.com/quantsaas/platform/internal/saas/ws"
)

// Deps bundles every collaborator the REST router needs. Assembled in main.go.
type Deps struct {
	DB        *store.DB
	Redis     *store.RedisClient
	Auth      *auth.Service
	Hub       *ws.Hub
	Manager   *instance.Manager
	EvolveSvc *epoch.EpochService
}

// corsMiddleware sets permissive CORS headers so browser clients on any origin
// can call the API. Adjust allowed origins for production hardening.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Register installs all routes on r using deps. Returns r for chaining.
func Register(r *gin.Engine, deps Deps) *gin.Engine {
	r.Use(corsMiddleware())

	// Root — basic API info, no auth required.
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "QuantSaaS API",
			"version": "v1",
			"status":  "ok",
			"docs":    "/api/v1",
		})
	})

	// Health probe — unauthenticated, no /api/v1 prefix, suitable for L4 probes.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Agent WebSocket — auth performed inside Hub.HandleConnection, not via JWTAuth.
	r.GET("/ws/agent", deps.Hub.HandleConnection)

	v1 := r.Group("/api/v1")

	// ── Public ───────────────────────────────────────────────────────────────
	authH := NewAuthHandler(deps.DB, deps.Auth)
	authH.RegisterRoutes(v1)

	// ── Authenticated user routes ────────────────────────────────────────────
	user := v1.Group("")
	user.Use(JWTAuth(deps.Auth))
	{
		NewStrategyHandler(deps.DB).RegisterRoutes(user)
		NewInstanceHandler(deps.DB, deps.Manager).RegisterRoutes(user)
		NewDashboardHandler(deps.DB).RegisterRoutes(user)
		NewAgentHandler(deps.Hub).RegisterRoutes(user)
	}

	// ── Lab/Dev only ─────────────────────────────────────────────────────────
	lab := v1.Group("")
	lab.Use(JWTAuth(deps.Auth), RequireLabRole())
	{
		NewEvolutionHandler(deps.EvolveSvc, deps.DB, deps.Redis).RegisterRoutes(lab)
		NewBacktestHandler(deps.DB).RegisterRoutes(lab)
	}

	return r
}
