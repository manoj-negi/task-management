package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"office-task-manager/backend/internal/auth"
)

type teamActivity struct {
	ID          string    `json:"id"`
	ActorName   string    `json:"actorName"`
	Action      string    `json:"action"`
	TaskTitle   string    `json:"taskTitle"`
	ProjectName string    `json:"projectName"`
	TargetName  string    `json:"targetName,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

func TeamActivity(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			activityJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
			activityJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}

		rows, err := db.QueryContext(r.Context(), `
WITH events AS (
  SELECT 'created:' || t.id::text AS id, creator.full_name AS actor_name,
         'task_created'::text AS action, t.title AS task_title,
         COALESCE(p.name, 'Unassigned project') AS project_name, ''::text AS target_name,
         t.created_at AS created_at
  FROM tasks t
  JOIN users creator ON creator.id = t.created_by
  LEFT JOIN projects p ON p.id = t.project_id
  WHERE t.deleted_at IS NULL
  UNION ALL
  SELECT 'assigned:' || ta.task_id::text || ':' || ta.user_id::text, assigner.full_name,
         'task_assigned'::text, t.title, COALESCE(p.name, 'Unassigned project'), assignee.full_name,
         ta.assigned_at
  FROM task_assignees ta
  JOIN tasks t ON t.id = ta.task_id AND t.deleted_at IS NULL
  JOIN users assigner ON assigner.id = ta.assigned_by
  JOIN users assignee ON assignee.id = ta.user_id
  LEFT JOIN projects p ON p.id = t.project_id
  UNION ALL
  SELECT 'review:' || tr.id::text, reviewer.full_name, tr.decision::text, t.title,
         COALESCE(p.name, 'Unassigned project'), ''::text, tr.created_at
  FROM task_reviews tr
  JOIN users reviewer ON reviewer.id = tr.reviewer_id
  JOIN tasks t ON t.id = tr.task_id AND t.deleted_at IS NULL
  LEFT JOIN projects p ON p.id = t.project_id
  UNION ALL
  SELECT 'activity:' || a.id::text, COALESCE(actor.full_name, 'Team member'), a.action,
         t.title, COALESCE(p.name, 'Unassigned project'), ''::text, a.created_at
  FROM task_activity a
  JOIN tasks t ON t.id = a.task_id AND t.deleted_at IS NULL
  LEFT JOIN users actor ON actor.id = a.actor_id
  LEFT JOIN projects p ON p.id = t.project_id
)
SELECT id, actor_name, action, task_title, project_name, target_name, created_at
FROM events
ORDER BY created_at DESC
LIMIT 50`)
		if err != nil {
			activityJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load team activity"})
			return
		}
		defer rows.Close()

		items := make([]teamActivity, 0)
		for rows.Next() {
			var item teamActivity
			if err := rows.Scan(&item.ID, &item.ActorName, &item.Action, &item.TaskTitle, &item.ProjectName, &item.TargetName, &item.CreatedAt); err != nil {
				activityJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not read team activity"})
				return
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			activityJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not read team activity"})
			return
		}
		activityJSON(w, http.StatusOK, items)
	}
}

func activityJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
