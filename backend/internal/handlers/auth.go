package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"office-task-manager/backend/internal/auth"
)

type AuthHandler struct {
	DB *sql.DB
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	User loginUser `json:"user"`
}

type loginUser struct {
	ID             string `json:"id"`
	FullName       string `json:"full_name"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	CanAssignTasks bool   `json:"can_assign_tasks"`
}

func (h AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeAuthJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	if h.DB == nil {
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "database unavailable",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req loginRequest
	if err := decoder.Decode(&req); err != nil {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	// Reject additional JSON values after the first object.
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{
			"error": "request must contain one JSON object",
		})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{
			"error": "email and password are required",
		})
		return
	}

	var user loginUser
	var passwordHash string
	var isActive bool

	err := h.DB.QueryRowContext(
		r.Context(),
		`SELECT u.id::text, u.full_name, u.email, r.name,
		        EXISTS (SELECT 1 FROM permissions p LEFT JOIN role_permissions rp ON rp.permission_id=p.id AND rp.role_id=u.role_id LEFT JOIN user_permissions up ON up.permission_id=p.id AND up.user_id=u.id WHERE p.code='assign_tasks' AND (rp.role_id IS NOT NULL OR up.user_id IS NOT NULL)),
		        u.password_hash, u.is_active
		 FROM users u JOIN roles r ON r.id=u.role_id
		 WHERE lower(email) = $1`,
		req.Email,
	).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.Role,
		&user.CanAssignTasks,
		&passwordHash,
		&isActive,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAuthJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid email or password",
			})
			return
		}

		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to process login",
		})
		return
	}

	if !isActive || bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(req.Password),
	) != nil {
		writeAuthJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "invalid email or password",
		})
		return
	}

	token, err := auth.SignToken(
		user.ID,
		user.Email,
		user.Role,
		user.CanAssignTasks,
	)
	if err != nil {
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to create authentication token",
		})
		return
	}

	http.SetCookie(w, sessionCookie(r, token, 28800))
	writeAuthJSON(w, http.StatusOK, loginResponse{User: user})
}

func (h AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeAuthJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	cookie := sessionCookie(r, "", -1)
	cookie.Expires = time.Unix(1, 0)
	http.SetCookie(w, cookie)
	writeAuthJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func sessionCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	secure := r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
	return &http.Cookie{
		Name: auth.SessionCookieName, Value: value, Path: "/api", HttpOnly: true,
		Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge,
	}
}

func writeAuthJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}
