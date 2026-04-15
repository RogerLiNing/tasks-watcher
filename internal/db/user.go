package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

// CreateUser creates a new user. Returns error if username already exists.
func (db *DB) CreateUser(u *models.User) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	u.CreatedAt = models.Now()
	_, err := db.conn.Exec(
		`INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		u.ID, u.Username, u.PasswordHash, u.CreatedAt,
	)
	return err
}

// GetUserByUsername retrieves a user by username.
func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	u := &models.User{}
	err := db.conn.QueryRow(
		`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// GetUserByID retrieves a user by ID.
func (db *DB) GetUserByID(id string) (*models.User, error) {
	u := &models.User{}
	err := db.conn.QueryRow(
		`SELECT id, username, password_hash, created_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// GetUsersByIDs retrieves multiple users by their IDs in a single query.
func (db *DB) GetUsersByIDs(ids []string) (map[string]*models.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(
		`SELECT id, username, password_hash, created_at FROM users WHERE id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*models.User)
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
			return nil, err
		}
		result[u.ID] = u
	}
	return result, rows.Err()
}

// CreateSession creates a new session.
func (db *DB) CreateSession(s *models.Session) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	s.CreatedAt = models.Now()
	_, err := db.conn.Exec(
		`INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		s.ID, s.UserID, s.TokenHash, s.ExpiresAt, s.CreatedAt,
	)
	return err
}

// GetSession retrieves a session by ID if not expired.
func (db *DB) GetSession(id string) (*models.Session, error) {
	s := &models.Session{}
	err := db.conn.QueryRow(
		`SELECT id, user_id, token_hash, expires_at, created_at FROM sessions WHERE id = ? AND expires_at > ?`, id, models.Now(),
	).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// DeleteSession deletes a session by ID.
func (db *DB) DeleteSession(id string) error {
	_, err := db.conn.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DenySession adds a token hash to the denylist.
func (db *DB) DenySession(tokenHash string) error {
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO session_denylist (token_hash, revoked_at) VALUES (?, ?)`, tokenHash, models.Now(),
	)
	return err
}

// IsSessionDenied checks if a token hash is in the denylist.
func (db *DB) IsSessionDenied(tokenHash string) bool {
	var exists int
	db.conn.QueryRow(`SELECT 1 FROM session_denylist WHERE token_hash = ?`, tokenHash).Scan(&exists)
	return exists == 1
}

// CleanExpiredSessions removes expired sessions.
func (db *DB) CleanExpiredSessions() error {
	_, err := db.conn.Exec(`DELETE FROM sessions WHERE expires_at < ?`, models.Now())
	return err
}

// ListUsers returns all users (for admin use).
func (db *DB) ListUsers() ([]models.User, error) {
	rows, err := db.conn.Query(`SELECT id, username, password_hash, created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
