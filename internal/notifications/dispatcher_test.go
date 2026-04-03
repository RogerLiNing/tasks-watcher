package notifications

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

func TestSendWebhooks_DeliversPayload(t *testing.T) {
	var receivedBody map[string]interface{}
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("X-Tasks-Watcher-Event") != "task.completed" {
			t.Errorf("expected X-Tasks-Watcher-Event: task.completed, got %s", r.Header.Get("X-Tasks-Watcher-Event"))
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	database := setupNotifierTestDB(t)
	defer database.Close()

	// Insert a test webhook pointing to our test server
	database.CreateWebhook(&models.WebhookConfig{
		URL:    server.URL,
		Events: "task.*",
		Active: true,
	})

	d := NewDispatcher(database, nil)
	task := &models.Task{ID: "task-1", Title: "Test task", Status: models.TaskStatusCompleted}
	d.sendWebhooks("task.completed", task)
	// sendWebhooks runs HTTP calls in goroutines; give them time to complete
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if receivedBody == nil {
		t.Error("expected webhook to receive a payload")
	} else if receivedBody["event"] != "task.completed" {
		t.Errorf("expected event task.completed, got %v", receivedBody["event"])
	}
}

// Verify sendWebhooks skips inactive webhooks
func TestSendWebhooks_SkipsInactive(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))
	defer server.Close()

	database := setupNotifierTestDB(t)
	defer database.Close()

	// Insert an inactive webhook — should be skipped
	database.CreateWebhook(&models.WebhookConfig{
		URL:    server.URL,
		Events: "task.*",
		Active: false, // inactive
	})

	d := NewDispatcher(database, nil)
	task := &models.Task{ID: "t", Title: "t"}
	d.sendWebhooks("task.completed", task)

	// Allow async goroutine to finish
	// Since sendWebhooks runs each webhook in a goroutine, give it a moment
	// Inactive webhook should be skipped
	if calls != 0 {
		t.Errorf("expected 0 calls with inactive webhook, got %d", calls)
	}
}

func TestSendWebhooks_MatchesEventFilter(t *testing.T) {
	// Test that task.* wildcard matches task.completed
	event := "task.completed"
	filter := "task.*"
	if !matchesEvent(event, filter) {
		t.Errorf("expected task.* to match task.completed")
	}
	if matchesEvent("project.created", filter) {
		t.Errorf("expected task.* to NOT match project.created")
	}
}

func TestSendWebhooks_MultipleFilters(t *testing.T) {
	cases := []struct {
		eventType string
		filter    string
		want      bool
	}{
		{"task.completed", "task.failed,task.completed", true},
		{"task.failed", "task.failed,task.completed", true},
		{"task.created", "task.failed,task.completed", false},
		{"task.started", "task.*", true},
		{"project.created", "task.*", false},
	}
	for _, tc := range cases {
		got := matchesEvent(tc.eventType, tc.filter)
		if got != tc.want {
			t.Errorf("matchesEvent(%q, %q) = %v, want %v", tc.eventType, tc.filter, got, tc.want)
		}
	}
}

func TestSendEmail_NoEmailConfigDoesNotPanic(t *testing.T) {
	database := setupNotifierTestDB(t)
	defer database.Close()
	d := NewDispatcher(database, nil)
	task := &models.Task{ID: "t", Title: "Test", Status: models.TaskStatusCompleted}
	d.sendEmail(task, "test message")
}

func TestSendEmail_DisabledConfigDoesNotPanic(t *testing.T) {
	database := setupNotifierTestDB(t)
	defer database.Close()
	database.UpsertNotificationConfig(&models.NotificationConfig{
		Type:    "email",
		Enabled: false,
		Config:  map[string]interface{}{"smtp_host": "smtp.example.com"},
	})
	d := NewDispatcher(database, nil)
	task := &models.Task{ID: "t", Title: "Test", Status: models.TaskStatusCompleted}
	d.sendEmail(task, "test message")
}

func TestMacosNotification_NonDarwinDoesNotPanic(t *testing.T) {
	d := &Dispatcher{db: nil, sse: nil}
	// Should not panic on non-darwin platforms
	d.macosNotification("body", "title")
}

func TestSendChannels_ShouldNotifyOS_Routes(t *testing.T) {
	cases := []struct {
		eventType string
		wantOS    bool
	}{
		{"task.started", true},
		{"task.completed", true},
		{"task.failed", true},
		{"task.created", false},
		{"task.cancelled", false},
		{"task.updated", false},
	}
	for _, tc := range cases {
		got := shouldNotifyOS(tc.eventType)
		if got != tc.wantOS {
			t.Errorf("shouldNotifyOS(%q) = %v, want %v", tc.eventType, got, tc.wantOS)
		}
	}
}

func setupNotifierTestDB(t *testing.T) *db.DB {
	origDir, _ := os.Getwd()
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("failed to chdir to project root %s: %v", projectRoot, err)
	}
	defer func() { os.Chdir(origDir) }()

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	return database
}

func TestNotify_SavesToDatabase(t *testing.T) {
	// Use nil dispatcher (SSE nil) with real DB
	db := setupNotifierTestDB(t)
	defer db.Close()

	d := NewDispatcher(db, nil)
	task := &models.Task{ID: "notif-task-1", Title: "Notify test task", Status: models.TaskStatusInProgress}

	// Call synchronously (not goroutine) by invoking the internal path
	d.Notify(models.EventTaskStarted, task)

	notifs, _ := db.ListNotifications(10)
	if len(notifs) == 0 {
		t.Fatal("expected at least one notification to be saved")
	}
	if !strings.Contains(notifs[0].Message, "Notify test task") {
		t.Errorf("unexpected message: %s", notifs[0].Message)
	}
}

func TestBuildMessage_AllEventTypes(t *testing.T) {
	task := &models.Task{
		Title:        "My Task",
		Status:       models.TaskStatusPending,
		ErrorMessage: "error details",
	}
	cases := []struct {
		eventType string
		substr    string
	}{
		{models.EventTaskCreated, "New task created: My Task"},
		{models.EventTaskStarted, "Task started: My Task"},
		{models.EventTaskCompleted, "Task completed: My Task"},
		{models.EventTaskFailed, "Task failed: My Task"},
		{models.EventTaskCancelled, "Task cancelled: My Task"},
		{"task.unknown", "Task updated: My Task"},
	}
	for _, tc := range cases {
		t.Run(tc.eventType, func(t *testing.T) {
			msg := buildMessage(tc.eventType, task)
			if !strings.Contains(msg, tc.substr) {
				t.Errorf("buildMessage(%q) = %q, want to contain %q", tc.eventType, msg, tc.substr)
			}
		})
	}
}

func TestBuildMessage_TaskFailedIncludesError(t *testing.T) {
	task := &models.Task{
		Title:        "Failing Task",
		Status:       models.TaskStatusFailed,
		ErrorMessage: "connection refused",
	}
	msg := buildMessage(models.EventTaskFailed, task)
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("expected message to include error details, got: %s", msg)
	}
}

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
