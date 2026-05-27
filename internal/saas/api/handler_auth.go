package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/auth"
	"github.com/quantsaas/platform/internal/saas/store"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthHandler exposes register and login.
type AuthHandler struct {
	db  *store.DB
	svc *auth.Service
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(db *store.DB, svc *auth.Service) *AuthHandler {
	return &AuthHandler{db: db, svc: svc}
}

// RegisterRoutes mounts /auth/register and /auth/login on r (the public group).
func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/auth")
	g.POST("/register", h.Register)
	g.POST("/login", h.Login)
}

type authRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// Register godoc
// POST /api/v1/auth/register
// Creates a new user with the default "free" plan and "user" role,
// then returns a freshly minted JWT for convenience.
func (h *AuthHandler) Register(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var existing store.User
	if err := h.db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	u := store.User{Email: req.Email, PasswordHash: string(hash), Plan: "free"}
	if err := h.db.Create(&u).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create user: " + err.Error()})
		return
	}

	token, err := h.svc.SignToken(u.ID, "user")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue token: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"user_id": u.ID,
		"email":   u.Email,
		"plan":    u.Plan,
		"token":   token,
	})
}

// Login godoc
// POST /api/v1/auth/login
// Verifies credentials and returns a JWT. The token's `role` claim is "user"
// for normal users; lab/dev role assignment is out of scope for this endpoint
// and is administered separately (e.g. via a Plan-style column or admin tool).
func (h *AuthHandler) Login(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var u store.User
	if err := h.db.Where("email = ?", req.Email).First(&u).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	role := planToRole(u.Plan)
	token, err := h.svc.SignToken(u.ID, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue token: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id": u.ID,
		"email":   u.Email,
		"plan":    u.Plan,
		"role":    role,
		"token":   token,
	})
}

// planToRole maps a subscription plan to the JWT role claim.
// "elite" plan members receive lab access; everyone else is a regular user.
// A dedicated admin tool is the right way to elevate to dev — keep that out
// of the user-facing login path.
func planToRole(plan string) string {
	if plan == "elite" {
		return "lab"
	}
	return "user"
}
