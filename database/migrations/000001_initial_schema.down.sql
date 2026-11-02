
-- =========================================================
-- Office Task Manager: Rollback Initial Schema
-- Objects are dropped in dependency-safe order.
-- =========================================================

-- Remove triggers first.
DROP TRIGGER IF EXISTS trg_task_comments_updated_at ON task_comments;
DROP TRIGGER IF EXISTS trg_tasks_updated_at ON tasks;
DROP TRIGGER IF EXISTS trg_projects_updated_at ON projects;
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;

DROP TRIGGER IF EXISTS trg_validate_task_reviewer ON task_reviews;
DROP TRIGGER IF EXISTS trg_validate_task_assignment ON task_assignees;
DROP TRIGGER IF EXISTS trg_prevent_task_cycle ON tasks;

-- Remove tables in reverse dependency order.
DROP TABLE IF EXISTS task_activity;
DROP TABLE IF EXISTS task_reviews;
DROP TABLE IF EXISTS task_comments;
DROP TABLE IF EXISTS task_assignees;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS users;

-- Remove functions.
DROP FUNCTION IF EXISTS set_updated_at();
DROP FUNCTION IF EXISTS validate_task_reviewer();
DROP FUNCTION IF EXISTS validate_task_assignment();
DROP FUNCTION IF EXISTS prevent_task_cycle();

-- Remove enum types.
DROP TYPE IF EXISTS task_review_decision;
DROP TYPE IF EXISTS assignee_work_status;
DROP TYPE IF EXISTS task_priority;
DROP TYPE IF EXISTS task_status;
DROP TYPE IF EXISTS project_status;
DROP TYPE IF EXISTS user_role;
