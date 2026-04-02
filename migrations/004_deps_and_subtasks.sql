-- Task dependencies: junction table for blocking/prerequisite relationships
-- task_id depends ON blocker_id; task_id cannot start until blocker_id completes
CREATE TABLE IF NOT EXISTS task_dependencies (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    blocker_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (blocker_id) REFERENCES tasks(id) ON DELETE CASCADE,
    UNIQUE(task_id, blocker_id)
);

-- Task subtasks: parent-child hierarchy
-- parent_id is the containing task; child_id is the subtask
-- UNIQUE on child_id enforces exactly one parent per subtask
CREATE TABLE IF NOT EXISTS task_subtasks (
    id TEXT PRIMARY KEY,
    parent_id TEXT NOT NULL,
    child_id TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (child_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Indexes for forward lookups
CREATE INDEX IF NOT EXISTS idx_deps_task_id ON task_dependencies(task_id);
CREATE INDEX IF NOT EXISTS idx_deps_blocker_id ON task_dependencies(blocker_id);
CREATE INDEX IF NOT EXISTS idx_subtasks_parent_id ON task_subtasks(parent_id);
CREATE INDEX IF NOT EXISTS idx_subtasks_child_id ON task_subtasks(child_id);

-- Indexes for reverse lookups ("who depends on X", "who is parent of X")
CREATE INDEX IF NOT EXISTS idx_deps_blocker_reverse ON task_dependencies(blocker_id, task_id);
CREATE INDEX IF NOT EXISTS idx_subtasks_child_reverse ON task_subtasks(child_id, parent_id);
