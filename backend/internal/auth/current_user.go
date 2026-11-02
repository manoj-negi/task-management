package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

// RequireCurrentUser refreshes authorization-relevant claims from PostgreSQL on
// every protected request. JWTs identify the user; the database remains the
// source of truth for role, active status, and task-assignment permission.
func RequireCurrentUser(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			writeCurrentUserError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		var role, email, permissionsJSON string
		var active bool
		err := db.QueryRowContext(r.Context(),
			`SELECT u.email, r.name, u.is_active,
				COALESCE((SELECT json_agg(DISTINCT p.code)::text FROM permissions p
				 LEFT JOIN role_permissions rp ON rp.permission_id=p.id AND rp.role_id=u.role_id
				 LEFT JOIN user_permissions up ON up.permission_id=p.id AND up.user_id=u.id
				 WHERE rp.role_id IS NOT NULL OR up.user_id IS NOT NULL), '[]')
			 FROM users u JOIN roles r ON r.id=u.role_id WHERE u.id = $1::uuid`,
			claims.UserID,
		).Scan(&email, &role, &active, &permissionsJSON)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && !active) {
			writeCurrentUserError(w, http.StatusUnauthorized, "user not found or inactive")
			return
		}
		if err != nil {
			writeCurrentUserError(w, http.StatusInternalServerError, "could not verify user")
			return
		}

		// Claims are request-local; updating them cannot modify the signed token.
		refreshed := *claims
		refreshed.Email = email
		refreshed.Role = role
		if err := json.Unmarshal([]byte(permissionsJSON), &refreshed.Permissions); err != nil {
			writeCurrentUserError(w, http.StatusInternalServerError, "could not verify user permissions")
			return
		}
		refreshed.CanAssignTasks = refreshed.HasPermission("assign_tasks")
		ctx := context.WithValue(r.Context(), claimsContextKey, &refreshed)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeCurrentUserError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
