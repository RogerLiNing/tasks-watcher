package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

func TestProjectCreate_WithDescriptionAndRepoPath(t *testing.T) {
	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
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
	cmd.SetArgs([]string{"--name", "my-proj", "--description", "A test project", "--repo-path", "/home/user/repos/my-proj"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if reqBody["name"] != "my-proj" {
		t.Errorf("expected name 'my-proj', got %v", reqBody["name"])
	}
	if reqBody["description"] != "A test project" {
		t.Errorf("expected description, got %v", reqBody["description"])
	}
	if reqBody["repo_path"] != "/home/user/repos/my-proj" {
		t.Errorf("expected repo_path, got %v", reqBody["repo_path"])
	}
}

func TestProjectList_WithProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"projects":[{"id":"p1","name":"Alpha"},{"id":"p2","name":"Beta"}]}`))
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

func TestAgentsOverview_WithCursorAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents/overview" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"agents":[{"name":"cursor","active_tasks":0,"pending_tasks":1,"completed_tasks":3,"failed_tasks":0,"total_tasks":4}]}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
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

func TestAgentsOverview_TasksAPIError(t *testing.T) {
	// tasks API returns error, but agents API succeeds — should still print overview
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents/overview" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"agents":[{"name":"claude-code","active_tasks":0,"pending_tasks":0,"completed_tasks":1,"failed_tasks":0,"total_tasks":1}]}`))
			return
		}
		// tasks API fails — agents overview should still print
		w.WriteHeader(http.StatusInternalServerError)
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

func TestAgentsOverview_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when agents API fails")
	}
}

func TestAgentsOverview_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json{`))
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
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when agents API returns invalid JSON")
	}
}

func TestAgentsOverview_WithManualAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents/overview" {
			w.Write([]byte(`{"agents":[{"name":"manual","active_tasks":0,"pending_tasks":1,"completed_tasks":2,"failed_tasks":0,"total_tasks":3}]}`))
			return
		}
		if r.URL.Path == "/api/tasks" {
			w.Write([]byte(`{"tasks":[]}`))
			return
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

	cmd := agentsOverviewCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentsOverview_LongTaskTitle(t *testing.T) {
	// Title > 45 chars should be truncated to 42 + "..."
	longTitle := strings.Repeat("A", 50)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents/overview" {
			w.Write([]byte(`{"agents":[{"name":"claude-code","active_tasks":1,"pending_tasks":0,"completed_tasks":0,"failed_tasks":0,"total_tasks":1}]}`))
			return
		}
		if r.URL.Path == "/api/tasks" {
			w.Write([]byte(fmt.Sprintf(`{"tasks":[{"assignee":"claude-code","title":%q,"status":"in_progress","updated_at":1700000000}]}`, longTitle)))
			return
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

	// Capture stdout to verify truncation output
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := agentsOverviewCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	var output string
	buf := make([]byte, 4096)
	for {
		n, _ := r.Read(buf)
		if n == 0 {
			break
		}
		output += string(buf[:n])
	}
	r.Close()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Title > 45 chars → truncated to 42 + "..." (50 A's should not appear intact)
	if strings.Contains(output, strings.Repeat("A", 50)) {
		t.Error("expected long title to be truncated")
	}
}

func TestDetectGitRepo_InGitDirectory(t *testing.T) {
	// Create a temp directory that IS a git repo
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Skipf("cannot create .git dir: %v", err)
	}

	oldCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot chdir to temp dir: %v", err)
	}
	defer os.Chdir(oldCwd)

	// detectGitRepo should find the .git directory and return the absolute path
	result := detectGitRepo()
	if result == "" {
		t.Error("expected detectGitRepo to find .git directory")
	}
	// The result should be the temp directory (absolute path)
	if !strings.HasPrefix(result, "/") {
		t.Errorf("expected absolute path, got %q", result)
	}
}

func TestDetectGitRepo_NonGitDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	oldCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot chdir to temp dir: %v", err)
	}
	defer os.Chdir(oldCwd)

	result := detectGitRepo()
	if result != "" {
		t.Errorf("expected empty result for non-git directory, got %q", result)
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

func TestTaskShow_WithFullDetails(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tasks/show-full":
			w.Write([]byte(`{"id":"show-full","title":"My Long Task Title","status":"in_progress","priority":"high","assignee":"bob","source":"cli","task_mode":"sequential","description":"This is a very long description that exceeds two hundred characters to test the truncation logic in the taskShowCmd function which cuts off at 200 chars","error_message":"Previous attempt failed"}`))
		case "/api/tasks/show-full/subtasks":
			w.Write([]byte(`{"subtasks":[{"id":"c1","title":"Child 1","status":"completed","position":1},{"id":"c2","title":"Child 2","status":"pending","position":2}]}`))
		case "/api/tasks/show-full/dependencies":
			w.Write([]byte(`{"blockers":[{"id":"b1","title":"Blocker","status":"completed"}]}`))
		case "/api/tasks/show-full/dependents":
			w.Write([]byte(`{"dependents":[{"id":"d1","title":"Dependent","status":"in_progress"}]}`))
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

	cmd := taskShowCmd()
	cmd.SetArgs([]string{"show-full"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify all expected endpoints were called
	expected := []string{
		"/api/tasks/show-full",
		"/api/tasks/show-full/subtasks",
		"/api/tasks/show-full/dependencies",
		"/api/tasks/show-full/dependents",
	}
	for _, p := range expected {
		found := false
		for _, called := range paths {
			if called == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected server call to %s, got: %v", p, paths)
		}
	}
}

func TestTaskShow_WithMinimalTask(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tasks/show-min":
			w.Write([]byte(`{"id":"show-min","title":"Min Task","status":"pending","priority":"medium"}`))
		case "/api/tasks/show-min/subtasks":
			w.Write([]byte(`{"subtasks":[]}`))
		case "/api/tasks/show-min/dependencies":
			w.Write([]byte(`{"blockers":[]}`))
		case "/api/tasks/show-min/dependents":
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

	cmd := taskShowCmd()
	cmd.SetArgs([]string{"show-min"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 4 {
		t.Errorf("expected 4 API calls, got %d: %v", len(paths), paths)
	}
}

func TestTaskCreate_WithTaskMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["task_mode"] != "sequential" {
			t.Errorf("expected task_mode=sequential, got: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"task-mode-123","title":"Sequential Task","status":"pending","priority":"medium"}`))
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
	cmd.SetArgs([]string{"--title", "Sequential Task", "--task-mode", "sequential", "--project", "test-proj"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskUpdate_WithAssigneeAndTaskMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/update-cli-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["assignee"] != "charlie" {
			t.Errorf("expected assignee=charlie, got: %v", body)
		}
		if body["task_mode"] != "parallel" {
			t.Errorf("expected task_mode=parallel, got: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"update-cli-123","title":"Updated","status":"pending","priority":"medium"}`))
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
	cmd.SetArgs([]string{"update-cli-123", "--assignee", "charlie", "--task-mode", "parallel"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubtaskCreate_WithAllFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/par-full/subtasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["description"] != "a description" {
			t.Errorf("expected description, got: %v", body)
		}
		if body["priority"] != "urgent" {
			t.Errorf("expected priority=urgent, got: %v", body)
		}
		if body["assignee"] != "dave" {
			t.Errorf("expected assignee=dave, got: %v", body)
		}
		if body["position"] != float64(2) {
			t.Errorf("expected position=2, got: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task":{"id":"full-child-123","title":"Full Child","status":"pending"}}`))
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
	cmd.SetArgs([]string{
		"--task-id", "par-full",
		"--title", "Full Child",
		"--description", "a description",
		"--priority", "urgent",
		"--assignee", "dave",
		"--position", "2",
	})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubtaskList_WithSubtasks(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/tasks/par-list/subtasks" {
			w.Write([]byte(`{"subtasks":[{"id":"c1","title":"First","status":"in_progress","position":1},{"id":"c2","title":"Second","status":"pending","position":2}]}`))
		} else {
			w.Write([]byte(`{"id":"par-list","title":"Parent Task"}`))
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
	cmd.SetArgs([]string{"--task-id", "par-list"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 API calls, got %d: %v", len(paths), paths)
	}
}

func TestSubtaskLink_WithPosition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["position"] != float64(1) {
			t.Errorf("expected position=1, got: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task":{"id":"linked-child-123","title":"Linked","status":"pending"}}`))
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
	cmd.SetArgs([]string{"--task-id", "parent-link", "--add", "child-link", "--position", "1"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubtaskReorder_InvalidPosition(t *testing.T) {
	serverURL = "http://localhost:59999"
	apiKey = "k"
	cmd := taskSubtaskReorderCmd()
	cmd.SetArgs([]string{"--task-id", "any", "--child-id", "any", "--position", "0"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for position < 1")
	}
}

func TestDepList_WithBlockersAndDependents(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/dependencies") {
			w.Write([]byte(`{"blockers":[{"id":"b1","title":"Blocker 1","status":"in_progress"},{"id":"b2","title":"Blocker 2","status":"pending"}]}`))
		} else {
			w.Write([]byte(`{"dependents":[{"id":"d1","title":"Dependent 1","status":"completed"}]}`))
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
	cmd.SetArgs([]string{"--task-id", "dep-full"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 API calls, got %d: %v", len(paths), paths)
	}
}

func TestDepAdd_Success_ShowsBlockerID(t *testing.T) {
	var reqBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"blocker_id":"blocker-abc"}`))
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
	cmd.SetArgs([]string{"--task-id", "task-abc", "--on", "blocker-abc"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqBody["blocker_id"] != "blocker-abc" {
		t.Errorf("expected blocker_id=blocker-abc, got: %v", reqBody)
	}
}

func TestTaskCreate_NoProjectFlag_TriggersGitDetection(t *testing.T) {
	// Without --project flag, resolveProjectFromGit() is called.
	// Create a temp non-git dir and chdir to it so detectGitRepo returns "".
	tmpDir := t.TempDir()
	oldCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot chdir to temp dir: %v", err)
	}
	defer os.Chdir(oldCwd)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		rawBody, _ := io.ReadAll(r.Body)
		json.Unmarshal(rawBody, &body)
		// project_name should not be present without --project flag
		if _, ok := body["project_name"].(string); ok {
			t.Errorf("expected no project_name without --project flag")
		}
		// priority should be "high" from --priority flag
		if pri, ok := body["priority"].(string); !ok || pri != "high" {
			t.Errorf("expected priority=high, got body=%s", rawBody)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"git-detect-123","title":"Git Task","status":"pending","priority":"high"}`))
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
	cmd.SetArgs([]string{"--title", "Git Task", "--priority", "high"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDepAdd_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for server error response")
	}
}

func TestDepAdd_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
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
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestDepRemove_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
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
	cmd.SetArgs([]string{"--task-id", "task-1", "--remove", "blocker-1"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for server error response")
	}
}

func TestProjectCreate_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
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
	cmd.SetArgs([]string{"--name", "test-proj"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestProjectUpdate_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
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
	cmd.SetArgs([]string{"proj-12345678", "--name", "Updated"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestProjectDelete_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
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
	cmd.SetArgs([]string{"proj-12345678"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for server error response")
	}
}

func TestDepCheck_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
	cmd.SetArgs([]string{"--task-id", "task-12345678"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for server error response")
	}
}

func TestDepCheck_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
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
	cmd.SetArgs([]string{"--task-id", "task-12345678"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestDepList_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
	cmd.SetArgs([]string{"--task-id", "task-12345678"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for server error response")
	}
}

func TestTaskDelete_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
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
	cmd.SetArgs([]string{"task-del-12345678"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for server error response")
	}
}

func TestTaskDelete_LongID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	// Task ID longer than 8 chars triggers the id = id[:8] branch
	cmd.SetArgs([]string{"verylongtaskid123"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubtaskCreate_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
	cmd.SetArgs([]string{"--task-id", "parent-12345678", "--title", "child"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for server error response")
	}
}

func TestSubtaskList_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
	cmd.SetArgs([]string{"--task-id", "parent-12345678"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for server error response")
	}
}

func TestSubtaskRemove_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
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
	cmd.SetArgs([]string{"--task-id", "parent-12345678", "--remove", "child-12345678"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for server error response")
	}
}

func TestSubtaskReorder_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
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
	cmd.SetArgs([]string{"--task-id", "parent-12345678", "--child-id", "child-12345678", "--position", "2"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for server error response")
	}
}

func TestTaskCreate_GitDetectionError(t *testing.T) {
	// In a non-git directory, detectGitRepo returns "", so resolveProjectFromGit
	// returns ("","",nil) with no project_id set. The project creation still succeeds.
	tmpDir := t.TempDir()
	oldCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot chdir to temp dir: %v", err)
	}
	defer os.Chdir(oldCwd)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["project_name"].(string); ok {
			t.Error("expected no project_name in non-git dir")
		}
		if body["assignee"] != "bob" {
			t.Errorf("expected assignee=bob, got: %v", body["assignee"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"git-no-proj-123","title":"Git Task","status":"pending","priority":"medium"}`))
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
	cmd.SetArgs([]string{"--title", "Git Task", "--assignee", "bob"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskCreate_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
	cmd.SetArgs([]string{"--title", "Fail Task", "--project", "proj"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for server error response")
	}
}

func TestTaskCreate_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
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
	cmd.SetArgs([]string{"--title", "Bad Task", "--project", "proj"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestTaskStart_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
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
	cmd.SetArgs([]string{"task-start-123"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestTaskComplete_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
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
	cmd.SetArgs([]string{"task-complete-123"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestTaskFail_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
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
	cmd.SetArgs([]string{"task-fail-123"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestTaskCancel_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
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
	cmd.SetArgs([]string{"task-cancel-123"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestResolveProjectFromGit_APIError(t *testing.T) {
	// Stay in git directory so detectGitRepo returns a path.
	// Make /api/projects/by-repo return 500, triggering the resp.StatusCode >= 400 error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects/by-repo" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"simulated"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"proj-123","title":"Task","status":"pending"}`))
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
	cmd.SetArgs([]string{"--title", "Git Error Task"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when resolveProjectFromGit hits API error")
	}
}

func TestResolveProjectFromGit_InvalidJSON(t *testing.T) {
	// Stay in git directory so detectGitRepo returns a path.
	// Make /api/projects/by-repo return 200 with invalid JSON.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects/by-repo" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`not valid json`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"proj-456","title":"Task","status":"pending"}`))
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
	cmd.SetArgs([]string{"--title", "Git Invalid Task"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when resolveProjectFromGit gets invalid JSON")
	}
}

func TestConfigShow_RunsSuccessfully(t *testing.T) {
	origURL := os.Getenv("TASKS_WATCHER_SERVER_URL")
	origKey := os.Getenv("TASKS_WATCHER_API_KEY")
	defer func() {
		if origURL != "" {
			os.Setenv("TASKS_WATCHER_SERVER_URL", origURL)
		}
		if origKey != "" {
			os.Setenv("TASKS_WATCHER_API_KEY", origKey)
		}
	}()
	os.Unsetenv("TASKS_WATCHER_SERVER_URL")
	os.Unsetenv("TASKS_WATCHER_API_KEY")
	serverURL = ""
	apiKey = ""

	cmd := ConfigCommand()
	cmd.SetArgs([]string{"show"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("config show failed: %v", err)
	}
}

func TestConfigApiKey_NoKey(t *testing.T) {
	origURL := os.Getenv("TASKS_WATCHER_SERVER_URL")
	origKey := os.Getenv("TASKS_WATCHER_API_KEY")
	defer func() {
		if origURL != "" {
			os.Setenv("TASKS_WATCHER_SERVER_URL", origURL)
		}
		if origKey != "" {
			os.Setenv("TASKS_WATCHER_API_KEY", origKey)
		}
	}()
	os.Unsetenv("TASKS_WATCHER_SERVER_URL")
	os.Unsetenv("TASKS_WATCHER_API_KEY")
	serverURL = ""
	apiKey = ""

	cmd := ConfigCommand()
	cmd.SetArgs([]string{"api-key"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
