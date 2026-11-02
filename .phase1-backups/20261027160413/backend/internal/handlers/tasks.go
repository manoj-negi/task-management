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
	Title           string   `json:"title"`
	Project         string   `json:"project"`
	Priority        string   `json:"priority"`
	DueDate         string   `json:"dueDate"`
	AssignedUserIDs []string `json:"assignedUserIds"`
}

type assignableUser struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
}

func AssignableUsers(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeTaskJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			writeTaskJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		var canAssign, active bool
		if err := db.QueryRowContext(r.Context(), `SELECT can_assign_tasks, is_active FROM users WHERE id = $1`, claims.UserID).Scan(&canAssign, &active); err != nil || !active {
			writeTaskJSON(w, http.StatusUnauthorized, map[string]string{"error": "active user required"})
			return
		}
		if claims.Role != "super_admin" && !canAssign {
			writeTaskJSON(w, http.StatusForbidden, map[string]string{"error": "you do not have permission to assign tasks"})
			return
		}
		project := strings.TrimSpace(r.URL.Query().Get("project"))
		if project == "" {
			writeTaskJSON(w, http.StatusOK, []assignableUser{})
			return
		}
		rows, err := db.QueryContext(r.Context(), `
SELECT DISTINCT u.id::text, u.full_name, u.email
FROM users u
JOIN project_members pm ON pm.user_id = u.id AND pm.removed_at IS NULL
JOIN projects p ON p.id = pm.project_id
WHERE u.is_active = TRUE AND p.name = $1 AND p.status <> 'archived'
ORDER BY u.full_name`, project)
		if err != nil {
			writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load project employees"})
			return
		}
		defer rows.Close()
		users := make([]assignableUser, 0)
		for rows.Next() {
			var user assignableUser
			if err := rows.Scan(&user.ID, &user.FullName, &user.Email); err != nil {
				writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not read project employees"})
				return
			}
			users = append(users, user)
		}
		if err := rows.Err(); err != nil {
			writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not read project employees"})
			return
		}
		writeTaskJSON(w, http.StatusOK, users)
	}
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
		var canAssign, active bool
		if err := db.QueryRowContext(r.Context(), `SELECT can_assign_tasks, is_active FROM users WHERE id = $1`, claims.UserID).Scan(&canAssign, &active); err != nil || !active {
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
		// Deduplicate selected employee IDs before validating and inserting them.
		seen := map[string]bool{}
		userIDs := make([]string, 0, len(req.AssignedUserIDs))
		for _, id := range req.AssignedUserIDs {
			id = strings.TrimSpace(id)
			if id != "" && !seen[id] {
				seen[id] = true
				userIDs = append(userIDs, id)
			}
		}
		var projectID string
		err := db.QueryRowContext(r.Context(), `SELECT id::text FROM projects WHERE name = $1 AND status <> 'archived' ORDER BY created_at DESC LIMIT 1`, req.Project).Scan(&projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "selected project was not found"})
				return
			}
			writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not find project"})
			return
		}
		// Every selected employee must be an active member of the selected project.
		for _, userID := range userIDs {
			var valid bool
			err := db.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM users u JOIN project_members pm ON pm.user_id = u.id AND pm.removed_at IS NULL WHERE u.id::text = $1 AND u.is_active = TRUE AND pm.project_id::text = $2)`, userID, projectID).Scan(&valid)
			if err != nil || !valid {
				writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "one or more selected employees are not active members of this project"})
				return
			}
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
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not start task transaction"})
			return
		}
		defer tx.Rollback()
		var taskID string
		err = tx.QueryRowContext(r.Context(), `INSERT INTO tasks (project_id, title, status, priority, created_by, due_at) VALUES ($1, $2, 'todo', $3, $4, $5) RETURNING id::text`, projectID, req.Title, req.Priority, claims.UserID, due).Scan(&taskID)
		if err != nil {
			writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save task"})
			return
		}
		for _, userID := range userIDs {
			if _, err := tx.ExecContext(r.Context(), `INSERT INTO task_assignees (task_id, project_id, user_id, assigned_by, work_status) VALUES ($1, $2, $3, $4, 'assigned')`, taskID, projectID, userID, claims.UserID); err != nil {
				writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "task was not saved because an assignment failed"})
				return
			}
		}
		if err := tx.Commit(); err != nil {
			writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not commit task"})
			return
		}
		writeTaskJSON(w, http.StatusCreated, map[string]any{"id": taskID, "title": req.Title, "project": req.Project, "priority": strings.Title(req.Priority), "status": "To do", "dueDate": req.DueDate, "assignedUserIds": userIDs})
	}
}

func writeTaskJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
