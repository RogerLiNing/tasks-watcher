-- Custom Kanban columns
CREATE TABLE IF NOT EXISTS task_columns (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    label TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '#86868b',
    position INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

-- Default columns
INSERT OR IGNORE INTO task_columns (id, key, label, color, position, created_at) VALUES
    (lower(hex(randomblob(16))), 'pending',    'Pending',     '#86868b', 0, strftime('%s', 'now')),
    (lower(hex(randomblob(16))), 'in_progress','In Progress', '#0071e3', 1, strftime('%s', 'now')),
    (lower(hex(randomblob(16))), 'completed',  'Completed',   '#34c759', 2, strftime('%s', 'now')),
    (lower(hex(randomblob(16))), 'failed',     'Failed',     '#ff3b30', 3, strftime('%s', 'now')),
    (lower(hex(randomblob(16))), 'cancelled',  'Cancelled',   '#ff9500', 4, strftime('%s', 'now'));
