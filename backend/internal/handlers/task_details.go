package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"office-task-manager/backend/internal/auth"
)

const taskAttachmentLimit = 10 << 20

type taskDetailsRouter struct{ db *sql.DB }

func TaskDetailsRouter(db *sql.DB) http.Handler {
	return taskDetailsRouter{db: db}
}

func (h taskDetailsRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeTaskActionJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "tasks" || parts[2] == "" {
		writeTaskActionJSON(w, http.StatusNotFound, map[string]string{"error": "task endpoint not found"})
		return
	}
	taskID := parts[2]
	viewAll := claims.HasPermission("view_all_tasks") || claims.HasPermission("manage_tasks") || claims.HasPermission("review_tasks")
	switch {
	case len(parts) == 4 && parts[3] == "details" && r.Method == http.MethodGet:
		h.details(w, r, taskID, claims.UserID, viewAll)
	case len(parts) == 4 && parts[3] == "comments":
		if r.Method == http.MethodGet {
			h.comments(w, r, taskID, claims.UserID, viewAll)
		} else if r.Method == http.MethodPost {
			h.addComment(w, r, taskID, claims.UserID, viewAll)
		} else {
			methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
		}
	case len(parts) == 4 && parts[3] == "activity" && r.Method == http.MethodGet:
		h.activity(w, r, taskID, claims.UserID, viewAll)
	case len(parts) == 4 && parts[3] == "status" && r.Method == http.MethodPost:
		h.updateStatus(w, r, taskID, claims.UserID)
	case len(parts) == 4 && parts[3] == "attachments":
		if r.Method == http.MethodGet {
			h.listAttachments(w, r, taskID, claims.UserID, viewAll)
		} else if r.Method == http.MethodPost {
			h.uploadAttachment(w, r, taskID, claims.UserID, viewAll)
		} else {
			methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
		}
	case len(parts) == 5 && parts[3] == "attachments" && r.Method == http.MethodGet:
		h.downloadAttachment(w, r, taskID, parts[4], claims.UserID, viewAll)
	case len(parts) == 6 && parts[3] == "subtasks" && parts[5] == "status" && r.Method == http.MethodPost:
		h.updateSubtaskStatus(w, r, taskID, parts[4], claims.UserID)
	case len(parts) == 4 && (parts[3] == "start" || parts[3] == "submit") && r.Method == http.MethodPost:
		TaskAction(h.db).ServeHTTP(w, r)
	default:
		writeTaskActionJSON(w, http.StatusNotFound, map[string]string{"error": "task endpoint not found"})
	}
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeTaskActionJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func (h taskDetailsRouter) canAccess(r *http.Request, taskID, userID string, viewAll bool) bool {
	var allowed bool
	err := h.db.QueryRowContext(r.Context(), `SELECT EXISTS (
SELECT 1 FROM tasks t WHERE t.id=$1::uuid AND t.deleted_at IS NULL AND ($3::boolean OR EXISTS (
  SELECT 1 FROM task_assignees ta WHERE ta.task_id=t.id AND ta.user_id=$2::uuid
)))`, taskID, userID, viewAll).Scan(&allowed)
	return err == nil && allowed
}

type taskAssigneeDetail struct {
	ID         string `json:"id"`
	FullName   string `json:"fullName"`
	Email      string `json:"email"`
	WorkStatus string `json:"workStatus"`
	AssignedBy string `json:"assignedBy"`
	AssignedAt string `json:"assignedAt"`
}

type subtaskDetail struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	WorkStatus string `json:"workStatus"`
	DueDate    string `json:"dueDate"`
	CanUpdate  bool   `json:"canUpdate"`
}

type attachmentDetail struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MediaType   string `json:"mediaType"`
	Size        int64  `json:"size"`
	Uploader    string `json:"uploader"`
	UploadedAt  string `json:"uploadedAt"`
	DownloadURL string `json:"downloadUrl"`
}

func (h taskDetailsRouter) details(w http.ResponseWriter, r *http.Request, taskID, userID string, viewAll bool) {
	if !h.canAccess(r, taskID, userID, viewAll) {
		writeTaskActionJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	var detail struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		Project      string `json:"project"`
		ParentTask   string `json:"parentTask"`
		Status       string `json:"status"`
		Priority     string `json:"priority"`
		StartDate    string `json:"startDate"`
		DueDate      string `json:"dueDate"`
		CreatedBy    string `json:"createdBy"`
		AssignedBy   string `json:"assignedBy"`
		AssignedAt   string `json:"assignedAt"`
		CreatedAt    string `json:"createdAt"`
		MyWork       string `json:"myWorkStatus"`
		Feedback     string `json:"reviewFeedback"`
		Decision     string `json:"reviewDecision"`
		Progress     int    `json:"progress"`
		SubtaskCount int    `json:"subtaskCount"`
		SubtasksDone int    `json:"subtasksDone"`
	}
	err := h.db.QueryRowContext(r.Context(), `
SELECT t.id::text,t.title,COALESCE(t.description,''),p.name,COALESCE(parent.title,''),t.status::text,t.priority::text,
       COALESCE(to_char(t.start_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
       COALESCE(to_char(t.due_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
       creator.full_name,COALESCE(assigner.full_name,''),
       COALESCE(to_char(mine.assigned_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
       to_char(t.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       COALESCE(mine.work_status::text,''),
       COALESCE((SELECT tr.feedback FROM task_reviews tr WHERE tr.task_id=t.id ORDER BY tr.created_at DESC LIMIT 1),''),
       COALESCE((SELECT tr.decision::text FROM task_reviews tr WHERE tr.task_id=t.id ORDER BY tr.created_at DESC LIMIT 1),''),
       count(child.id) FILTER (WHERE child.status='approved')::int,
       count(child.id)::int
FROM tasks t JOIN projects p ON p.id=t.project_id JOIN users creator ON creator.id=t.created_by LEFT JOIN tasks parent ON parent.id=t.parent_task_id
LEFT JOIN task_assignees mine ON mine.task_id=t.id AND mine.user_id=$2::uuid
LEFT JOIN users assigner ON assigner.id=mine.assigned_by
LEFT JOIN tasks child ON child.parent_task_id=t.id AND child.deleted_at IS NULL

WHERE t.id=$1::uuid AND t.deleted_at IS NULL GROUP BY t.id,p.name,parent.title,creator.full_name,assigner.full_name,mine.assigned_at,mine.work_status`, taskID, userID).Scan(
		&detail.ID, &detail.Title, &detail.Description, &detail.Project, &detail.ParentTask, &detail.Status, &detail.Priority,
		&detail.StartDate, &detail.DueDate, &detail.CreatedBy, &detail.AssignedBy, &detail.AssignedAt, &detail.CreatedAt, &detail.MyWork,
		&detail.Feedback, &detail.Decision, &detail.SubtasksDone, &detail.SubtaskCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		writeTaskActionJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	if err != nil {
		writeTaskActionJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load task details"})
		return
	}
	if detail.SubtaskCount > 0 {
		detail.Progress = detail.SubtasksDone * 100 / detail.SubtaskCount
	} else if detail.Status == "approved" {
		detail.Progress = 100
	} else if detail.Status == "in_review" {
		detail.Progress = 90
	} else if detail.Status == "in_progress" || detail.Status == "blocked" || detail.Status == "changes_requested" {
		detail.Progress = 50
	}
	var assignees []taskAssigneeDetail
	rows, err := h.db.QueryContext(r.Context(), `SELECT u.id::text,u.full_name,u.email,ta.work_status::text,assigner.full_name,
to_char(ta.assigned_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM task_assignees ta JOIN users u ON u.id=ta.user_id JOIN users assigner ON assigner.id=ta.assigned_by
WHERE ta.task_id=$1::uuid ORDER BY ta.assigned_at`, taskID)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load task assignees"})
		return
	}
	assignees = make([]taskAssigneeDetail, 0)
	for rows.Next() {
		var a taskAssigneeDetail
		if err := rows.Scan(&a.ID, &a.FullName, &a.Email, &a.WorkStatus, &a.AssignedBy, &a.AssignedAt); err != nil {
			rows.Close()
			writeTaskActionJSON(w, 500, map[string]string{"error": "could not read task assignees"})
			return
		}
		assignees = append(assignees, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not read task assignees"})
		return
	}
	if err := rows.Close(); err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not read task assignees"})
		return
	}

	subtasks := make([]subtaskDetail, 0)
	rows, err = h.db.QueryContext(r.Context(), `SELECT t.id::text,t.title,t.status::text,COALESCE(ta.work_status::text,''),COALESCE(to_char(t.due_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),ta.user_id IS NOT NULL
FROM tasks t LEFT JOIN task_assignees ta ON ta.task_id=t.id AND ta.user_id=$2::uuid
WHERE t.parent_task_id=$1::uuid AND t.deleted_at IS NULL ORDER BY t.created_at`, taskID, userID)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load subtasks"})
		return
	}
	for rows.Next() {
		var s subtaskDetail
		if err := rows.Scan(&s.ID, &s.Title, &s.Status, &s.WorkStatus, &s.DueDate, &s.CanUpdate); err != nil {
			rows.Close()
			writeTaskActionJSON(w, 500, map[string]string{"error": "could not read subtasks"})
			return
		}
		subtasks = append(subtasks, s)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not read subtasks"})
		return
	}
	rows.Close()

	attachments := h.getAttachmentDetails(r, taskID)
	if attachments == nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load attachments"})
		return
	}
	var lastSummary string
	var lastLinksJSON string
	var lastAttachmentIDsJSON string
	var submittedAt string
	err = h.db.QueryRowContext(r.Context(), `SELECT s.summary,s.reference_links::text,to_char(s.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM task_submissions s WHERE s.task_id=$1::uuid ORDER BY s.created_at DESC LIMIT 1`, taskID).Scan(&lastSummary, &lastLinksJSON, &submittedAt)
	if errors.Is(err, sql.ErrNoRows) {
		lastSummary = ""
		lastLinksJSON = "[]"
		lastAttachmentIDsJSON = "[]"
		submittedAt = ""
	} else if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load latest submission"})
		return
	}
	if submittedAt != "" {
		err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(json_agg(attachment_id::text)::text,'[]') FROM task_submission_attachments WHERE submission_id=(SELECT id FROM task_submissions WHERE task_id=$1::uuid ORDER BY created_at DESC LIMIT 1)`, taskID).Scan(&lastAttachmentIDsJSON)
		if err != nil {
			writeTaskActionJSON(w, 500, map[string]string{"error": "could not load submission attachments"})
			return
		}
	}
	var links []string
	_ = json.Unmarshal([]byte(lastLinksJSON), &links)
	var submissionAttachmentIDs []string
	_ = json.Unmarshal([]byte(lastAttachmentIDsJSON), &submissionAttachmentIDs)
	submissions := make([]map[string]any, 0)
	rows, err = h.db.QueryContext(r.Context(), `SELECT s.id::text,u.full_name,s.summary,s.reference_links::text,
COALESCE((SELECT json_agg(sa.attachment_id::text)::text FROM task_submission_attachments sa WHERE sa.submission_id=s.id),'[]'),
to_char(s.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM task_submissions s JOIN users u ON u.id=s.submitted_by WHERE s.task_id=$1::uuid ORDER BY s.created_at DESC`, taskID)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load submission history"})
		return
	}
	for rows.Next() {
		var id, submitter, summary, referenceJSON, attachmentJSON, at string
		if err := rows.Scan(&id, &submitter, &summary, &referenceJSON, &attachmentJSON, &at); err != nil {
			rows.Close()
			writeTaskActionJSON(w, 500, map[string]string{"error": "could not read submission history"})
			return
		}
		var refs, attachmentIDs []string
		_ = json.Unmarshal([]byte(referenceJSON), &refs)
		_ = json.Unmarshal([]byte(attachmentJSON), &attachmentIDs)
		submissions = append(submissions, map[string]any{"id": id, "submittedBy": submitter, "summary": summary, "referenceLinks": refs, "attachmentIds": attachmentIDs, "submittedAt": at})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not read submission history"})
		return
	}
	rows.Close()
	reviews := make([]map[string]string, 0)
	rows, err = h.db.QueryContext(r.Context(), `SELECT r.decision::text,COALESCE(r.feedback,''),u.full_name,
to_char(r.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM task_reviews r JOIN users u ON u.id=r.reviewer_id WHERE r.task_id=$1::uuid ORDER BY r.created_at DESC`, taskID)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load review history"})
		return
	}
	for rows.Next() {
		var decision, feedback, reviewer, at string
		if err := rows.Scan(&decision, &feedback, &reviewer, &at); err != nil {
			rows.Close()
			writeTaskActionJSON(w, 500, map[string]string{"error": "could not read review history"})
			return
		}
		reviews = append(reviews, map[string]string{"decision": decision, "feedback": feedback, "reviewer": reviewer, "createdAt": at})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not read review history"})
		return
	}
	rows.Close()
	writeTaskActionJSON(w, http.StatusOK, map[string]any{"task": detail, "assignees": assignees, "subtasks": subtasks, "attachments": attachments, "latestSubmission": map[string]any{"summary": lastSummary, "referenceLinks": links, "attachmentIds": submissionAttachmentIDs, "submittedAt": submittedAt}, "submissions": submissions, "reviews": reviews})
}

func pagination(r *http.Request) (int, int) {
	limit := 25
	if parsed, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && parsed > 0 {
		limit = parsed
	}
	if limit > 100 {
		limit = 100
	}
	offset := 0
	if parsed, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && parsed > 0 {
		offset = parsed
	}
	return limit, offset
}

type taskComment struct {
	ID        string `json:"id"`
	ParentID  string `json:"parentId,omitempty"`
	Author    string `json:"author"`
	Role      string `json:"role"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"createdAt"`
}

func (h taskDetailsRouter) comments(w http.ResponseWriter, r *http.Request, taskID, userID string, viewAll bool) {
	if !h.canAccess(r, taskID, userID, viewAll) {
		writeTaskActionJSON(w, 404, map[string]string{"error": "task not found"})
		return
	}
	limit, offset := pagination(r)
	rows, err := h.db.QueryContext(r.Context(), `SELECT c.id::text,COALESCE(c.parent_comment_id::text,''),u.full_name,r.name,c.comment,to_char(c.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM task_comments c JOIN users u ON u.id=c.user_id JOIN roles r ON r.id=u.role_id WHERE c.task_id=$1::uuid ORDER BY c.created_at ASC,c.id ASC LIMIT $2 OFFSET $3`, taskID, limit, offset)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load comments"})
		return
	}
	defer rows.Close()
	items := make([]taskComment, 0)
	for rows.Next() {
		var c taskComment
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Author, &c.Role, &c.Comment, &c.CreatedAt); err != nil {
			writeTaskActionJSON(w, 500, map[string]string{"error": "could not read comments"})
			return
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not read comments"})
		return
	}
	writeTaskActionJSON(w, 200, map[string]any{"comments": items, "limit": limit, "offset": offset})
}
func (h taskDetailsRouter) addComment(w http.ResponseWriter, r *http.Request, taskID, userID string, viewAll bool) {
	if !h.canAccess(r, taskID, userID, viewAll) {
		writeTaskActionJSON(w, 404, map[string]string{"error": "task not found"})
		return
	}
	var body struct {
		Comment  string `json:"comment"`
		ParentID string `json:"parentCommentId"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 20<<10)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&body); err != nil {
		writeTaskActionJSON(w, 400, map[string]string{"error": "invalid comment request"})
		return
	}
	body.Comment = strings.TrimSpace(body.Comment)
	if body.Comment == "" || len([]rune(body.Comment)) > 5000 {
		writeTaskActionJSON(w, 400, map[string]string{"error": "comment must contain 1 to 5000 characters"})
		return
	}
	var parent any
	if body.ParentID != "" {
		var exists bool
		err := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM task_comments WHERE id=$1::uuid AND task_id=$2::uuid)`, body.ParentID, taskID).Scan(&exists)
		if err != nil || !exists {
			writeTaskActionJSON(w, 400, map[string]string{"error": "reply target was not found on this task"})
			return
		}
		parent = body.ParentID
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not start comment"})
		return
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(r.Context(), `INSERT INTO task_comments(task_id,user_id,comment,parent_comment_id) VALUES($1::uuid,$2::uuid,$3,$4::uuid) RETURNING id::text`, taskID, userID, body.Comment, parent).Scan(&id)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not save comment"})
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO task_activity(task_id,actor_id,action,new_values) VALUES($1::uuid,$2::uuid,'comment_added',jsonb_build_object('comment_id',$3::text))`, taskID, userID, id); err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not record comment activity"})
		return
	}
	if err = tx.Commit(); err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not save comment"})
		return
	}
	writeTaskActionJSON(w, 201, map[string]string{"id": id, "message": "Comment added"})
}

type taskTimelineEvent struct {
	ID        string `json:"id"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	Detail    string `json:"detail,omitempty"`
	CreatedAt string `json:"createdAt"`
}

func (h taskDetailsRouter) activity(w http.ResponseWriter, r *http.Request, taskID, userID string, viewAll bool) {
	if !h.canAccess(r, taskID, userID, viewAll) {
		writeTaskActionJSON(w, 404, map[string]string{"error": "task not found"})
		return
	}
	limit, offset := pagination(r)
	rows, err := h.db.QueryContext(r.Context(), `WITH events AS (
SELECT 'created:'||t.id::text id,u.full_name actor,'task_created'::text action,''::text detail,t.created_at FROM tasks t JOIN users u ON u.id=t.created_by WHERE t.id=$1::uuid AND t.deleted_at IS NULL
UNION ALL SELECT 'activity:'||a.id::text,COALESCE(u.full_name,'Team member'),a.action,COALESCE(a.new_values->>'summary',a.new_values->>'status',a.new_values->>'title',''),a.created_at FROM task_activity a LEFT JOIN users u ON u.id=a.actor_id WHERE a.task_id=$1::uuid AND a.action <> 'comment_added'
UNION ALL SELECT 'subtask_activity:'||a.id::text,COALESCE(u.full_name,'Team member'),a.action,COALESCE(a.new_values->>'status',''),a.created_at FROM task_activity a JOIN tasks child ON child.id=a.task_id AND child.parent_task_id=$1::uuid AND child.deleted_at IS NULL LEFT JOIN users u ON u.id=a.actor_id
UNION ALL SELECT 'comment:'||c.id::text,u.full_name,'comment_added',c.comment,c.created_at FROM task_comments c JOIN users u ON u.id=c.user_id WHERE c.task_id=$1::uuid
UNION ALL SELECT 'review:'||r.id::text,u.full_name,r.decision::text,COALESCE(r.feedback,''),r.created_at FROM task_reviews r JOIN users u ON u.id=r.reviewer_id WHERE r.task_id=$1::uuid
UNION ALL SELECT 'assignment:'||ta.task_id::text||':'||ta.user_id::text,u.full_name,'task_assigned',assignee.full_name,ta.assigned_at FROM task_assignees ta JOIN users u ON u.id=ta.assigned_by JOIN users assignee ON assignee.id=ta.user_id WHERE ta.task_id=$1::uuid
) SELECT id,actor,action,detail,to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM events ORDER BY created_at ASC,id ASC LIMIT $2 OFFSET $3`, taskID, limit, offset)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load task activity"})
		return
	}
	defer rows.Close()
	items := make([]taskTimelineEvent, 0)
	for rows.Next() {
		var e taskTimelineEvent
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Detail, &e.CreatedAt); err != nil {
			writeTaskActionJSON(w, 500, map[string]string{"error": "could not read task activity"})
			return
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not read task activity"})
		return
	}
	writeTaskActionJSON(w, 200, map[string]any{"activity": items, "limit": limit, "offset": offset})
}

func (h taskDetailsRouter) updateStatus(w http.ResponseWriter, r *http.Request, taskID, userID string) {
	var body struct {
		Status string `json:"status"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeTaskActionJSON(w, 400, map[string]string{"error": "invalid status request"})
		return
	}
	body.Status = strings.ToLower(strings.TrimSpace(body.Status))
	if body.Status != "blocked" && body.Status != "in_progress" {
		writeTaskActionJSON(w, 400, map[string]string{"error": "employees may only block a task or resume it"})
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not update task"})
		return
	}
	defer tx.Rollback()
	var old string
	err = tx.QueryRowContext(r.Context(), `SELECT t.status::text FROM tasks t JOIN task_assignees ta ON ta.task_id=t.id AND ta.user_id=$2::uuid WHERE t.id=$1::uuid AND t.deleted_at IS NULL FOR UPDATE OF t`, taskID, userID).Scan(&old)
	if errors.Is(err, sql.ErrNoRows) {
		writeTaskActionJSON(w, 404, map[string]string{"error": "task not found"})
		return
	}
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load task status"})
		return
	}
	var workStatus string
	if err := tx.QueryRowContext(r.Context(), `SELECT work_status::text FROM task_assignees WHERE task_id=$1::uuid AND user_id=$2::uuid`, taskID, userID).Scan(&workStatus); err != nil {
		writeTaskActionJSON(w, 404, map[string]string{"error": "task not found"})
		return
	}
	valid := body.Status == "blocked" && (old == "in_progress" || old == "changes_requested") && workStatus == "in_progress" || body.Status == "in_progress" && old == "blocked"
	if !valid {
		writeTaskActionJSON(w, 409, map[string]string{"error": "that status transition is not allowed"})
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE tasks SET status=$2::task_status,updated_at=NOW(),start_at=CASE WHEN $2='in_progress' THEN COALESCE(start_at,NOW()) ELSE start_at END WHERE id=$1::uuid`, taskID, body.Status)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not update task status"})
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO task_activity(task_id,actor_id,action,old_values,new_values) VALUES($1::uuid,$2::uuid,$3,jsonb_build_object('status',$4::text),jsonb_build_object('status',$5::text))`, taskID, userID, "task_status_changed", old, body.Status); err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not record status activity"})
		return
	}
	if err = tx.Commit(); err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not save task status"})
		return
	}
	writeTaskActionJSON(w, 200, map[string]string{"message": "Task status updated", "status": body.Status})
}

func (h taskDetailsRouter) updateSubtaskStatus(w http.ResponseWriter, r *http.Request, parentID, subtaskID, userID string) {
	var body struct {
		Status  string `json:"status"`
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		writeTaskActionJSON(w, 400, map[string]string{"error": "invalid subtask status request"})
		return
	}
	body.Status = strings.ToLower(strings.TrimSpace(body.Status))
	if body.Status != "todo" && body.Status != "in_progress" && body.Status != "blocked" && body.Status != "in_review" {
		writeTaskActionJSON(w, 400, map[string]string{"error": "subtask status must be todo, in_progress, blocked, or in_review"})
		return
	}
	body.Summary = strings.TrimSpace(body.Summary)
	if body.Status == "in_review" && body.Summary == "" {
		writeTaskActionJSON(w, 400, map[string]string{"error": "a work summary is required to submit this subtask"})
		return
	}
	if utf8.RuneCountInString(body.Summary) > 10000 {
		writeTaskActionJSON(w, 400, map[string]string{"error": "subtask summary must be no longer than 10000 characters"})
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not update subtask"})
		return
	}
	defer tx.Rollback()
	var old, oldWorkStatus string
	err = tx.QueryRowContext(r.Context(), `SELECT child.status::text,ca.work_status::text FROM tasks parent JOIN task_assignees pa ON pa.task_id=parent.id AND pa.user_id=$3::uuid JOIN tasks child ON child.parent_task_id=parent.id AND child.id=$2::uuid JOIN task_assignees ca ON ca.task_id=child.id AND ca.user_id=$3::uuid WHERE parent.id=$1::uuid AND parent.deleted_at IS NULL AND child.deleted_at IS NULL FOR UPDATE OF child`, parentID, subtaskID, userID).Scan(&old, &oldWorkStatus)
	if errors.Is(err, sql.ErrNoRows) {
		writeTaskActionJSON(w, 404, map[string]string{"error": "subtask not found"})
		return
	}
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load subtask"})
		return
	}
	if old == "approved" || old == "cancelled" || old == "in_review" {
		writeTaskActionJSON(w, 409, map[string]string{"error": "subtask is not available for progress updates"})
		return
	}
	valid := body.Status == "todo" && old == "todo" || body.Status == "in_progress" && (old == "todo" || old == "blocked" || old == "in_progress") || body.Status == "blocked" && old == "in_progress" || body.Status == "in_review" && old == "in_progress" && oldWorkStatus == "in_progress"
	if !valid {
		writeTaskActionJSON(w, 409, map[string]string{"error": "that subtask status transition is not allowed"})
		return
	}
	if body.Status == "in_review" {
		if _, err = tx.ExecContext(r.Context(), `UPDATE task_assignees SET work_status='ready_for_review' WHERE task_id=$1::uuid AND user_id=$2::uuid`, subtaskID, userID); err != nil {
			writeTaskActionJSON(w, 500, map[string]string{"error": "could not submit subtask"})
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO task_submissions(task_id,submitted_by,summary,reference_links) VALUES($1::uuid,$2::uuid,$3,'[]'::jsonb)`, subtaskID, userID, body.Summary); err != nil {
			writeTaskActionJSON(w, 500, map[string]string{"error": "could not save subtask summary"})
			return
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE tasks t SET status='in_review',submitted_at=NOW(),updated_at=NOW() WHERE t.id=$1::uuid AND NOT EXISTS(SELECT 1 FROM task_assignees other WHERE other.task_id=t.id AND other.work_status<>'ready_for_review')`, subtaskID)
	} else {
		if body.Status == "in_progress" {
			if _, err = tx.ExecContext(r.Context(), `UPDATE task_assignees SET work_status='in_progress' WHERE task_id=$1::uuid AND user_id=$2::uuid`, subtaskID, userID); err != nil {
				writeTaskActionJSON(w, 500, map[string]string{"error": "could not update subtask progress"})
				return
			}
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE tasks SET status=$2::task_status,updated_at=NOW(),start_at=CASE WHEN $2='in_progress' THEN COALESCE(start_at,NOW()) ELSE start_at END WHERE id=$1::uuid`, subtaskID, body.Status)
	}
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not update subtask"})
		return
	}
	action := "subtask_status_changed"
	if body.Status == "in_review" {
		action = "task_submitted"
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO task_activity(task_id,actor_id,action,old_values,new_values) VALUES($1::uuid,$2::uuid,$3,jsonb_build_object('status',$4::text),jsonb_build_object('status',$5::text,'summary',$6::text))`, subtaskID, userID, action, old, body.Status, body.Summary); err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not record subtask activity"})
		return
	}
	if err = tx.Commit(); err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not save subtask"})
		return
	}
	writeTaskActionJSON(w, 200, map[string]string{"message": "Subtask status updated", "status": body.Status})
}

func (h taskDetailsRouter) getAttachmentDetails(r *http.Request, taskID string) []attachmentDetail {
	rows, err := h.db.QueryContext(r.Context(), `SELECT a.id::text,a.original_name,a.media_type,a.byte_size,u.full_name,to_char(a.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM task_attachments a JOIN users u ON u.id=a.uploaded_by WHERE a.task_id=$1::uuid ORDER BY a.created_at DESC LIMIT 100`, taskID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]attachmentDetail, 0)
	for rows.Next() {
		var a attachmentDetail
		if rows.Scan(&a.ID, &a.Name, &a.MediaType, &a.Size, &a.Uploader, &a.UploadedAt) != nil {
			return nil
		}
		a.DownloadURL = fmt.Sprintf("/api/tasks/%s/attachments/%s", url.PathEscape(taskID), url.PathEscape(a.ID))
		out = append(out, a)
	}
	if rows.Err() != nil {
		return nil
	}
	return out
}

func (h taskDetailsRouter) listAttachments(w http.ResponseWriter, r *http.Request, taskID, userID string, viewAll bool) {
	if !h.canAccess(r, taskID, userID, viewAll) {
		writeTaskActionJSON(w, 404, map[string]string{"error": "task not found"})
		return
	}
	items := h.getAttachmentDetails(r, taskID)
	if items == nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load attachments"})
		return
	}
	writeTaskActionJSON(w, 200, map[string]any{"attachments": items})
}

func (h taskDetailsRouter) uploadAttachment(w http.ResponseWriter, r *http.Request, taskID, userID string, viewAll bool) {
	if !h.canAccess(r, taskID, userID, viewAll) {
		writeTaskActionJSON(w, 404, map[string]string{"error": "task not found"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, taskAttachmentLimit+(1<<20))
	if err := r.ParseMultipartForm(taskAttachmentLimit); err != nil {
		writeTaskActionJSON(w, 413, map[string]string{"error": "file is too large (maximum 10 MB)"})
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	file, header, err := r.FormFile("file")
	if err != nil {
		writeTaskActionJSON(w, 400, map[string]string{"error": "choose a file to upload"})
		return
	}
	defer file.Close()
	if header.Size < 1 || header.Size > taskAttachmentLimit {
		writeTaskActionJSON(w, 413, map[string]string{"error": "file must be between 1 byte and 10 MB"})
		return
	}
	buffer := make([]byte, 512)
	n, readErr := io.ReadFull(file, buffer)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		writeTaskActionJSON(w, 400, map[string]string{"error": "could not read uploaded file"})
		return
	}
	mediaType := http.DetectContentType(buffer[:n])
	mediaType, _, _ = mime.ParseMediaType(mediaType)
	if !allowedAttachmentType(mediaType) {
		writeTaskActionJSON(w, 400, map[string]string{"error": "file type is not supported; upload an image, PDF, text, or Office document"})
		return
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		writeTaskActionJSON(w, 400, map[string]string{"error": "could not read uploaded file"})
		return
	}
	name := safeAttachmentName(header.Filename)
	if name == "" {
		writeTaskActionJSON(w, 400, map[string]string{"error": "file name is invalid"})
		return
	}
	var id, storageKey string
	if err := h.db.QueryRowContext(r.Context(), `SELECT gen_random_uuid()::text,gen_random_uuid()::text`).Scan(&id, &storageKey); err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not prepare file upload"})
		return
	}
	dir := strings.TrimSpace(os.Getenv("TASK_ATTACHMENT_DIR"))
	if dir == "" {
		dir = filepath.Join("uploads", "task-attachments")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "file storage is unavailable"})
		return
	}
	fullPath := filepath.Join(dir, storageKey)
	out, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not store uploaded file"})
		return
	}
	written, copyErr := io.Copy(out, io.LimitReader(file, taskAttachmentLimit+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || written != header.Size {
		_ = os.Remove(fullPath)
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not finish file upload"})
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		_ = os.Remove(fullPath)
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not start attachment record"})
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO task_attachments(id,task_id,uploaded_by,storage_key,original_name,media_type,byte_size) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7)`, id, taskID, userID, storageKey, name, mediaType, written); err != nil {
		_ = os.Remove(fullPath)
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not save attachment details"})
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO task_activity(task_id,actor_id,action,new_values) VALUES($1::uuid,$2::uuid,'attachment_uploaded',jsonb_build_object('attachment_id',$3::text,'name',$4::text))`, taskID, userID, id, name); err != nil {
		_ = os.Remove(fullPath)
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not record attachment activity"})
		return
	}
	if err = tx.Commit(); err != nil {
		_ = os.Remove(fullPath)
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not save attachment"})
		return
	}
	writeTaskActionJSON(w, 201, map[string]any{"attachment": attachmentDetail{ID: id, Name: name, MediaType: mediaType, Size: written, Uploader: "You", UploadedAt: time.Now().UTC().Format(time.RFC3339), DownloadURL: fmt.Sprintf("/api/tasks/%s/attachments/%s", taskID, id)}})
}

func safeAttachmentName(value string) string {
	name := filepath.Base(strings.ReplaceAll(value, "\\", "/"))
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name))
	if utf8.RuneCountInString(name) > 255 {
		name = string([]rune(name)[:255])
	}
	if name == "." || name == "/" || name == "" {
		return ""
	}
	return name
}
func allowedAttachmentType(value string) bool {
	base, _, _ := mime.ParseMediaType(value)
	switch base {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "application/pdf", "text/plain", "text/csv", "application/csv", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.ms-powerpoint", "application/vnd.openxmlformats-officedocument.presentationml.presentation", "application/zip", "application/x-zip-compressed":
		return true
	}
	return false
}

func (h taskDetailsRouter) downloadAttachment(w http.ResponseWriter, r *http.Request, taskID, attachmentID, userID string, viewAll bool) {
	if !h.canAccess(r, taskID, userID, viewAll) {
		writeTaskActionJSON(w, 404, map[string]string{"error": "task not found"})
		return
	}
	var storageKey, name, mediaType string
	err := h.db.QueryRowContext(r.Context(), `SELECT storage_key::text,original_name,media_type FROM task_attachments WHERE id=$1::uuid AND task_id=$2::uuid`, attachmentID, taskID).Scan(&storageKey, &name, &mediaType)
	if errors.Is(err, sql.ErrNoRows) {
		writeTaskActionJSON(w, 404, map[string]string{"error": "attachment not found"})
		return
	}
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not load attachment"})
		return
	}
	dir := strings.TrimSpace(os.Getenv("TASK_ATTACHMENT_DIR"))
	if dir == "" {
		dir = filepath.Join("uploads", "task-attachments")
	}
	f, err := os.Open(filepath.Join(dir, storageKey))
	if err != nil {
		writeTaskActionJSON(w, 404, map[string]string{"error": "attachment file is unavailable"})
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		writeTaskActionJSON(w, 500, map[string]string{"error": "could not read attachment"})
		return
	}
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	disposition := "attachment"
	if strings.HasPrefix(mediaType, "image/") {
		disposition = "inline"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	http.ServeContent(w, r, name, stat.ModTime(), f)
}
