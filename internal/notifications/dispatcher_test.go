package notifications

import (
	"strings"
	"testing"

	"github.com/rogerrlee/tasks-watcher/internal/models"
)

func TestBuildMessage(t *testing.T) {
	cases := []struct {
		eventType string
		task     *models.Task
		want     string
	}{
		{models.EventTaskCreated, &models.Task{Title: "Fix bug", Status: models.TaskStatusPending}, "New task created: Fix bug"},
		{models.EventTaskStarted, &models.Task{Title: "My task", Status: models.TaskStatusInProgress}, "Task started: My task"},
		{models.EventTaskCompleted, &models.Task{Title: "Done task", Status: models.TaskStatusCompleted}, "Task completed: Done task"},
		{models.EventTaskFailed, &models.Task{Title: "Failing task", Status: models.TaskStatusFailed, ErrorMessage: "oops"}, "Task failed: Failing task — oops"},
		{models.EventTaskCancelled, &models.Task{Title: "Cancelled task", Status: models.TaskStatusCancelled}, "Task cancelled: Cancelled task"},
		{"task.unknown_event", &models.Task{Title: "Some task", Status: models.TaskStatusInProgress}, "Task updated: Some task"},
	}
	for _, tc := range cases {
		t.Run(tc.eventType, func(t *testing.T) {
			got := buildMessage(tc.eventType, tc.task)
			if got != tc.want {
				t.Errorf("buildMessage(%q, task{%q}) = %q, want %q",
					tc.eventType, tc.task.Title, got, tc.want)
			}
		})
	}
}

func TestShouldNotifyOS(t *testing.T) {
	cases := []struct {
		eventType string
		want      bool
	}{
		{models.EventTaskStarted, true},
		{models.EventTaskCompleted, true},
		{models.EventTaskFailed, true},
		{models.EventTaskCreated, false},
		{models.EventTaskCancelled, false},
		{models.EventTaskUpdated, false},
		{"project.created", false},
		{"column.created", false},
	}
	for _, tc := range cases {
		t.Run(tc.eventType, func(t *testing.T) {
			got := shouldNotifyOS(tc.eventType)
			if got != tc.want {
				t.Errorf("shouldNotifyOS(%q) = %v, want %v", tc.eventType, got, tc.want)
			}
		})
	}
}

func TestMatchesEvent(t *testing.T) {
	cases := []struct {
		eventType string
		filter    string
		want      bool
	}{
		// Wildcard
		{"task.created", "*", true},
		{"task.completed", "*", true},
		{"project.created", "*", true},
		// Task wildcard
		{"task.created", "task.*", true},
		{"task.failed", "task.*", true},
		{"project.created", "task.*", false},
		// Exact match
		{"task.completed", "task.completed", true},
		{"task.completed", "task.failed", false},
		// Multiple filters
		{"task.completed", "task.failed,task.completed", true},
		{"task.failed", "task.failed,task.completed", true},
		{"task.created", "task.failed,task.completed", false},
		// Empty
		{"task.created", "", true},
		// Whitespace
		{"task.completed", " task.completed , task.failed ", true},
		{"task.created", " task.failed ", false},
	}
	for _, tc := range cases {
		t.Run(tc.filter+"__"+tc.eventType, func(t *testing.T) {
			got := matchesEvent(tc.eventType, tc.filter)
			if got != tc.want {
				t.Errorf("matchesEvent(%q, %q) = %v, want %v",
					tc.eventType, tc.filter, got, tc.want)
			}
		})
	}
}

func TestParseEmailConfig(t *testing.T) {
	cases := []struct {
		name   string
		config map[string]interface{}
		check  func(t *testing.T, cfg models.EmailConfig)
	}{
		{
			name:   "empty",
			config: map[string]interface{}{},
			check: func(t *testing.T, cfg models.EmailConfig) {
				if cfg.SMTPHost != "" {
					t.Errorf("expected empty SMTPHost, got %q", cfg.SMTPHost)
				}
				if cfg.SMTPPort != 587 {
					t.Errorf("expected default port 587, got %d", cfg.SMTPPort)
				}
			},
		},
		{
			name: "full config",
			config: map[string]interface{}{
				"smtp_host":     "smtp.example.com",
				"smtp_port":     float64(465),
				"smtp_username": "user@example.com",
				"smtp_password": "secret",
				"from_address":  "sender@example.com",
				"to_addresses":  []interface{}{"a@x.com", "b@x.com"},
			},
			check: func(t *testing.T, cfg models.EmailConfig) {
				if cfg.SMTPHost != "smtp.example.com" {
					t.Errorf("SMTPHost: got %q", cfg.SMTPHost)
				}
				if cfg.SMTPPort != 465 {
					t.Errorf("SMTPPort: got %d", cfg.SMTPPort)
				}
				if cfg.SMTPUsername != "user@example.com" {
					t.Errorf("SMTPUsername: got %q", cfg.SMTPUsername)
				}
				if cfg.SMTPPassword != "secret" {
					t.Errorf("SMTPPassword: got %q", cfg.SMTPPassword)
				}
				if cfg.FromAddress != "sender@example.com" {
					t.Errorf("FromAddress: got %q", cfg.FromAddress)
				}
				if len(cfg.ToAddresses) != 2 {
					t.Errorf("ToAddresses: got %d, want 2", len(cfg.ToAddresses))
				}
			},
		},
		{
			name: "partial config missing port",
			config: map[string]interface{}{
				"smtp_host": "smtp.example.com",
			},
			check: func(t *testing.T, cfg models.EmailConfig) {
				if cfg.SMTPPort != 587 {
					t.Errorf("expected default port 587, got %d", cfg.SMTPPort)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := parseEmailConfig(tc.config)
			tc.check(t, cfg)
		})
	}
}

func TestBuildEmailMsg(t *testing.T) {
	msg := buildEmailMsg("from@test.com", "to@test.com", "Subject Line", "Body text")
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if !strings.Contains(msg, "From: from@test.com") {
		t.Error("missing From header")
	}
	if !strings.Contains(msg, "To: to@test.com") {
		t.Error("missing To header")
	}
	if !strings.Contains(msg, "Subject: Subject Line") {
		t.Error("missing Subject header")
	}
	if !strings.Contains(msg, "Body text") {
		t.Error("missing body")
	}
}
