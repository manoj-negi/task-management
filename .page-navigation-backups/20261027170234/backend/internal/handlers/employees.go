package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"office-task-manager/backend/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

type employeeHandler struct { DB *sql.DB }
type employeeRecord struct {
	ID string `json:"id"`
	FullName string `json:"full_name"`
	Email string `json:"email"`
	Role string `json:"role"`
	CanAssignTasks bool `json:"can_assign_tasks"`
	IsActive bool `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

func Employees(db *sql.DB) http.Handler { return employeeHandler{DB: db} }
func (h employeeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims.Role != "super_admin" {
		writeEmployeeJSON(w, http.StatusForbidden, map[string]string{"error":"super admin access required"}); return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/employees"), "/")
	if path == "" {
		switch r.Method { case http.MethodGet: h.list(w,r); case http.MethodPost: h.create(w,r); default: w.Header().Set("Allow","GET, POST"); writeEmployeeJSON(w,http.StatusMethodNotAllowed,map[string]string{"error":"method not allowed"}) }; return
	}
	parts := strings.Split(path,"/")
	if len(parts) == 2 && parts[1] == "password" && r.Method == http.MethodPost { h.resetPassword(w,r,parts[0]); return }
	if len(parts) == 2 && parts[1] == "permissions" && r.Method == http.MethodPatch { h.setAssigner(w,r,parts[0]); return }
	if len(parts) == 1 && r.Method == http.MethodPatch { h.setActive(w,r,parts[0]); return }
	writeEmployeeJSON(w,http.StatusNotFound,map[string]string{"error":"endpoint not found"})
}
func (h employeeHandler) list(w http.ResponseWriter,r *http.Request) {
	rows,err:=h.DB.QueryContext(r.Context(),`SELECT id::text, full_name, email, role::text, can_assign_tasks, is_active, to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM users ORDER BY is_active DESC, full_name ASC`)
	if err!=nil { writeEmployeeJSON(w,500,map[string]string{"error":"could not load employees"}); return }; defer rows.Close()
	out:=[]employeeRecord{}
	for rows.Next(){ var e employeeRecord; if err:=rows.Scan(&e.ID,&e.FullName,&e.Email,&e.Role,&e.CanAssignTasks,&e.IsActive,&e.CreatedAt);err!=nil{writeEmployeeJSON(w,500,map[string]string{"error":"could not read employees"});return}; out=append(out,e) }
	if err:=rows.Err();err!=nil{writeEmployeeJSON(w,500,map[string]string{"error":"could not read employees"});return}; writeEmployeeJSON(w,200,map[string]any{"employees":out})
}
func (h employeeHandler) create(w http.ResponseWriter,r *http.Request) {
	var req struct{ FullName string `json:"full_name"`; Email string `json:"email"`; Password string `json:"password"` }
	r.Body=http.MaxBytesReader(w,r.Body,1<<20); defer r.Body.Close(); d:=json.NewDecoder(r.Body); d.DisallowUnknownFields(); if err:=d.Decode(&req);err!=nil{writeEmployeeJSON(w,400,map[string]string{"error":"invalid request body"});return}
	req.FullName=strings.TrimSpace(req.FullName); req.Email=strings.ToLower(strings.TrimSpace(req.Email)); if len(req.FullName)<2||len(req.FullName)>120||!strings.Contains(req.Email,"@")||len(req.Email)>254||len(req.Password)<12||len(req.Password)>72 {writeEmployeeJSON(w,400,map[string]string{"error":"name, valid email, and password (12–72 characters) are required"});return}
	hash,err:=bcrypt.GenerateFromPassword([]byte(req.Password),bcrypt.DefaultCost);if err!=nil{writeEmployeeJSON(w,500,map[string]string{"error":"could not secure password"});return}
	var e employeeRecord; err=h.DB.QueryRowContext(r.Context(),`INSERT INTO users (full_name,email,password_hash,role,can_assign_tasks,is_active) VALUES ($1,$2,$3,'employee',false,true) RETURNING id::text,full_name,email,role::text,can_assign_tasks,is_active,to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,req.FullName,req.Email,string(hash)).Scan(&e.ID,&e.FullName,&e.Email,&e.Role,&e.CanAssignTasks,&e.IsActive,&e.CreatedAt)
	if err!=nil { if strings.Contains(strings.ToLower(err.Error()),"unique") {writeEmployeeJSON(w,409,map[string]string{"error":"an account with this email already exists"})} else {writeEmployeeJSON(w,500,map[string]string{"error":"could not create employee"})};return }; writeEmployeeJSON(w,201,map[string]any{"employee":e})
}
func (h employeeHandler) setAssigner(w http.ResponseWriter,r *http.Request,id string) {
	var req struct{ CanAssignTasks bool `json:"can_assign_tasks"` }; if err:=json.NewDecoder(http.MaxBytesReader(w,r.Body,1<<20)).Decode(&req);err!=nil{writeEmployeeJSON(w,400,map[string]string{"error":"invalid request body"});return}
	res,err:=h.DB.ExecContext(r.Context(),`UPDATE users SET can_assign_tasks=$1, updated_at=now() WHERE id=$2::uuid AND role='employee' AND is_active=true`,req.CanAssignTasks,id); if err!=nil{writeEmployeeJSON(w,400,map[string]string{"error":"could not update permissions; check employee ID"});return}; n,_:=res.RowsAffected();if n==0{writeEmployeeJSON(w,404,map[string]string{"error":"active employee not found"});return};writeEmployeeJSON(w,200,map[string]any{"ok":true,"can_assign_tasks":req.CanAssignTasks})
}
func (h employeeHandler) setActive(w http.ResponseWriter,r *http.Request,id string) {
	var req struct{ IsActive bool `json:"is_active"` };if err:=json.NewDecoder(http.MaxBytesReader(w,r.Body,1<<20)).Decode(&req);err!=nil{writeEmployeeJSON(w,400,map[string]string{"error":"invalid request body"});return}
	res,err:=h.DB.ExecContext(r.Context(),`UPDATE users SET is_active=$1, can_assign_tasks=CASE WHEN $1 THEN can_assign_tasks ELSE false END, updated_at=now() WHERE id=$2::uuid AND role='employee'`,req.IsActive,id);if err!=nil{writeEmployeeJSON(w,400,map[string]string{"error":"could not update employee; check employee ID"});return};n,_:=res.RowsAffected();if n==0{writeEmployeeJSON(w,404,map[string]string{"error":"employee not found"});return};writeEmployeeJSON(w,200,map[string]any{"ok":true,"is_active":req.IsActive})
}
func (h employeeHandler) resetPassword(w http.ResponseWriter,r *http.Request,id string) {
	// A one-time generated password is returned only to the authenticated super admin.
	b:=make([]byte,18);if _,err:=rand.Read(b);err!=nil{writeEmployeeJSON(w,500,map[string]string{"error":"could not generate password"});return}; password:="Tm!"+hex.EncodeToString(b);hash,err:=bcrypt.GenerateFromPassword([]byte(password),bcrypt.DefaultCost);if err!=nil{writeEmployeeJSON(w,500,map[string]string{"error":"could not secure password"});return}
	res,err:=h.DB.ExecContext(r.Context(),`UPDATE users SET password_hash=$1, updated_at=now() WHERE id=$2::uuid AND role='employee' AND is_active=true`,string(hash),id);if err!=nil{writeEmployeeJSON(w,400,map[string]string{"error":"could not reset password; check employee ID"});return};n,_:=res.RowsAffected();if n==0{writeEmployeeJSON(w,404,map[string]string{"error":"active employee not found"});return};writeEmployeeJSON(w,200,map[string]any{"ok":true,"temporary_password":password,"message":"Copy this password now and share it securely. It will not be shown again."})
}
func writeEmployeeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
