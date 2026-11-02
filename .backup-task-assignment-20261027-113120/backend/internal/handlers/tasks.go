package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"office-task-manager/backend/internal/auth"
)

type createTaskRequest struct {
	Title    string `json:"title"`
	Project  string `json:"project"`
	Priority string `json:"priority"`
	DueDate  string `json:"dueDate"`
}

func CreateTask(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeTaskJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			writeTaskJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		var canAssign bool
		var active bool
		err := db.QueryRowContext(r.Context(), `SELECT can_assign_tasks, is_active FROM users WHERE id = $1`, claims.UserID).Scan(&canAssign, &active)
		if err != nil || !active {
			writeTaskJSON(w, http.StatusUnauthorized, map[string]string{"error": "active user required"})
			return
		}
		if claims.Role != "super_admin" && !canAssign {
			writeTaskJSON(w, http.StatusForbidden, map[string]string{"error": "you do not have permission to create tasks"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer r.Body.Close()
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var req createTaskRequest
		if err := decoder.Decode(&req); err != nil {
			writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "request must contain one JSON object"})
			return
		}
		req.Title = strings.TrimSpace(req.Title)
		req.Project = strings.TrimSpace(req.Project)
		req.Priority = strings.ToLower(strings.TrimSpace(req.Priority))
		if req.Title == "" || len([]rune(req.Title)) > 250 || req.Project == "" {
			writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "task title (1-250 characters) and project are required"})
			return
		}
		switch req.Priority {
		case "low", "medium", "high", "urgent":
		default:
			writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "priority must be low, medium, high, or urgent"})
			return
		}
		var projectID string
		err = db.QueryRowContext(r.Context(), `SELECT id::text FROM projects WHERE name = $1 AND status <> 'archived' ORDER BY created_at DESC LIMIT 1`, req.Project).Scan(&projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "selected project was not found"})
				return
			}
			writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not find project"})
			return
		}
		var due any
		if strings.TrimSpace(req.DueDate) != "" {
			parsed, parseErr := time.Parse("2006-01-02", req.DueDate)
			if parseErr != nil {
				writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "dueDate must use YYYY-MM-DD format"})
				return
			}
			due = parsed
		}
		var taskID string
		err = db.QueryRowContext(r.Context(), `INSERT INTO tasks (project_id, title, status, priority, created_by, due_at) VALUES ($1, $2, 'todo', $3, $4, $5) RETURNING id::text`, projectID, req.Title, req.Priority, claims.UserID, due).Scan(&taskID)
		if err != nil {
			writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save task"})
			return
		}
		writeTaskJSON(w, http.StatusCreated, map[string]any{"id": taskID, "title": req.Title, "project": req.Project, "priority": strings.Title(req.Priority), "status": "To do", "dueDate": req.DueDate})
	}
}

func writeTaskJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
