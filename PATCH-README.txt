Office Task Manager — Project Management patch

Included:
- backend/internal/handlers/projects.go (new project CRUD and member management endpoints)
- backend/internal/handlers/tasks.go (assignment list and validation require active project membership)
- backend/internal/handlers/dashboard.go (archived projects excluded from task-creation/dashboard project choices)
- backend/internal/routes/routes.go (register /api/projects routes)
- frontend/src/services/projects.ts (new API client)
- frontend/src/components/ProjectManagement.tsx (new project management screen)
- frontend/src/App.tsx (integrate Project Management screen)

Apply from the repository root (~/office-task-manager):
  unzip -o office-task-manager-project-management-patch.zip -d .

Then run:
  gofmt -w backend/internal/handlers/projects.go backend/internal/handlers/tasks.go backend/internal/handlers/dashboard.go backend/internal/routes/routes.go
  go test ./...

In a second terminal:
  cd frontend
  npm run build

API behavior:
- GET /api/projects
- POST /api/projects (super_admin)
- GET /api/projects/{id}
- POST /api/projects/{id} (edit; super_admin)
- GET /api/projects/{id}/members
- POST /api/projects/{id}/members/add (super_admin)
- POST /api/projects/{id}/members/remove (super_admin)
- POST /api/projects/{id}/archive (super_admin)
- POST /api/projects/{id}/delete (super_admin; only when project has no tasks)

Projects with task history cannot be deleted; archive them instead. Removing a member is a soft removal (removed_at is set), preserving existing task assignment history. Re-adding the same employee reactivates the existing membership row.

Before deploying, test with your local database and confirm your app's current Go and Node versions. This patch intentionally does not add migrations because it uses the existing tables/enums described in your database output.
