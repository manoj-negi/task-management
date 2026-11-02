Office Task Manager — Backend Foundation (Batch 1)

This archive adds a structured Go backend foundation:
- main.go starts the HTTP server
- internal/config reads PORT and DATABASE_URL
- CORS_ALLOWED_ORIGINS is a comma-separated list of browser origins allowed to call the API; defaults to the Vite development origins http://localhost:5173 and http://127.0.0.1:5173
- internal/database creates and checks a PostgreSQL connection using pgx
- internal/handlers implements GET /api/health
- internal/routes registers API routes

It intentionally does not modify database migrations or frontend files.

Apply:
1. Download the ZIP to ~/Downloads.
2. From the project root (office-task-manager), run:
   unzip -o ~/Downloads/office-task-manager-backend-foundation.zip -d .
3. Open backend/.env.example and note the required environment variables.
4. Set DATABASE_URL in your terminal using your actual PostgreSQL username:
   export DATABASE_URL='postgres://YOUR_POSTGRES_USERNAME@localhost:5432/office_task_manager?sslmode=disable'
   export PORT=8080
   export CORS_ALLOWED_ORIGINS='http://localhost:5173,http://127.0.0.1:5173'
5. Run:
   cd backend
   go mod tidy
   gofmt -w main.go internal
   go run .
6. In a second terminal, test:
   curl -i http://localhost:8080/api/health

Expected success response: HTTP 200 with JSON status=ok and database=connected.

If Go reports that your installed version does not support go 1.27.1, do not change it blindly; share `go version` and the error. This archive retains the Go version from the existing project go.mod.

Safety:
- No real credentials are included.
- Existing DB schema/migrations are not changed.
- The health endpoint now requires a valid DATABASE_URL and checks PostgreSQL connectivity.
