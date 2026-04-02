-- Notification configs table
CREATE TABLE IF NOT EXISTS notification_configs (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL UNIQUE,
    enabled INTEGER NOT NULL DEFAULT 0,
    config_json TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Insert default configs
INSERT OR IGNORE INTO notification_configs (id, type, enabled, config_json, created_at, updated_at)
VALUES
    (lower(hex(randomblob(16))), 'macos', 1, '{}', strftime('%s', 'now'), strftime('%s', 'now')),
    (lower(hex(randomblob(16))), 'email', 0, '{}', strftime('%s', 'now'), strftime('%s', 'now'));
