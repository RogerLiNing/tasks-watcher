package db

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

// CreateComment creates a new comment on a task.
func (db *DB) CreateComment(c *models.TaskComment) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	c.CreatedAt = models.Now()
	c.UpdatedAt = models.Now()
	_, err := db.conn.Exec(
		`INSERT INTO task_comments (id, task_id, author, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.TaskID, c.Author, c.Content, c.CreatedAt, c.UpdatedAt,
	)
	return err
}

// GetComment retrieves a single comment by ID.
func (db *DB) GetComment(id string) (*models.TaskComment, error) {
	c := &models.TaskComment{}
	err := db.conn.QueryRow(
		`SELECT id, task_id, author, content, created_at, updated_at FROM task_comments WHERE id = ?`, id,
	).Scan(&c.ID, &c.TaskID, &c.Author, &c.Content, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// ListComments returns all comments for a given task, ordered by creation time.
func (db *DB) ListComments(taskID string) ([]models.TaskComment, error) {
	rows, err := db.conn.Query(
		`SELECT id, task_id, author, content, created_at, updated_at FROM task_comments WHERE task_id = ? ORDER BY created_at ASC`, taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.TaskComment
	for rows.Next() {
		var c models.TaskComment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Author, &c.Content, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// UpdateComment updates the content of an existing comment.
func (db *DB) UpdateComment(c *models.TaskComment) error {
	c.UpdatedAt = models.Now()
	_, err := db.conn.Exec(
		`UPDATE task_comments SET author = ?, content = ?, updated_at = ? WHERE id = ?`,
		c.Author, c.Content, c.UpdatedAt, c.ID,
	)
	return err
}

// DeleteComment deletes a comment by ID.
func (db *DB) DeleteComment(id string) error {
	_, err := db.conn.Exec(`DELETE FROM task_comments WHERE id = ?`, id)
	return err
}
