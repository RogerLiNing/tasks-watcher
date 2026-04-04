package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// --- Project command tests ---

func setupTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
}

func TestProjectCreate_MissingName(t *testing.T) {
	cmd := projectCreateCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing name")
	}
}

func TestProjectCreate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"proj-id","name":"my-proj"}`))
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

	cmd := projectCreateCmd()
	cmd.SetArgs([]string{"--name", "my-proj"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProjectUpdate_NoFields(t *testing.T) {
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

	cmd := projectUpdateCmd()
	cmd.SetArgs([]string{"proj-id"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for no update fields")
	}
}

func TestProjectUpdate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/proj-id" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"proj-id","name":"updated"}`))
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

	cmd := projectUpdateCmd()
	cmd.SetArgs([]string{"proj-id", "--name", "updated"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProjectList_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"projects":[]}`))
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

	cmd := projectListCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProjectDelete_Success(t *testing.T) {
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

	cmd := projectDeleteCmd()
	cmd.SetArgs([]string{"proj-id-xxx"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deletedID != "/api/projects/proj-id-xxx" {
		t.Errorf("expected /api/projects/proj-id-xxx, got %s", deletedID)
	}
}

// --- Dependency command tests ---

func TestDepAdd_MissingTaskID(t *testing.T) {
	cmd := taskDepAddCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing task-id")
	}
}

func TestDepAdd_MissingBlockerID(t *testing.T) {
	cmd := taskDepAddCmd()
	cmd.SetArgs([]string{"--task-id", "task-1"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing blocker id")
	}
}

func TestDepAdd_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/task-1/dependencies" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"blocker_id":"blocker-1"}`))
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

	cmd := taskDepAddCmd()
	cmd.SetArgs([]string{"--task-id", "task-1", "--on", "blocker-1"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDepRemove_MissingTaskID(t *testing.T) {
	cmd := taskDepRemoveCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing task-id")
	}
}

func TestDepRemove_Success(t *testing.T) {
	var deletedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deletedPath = r.URL.Path
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

	cmd := taskDepRemoveCmd()
	cmd.SetArgs([]string{"--task-id", "t1", "--remove", "b1"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deletedPath != "/api/tasks/t1/dependencies/b1" {
		t.Errorf("expected /api/tasks/t1/dependencies/b1, got %s", deletedPath)
	}
}

func TestDepList_MissingTaskID(t *testing.T) {
	cmd := taskDepListCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing task-id")
	}
}

func TestDepList_Success(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/dependencies") {
			w.Write([]byte(`{"blockers":[]}`))
		} else {
			w.Write([]byte(`{"dependents":[]}`))
		}
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

	cmd := taskDepListCmd()
	cmd.SetArgs([]string{"--task-id", "task-id"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 API calls, got %d", len(paths))
	}
}

func TestDepCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/check-id/can-start" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"can_start":true,"blockers":[]}`))
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

	cmd := taskDepCheckCmd()
	cmd.SetArgs([]string{"--task-id", "check-id"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Subtask command tests ---

func TestSubtaskCreate_MissingTaskID(t *testing.T) {
	cmd := taskSubtaskCreateCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing task-id")
	}
}

func TestSubtaskCreate_MissingTitle(t *testing.T) {
	cmd := taskSubtaskCreateCmd()
	cmd.SetArgs([]string{"--task-id", "parent-id"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing title")
	}
}

func TestSubtaskCreate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/parent-id/subtasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task":{"id":"child-id","title":"child-task"}}`))
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

	cmd := taskSubtaskCreateCmd()
	cmd.SetArgs([]string{"--task-id", "parent-id", "--title", "child-task"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubtaskLink_MissingTaskID(t *testing.T) {
	cmd := taskSubtaskLinkCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing task-id")
	}
}

func TestSubtaskLink_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/parent-id/subtasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task":{"id":"child-id"}}`))
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

	cmd := taskSubtaskLinkCmd()
	cmd.SetArgs([]string{"--task-id", "parent-id", "--add", "child-id"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubtaskList_MissingTaskID(t *testing.T) {
	cmd := taskSubtaskListCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing task-id")
	}
}

func TestSubtaskList_Success(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/tasks/parent-id/subtasks" {
			w.Write([]byte(`{"subtasks":[]}`))
		} else {
			w.Write([]byte(`{"id":"parent-id","title":"Parent Task"}`))
		}
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

	cmd := taskSubtaskListCmd()
	cmd.SetArgs([]string{"--task-id", "parent-id"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 API calls, got %d: %v", len(paths), paths)
	}
}

func TestSubtaskRemove_MissingTaskID(t *testing.T) {
	cmd := taskSubtaskRemoveCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing task-id")
	}
}

func TestSubtaskRemove_Success(t *testing.T) {
	var removedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		removedPath = r.URL.Path
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

	cmd := taskSubtaskRemoveCmd()
	cmd.SetArgs([]string{"--task-id", "parent-id", "--remove", "child-id"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if removedPath != "/api/tasks/parent-id/subtasks/child-id" {
		t.Errorf("expected /api/tasks/parent-id/subtasks/child-id, got %s", removedPath)
	}
}

func TestSubtaskReorder_MissingTaskID(t *testing.T) {
	cmd := taskSubtaskReorderCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing task-id")
	}
}

func TestSubtaskReorder_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/parent-id/subtasks/child-id/position" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
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

	cmd := taskSubtaskReorderCmd()
	cmd.SetArgs([]string{"--task-id", "parent-id", "--child-id", "child-id", "--position", "2"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Agents command tests ---

func TestAgentsOverview_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents/overview" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"agents":[{"name":"claude-code","active_tasks":1,"pending_tasks":2,"completed_tasks":5,"failed_tasks":0,"total_tasks":8}]}`))
			return
		}
		if r.URL.Path == "/api/tasks" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tasks":[],"total":0}`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
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

	cmd := agentsOverviewCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentsOverview_NoAgents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"agents":[]}`))
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

	cmd := agentsOverviewCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		name string
		ts   int64
		want string
	}{
		{"zero", 0, "just now"},
		{"recent", time.Now().Unix(), "just now"},
		{"minutes", time.Now().Add(-5 * time.Minute).Unix(), "5m ago"},
		{"hours", time.Now().Add(-3 * time.Hour).Unix(), "3h ago"},
		{"days", time.Now().Add(-7 * 24 * time.Hour).Unix(), "7d ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeTime(tt.ts)
			if got != tt.want {
				t.Errorf("relativeTime(%d) = %q, want %q", tt.ts, got, tt.want)
			}
		})
	}
}

func TestAgentsOverview_WithActiveTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents/overview" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"agents":[{"name":"agent-1","active_tasks":2,"pending_tasks":1,"completed_tasks":10,"failed_tasks":1,"total_tasks":14}]}`))
			return
		}
		if r.URL.Path == "/api/tasks" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tasks":[{"assignee":"agent-1","title":"Fix bug","status":"in_progress"}]}`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
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

	cmd := agentsOverviewCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDepCheck_Blocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"can_start":false,"blockers":["other-task"],"child_titles":["child-a","child-b"],"blocked_by_sequential":true,"sequential_blocker":"first-child"}`))
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

	cmd := taskDepCheckCmd()
	cmd.SetArgs([]string{"--task-id", "task-id"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskList_EmptyTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":null}`))
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
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskList_WithTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[{"id":"t1","title":"Fix bug","status":"in_progress","priority":"high","assignee":"alice","task_mode":"parallel","source":"claude-code"}],"total":1}`))
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
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveConfig_FileKey(t *testing.T) {
	origURL := os.Getenv("TASKS_WATCHER_SERVER_URL")
	origKey := os.Getenv("TASKS_WATCHER_API_KEY")
	defer func() {
		os.Setenv("TASKS_WATCHER_SERVER_URL", origURL)
		os.Setenv("TASKS_WATCHER_API_KEY", origKey)
	}()

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
	os.Unsetenv("TASKS_WATCHER_SERVER_URL")
	os.Unsetenv("TASKS_WATCHER_API_KEY")
	serverURL = "http://localhost:4242"
	apiKey = ""

	url, key := resolveConfig()
	if url != "http://localhost:4242" {
		t.Errorf("expected default URL, got %s", url)
	}
	if key != "file-key" {
		t.Errorf("expected file key, got %s", key)
	}
}

func TestResolveConfig_BothMissing(t *testing.T) {
	origURL := os.Getenv("TASKS_WATCHER_SERVER_URL")
	origKey := os.Getenv("TASKS_WATCHER_API_KEY")
	defer func() {
		os.Setenv("TASKS_WATCHER_SERVER_URL", origURL)
		os.Setenv("TASKS_WATCHER_API_KEY", origKey)
	}()

	// Use a temp HOME with no key file so file reading yields no key.
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpDir)
	os.Unsetenv("TASKS_WATCHER_SERVER_URL")
	os.Unsetenv("TASKS_WATCHER_API_KEY")
	serverURL = ""
	apiKey = ""

	url, key := resolveConfig()
	if url != "" {
		t.Errorf("expected empty URL, got %s", url)
	}
	if key != "" {
		t.Errorf("expected empty key, got %s", key)
	}
}
