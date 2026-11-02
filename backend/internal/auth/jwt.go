package auth

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrMissingJWTSecret = errors.New("JWT_SECRET is not set or is too short")

type Claims struct {
	UserID         string   `json:"user_id"`
	Email          string   `json:"email"`
	Role           string   `json:"role"`
	CanAssignTasks bool     `json:"can_assign_tasks"`
	Permissions    []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

func (c *Claims) HasPermission(code string) bool {
	for _, permission := range c.Permissions {
		if permission == code {
			return true
		}
	}
	return false
}

func SignToken(userID, email, role string, canAssignTasks bool) (string, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if len(secret) < 32 {
		return "", ErrMissingJWTSecret
	}

	now := time.Now()
	claims := Claims{
		UserID: userID, Email: email, Role: role, CanAssignTasks: canAssignTasks,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID, IssuedAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(8 * time.Hour)),
			Issuer:    "office-task-manager",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}
