# Task assignee visibility fix

## What this changes
- The assignable-users endpoint now lists all active employee accounts, not only employees already present in `project_members`.
- Task creation validates that each selected assignee is an active employee, rather than requiring an existing project membership row.
- The current user who creates a task is still restricted to projects they belong to (unless they are a super admin).
- Only backend handler `backend/internal/handlers/tasks.go` is changed. Frontend and database schema are untouched.

This makes newly created employees such as Gaurav available in the task assignee picker. Task assignment itself is stored in `task_assignees`, which the dashboard uses to find an employee's assigned tasks.

## Apply
From the repository root:
```bash
unzip -o ~/Downloads/task-assignee-visibility-fix.zip
cd backend
gofmt -w internal/handlers/tasks.go
go test ./...
```
Then restart the Go backend and reload the frontend.

## Design note
This intentionally separates project membership from task assignment. If the product should require formal project membership before someone can receive a task, the correct alternative is to add a project-membership management UI and add employees to the selected project first.
