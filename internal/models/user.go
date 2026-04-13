package models

// User represents an authenticated user.
type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"` // never expose hash in JSON
	CreatedAt    int64  `json:"created_at"`
}

// Session represents an active user session.
type Session struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	TokenHash string `json:"-"` // never expose token hash
	ExpiresAt int64  `json:"expires_at"`
	CreatedAt int64  `json:"created_at"`
}
