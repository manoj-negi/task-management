package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const claimsContextKey contextKey = "auth_claims"
const SessionCookieName = "office_task_manager_session"

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeUnauthorized := func() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "authentication required",
			})
		}

		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeUnauthorized()
			return
		}

		secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
		if len(secret) < 32 {
			http.Error(
				w,
				"authentication service unavailable",
				http.StatusInternalServerError,
			)
			return
		}

		claims := &Claims{}

		token, err := jwt.ParseWithClaims(
			cookie.Value,
			claims,
			func(token *jwt.Token) (any, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, errors.New("unexpected signing algorithm")
				}
				return []byte(secret), nil
			},
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithIssuer("office-task-manager"),
		)

		if err != nil || token == nil || !token.Valid ||
			claims.ExpiresAt == nil ||
			strings.TrimSpace(claims.UserID) == "" ||
			strings.TrimSpace(claims.Email) == "" ||
			strings.TrimSpace(claims.Role) == "" {
			writeUnauthorized()
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*Claims)
	return claims, ok && claims != nil
}
