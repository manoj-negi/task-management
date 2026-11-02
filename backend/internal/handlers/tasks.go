package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"office-task-manager/backend/internal/auth"
)

type createTaskRequest struct {
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	ProjectID       string   `json:"projectId"`
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
		if !claims.HasPermission("create_tasks") && !claims.HasPermission("assign_tasks") {
			writeTaskJSON(w, http.StatusForbidden, map[string]string{"error": "you do not have permission to assign tasks"})
			return
		}
		project := strings.TrimSpace(r.URL.Query().Get("project"))
		if project == "" {
			writeTaskJSON(w, http.StatusOK, []assignableUser{})
			return
		}
		if !claims.HasPermission("view_all_projects") {
			var allowed bool
			err := db.QueryRowContext(r.Context(), `SELECT EXISTS (
				SELECT 1 FROM project_members pm JOIN projects p ON p.id = pm.project_id
				WHERE pm.user_id = $1::uuid AND pm.removed_at IS NULL AND p.name = $2 AND p.status <> 'archived'
			)`, claims.UserID, project).Scan(&allowed)
			if err != nil || !allowed {
				writeTaskJSON(w, http.StatusForbidden, map[string]string{"error": "you can only assign tasks in projects you belong to"})
				return
			}
		}
		// Only active members of the selected project can be assigned tasks.
		rows, err := db.QueryContext(r.Context(), `
SELECT u.id::text, u.full_name, u.email
FROM users u
JOIN project_members pm ON pm.user_id = u.id
JOIN projects p ON p.id = pm.project_id
JOIN roles r ON r.id=u.role_id
WHERE u.is_active = TRUE AND r.name = 'employee'
  AND pm.removed_at IS NULL AND p.status <> 'archived'
  AND p.name = $1
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
		if !claims.HasPermission("create_tasks") && !claims.HasPermission("assign_tasks") {
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
		req.Description = strings.TrimSpace(req.Description)
		req.Project = strings.TrimSpace(req.Project)
		req.Priority = strings.ToLower(strings.TrimSpace(req.Priority))
		if req.Title == "" || len([]rune(req.Title)) > 250 || len([]rune(req.Description)) > 10000 || req.Project == "" {
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
		var err error
		if strings.TrimSpace(req.ProjectID) != "" {
			err = db.QueryRowContext(r.Context(), `SELECT id::text FROM projects WHERE id = $1::uuid AND status <> 'archived'`, strings.TrimSpace(req.ProjectID)).Scan(&projectID)
		} else {
			// Backwards-compatible name lookup: reject duplicates rather than
			// silently attaching the task to an arbitrary project.
			var matches int
			err = db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM projects WHERE name = $1 AND status <> 'archived'`, req.Project).Scan(&matches)
			if err == nil && matches > 1 {
				writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "multiple projects share this name; select a project ID"})
				return
			}
			if err == nil && matches == 1 {
				err = db.QueryRowContext(r.Context(), `SELECT id::text FROM projects WHERE name = $1 AND status <> 'archived'`, req.Project).Scan(&projectID)
			} else if err == nil {
				err = sql.ErrNoRows
			}
		}
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "selected project was not found"})
				return
			}
			writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project ID or project"})
			return
		}
		// Non-admin task assigners are scoped to projects they are active members of.
		if !claims.HasPermission("view_all_projects") {
			var member bool
			err = db.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM project_members WHERE project_id = $1::uuid AND user_id = $2::uuid AND removed_at IS NULL)`, projectID, claims.UserID).Scan(&member)
			if err != nil || !member {
				writeTaskJSON(w, http.StatusForbidden, map[string]string{"error": "you can only create tasks in projects you belong to"})
				return
			}
		}
		// Selected assignees must be active employees and active members of this project.
		for _, userID := range userIDs {
			var valid bool
			err := db.QueryRowContext(r.Context(), `SELECT EXISTS (
				SELECT 1 FROM users u
				JOIN project_members pm ON pm.user_id = u.id
				JOIN roles r ON r.id=u.role_id
				WHERE u.id::text = $1 AND u.is_active = TRUE AND r.name = 'employee'
				  AND pm.project_id = $2::uuid AND pm.removed_at IS NULL
			)`, userID, projectID).Scan(&valid)
			if err != nil || !valid {
				writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "every assignee must be an active employee who is a member of this project"})
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
		err = tx.QueryRowContext(r.Context(), `INSERT INTO tasks (project_id, title, description, status, priority, created_by, due_at) VALUES ($1, $2, NULLIF($3, ''), 'todo', $4, $5, $6) RETURNING id::text`, projectID, req.Title, req.Description, req.Priority, claims.UserID, due).Scan(&taskID)
		if err != nil {
			writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save task"})
			return
		}
		for _, userID := range userIDs {
			if _, err := tx.ExecContext(
				r.Context(),
				`INSERT INTO task_assignees
        (task_id, project_id, user_id, assigned_by, work_status)
     VALUES ($1, $2, $3, $4, 'assigned')`,
				taskID, projectID, userID, claims.UserID,
			); err != nil {
				log.Printf(
					"TASK ASSIGNMENT INSERT FAILED: task_id=%s project_id=%s user_id=%s error=%v",
					taskID, projectID, userID, err,
				)

				writeTaskJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "task was not saved because an assignment failed",
				})
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
