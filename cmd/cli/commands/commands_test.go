package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfig_EnvOverrides(t *testing.T) {
	origURL := os.Getenv("TASKS_WATCHER_SERVER_URL")
	origKey := os.Getenv("TASKS_WATCHER_API_KEY")
	defer func() {
		os.Setenv("TASKS_WATCHER_SERVER_URL", origURL)
		os.Setenv("TASKS_WATCHER_API_KEY", origKey)
	}()

	os.Setenv("TASKS_WATCHER_SERVER_URL", "http://custom:9999")
	os.Setenv("TASKS_WATCHER_API_KEY", "test-key-123")
	serverURL = "http://localhost:4242"
	apiKey = ""

	url, key := resolveConfig()
	if url != "http://custom:9999" {
		t.Errorf("expected http://custom:9999, got %s", url)
	}
	if key != "test-key-123" {
		t.Errorf("expected test-key-123, got %s", key)
	}
}

func TestResolveConfig_DefaultURL(t *testing.T) {
	origURL := os.Getenv("TASKS_WATCHER_SERVER_URL")
	origKey := os.Getenv("TASKS_WATCHER_API_KEY")
	defer func() {
		os.Setenv("TASKS_WATCHER_SERVER_URL", origURL)
		os.Setenv("TASKS_WATCHER_API_KEY", origKey)
	}()

	os.Unsetenv("TASKS_WATCHER_SERVER_URL")
	os.Setenv("TASKS_WATCHER_API_KEY", "key-from-env")
	serverURL = "http://localhost:4242"
	apiKey = ""

	url, key := resolveConfig()
	if url != "http://localhost:4242" {
		t.Errorf("expected default localhost:4242, got %s", url)
	}
	if key != "key-from-env" {
		t.Errorf("expected key-from-env, got %s", key)
	}
}

func TestResolveConfig_EnvTakesPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, ".tasks-watcher", "api.key")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("file-key"), 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpDir)
	os.Setenv("TASKS_WATCHER_API_KEY", "env-key")
	serverURL = ""
	apiKey = ""

	_, key := resolveConfig()
	if key != "env-key" {
		t.Errorf("expected env key to take precedence, got %s", key)
	}
}

// --- Cobra command parsing / validation tests ---

func TestTaskCreate_MissingTitle(t *testing.T) {
	cmd := taskCreateCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing title")
	}
}

func TestTaskCreate_WithProjectFlag_SkipsGitDetection(t *testing.T) {
	// With --project set, resolveProjectFromGit() is never called.
	// Only POST /api/tasks should be called.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"test-id","title":"My Task","status":"pending"}`))
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "test-key")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskCreateCmd()
	cmd.SetArgs([]string{"--title", "My Task", "--project", "myproj"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskCreate_WithAllFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"id","title":"Task"}`))
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskCreateCmd()
	cmd.SetArgs([]string{
		"--title", "Task",
		"--project", "proj",
		"--description", "A description",
		"--priority", "high",
		"--assignee", "me",
		"--task-mode", "sequential",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskUpdate_NoFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call API when no fields provided")
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskUpdateCmd()
	cmd.SetArgs([]string{"task-id-123"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for no update fields")
	}
}

func TestTaskUpdate_WithFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/my-id" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"my-id","title":"Updated"}`))
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskUpdateCmd()
	cmd.SetArgs([]string{"my-id", "--title", "Updated"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskUpdate_MissingTaskID(t *testing.T) {
	cmd := taskUpdateCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing task-id")
	}
}

func TestTaskList_Defaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskListCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskList_WithFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("status"); got != "pending" {
			t.Errorf("expected status=pending, got %s", got)
		}
		if got := r.URL.Query().Get("assignee"); got != "me" {
			t.Errorf("expected assignee=me, got %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskListCmd()
	cmd.SetArgs([]string{"--status", "pending", "--assignee", "me"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskShow_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/api/tasks/show-id" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"show-id","title":"Show Task","status":"pending"}`))
			return
		}
		if path == "/api/tasks/show-id/subtasks" || path == "/api/tasks/show-id/dependencies" || path == "/api/tasks/show-id/dependents" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"subtasks":[]}`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, path)
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskShowCmd()
	cmd.SetArgs([]string{"show-id"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskDelete_Success(t *testing.T) {
	var deletedID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deletedID = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskDeleteCmd()
	cmd.SetArgs([]string{"del-id"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deletedID != "/api/tasks/del-id" {
		t.Errorf("expected delete /api/tasks/del-id, got %s", deletedID)
	}
}

func TestTaskStart_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/start-id/status" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"start-id","status":"in_progress"}`))
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskStartCmd()
	cmd.SetArgs([]string{"start-id"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskComplete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"done-id","status":"completed"}`))
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskCompleteCmd()
	cmd.SetArgs([]string{"done-id"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskFail_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"fail-id","status":"failed"}`))
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskFailCmd()
	cmd.SetArgs([]string{"fail-id", "-r", "network timeout"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskCancel_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"cancel-id","status":"cancelled"}`))
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskCancelCmd()
	cmd.SetArgs([]string{"cancel-id"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskHeartbeat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/hb-id/heartbeat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	cmd := taskHeartbeatCmd()
	cmd.SetArgs([]string{"hb-id"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAPIRequest_ConnectionError(t *testing.T) {
	os.Setenv("TASKS_WATCHER_SERVER_URL", "http://localhost:59999")
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	_, err := apiRequest("GET", "/api/tasks", nil)
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestAPIRequest_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	os.Setenv("TASKS_WATCHER_SERVER_URL", server.URL)
	os.Setenv("TASKS_WATCHER_API_KEY", "k")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		os.Unsetenv("TASKS_WATCHER_API_KEY")
	}()
	serverURL = ""
	apiKey = ""

	_, err := apiRequest("GET", "/api/tasks", nil)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}
