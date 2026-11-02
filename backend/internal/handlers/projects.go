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

type projectRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	StartDate   string `json:"startDate"`
	DueDate     string `json:"dueDate"`
	CreatedAt   string `json:"createdAt"`
	TotalTasks  int    `json:"totalTasks"`
	Completed   int    `json:"completedTasks"`
	MemberCount int    `json:"memberCount"`
}

type projectMemberRecord struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	IsActive bool   `json:"isActive"`
	JoinedAt string `json:"joinedAt"`
}

func Projects(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			writeTaskJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/projects"), "/")
		if path == "" {
			switch r.Method {
			case http.MethodGet:
				listProjects(db, w, r, claims.UserID, claims.Role)
			case http.MethodPost:
				if !claims.HasPermission("manage_projects") {
					writeTaskJSON(w, http.StatusForbidden, map[string]string{"error": "super admin access required to create projects"})
					return
				}
				createProject(db, w, r, claims.UserID)
			default:
				w.Header().Set("Allow", "GET, POST")
				writeTaskJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			}
			return
		}

		parts := strings.Split(path, "/")
		if len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodGet {
			listProjectMembers(db, w, r, parts[0], claims.UserID, claims.Role)
			return
		}
		if len(parts) == 3 && parts[1] == "members" && r.Method == http.MethodPost {
			if !claims.HasPermission("manage_projects") {
				writeTaskJSON(w, http.StatusForbidden, map[string]string{"error": "super admin access required to manage project members"})
				return
			}
			switch parts[2] {
			case "add":
				changeProjectMember(db, w, r, parts[0], true)
			case "remove":
				changeProjectMember(db, w, r, parts[0], false)
			default:
				writeTaskJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
			}
			return
		}
		if len(parts) == 2 && r.Method == http.MethodPost {
			if !claims.HasPermission("manage_projects") {
				writeTaskJSON(w, http.StatusForbidden, map[string]string{"error": "super admin access required to manage projects"})
				return
			}
			switch parts[1] {
			case "archive":
				updateProjectStatus(db, w, r, parts[0], "archived")
			case "delete":
				deleteProject(db, w, r, parts[0])
			default:
				writeTaskJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
			}
			return
		}
		if len(parts) == 1 {
			if r.Method == http.MethodGet {
				getProject(db, w, r, parts[0], claims.UserID, claims.Role)
				return
			}
			if r.Method == http.MethodPost {
				if !claims.HasPermission("manage_projects") {
					writeTaskJSON(w, http.StatusForbidden, map[string]string{"error": "super admin access required to edit projects"})
					return
				}
				updateProject(db, w, r, parts[0])
				return
			}
		}
		writeTaskJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
	})
}

func listProjects(db *sql.DB, w http.ResponseWriter, r *http.Request, userID, role string) {
	rows, err := db.QueryContext(r.Context(), `
SELECT p.id::text, p.name, COALESCE(p.description,''), p.status::text,
       COALESCE(to_char(p.start_date,'YYYY-MM-DD'), ''),
       COALESCE(to_char(p.due_date,'YYYY-MM-DD'), ''),
       to_char(p.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       COUNT(DISTINCT t.id)::int,
       COUNT(DISTINCT t.id) FILTER (WHERE t.status='approved')::int,
       COUNT(DISTINCT pm.user_id) FILTER (WHERE pm.removed_at IS NULL)::int
FROM projects p
LEFT JOIN tasks t ON t.project_id=p.id
LEFT JOIN project_members pm ON pm.project_id=p.id
WHERE ($1='super_admin' OR EXISTS (
  SELECT 1 FROM project_members mine
  WHERE mine.project_id=p.id AND mine.user_id=$2::uuid AND mine.removed_at IS NULL
))
GROUP BY p.id
ORDER BY CASE WHEN p.status='archived' THEN 1 ELSE 0 END, p.created_at DESC`, role, userID)
	if err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not load projects"})
		return
	}
	defer rows.Close()
	out := make([]projectRecord, 0)
	for rows.Next() {
		var p projectRecord
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Status, &p.StartDate, &p.DueDate, &p.CreatedAt, &p.TotalTasks, &p.Completed, &p.MemberCount); err != nil {
			writeTaskJSON(w, 500, map[string]string{"error": "could not read projects"})
			return
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not read projects"})
		return
	}
	writeTaskJSON(w, 200, out)
}

func getProject(db *sql.DB, w http.ResponseWriter, r *http.Request, id, userID, role string) {
	var p projectRecord
	err := db.QueryRowContext(r.Context(), `
SELECT p.id::text, p.name, COALESCE(p.description,''), p.status::text,
       COALESCE(to_char(p.start_date,'YYYY-MM-DD'), ''),
       COALESCE(to_char(p.due_date,'YYYY-MM-DD'), ''),
       to_char(p.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       (SELECT COUNT(*)::int FROM tasks t WHERE t.project_id=p.id),
       (SELECT COUNT(*)::int FROM tasks t WHERE t.project_id=p.id AND t.status='approved'),
       (SELECT COUNT(*)::int FROM project_members pm WHERE pm.project_id=p.id AND pm.removed_at IS NULL)
FROM projects p
WHERE p.id=$1::uuid AND ($2='super_admin' OR EXISTS (
 SELECT 1 FROM project_members pm WHERE pm.project_id=p.id AND pm.user_id=$3::uuid AND pm.removed_at IS NULL
))`, id, role, userID).Scan(&p.ID, &p.Name, &p.Description, &p.Status, &p.StartDate, &p.DueDate, &p.CreatedAt, &p.TotalTasks, &p.Completed, &p.MemberCount)
	if errors.Is(err, sql.ErrNoRows) {
		writeTaskJSON(w, 404, map[string]string{"error": "project not found"})
		return
	}
	if err != nil {
		writeTaskJSON(w, 400, map[string]string{"error": "invalid project ID"})
		return
	}
	writeTaskJSON(w, 200, p)
}

func decodeProjectBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		writeTaskJSON(w, 400, map[string]string{"error": "invalid request body"})
		return false
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		writeTaskJSON(w, 400, map[string]string{"error": "request must contain one JSON object"})
		return false
	}
	return true
}

type projectInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	StartDate   string `json:"startDate"`
	DueDate     string `json:"dueDate"`
}

func validateProjectInput(w http.ResponseWriter, p *projectInput, allowStatus bool) bool {
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	p.Status = strings.TrimSpace(strings.ToLower(p.Status))
	if len([]rune(p.Name)) < 2 || len([]rune(p.Name)) > 160 {
		writeTaskJSON(w, 400, map[string]string{"error": "project name must be 2-160 characters"})
		return false
	}
	if len(p.Description) > 5000 {
		writeTaskJSON(w, 400, map[string]string{"error": "description must be 5000 characters or fewer"})
		return false
	}
	if p.Status == "" {
		p.Status = "planned"
	}
	switch p.Status {
	case "planned", "active", "on_hold", "completed", "archived":
	default:
		writeTaskJSON(w, 400, map[string]string{"error": "status must be planned, active, on_hold, completed, or archived"})
		return false
	}
	if !allowStatus && p.Status == "archived" {
		writeTaskJSON(w, 400, map[string]string{"error": "use the archive action to archive a project"})
		return false
	}
	for _, date := range []string{p.StartDate, p.DueDate} {
		if strings.TrimSpace(date) != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				writeTaskJSON(w, 400, map[string]string{"error": "dates must use YYYY-MM-DD format"})
				return false
			}
		}
	}
	if p.StartDate != "" && p.DueDate != "" && p.DueDate < p.StartDate {
		writeTaskJSON(w, 400, map[string]string{"error": "due date cannot be before start date"})
		return false
	}
	return true
}

func createProject(db *sql.DB, w http.ResponseWriter, r *http.Request, userID string) {
	var p projectInput
	if !decodeProjectBody(w, r, &p) || !validateProjectInput(w, &p, false) {
		return
	}
	var out projectRecord
	err := db.QueryRowContext(r.Context(), `
INSERT INTO projects (name,description,status,start_date,due_date,created_by)
VALUES ($1,$2,$3::project_status,NULLIF($4,'')::date,NULLIF($5,'')::date,$6::uuid)
RETURNING id::text,name,COALESCE(description,''),status::text,
COALESCE(to_char(start_date,'YYYY-MM-DD'),''),COALESCE(to_char(due_date,'YYYY-MM-DD'),''),
to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
		p.Name, p.Description, p.Status, p.StartDate, p.DueDate, userID).
		Scan(&out.ID, &out.Name, &out.Description, &out.Status, &out.StartDate, &out.DueDate, &out.CreatedAt)
	if err != nil {
		writeTaskJSON(w, 400, map[string]string{"error": "could not create project; check the project details"})
		return
	}
	writeTaskJSON(w, 201, out)
}

func updateProject(db *sql.DB, w http.ResponseWriter, r *http.Request, id string) {
	var p projectInput
	if !decodeProjectBody(w, r, &p) || !validateProjectInput(w, &p, false) {
		return
	}
	var out projectRecord
	err := db.QueryRowContext(r.Context(), `
UPDATE projects SET name=$2, description=$3, status=$4::project_status,
start_date=NULLIF($5,'')::date, due_date=NULLIF($6,'')::date
WHERE id=$1::uuid
RETURNING id::text,name,COALESCE(description,''),status::text,
COALESCE(to_char(start_date,'YYYY-MM-DD'),''),COALESCE(to_char(due_date,'YYYY-MM-DD'),''),
to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
		id, p.Name, p.Description, p.Status, p.StartDate, p.DueDate).
		Scan(&out.ID, &out.Name, &out.Description, &out.Status, &out.StartDate, &out.DueDate, &out.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeTaskJSON(w, 404, map[string]string{"error": "project not found"})
		return
	}
	if err != nil {
		writeTaskJSON(w, 400, map[string]string{"error": "could not update project; check the project ID and details"})
		return
	}
	writeTaskJSON(w, 200, out)
}

func updateProjectStatus(db *sql.DB, w http.ResponseWriter, r *http.Request, id, status string) {
	res, err := db.ExecContext(r.Context(), `UPDATE projects SET status=$2::project_status WHERE id=$1::uuid`, id, status)
	if err != nil {
		writeTaskJSON(w, 400, map[string]string{"error": "could not archive project"})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeTaskJSON(w, 404, map[string]string{"error": "project not found"})
		return
	}
	writeTaskJSON(w, 200, map[string]any{"ok": true, "status": status})
}

func deleteProject(db *sql.DB, w http.ResponseWriter, r *http.Request, id string) {
	var taskCount int
	if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tasks WHERE project_id=$1::uuid`, id).Scan(&taskCount); err != nil {
		writeTaskJSON(w, 400, map[string]string{"error": "invalid project ID"})
		return
	}
	if taskCount > 0 {
		writeTaskJSON(w, 409, map[string]string{"error": "this project has tasks; archive it instead to preserve task history"})
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not start delete transaction"})
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM project_members WHERE project_id=$1::uuid`, id); err != nil {
		writeTaskJSON(w, 409, map[string]string{"error": "could not remove project members"})
		return
	}
	res, err := tx.ExecContext(r.Context(), `DELETE FROM projects WHERE id=$1::uuid`, id)
	if err != nil {
		writeTaskJSON(w, 409, map[string]string{"error": "could not delete project"})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeTaskJSON(w, 404, map[string]string{"error": "project not found"})
		return
	}
	if err = tx.Commit(); err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not commit project deletion"})
		return
	}
	writeTaskJSON(w, 200, map[string]any{"ok": true})
}

func listProjectMembers(db *sql.DB, w http.ResponseWriter, r *http.Request, projectID, userID, role string) {
	var allowed bool
	err := db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM projects p WHERE p.id=$1::uuid AND ($2='super_admin' OR EXISTS(SELECT 1 FROM project_members pm WHERE pm.project_id=p.id AND pm.user_id=$3::uuid AND pm.removed_at IS NULL)))`, projectID, role, userID).Scan(&allowed)
	if err != nil || !allowed {
		writeTaskJSON(w, 404, map[string]string{"error": "project not found"})
		return
	}
	rows, err := db.QueryContext(r.Context(), `
SELECT u.id::text,u.full_name,u.email,u.is_active,
to_char(pm.joined_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM project_members pm JOIN users u ON u.id=pm.user_id
WHERE pm.project_id=$1::uuid AND pm.removed_at IS NULL
ORDER BY u.full_name`, projectID)
	if err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not load project members"})
		return
	}
	defer rows.Close()
	out := make([]projectMemberRecord, 0)
	for rows.Next() {
		var m projectMemberRecord
		if err := rows.Scan(&m.ID, &m.FullName, &m.Email, &m.IsActive, &m.JoinedAt); err != nil {
			writeTaskJSON(w, 500, map[string]string{"error": "could not read project members"})
			return
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		writeTaskJSON(w, 500, map[string]string{"error": "could not read project members"})
		return
	}
	writeTaskJSON(w, 200, out)
}

func changeProjectMember(db *sql.DB, w http.ResponseWriter, r *http.Request, projectID string, add bool) {
	var req struct {
		UserID string `json:"userId"`
	}
	if !decodeProjectBody(w, r, &req) {
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		writeTaskJSON(w, 400, map[string]string{"error": "userId is required"})
		return
	}
	var projectExists bool
	if err := db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1::uuid AND status <> 'archived')`, projectID).Scan(&projectExists); err != nil || !projectExists {
		writeTaskJSON(w, 404, map[string]string{"error": "active project not found"})
		return
	}
	if add {
		var activeEmployee bool
		if err := db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM users u JOIN roles r ON r.id=u.role_id WHERE u.id=$1::uuid AND r.name='employee' AND u.is_active=true)`, req.UserID).Scan(&activeEmployee); err != nil || !activeEmployee {
			writeTaskJSON(w, 400, map[string]string{"error": "an active employee account is required"})
			return
		}
		_, err := db.ExecContext(r.Context(), `
INSERT INTO project_members(project_id,user_id,joined_at,removed_at)
VALUES($1::uuid,$2::uuid,now(),NULL)
ON CONFLICT(project_id,user_id) DO UPDATE SET joined_at=now(),removed_at=NULL`, projectID, req.UserID)
		if err != nil {
			writeTaskJSON(w, 400, map[string]string{"error": "could not add employee to project"})
			return
		}
		writeTaskJSON(w, 200, map[string]any{"ok": true, "message": "employee added to project"})
		return
	}
	res, err := db.ExecContext(r.Context(), `UPDATE project_members SET removed_at=now() WHERE project_id=$1::uuid AND user_id=$2::uuid AND removed_at IS NULL`, projectID, req.UserID)
	if err != nil {
		writeTaskJSON(w, 400, map[string]string{"error": "could not remove project member"})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeTaskJSON(w, 404, map[string]string{"error": "active project membership not found"})
		return
	}
	writeTaskJSON(w, 200, map[string]any{"ok": true, "message": "employee removed from project"})
}
