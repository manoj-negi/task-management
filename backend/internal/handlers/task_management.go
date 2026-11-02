package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"office-task-manager/backend/internal/auth"
)

type adminTaskDetails struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	ProjectID   string   `json:"projectId"`
	Project     string   `json:"project"`
	Priority    string   `json:"priority"`
	Status      string   `json:"status"`
	StartDate   string   `json:"startDate"`
	DueDate     string   `json:"dueDate"`
	Assignees   []string `json:"assignees"`
}

// AdminTaskRouter keeps the existing review endpoint and adds task detail/edit/delete actions.
func AdminTaskRouter(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) == 4 && parts[0] == "api" && parts[1] == "admin" && parts[2] == "tasks" {
			ReviewTask(db).ServeHTTP(w, r)
			return
		}
		if len(parts) != 5 || parts[0] != "api" || parts[1] != "admin" || parts[2] != "tasks" || strings.TrimSpace(parts[3]) == "" {
			writeTaskJSON(w, http.StatusNotFound, map[string]string{"error": "task management endpoint not found"})
			return
		}
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			writeTaskJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		if !claims.HasPermission("manage_tasks") {
			writeTaskJSON(w, http.StatusForbidden, map[string]string{"error": "super admin access required"})
			return
		}
		if len(parts) == 5 && parts[4] == "subtasks" {
			if r.Method != http.MethodPost {
				writeTaskJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
				return
			}
			createAdminSubtask(db, w, r, parts[3], claims.UserID)
			return
		}
		switch parts[4] {
		case "details":
			if r.Method != http.MethodGet {
				writeTaskJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
				return
			}
			getAdminTaskDetails(db, w, r, parts[3])
		case "manage":
			if r.Method != http.MethodPost {
				writeTaskJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
				return
			}
			updateAdminTask(db, w, r, parts[3], claims.UserID)
		case "delete":
			if r.Method != http.MethodPost {
				writeTaskJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
				return
			}
			deleteAdminTask(db, w, r, parts[3], claims.UserID)
		default:
			writeTaskJSON(w, http.StatusNotFound, map[string]string{"error": "task management action not found"})
		}
	}
}

func createAdminSubtask(db *sql.DB, w http.ResponseWriter, r *http.Request, parentID, adminID string) {
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Priority    string   `json:"priority"`
		DueDate     string   `json:"dueDate"`
		Assignees   []string `json:"assignedUserIds"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&req); err != nil {
		writeTaskJSON(w, 400, map[string]string{"error": "invalid subtask request"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.Priority = strings.ToLower(strings.TrimSpace(req.Priority))
	if req.Title == "" || len([]rune(req.Title)) > 250 || len([]rune(req.Description)) > 10000 {
		writeTaskJSON(w, 400, map[string]string{"error": "title is required and description must be under 10000 characters"})
		return
	}
	switch req.Priority {
	case "low", "medium", "high", "urgent":
	default:
		writeTaskJSON(w, 400, map[string]string{"error": "invalid subtask priority"})
		return
	}
	var due any
	if strings.TrimSpace(req.DueDate) != "" {
		parsed, err := time.Parse("2006-01-02", req.DueDate)
		if err != nil {
			writeTaskJSON(w, 400, map[string]string{"error": "dueDate must use YYYY-MM-DD format"})
			return
		}
		due = parsed
	}
	seen := map[string]bool{}
	ids := make([]string, 0, len(req.Assignees))
	for _, id := range req.Assignees {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not start subtask creation"})
		return
	}
	defer tx.Rollback()
	var projectID string
	if err = tx.QueryRowContext(r.Context(), `SELECT project_id::text FROM tasks WHERE id=$1::uuid AND deleted_at IS NULL FOR UPDATE`, parentID).Scan(&projectID); err != nil {
		writeTaskJSON(w, 404, map[string]string{"error": "parent task not found"})
		return
	}
	for _, id := range ids {
		var valid bool
		err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM users u JOIN roles r ON r.id=u.role_id JOIN project_members pm ON pm.user_id=u.id WHERE u.id=$1::uuid AND u.is_active AND r.name='employee' AND pm.project_id=$2::uuid AND pm.removed_at IS NULL)`, id, projectID).Scan(&valid)
		if err != nil || !valid {
			writeTaskJSON(w, 400, map[string]string{"error": "every assignee must be an active employee in this project"})
			return
		}
	}
	var subtaskID string
	err = tx.QueryRowContext(r.Context(), `INSERT INTO tasks(project_id,parent_task_id,title,description,priority,due_at,created_by) VALUES($1::uuid,$2::uuid,$3,NULLIF($4,''),$5::task_priority,$6,$7::uuid) RETURNING id::text`, projectID, parentID, req.Title, req.Description, req.Priority, due, adminID).Scan(&subtaskID)
	if err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not create subtask"})
		return
	}
	for _, id := range ids {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO task_assignees(task_id,project_id,user_id,assigned_by) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid)`, subtaskID, projectID, id, adminID); err != nil {
			writeTaskJSON(w, 400, map[string]string{"error": "could not assign subtask"})
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO task_activity(task_id,actor_id,action,new_values) VALUES($1::uuid,$2::uuid,'task_created',jsonb_build_object('title',$3,'parent_task_id',$4::text))`, subtaskID, adminID, req.Title, parentID); err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not record subtask creation"})
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO task_activity(task_id,actor_id,action,new_values) VALUES($1::uuid,$2::uuid,'subtask_created',jsonb_build_object('subtask_id',$3::text,'title',$4))`, parentID, adminID, subtaskID, req.Title); err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not record subtask creation"})
		return
	}
	if err = tx.Commit(); err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not save subtask"})
		return
	}
	writeTaskJSON(w, 201, map[string]string{"id": subtaskID, "message": "Subtask created"})
}

func getAdminTaskDetails(db *sql.DB, w http.ResponseWriter, r *http.Request, taskID string) {
	var task adminTaskDetails
	var start, due sql.NullTime
	err := db.QueryRowContext(r.Context(), `
SELECT t.id::text, t.title, COALESCE(t.description,''), p.id::text, p.name,
       t.priority::text, t.status::text, t.start_at, t.due_at
FROM tasks t JOIN projects p ON p.id=t.project_id WHERE t.id=$1::uuid AND t.deleted_at IS NULL`, taskID).Scan(
		&task.ID, &task.Title, &task.Description, &task.ProjectID, &task.Project, &task.Priority, &task.Status, &start, &due)
	if errors.Is(err, sql.ErrNoRows) {
		writeTaskJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	if err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load task details"})
		return
	}
	if start.Valid {
		task.StartDate = start.Time.Format("2006-01-02")
	}
	if due.Valid {
		task.DueDate = due.Time.Format("2006-01-02")
	}
	task.Assignees = make([]string, 0)
	rows, err := db.QueryContext(r.Context(), `SELECT user_id::text FROM task_assignees WHERE task_id=$1::uuid ORDER BY assigned_at`, taskID)
	if err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load task assignees"})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not read task assignees"})
			return
		}
		task.Assignees = append(task.Assignees, id)
	}
	if err := rows.Err(); err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not read task assignees"})
		return
	}
	writeTaskJSON(w, http.StatusOK, task)
}

func updateAdminTask(db *sql.DB, w http.ResponseWriter, r *http.Request, taskID, adminID string) {
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Project     string   `json:"project"`
		Priority    string   `json:"priority"`
		Status      string   `json:"status"`
		StartDate   string   `json:"startDate"`
		DueDate     string   `json:"dueDate"`
		Assignees   []string `json:"assignedUserIds"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.Project = strings.TrimSpace(req.Project)
	req.Priority = strings.ToLower(strings.TrimSpace(req.Priority))
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Title == "" || len([]rune(req.Title)) > 250 || len([]rune(req.Description)) > 10000 {
		writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required (maximum 250 characters); description must be under 10000 characters"})
		return
	}
	switch req.Status {
	case "todo", "in_progress", "in_review", "changes_requested", "approved", "blocked", "cancelled":
	default:
		writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid task status"})
		return
	}
	switch req.Priority {
	case "low", "medium", "high", "urgent":
	default:
		writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "priority must be low, medium, high, or urgent"})
		return
	}
	var start, due any
	if strings.TrimSpace(req.StartDate) != "" {
		parsed, err := time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "startDate must use YYYY-MM-DD format"})
			return
		}
		start = parsed
	}
	if strings.TrimSpace(req.DueDate) != "" {
		parsed, err := time.Parse("2006-01-02", req.DueDate)
		if err != nil {
			writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "dueDate must use YYYY-MM-DD format"})
			return
		}
		due = parsed
	}
	if startTime, ok := start.(time.Time); ok {
		if dueTime, dueOK := due.(time.Time); dueOK && dueTime.Before(startTime) {
			writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "due date cannot be before the start date"})
			return
		}
	}
	seen := map[string]bool{}
	ids := make([]string, 0, len(req.Assignees))
	for _, id := range req.Assignees {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not start task update"})
		return
	}
	defer tx.Rollback()
	var oldProjectID, parentID string
	err = tx.QueryRowContext(r.Context(), `SELECT project_id::text,COALESCE(parent_task_id::text,'') FROM tasks WHERE id=$1::uuid AND deleted_at IS NULL FOR UPDATE`, taskID).Scan(&oldProjectID, &parentID)
	if errors.Is(err, sql.ErrNoRows) {
		writeTaskJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	projectID := oldProjectID
	if req.Project != "" {
		err = tx.QueryRowContext(r.Context(), `SELECT id::text FROM projects WHERE name=$1 AND status <> 'archived'`, req.Project).Scan(&projectID)
		if errors.Is(err, sql.ErrNoRows) {
			writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "select an active project"})
			return
		}
		if err != nil {
			writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not validate project"})
			return
		}
	}
	projectChanged := projectID != oldProjectID
	if projectChanged {
		var hasChildren bool
		if parentID != "" || tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM tasks WHERE parent_task_id=$1::uuid)`, taskID).Scan(&hasChildren) != nil || hasChildren {
			writeTaskJSON(w, http.StatusConflict, map[string]string{"error": "move subtasks separately before changing the project of a parent or subtask"})
			return
		}
	}
	if err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load task"})
		return
	}
	for _, id := range ids {
		var valid bool
		err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (
SELECT 1 FROM users u JOIN roles r ON r.id=u.role_id JOIN project_members pm ON pm.user_id=u.id
WHERE u.id=$1::uuid AND u.is_active=TRUE AND r.name='employee'
AND pm.project_id=$2::uuid AND pm.removed_at IS NULL)`, id, projectID).Scan(&valid)
		if err != nil || !valid {
			writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "every assignee must be an active employee who is a member of this project"})
			return
		}
	}
	var oldTitle, oldDescription, oldPriority, oldStatus string
	err = tx.QueryRowContext(r.Context(), `SELECT title,COALESCE(description,''),priority::text,status::text FROM tasks WHERE id=$1::uuid`, taskID).Scan(&oldTitle, &oldDescription, &oldPriority, &oldStatus)
	if err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not load existing task values"})
		return
	}
	if (req.Status == "in_review" || req.Status == "changes_requested") && req.Status != oldStatus {
		writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "use the Approvals workflow to submit or return work for review"})
		return
	}
	var oldAssigneesJSON string
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(json_agg(user_id::text),'[]'::json)::text FROM task_assignees WHERE task_id=$1::uuid`, taskID).Scan(&oldAssigneesJSON); err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not load current assignees"})
		return
	}
	if projectChanged {
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM task_assignees WHERE task_id=$1::uuid`, taskID); err != nil {
			writeTaskJSON(w, 500, map[string]string{"error": "could not reassign project members"})
			return
		}
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE tasks SET project_id=$2::uuid,title=$3,description=NULLIF($4,''),priority=$5::task_priority,status=$6::task_status,start_at=$7,due_at=$8,approved_at=CASE WHEN $6='approved' THEN COALESCE(approved_at,NOW()) ELSE NULL END,updated_at=NOW() WHERE id=$1::uuid`, taskID, projectID, req.Title, req.Description, req.Priority, req.Status, start, due)
	if err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update task details"})
		return
	}
	// Preserve current assignee work state unless moving the task to another project.
	for _, id := range ids {
		workStatus := "assigned"
		_, err = tx.ExecContext(r.Context(), `INSERT INTO task_assignees (task_id, project_id, user_id, assigned_by, work_status)
VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::assignee_work_status) ON CONFLICT (task_id,user_id) DO NOTHING`, taskID, projectID, id, adminID, workStatus)
		if err != nil {
			writeTaskJSON(w, http.StatusBadRequest, map[string]string{"error": "could not add an assignee; verify project membership"})
			return
		}
	}
	if len(ids) == 0 {
		_, err = tx.ExecContext(r.Context(), `DELETE FROM task_assignees WHERE task_id=$1::uuid`, taskID)
	} else {
		_, err = tx.ExecContext(r.Context(), `DELETE FROM task_assignees WHERE task_id=$1::uuid AND NOT (user_id::text = ANY($2::text[]))`, taskID, postgresTextArray(ids))
	}
	if err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update task assignees"})
		return
	}
	newAssigneesJSON, _ := json.Marshal(ids)
	oldValues, err := json.Marshal(struct {
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Priority    string          `json:"priority"`
		Status      string          `json:"status"`
		ProjectID   string          `json:"project_id"`
		Assignees   json.RawMessage `json:"assignees"`
	}{oldTitle, oldDescription, oldPriority, oldStatus, oldProjectID, json.RawMessage(oldAssigneesJSON)})
	if err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not prepare task audit history"})
		return
	}
	newValues, err := json.Marshal(struct {
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Priority    string          `json:"priority"`
		Status      string          `json:"status"`
		ProjectID   string          `json:"project_id"`
		Assignees   json.RawMessage `json:"assignees"`
	}{req.Title, req.Description, req.Priority, req.Status, projectID, json.RawMessage(newAssigneesJSON)})
	if err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not prepare task audit history"})
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO task_activity(task_id,actor_id,action,old_values,new_values)
VALUES($1::uuid,$2::uuid,'task_details_updated',$3::jsonb,$4::jsonb)`, taskID, adminID, string(oldValues), string(newValues))
	if err != nil {
		log.Printf("record task edit activity for task %s failed: %v", taskID, err)
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not record task changes"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save task changes"})
		return
	}
	writeTaskJSON(w, http.StatusOK, map[string]string{"message": "Task updated successfully"})
}

// postgresTextArray safely encodes UUID strings for PostgreSQL's text[] comparison.
func postgresTextArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, `"`+strings.ReplaceAll(value, `"`, `\"`)+`"`)
	}
	return "{" + strings.Join(quoted, ",") + "}"
}

func deleteAdminTask(db *sql.DB, w http.ResponseWriter, r *http.Request, taskID, adminID string) {
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not start task deletion"})
		return
	}
	defer tx.Rollback()
	var exists, hasChildren bool
	err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM tasks WHERE id=$1::uuid AND deleted_at IS NULL)`, taskID).Scan(&exists)
	if err != nil || !exists {
		writeTaskJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM tasks WHERE parent_task_id=$1::uuid)`, taskID).Scan(&hasChildren); err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not check subtasks"})
		return
	}
	if hasChildren {
		writeTaskJSON(w, http.StatusConflict, map[string]string{"error": "this task has subtasks; move or delete its subtasks before deleting the parent task"})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO task_activity(task_id,actor_id,action) VALUES($1::uuid,$2::uuid,'task_deleted')`, taskID, adminID); err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not record task deletion"})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE tasks SET deleted_at=NOW(),updated_at=NOW() WHERE id=$1::uuid AND deleted_at IS NULL`, taskID); err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not delete task"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeTaskJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not finish task deletion"})
		return
	}
	writeTaskJSON(w, http.StatusOK, map[string]string{"message": "Task deleted successfully"})
}
