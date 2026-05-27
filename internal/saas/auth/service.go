package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/quantsaas/platform/internal/saas/config"
)

var ErrInvalidToken = errors.New("invalid token")

// Claims embeds the standard JWT claims plus application-level fields.
type Claims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Service signs and parses JWTs using HS256.
type Service struct {
	secret      []byte
	expiryHours int
}

// NewService constructs a Service from the JWT section of the app config.
func NewService(cfg *config.Config) *Service {
	return &Service{
		secret:      []byte(cfg.JWT.Secret),
		expiryHours: cfg.JWT.ExpiryHours,
	}
}

// SignToken returns a signed JWT for the given user and role.
func (s *Service) SignToken(userID uint, role string) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(s.expiryHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// ParseToken validates the token string and returns its claims.
func (s *Service) ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
