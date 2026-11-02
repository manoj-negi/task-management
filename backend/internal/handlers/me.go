package handlers

import (
	"database/sql"
	"errors"
	"net/http"

	"office-task-manager/backend/internal/auth"
)

func (h AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAuthJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
		return
	}

	user := loginUser{ID: claims.UserID, Email: claims.Email, Role: claims.Role, CanAssignTasks: claims.CanAssignTasks}

	err := h.DB.QueryRowContext(
		r.Context(),
		`SELECT full_name FROM users WHERE id = $1`,
		claims.UserID,
	).Scan(&user.FullName)

	if errors.Is(err, sql.ErrNoRows) {
		writeAuthJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found or inactive"})
		return
	}
	if err != nil {
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load current user"})
		return
	}

	writeAuthJSON(w, http.StatusOK, map[string]any{"user": user})
}
