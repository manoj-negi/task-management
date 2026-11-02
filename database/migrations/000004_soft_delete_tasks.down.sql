DROP INDEX IF EXISTS idx_tasks_active;
ALTER TABLE tasks DROP COLUMN IF EXISTS deleted_at;
