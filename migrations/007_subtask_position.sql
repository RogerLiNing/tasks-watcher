-- Add position to subtasks for sequential ordering
ALTER TABLE task_subtasks ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

-- Index for position-ordered subtask queries
CREATE INDEX IF NOT EXISTS idx_subtasks_position ON task_subtasks(parent_id, position);
