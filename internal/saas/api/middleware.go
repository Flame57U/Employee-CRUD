package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/auth"
)

// Gin context keys for claims propagated by JWTAuth.
const (
	ctxUserID = "auth.user_id"
	ctxRole   = "auth.role"
)

// Roles that may access /api/v1/evolution and /api/v1/backtests.
var labRoles = map[string]struct{}{
	"lab": {},
	"dev": {},
}

// JWTAuth validates Bearer tokens and stuffs UserID / Role into the Gin context.
// Rejects requests whose Authorization header is absent or malformed.
func JWTAuth(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if h == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}
		const bearer = "Bearer "
		if !strings.HasPrefix(h, bearer) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header must be Bearer scheme"})
			return
		}
		claims, err := svc.ParseToken(strings.TrimPrefix(h, bearer))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set(ctxUserID, claims.UserID)
		c.Set(ctxRole, claims.Role)
		c.Next()
	}
}

// RequireLabRole gates routes behind app_role ∈ {lab, dev}.
// Must run after JWTAuth.
func RequireLabRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(ctxRole)
		r, _ := role.(string)
		if _, ok := labRoles[r]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "lab or dev role required"})
			return
		}
		c.Next()
	}
}

// userIDFromCtx extracts the JWT-derived userID written by JWTAuth.
// Panics-as-zero are intentional: routes that need a user must be behind JWTAuth.
func userIDFromCtx(c *gin.Context) uint {
	v, ok := c.Get(ctxUserID)
	if !ok {
		return 0
	}
	id, _ := v.(uint)
	return id
}
