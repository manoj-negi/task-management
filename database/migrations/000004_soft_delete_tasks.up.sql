-- Keep task records and their audit trail when an administrator removes a task.
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_tasks_active ON tasks(id) WHERE deleted_at IS NULL;
