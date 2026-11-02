package routes

import (
	"database/sql"
	"net/http"
	"os"
	"strings"

	"office-task-manager/backend/internal/auth"
	"office-task-manager/backend/internal/handlers"
)

func New(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", handlers.Health(db))

	authHandler := handlers.AuthHandler{DB: db}
	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)

	mux.Handle(
		"/api/auth/me",
		protected(db, http.HandlerFunc(authHandler.Me)),
	)
	mux.Handle(
		"/api/dashboard",
		protected(db, http.HandlerFunc(handlers.Dashboard(db))),
	)
	mux.Handle("/api/activity", protected(db, http.HandlerFunc(handlers.TeamActivity(db))))
	mux.Handle("/api/projects", protected(db, handlers.Projects(db)))
	mux.Handle("/api/projects/", protected(db, handlers.Projects(db)))
	mux.Handle("/api/tasks", protected(db, http.HandlerFunc(handlers.CreateTask(db))))
	mux.Handle("/api/assignable-users", protected(db, http.HandlerFunc(handlers.AssignableUsers(db))))

	mux.Handle("/api/tasks/", protected(db, handlers.TaskDetailsRouter(db)))

	mux.Handle("/api/admin/employees", protected(db, handlers.Employees(db)))
	mux.Handle("/api/admin/employees/", protected(db, handlers.Employees(db)))
	mux.Handle("/api/admin/reviews", protected(db, http.HandlerFunc(handlers.AdminReviews(db))))
	mux.Handle("/api/admin/tasks/", protected(db, http.HandlerFunc(handlers.AdminTaskRouter(db))))

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := map[string]struct{}{}
	configured := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if configured == "" {
		configured = "http://localhost:5173,http://127.0.0.1:5173"
	}
	for _, origin := range strings.Split(configured, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowedOrigins[origin] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if _, allowed := allowedOrigins[origin]; !allowed {
				http.Error(w, `{"error":"origin not allowed"}`, http.StatusForbidden)
				return
			}
		}
		if _, allowed := allowedOrigins[origin]; allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "600")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// protected validates the signed token and then refreshes role/permissions from
// PostgreSQL before a handler makes authorization decisions.
func protected(db *sql.DB, handler http.Handler) http.Handler {
	return auth.RequireAuth(auth.RequireCurrentUser(db, handler))
}
