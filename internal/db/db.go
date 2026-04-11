package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/rogerrlee/tasks-watcher/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func Open(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db dir: %w", err)
	}

	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	conn.SetMaxOpenConns(1) // SQLite best practice

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn exposes the underlying sql.DB for advanced queries.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// migrate runs all pending migrations
func (db *DB) migrate() error {
	// Create migrations table if not exists
	_, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`)
	if err != nil {
		return err
	}

	// Read migration files
	migFiles, _ := filepath.Glob("migrations/*.sql")
	sort.Strings(migFiles)

	for _, file := range migFiles {
		version := strings.TrimSuffix(filepath.Base(file), ".sql")
		var exists int
		err := db.conn.QueryRow(`SELECT 1 FROM schema_migrations WHERE version = ?`, version).Scan(&exists)
		if err == nil {
			continue // already applied
		}

		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		_, err = db.conn.Exec(string(sqlBytes))
		if err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", file, err)
		}

		_, err = db.conn.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, models.Now())
		if err != nil {
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}
	}

	return nil
}

// --- Projects ---

func (db *DB) CreateProject(p *models.Project) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	p.CreatedAt = models.Now()
	p.UpdatedAt = models.Now()
	_, err := db.conn.Exec(
		`INSERT INTO projects (id, name, description, repo_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.RepoPath, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (db *DB) GetProject(id string) (*models.Project, error) {
	p := &models.Project{}
	err := db.conn.QueryRow(
		`SELECT id, name, description, repo_path, created_at, updated_at FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.RepoPath, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (db *DB) GetProjectByName(name string) (*models.Project, error) {
	p := &models.Project{}
	err := db.conn.QueryRow(
		`SELECT id, name, description, repo_path, created_at, updated_at FROM projects WHERE name = ?`, name,
	).Scan(&p.ID, &p.Name, &p.Description, &p.RepoPath, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (db *DB) ListProjects() ([]models.Project, error) {
	rows, err := db.conn.Query(`SELECT id, name, description, repo_path, created_at, updated_at FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.RepoPath, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (db *DB) UpdateProject(p *models.Project) error {
	p.UpdatedAt = models.Now()
	_, err := db.conn.Exec(
		`UPDATE projects SET name = ?, description = ?, repo_path = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.Description, p.RepoPath, p.UpdatedAt, p.ID,
	)
	return err
}

func (db *DB) DeleteProject(id string) error {
	_, err := db.conn.Exec(`DELETE FROM projects WHERE id = ?`, id)
	return err
}

func (db *DB) GetOrCreateProject(name string) (*models.Project, error) {
	p, err := db.GetProjectByName(name)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p, nil
	}
	p = &models.Project{
		ID:          uuid.New().String(),
		Name:        name,
		Description: "",
		RepoPath:    "",
		CreatedAt:   models.Now(),
		UpdatedAt:   models.Now(),
	}
	if err := db.CreateProject(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (db *DB) GetProjectByRepoPath(repoPath string) (*models.Project, error) {
	if repoPath == "" {
		return nil, nil
	}
	p := &models.Project{}
	err := db.conn.QueryRow(
		`SELECT id, name, description, repo_path, created_at, updated_at FROM projects WHERE repo_path = ?`, repoPath,
	).Scan(&p.ID, &p.Name, &p.Description, &p.RepoPath, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// GetOrCreateByRepoPath returns an existing project with this repo_path, or creates a new one.
// The project name defaults to the repo directory name.
func (db *DB) GetOrCreateByRepoPath(repoPath string) (*models.Project, error) {
	if repoPath == "" {
		return nil, nil
	}
	p, err := db.GetProjectByRepoPath(repoPath)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p, nil
	}
	// Extract name from repo path
	name := filepath.Base(repoPath)
	if name == "" || name == "." || name == "/" {
		name = "default"
	}
	// Check if a project with this name already exists
	existing, err := db.GetProjectByName(name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// Project exists by name but has no repo_path — update it
		if existing.RepoPath == "" {
			existing.RepoPath = repoPath
			if err := db.UpdateProject(existing); err != nil {
				return nil, err
			}
		}
		return existing, nil
	}
	// Create brand new project
	p = &models.Project{
		ID:          uuid.New().String(),
		Name:        name,
		Description: "",
		RepoPath:    repoPath,
		CreatedAt:   models.Now(),
		UpdatedAt:   models.Now(),
	}
	if err := db.CreateProject(p); err != nil {
		return nil, err
	}
	return p, nil
}

// --- Tasks ---

func (db *DB) CreateTask(t *models.Task) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	t.CreatedAt = models.Now()
	t.UpdatedAt = models.Now()
	_, err := db.conn.Exec(
		`INSERT INTO tasks (id, project_id, title, description, status, priority, assignee, source, task_mode, error_message, heartbeat_at, created_at, updated_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ProjectID, t.Title, models.SerializeDescription(t.Description), t.Status, t.Priority, models.SerializeAssignees(t.Assignees), t.Source, t.TaskMode, t.ErrorMessage, t.HeartbeatAt, t.CreatedAt, t.UpdatedAt, t.CompletedAt,
	)
	return err
}

func (db *DB) GetTask(id string) (*models.Task, error) {
	t := &models.Task{}
	var completedAt sql.NullInt64
	var heartbeatAt sql.NullInt64
	var errorMsg sql.NullString
	var descStr sql.NullString
	var taskMode sql.NullString
	var assigneesStr string
	err := db.conn.QueryRow(
		`SELECT id, project_id, title, description, status, priority, assignee, source, task_mode, error_message, heartbeat_at, created_at, updated_at, completed_at
		 FROM tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.ProjectID, &t.Title, &descStr, &t.Status, &t.Priority, &assigneesStr, &t.Source, &taskMode, &errorMsg, &heartbeatAt, &t.CreatedAt, &t.UpdatedAt, &completedAt)
	t.Assignees = models.ParseAssignees(assigneesStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if descStr.Valid && descStr.String != "" {
		var parsed map[string]string
		if err := json.Unmarshal([]byte(descStr.String), &parsed); err == nil {
			t.Description = parsed
		} else {
			// Legacy plain-string description — treat as English
			t.Description = map[string]string{"en": descStr.String}
		}
	}
	if taskMode.Valid && taskMode.String != "" {
		t.TaskMode = models.TaskMode(taskMode.String)
	}
	if errorMsg.Valid {
		t.ErrorMessage = errorMsg.String
	}
	if heartbeatAt.Valid {
		t.HeartbeatAt = heartbeatAt.Int64
	}
	if completedAt.Valid {
		t.CompletedAt = completedAt.Int64
	}
	return t, nil
}

func (db *DB) ListTasks(projectID, status, assignee, search, source string, limit, offset int) ([]models.Task, int, error) {
	query := `SELECT id, project_id, title, description, status, priority, assignee, source, task_mode, error_message, heartbeat_at, created_at, updated_at, completed_at FROM tasks WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM tasks WHERE 1=1`
	args := []interface{}{}
	countArgs := []interface{}{}
	appendArg := func(v string) {
		args = append(args, v)
		countArgs = append(countArgs, v)
	}
	if projectID != "" {
		query += " AND project_id = ?"
		countQuery += " AND project_id = ?"
		appendArg(projectID)
	}
	if status != "" {
		query += " AND status = ?"
		countQuery += " AND status = ?"
		appendArg(status)
	}
	if assignee != "" {
		query += " AND (assignee = ? OR assignee LIKE ? OR assignee LIKE ? OR assignee LIKE ?)"
		countQuery += " AND (assignee = ? OR assignee LIKE ? OR assignee LIKE ? OR assignee LIKE ?)"
		appendArg(assignee)
		appendArg(assignee + ",%")
		appendArg("%," + assignee + ",%")
		appendArg("%," + assignee)
	}
	if search != "" {
		query += " AND title LIKE ?"
		countQuery += " AND title LIKE ?"
		appendArg("%" + search + "%")
	}
	if source != "" {
		query += " AND source = ?"
		countQuery += " AND source = ?"
		appendArg(source)
	}

	var total int
	if err := db.conn.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		var completedAt, heartbeatAt sql.NullInt64
		var errorMsg sql.NullString
		var descStr sql.NullString
		var taskMode sql.NullString
		var assigneesStr string
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &descStr, &t.Status, &t.Priority, &assigneesStr, &t.Source, &taskMode, &errorMsg, &t.CreatedAt, &t.UpdatedAt, &heartbeatAt, &completedAt); err != nil {
			return nil, 0, err
		}
		t.Assignees = models.ParseAssignees(assigneesStr)
		if descStr.Valid && descStr.String != "" {
			var parsed map[string]string
			if err := json.Unmarshal([]byte(descStr.String), &parsed); err == nil {
				t.Description = parsed
			} else {
				t.Description = map[string]string{"en": descStr.String}
			}
		}
		if taskMode.Valid && taskMode.String != "" {
			t.TaskMode = models.TaskMode(taskMode.String)
		}
		if errorMsg.Valid {
			t.ErrorMessage = errorMsg.String
		}
		if heartbeatAt.Valid {
			t.HeartbeatAt = heartbeatAt.Int64
		}
		if completedAt.Valid {
			t.CompletedAt = completedAt.Int64
		}
		tasks = append(tasks, t)
	}
	return tasks, total, rows.Err()
}

func (db *DB) UpdateTask(t *models.Task) error {
	t.UpdatedAt = models.Now()
	_, err := db.conn.Exec(
		`UPDATE tasks SET project_id = ?, title = ?, description = ?, status = ?, priority = ?, assignee = ?, source = ?, task_mode = ?, error_message = ?, heartbeat_at = ?, updated_at = ?, completed_at = ? WHERE id = ?`,
		t.ProjectID, t.Title, models.SerializeDescription(t.Description), t.Status, t.Priority, models.SerializeAssignees(t.Assignees), t.Source, t.TaskMode, t.ErrorMessage, t.HeartbeatAt, t.UpdatedAt, t.CompletedAt, t.ID,
	)
	return err
}

func (db *DB) UpdateTaskStatus(id string, status models.TaskStatus, errorMsg string) error {
	now := models.Now()
	var completedAt interface{} = 0
	if status == models.TaskStatusCompleted || status == models.TaskStatusFailed || status == models.TaskStatusCancelled {
		completedAt = now
	}
	_, err := db.conn.Exec(
		`UPDATE tasks SET status = ?, error_message = ?, updated_at = ?, completed_at = ? WHERE id = ?`,
		status, errorMsg, now, completedAt, id,
	)
	return err
}

func (db *DB) HeartbeatTask(id string) error {
	_, err := db.conn.Exec(`UPDATE tasks SET heartbeat_at = ?, updated_at = ? WHERE id = ?`, models.Now(), models.Now(), id)
	return err
}

func (db *DB) DeleteTask(id string) error {
	_, err := db.conn.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

func (db *DB) ListAgents() ([]string, error) {
	rows, err := db.conn.Query(`SELECT DISTINCT assignee FROM tasks WHERE assignee != '' ORDER BY assignee`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[string]bool{}
	var agents []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		for _, name := range models.ParseAssignees(a) {
			if !seen[name] {
				seen[name] = true
				agents = append(agents, name)
			}
		}
	}
	return agents, rows.Err()
}

// --- Notifications ---

func (db *DB) CreateNotification(n *models.Notification) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	n.CreatedAt = models.Now()
	_, err := db.conn.Exec(
		`INSERT INTO notifications (id, task_id, type, message, read, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		n.ID, n.TaskID, n.Type, n.Message, boolToInt(n.Read), n.CreatedAt,
	)
	return err
}

func (db *DB) ListNotifications(limit int) ([]models.Notification, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.conn.Query(
		`SELECT id, task_id, type, message, read, created_at FROM notifications ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifs []models.Notification
	for rows.Next() {
		var n models.Notification
		var read int
		if err := rows.Scan(&n.ID, &n.TaskID, &n.Type, &n.Message, &read, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.Read = read == 1
		notifs = append(notifs, n)
	}
	return notifs, rows.Err()
}

func (db *DB) MarkNotificationRead(id string) error {
	_, err := db.conn.Exec(`UPDATE notifications SET read = 1 WHERE id = ?`, id)
	return err
}

func (db *DB) MarkAllNotificationsRead() error {
	_, err := db.conn.Exec(`UPDATE notifications SET read = 1 WHERE read = 0`)
	return err
}

func (db *DB) ClearNotifications() error {
	_, err := db.conn.Exec(`DELETE FROM notifications`)
	return err
}

func (db *DB) GetUnreadNotificationCount() (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM notifications WHERE read = 0`).Scan(&count)
	return count, err
}

// --- Webhooks ---

func (db *DB) CreateWebhook(w *models.WebhookConfig) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	w.CreatedAt = models.Now()
	_, err := db.conn.Exec(
		`INSERT INTO webhook_configs (id, url, events, active, created_at) VALUES (?, ?, ?, ?, ?)`,
		w.ID, w.URL, w.Events, boolToInt(w.Active), w.CreatedAt,
	)
	return err
}

func (db *DB) ListWebhooks() ([]models.WebhookConfig, error) {
	rows, err := db.conn.Query(`SELECT id, url, events, active, created_at FROM webhook_configs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []models.WebhookConfig
	for rows.Next() {
		var w models.WebhookConfig
		var active int
		if err := rows.Scan(&w.ID, &w.URL, &w.Events, &active, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.Active = active == 1
		webhooks = append(webhooks, w)
	}
	return webhooks, rows.Err()
}

func (db *DB) DeleteWebhook(id string) error {
	_, err := db.conn.Exec(`DELETE FROM webhook_configs WHERE id = ?`, id)
	return err
}

// --- Notification Configs ---

func (db *DB) GetNotificationConfig(notifType string) (*models.NotificationConfig, error) {
	c := &models.NotificationConfig{}
	var configJSON string
	var enabled int
	err := db.conn.QueryRow(
		`SELECT id, type, enabled, config_json, created_at, updated_at FROM notification_configs WHERE type = ?`, notifType,
	).Scan(&c.ID, &c.Type, &enabled, &configJSON, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Enabled = enabled == 1
	if configJSON != "" {
		json.Unmarshal([]byte(configJSON), &c.Config)
	}
	return c, nil
}

func (db *DB) ListNotificationConfigs() ([]models.NotificationConfig, error) {
	rows, err := db.conn.Query(`SELECT id, type, enabled, config_json, created_at, updated_at FROM notification_configs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []models.NotificationConfig
	for rows.Next() {
		var c models.NotificationConfig
		var enabled int
		var configJSON string
		if err := rows.Scan(&c.ID, &c.Type, &enabled, &configJSON, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Enabled = enabled == 1
		if configJSON != "" {
			json.Unmarshal([]byte(configJSON), &c.Config)
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func (db *DB) UpsertNotificationConfig(c *models.NotificationConfig) error {
	configJSON, _ := json.Marshal(c.Config)
	now := models.Now()
	_, err := db.conn.Exec(
		`INSERT INTO notification_configs (id, type, enabled, config_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(type) DO UPDATE SET enabled = excluded.enabled, config_json = excluded.config_json, updated_at = excluded.updated_at`,
		c.ID, c.Type, boolToInt(c.Enabled), string(configJSON), c.CreatedAt, now,
	)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (db *DB) ExportAll() (map[string]interface{}, error) {
	projects, err := db.ListProjects()
	if err != nil {
		return nil, err
	}
	tasks, _, err := db.ListTasks("", "", "", "", "", 0, 0)
	if err != nil {
		return nil, err
	}
	notifications, err := db.ListNotifications(1000)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"projects":      projects,
		"tasks":         tasks,
		"notifications": notifications,
		"exported_at":    models.Now(),
	}, nil
}

// --- Columns ---

func (db *DB) ListColumns() ([]models.TaskColumn, error) {
	rows, err := db.conn.Query(
		`SELECT id, key, label, color, position FROM task_columns ORDER BY position`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []models.TaskColumn
	for rows.Next() {
		var c models.TaskColumn
		if err := rows.Scan(&c.ID, &c.Key, &c.Label, &c.Color, &c.Position); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func (db *DB) CreateColumn(c *models.TaskColumn) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	_, err := db.conn.Exec(
		`INSERT INTO task_columns (id, key, label, color, position, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Key, c.Label, c.Color, c.Position, models.Now(),
	)
	return err
}

func (db *DB) UpdateColumn(c *models.TaskColumn) error {
	_, err := db.conn.Exec(
		`UPDATE task_columns SET label = ?, color = ?, position = ? WHERE id = ?`,
		c.Label, c.Color, c.Position, c.ID,
	)
	return err
}

func (db *DB) DeleteColumn(id string) error {
	_, err := db.conn.Exec(`DELETE FROM task_columns WHERE id = ?`, id)
	return err
}
