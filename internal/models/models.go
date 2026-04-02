package models

import (
	"time"
)

type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RepoPath    string `json:"repo_path"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

type Task struct {
	ID           string     `json:"id"`
	ProjectID    string     `json:"project_id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Status       TaskStatus `json:"status"`
	Priority     Priority   `json:"priority"`
	Assignee     string     `json:"assignee"`
	Source       string     `json:"source"`
	ErrorMessage string     `json:"error_message,omitempty"`
	HeartbeatAt  int64      `json:"heartbeat_at,omitempty"`
	CreatedAt    int64      `json:"created_at"`
	UpdatedAt    int64      `json:"updated_at"`
	CompletedAt  int64      `json:"completed_at,omitempty"`
}

type Notification struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Read      bool   `json:"read"`
	CreatedAt int64  `json:"created_at"`
}

type WebhookConfig struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Events    string `json:"events"`
	Active    bool   `json:"active"`
	CreatedAt int64  `json:"created_at"`
}

// Event types
const (
	EventTaskCreated   = "task.created"
	EventTaskStarted   = "task.started"
	EventTaskCompleted = "task.completed"
	EventTaskFailed    = "task.failed"
	EventTaskCancelled = "task.cancelled"
	EventTaskHeartbeat = "task.heartbeat"
	EventProjectCreated = "project.created"
	EventProjectUpdated = "project.updated"
	EventProjectDeleted = "project.deleted"
)

// SSEEvent is broadcast to all connected clients
type SSEEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	Time    int64       `json:"time"`
}

// NotificationConfig stores per-channel notification settings
type NotificationConfig struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // "macos" or "email"
	Enabled   bool                   `json:"enabled"`
	Config    map[string]interface{} `json:"config"`
	CreatedAt int64                 `json:"created_at"`
	UpdatedAt int64                 `json:"updated_at"`
}

// EmailConfig is the structure stored in config_json for email type
type EmailConfig struct {
	SMTPHost     string   `json:"smtp_host"`
	SMTPPort     int      `json:"smtp_port"`
	SMTPUsername string   `json:"smtp_username"`
	SMTPPassword string   `json:"smtp_password"`
	FromAddress  string   `json:"from_address"`
	ToAddresses  []string `json:"to_addresses"`
}

func Now() int64 {
	return time.Now().Unix()
}

func (t *Task) IsTerminal() bool {
	return t.Status == TaskStatusCompleted || t.Status == TaskStatusFailed || t.Status == TaskStatusCancelled
}

func ValidTaskStatus(s string) bool {
	switch TaskStatus(s) {
	case TaskStatusPending, TaskStatusInProgress, TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled:
		return true
	}
	return false
}

func ValidPriority(p string) bool {
	switch Priority(p) {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}
