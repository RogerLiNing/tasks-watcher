-- Add task_mode to tasks: "" (default/parallel), "sequential", "parallel"
ALTER TABLE tasks ADD COLUMN task_mode TEXT NOT NULL DEFAULT '';
