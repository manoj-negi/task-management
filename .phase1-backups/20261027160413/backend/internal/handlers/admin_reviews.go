package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"office-task-manager/backend/internal/auth"
	"strings"
	"time"
)

type reviewItem struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Project     string   `json:"project"`
	Priority    string   `json:"priority"`
	SubmittedAt string   `json:"submittedAt"`
	Assignees   []string `json:"assignees"`
}

func AdminReviews(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			reviewJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			reviewJSON(w, 401, map[string]string{"error": "authentication required"})
			return
		}
		if claims.Role != "super_admin" {
			reviewJSON(w, 403, map[string]string{"error": "admin access required"})
			return
		}
		rows, err := db.QueryContext(r.Context(), `
SELECT t.id::text, t.title, COALESCE(p.name, 'Unassigned project'), t.priority::text,
       COALESCE(to_char(t.submitted_at, 'YYYY-MM-DD HH24:MI'), ''),
       COALESCE((SELECT json_agg(x.full_name)::text FROM (
           SELECT DISTINCT u.full_name FROM task_assignees ta JOIN users u ON u.id = ta.user_id
           WHERE ta.task_id = t.id ORDER BY u.full_name
       ) x), '[]')
FROM tasks t LEFT JOIN projects p ON p.id = t.project_id
WHERE t.status = 'in_review'
ORDER BY t.submitted_at ASC NULLS FIRST, t.created_at ASC LIMIT 200`)
		if err != nil {
			reviewJSON(w, 500, map[string]string{"error": "could not load pending reviews"})
			return
		}
		defer rows.Close()
		items := make([]reviewItem, 0)
		for rows.Next() {
			var item reviewItem
			var namesJSON string
			if err := rows.Scan(&item.ID, &item.Title, &item.Project, &item.Priority, &item.SubmittedAt, &namesJSON); err != nil {
				reviewJSON(w, 500, map[string]string{"error": "could not read pending reviews"})
				return
			}
			if item.SubmittedAt != "" {
				if parsed, err := time.Parse("2006-01-02 15:04", item.SubmittedAt); err == nil {
					item.SubmittedAt = parsed.Format("Jan 2, 2006 3:04 PM")
				}
			}
			item.Assignees = []string{}
			_ = json.Unmarshal([]byte(namesJSON), &item.Assignees)
			items = append(items, item)
		}
		if rows.Err() != nil {
			reviewJSON(w, 500, map[string]string{"error": "could not read pending reviews"})
			return
		}
		reviewJSON(w, 200, map[string]any{"reviews": items})
	}
}

func ReviewTask(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			reviewJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			reviewJSON(w, 401, map[string]string{"error": "authentication required"})
			return
		}
		if claims.Role != "super_admin" {
			reviewJSON(w, 403, map[string]string{"error": "admin access required"})
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "admin" || parts[2] != "tasks" || parts[3] == "" {
			reviewJSON(w, 404, map[string]string{"error": "review route not found"})
			return
		}
		var body struct {
			Decision string `json:"decision"`
			Feedback string `json:"feedback"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&body); err != nil {
			reviewJSON(w, 400, map[string]string{"error": "invalid JSON body"})
			return
		}
		body.Decision = strings.TrimSpace(body.Decision)
		body.Feedback = strings.TrimSpace(body.Feedback)
		if body.Decision != "approved" && body.Decision != "changes_requested" {
			reviewJSON(w, 400, map[string]string{"error": "decision must be approved or changes_requested"})
			return
		}
		if body.Decision == "changes_requested" && body.Feedback == "" {
			reviewJSON(w, 400, map[string]string{"error": "feedback is required when requesting changes"})
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			reviewJSON(w, 500, map[string]string{"error": "could not start review"})
			return
		}
		defer tx.Rollback()
		var currentStatus string
		err = tx.QueryRowContext(r.Context(), `SELECT status::text FROM tasks WHERE id = $1::uuid FOR UPDATE`, parts[3]).Scan(&currentStatus)
		if err == sql.ErrNoRows {
			reviewJSON(w, 404, map[string]string{"error": "task not found"})
			return
		}
		if err != nil {
			reviewJSON(w, 500, map[string]string{"error": "could not load task"})
			return
		}
		if currentStatus != "in_review" {
			reviewJSON(w, 409, map[string]string{"error": "task is not awaiting review"})
			return
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO task_reviews (task_id, reviewer_id, decision, feedback) VALUES ($1::uuid, $2::uuid, $3::task_review_decision, NULLIF($4, ''))`, parts[3], claims.UserID, body.Decision, body.Feedback)
		if err != nil {
			reviewJSON(w, 500, map[string]string{"error": "could not save review decision"})
			return
		}
		if body.Decision == "changes_requested" {
			_, err = tx.ExecContext(r.Context(), `UPDATE task_assignees SET work_status = 'assigned' WHERE task_id = $1::uuid`, parts[3])
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE tasks SET status = $2::task_status, approved_at = CASE WHEN $2 = 'approved' THEN NOW() ELSE NULL END, submitted_at = CASE WHEN $2 = 'changes_requested' THEN NULL ELSE submitted_at END, updated_at = NOW() WHERE id = $1::uuid`, parts[3], body.Decision)
		}
		if err != nil {
			reviewJSON(w, 500, map[string]string{"error": "could not update task status"})
			return
		}
		if err := tx.Commit(); err != nil {
			reviewJSON(w, 500, map[string]string{"error": "could not save review"})
			return
		}
		message := "Task approved"
		if body.Decision == "changes_requested" {
			message = "Changes requested; task returned to assignees"
		}
		reviewJSON(w, 200, map[string]string{"message": message, "status": body.Decision})
	}
}

func reviewJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
