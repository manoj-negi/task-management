package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"office-task-manager/backend/internal/auth"
)

func TaskAction(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeTaskActionJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			writeTaskActionJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "tasks" {
			writeTaskActionJSON(w, http.StatusNotFound, map[string]string{"error": "action not found"})
			return
		}
		taskID := parts[2]
		if taskID == "" {
			writeTaskActionJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		switch parts[3] {
		case "start":
			startAssignedTask(w, r, db, taskID, claims.UserID)
		case "submit":
			submitAssignedTask(w, r, db, taskID, claims.UserID)
		default:
			writeTaskActionJSON(w, http.StatusNotFound, map[string]string{"error": "action not found"})
		}
	}
}

func startAssignedTask(w http.ResponseWriter, r *http.Request, db *sql.DB, taskID, userID string) {
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not start task"})
		return
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(r.Context(), `
UPDATE task_assignees
SET work_status = 'in_progress'
WHERE task_id = $1::uuid
  AND user_id = $2::uuid
  AND work_status IN ('assigned', 'in_progress')
  AND EXISTS (SELECT 1 FROM tasks t WHERE t.id = $1::uuid AND t.status NOT IN ('approved', 'cancelled'))`,
		taskID, userID)
	if err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not start task"})
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeTaskActionJSON(w, http.StatusConflict, map[string]string{"error": "task is not assigned to you or cannot be started"})
		return
	}
	_, err = tx.ExecContext(r.Context(), `
UPDATE tasks SET status = 'in_progress', updated_at = NOW()
WHERE id = $1::uuid AND status IN ('todo', 'changes_requested', 'in_progress')`, taskID)
	if err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update task status"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save task update"})
		return
	}
	writeTaskActionJSON(w, http.StatusOK, map[string]string{"message": "Task started"})
}

func submitAssignedTask(w http.ResponseWriter, r *http.Request, db *sql.DB, taskID, userID string) {
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not submit task"})
		return
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(r.Context(), `
UPDATE task_assignees
SET work_status = 'ready_for_review'
WHERE task_id = $1::uuid
  AND user_id = $2::uuid
  AND work_status IN ('assigned', 'in_progress', 'ready_for_review')
  AND EXISTS (SELECT 1 FROM tasks t WHERE t.id = $1::uuid AND t.status NOT IN ('approved', 'cancelled'))`,
		taskID, userID)
	if err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not submit task"})
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeTaskActionJSON(w, http.StatusConflict, map[string]string{"error": "task is not assigned to you or cannot be submitted"})
		return
	}

	_, err = tx.ExecContext(r.Context(), `
UPDATE tasks t
SET status = 'in_review', submitted_at = NOW(), updated_at = NOW()
WHERE t.id = $1::uuid
  AND t.status NOT IN ('approved', 'cancelled')
  AND NOT EXISTS (
    SELECT 1 FROM task_assignees other
    WHERE other.task_id = t.id AND other.work_status <> 'ready_for_review'
  )`, taskID)
	if err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update review status"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save submission"})
		return
	}
	writeTaskActionJSON(w, http.StatusOK, map[string]string{"message": "Your work was submitted. The task will enter review when all assignees have submitted."})
}

func writeTaskActionJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
