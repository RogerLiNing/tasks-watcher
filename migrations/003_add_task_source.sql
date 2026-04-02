ALTER TABLE tasks ADD COLUMN source TEXT NOT NULL DEFAULT 'manual';
CREATE INDEX IF NOT EXISTS idx_tasks_source ON tasks(source);
