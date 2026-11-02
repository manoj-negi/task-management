package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

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
  AND EXISTS (SELECT 1 FROM tasks t WHERE t.id = $1::uuid AND t.deleted_at IS NULL AND t.status IN ('todo', 'changes_requested', 'in_progress'))`,
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
UPDATE tasks SET status = 'in_progress', start_at = COALESCE(start_at, NOW()), updated_at = NOW()
WHERE id = $1::uuid AND deleted_at IS NULL AND status IN ('todo', 'changes_requested', 'in_progress')`, taskID)
	if err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update task status"})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO task_activity (task_id, actor_id, action) VALUES ($1::uuid, $2::uuid, 'task_started')`, taskID, userID); err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not record task activity"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save task update"})
		return
	}
	writeTaskActionJSON(w, http.StatusOK, map[string]string{"message": "Task started"})
}

func submitAssignedTask(w http.ResponseWriter, r *http.Request, db *sql.DB, taskID, userID string) {
	var submission struct {
		Summary        string   `json:"summary"`
		ReferenceLinks []string `json:"referenceLinks"`
		AttachmentIDs  []string `json:"attachmentIds"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&submission); err != nil && !errors.Is(err, io.EOF) {
		writeTaskActionJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid submission details"})
		return
	}
	submission.Summary = strings.TrimSpace(submission.Summary)
	if submission.ReferenceLinks == nil {
		submission.ReferenceLinks = []string{}
	}
	if utf8.RuneCountInString(submission.Summary) < 1 || utf8.RuneCountInString(submission.Summary) > 10000 {
		writeTaskActionJSON(w, http.StatusBadRequest, map[string]string{"error": "work summary is required and must be under 10000 characters"})
		return
	}
	if len(submission.ReferenceLinks) > 10 {
		writeTaskActionJSON(w, http.StatusBadRequest, map[string]string{"error": "add no more than 10 reference links"})
		return
	}
	if len(submission.AttachmentIDs) > 20 {
		writeTaskActionJSON(w, http.StatusBadRequest, map[string]string{"error": "attach no more than 20 files to one submission"})
		return
	}
	for i, raw := range submission.ReferenceLinks {
		raw = strings.TrimSpace(raw)
		parsed, err := url.ParseRequestURI(raw)
		if len(raw) > 2048 || err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			writeTaskActionJSON(w, http.StatusBadRequest, map[string]string{"error": "reference links must be valid http or https URLs"})
			return
		}
		submission.ReferenceLinks[i] = raw
	}
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
  AND work_status = 'in_progress'
  AND EXISTS (SELECT 1 FROM tasks t WHERE t.id = $1::uuid AND t.deleted_at IS NULL AND t.status IN ('in_progress', 'changes_requested'))`,
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
	linksJSON, err := json.Marshal(submission.ReferenceLinks)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not read submission links"})
		return
	}
	var submissionID string
	if err := tx.QueryRowContext(r.Context(), `INSERT INTO task_submissions(task_id,submitted_by,summary,reference_links) VALUES($1::uuid,$2::uuid,$3,$4::jsonb) RETURNING id::text`, taskID, userID, submission.Summary, string(linksJSON)).Scan(&submissionID); err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save work summary"})
		return
	}
	seenAttachments := make(map[string]bool)
	for _, attachmentID := range submission.AttachmentIDs {
		attachmentID = strings.TrimSpace(attachmentID)
		if attachmentID == "" || seenAttachments[attachmentID] {
			continue
		}
		seenAttachments[attachmentID] = true
		var owned bool
		if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM task_attachments WHERE id=$1::uuid AND task_id=$2::uuid AND uploaded_by=$3::uuid)`, attachmentID, taskID, userID).Scan(&owned); err != nil || !owned {
			writeTaskActionJSON(w, http.StatusBadRequest, map[string]string{"error": "submission attachments must belong to this task and your account"})
			return
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO task_submission_attachments(submission_id,attachment_id) VALUES($1::uuid,$2::uuid)`, submissionID, attachmentID); err != nil {
			writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not attach files to submission"})
			return
		}
	}

	_, err = tx.ExecContext(r.Context(), `
UPDATE tasks t
SET status = 'in_review', submitted_at = NOW(), updated_at = NOW()
WHERE t.id = $1::uuid
  AND t.deleted_at IS NULL
  AND t.status NOT IN ('approved', 'cancelled')
  AND NOT EXISTS (
    SELECT 1 FROM task_assignees other
    WHERE other.task_id = t.id AND other.work_status <> 'ready_for_review'
  )`, taskID)
	if err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update review status"})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO task_activity (task_id, actor_id, action, new_values) VALUES ($1::uuid, $2::uuid, 'task_submitted', jsonb_build_object('summary',$3::text))`, taskID, userID, submission.Summary); err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not record task activity"})
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
