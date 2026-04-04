package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStr(t *testing.T) {
	if got := str(nil, "default"); got != "default" {
		t.Errorf("str(nil) = %q, want default", got)
	}
	if got := str("  hello  ", ""); got != "hello" {
		t.Errorf("str(space) = %q, want trimmed", got)
	}
	if got := str("hello", ""); got != "hello" {
		t.Errorf("str(hello) = %q, want hello", got)
	}
	if got := str(123, "default"); got != "default" {
		t.Errorf("str(int) = %q, want default", got)
	}
}

func TestIntArg(t *testing.T) {
	if got := intArg(nil); got != 0 {
		t.Errorf("intArg(nil) = %d, want 0", got)
	}
	if got := intArg(float64(42)); got != 42 {
		t.Errorf("intArg(float64) = %d, want 42", got)
	}
	if got := intArg(int(10)); got != 10 {
		t.Errorf("intArg(int) = %d, want 10", got)
	}
	if got := intArg(int64(99)); got != 99 {
		t.Errorf("intArg(int64) = %d, want 99", got)
	}
	if got := intArg("not-a-number"); got != 0 {
		t.Errorf("intArg(string) = %d, want 0", got)
	}
}

func TestDetectGitRepo(t *testing.T) {
	// Create a temp dir and use it as cwd
	tmpDir := t.TempDir()
	oldCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	defer os.Chdir(oldCwd)

	// No .git in tmpDir → should return ""
	if got := detectGitRepo(); got != "" {
		t.Errorf("detectGitRepo() in non-git dir = %q, want empty", got)
	}

	// Create a .git directory to simulate a repo
	gitDir := tmpDir + "/.git"
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("cannot create .git dir: %v", err)
	}
	got := detectGitRepo()
	// On macOS /private/var/folders is a symlink to /var/folders; resolve both for comparison
	gotEval, _ := filepath.EvalSymlinks(got)
	tmpDirEval, _ := filepath.EvalSymlinks(tmpDir)
	if gotEval != tmpDirEval {
		t.Errorf("detectGitRepo() in git dir = %q, want %s", got, tmpDir)
	}
}

func newTestClient(serverURL string) *Client {
	return &Client{
		BaseURL:    serverURL,
		APIKey:     "test-key",
		HTTPClient: &http.Client{},
	}
}

func TestClient_TaskCreate_Success(t *testing.T) {
	taskID := "task-id-12345678"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/tasks":
			w.Write([]byte(`{"id":"task-id-12345678","title":"Test Task","status":"in_progress","priority":"medium"}`))
		case r.Method == "PATCH" && r.URL.Path == "/api/tasks/"+taskID+"/status":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := newTestClient(server.URL)

	result, err := c.TaskCreate(map[string]interface{}{"title": "Test Task", "project_name": "test-proj"})
	if err != nil {
		t.Fatalf("TaskCreate failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
}

func TestClient_TaskCreate_MissingTitle(t *testing.T) {
	c := &Client{}
	_, err := c.TaskCreate(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing title")
	}
}

func TestClient_TaskList_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	_, err := c.TaskList(map[string]interface{}{})
	if err != nil {
		t.Fatalf("TaskList failed: %v", err)
	}
}

func TestClient_TaskList_WithFilters(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path + "?" + r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	_, err := c.TaskList(map[string]interface{}{
		"project_id": "proj-1",
		"status":     "in_progress",
		"assignee":   "me",
		"search":     "auth",
	})
	if err != nil {
		t.Fatalf("TaskList failed: %v", err)
	}
	if receivedPath != "/api/tasks?project_id=proj-1&status=in_progress&assignee=me&search=auth&" {
		t.Errorf("unexpected query: %s", receivedPath)
	}
}

func TestClient_TaskShow_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tasks/show-id-12345678":
			w.Write([]byte(`{"id":"show-id-12345678","title":"Show Task","status":"pending","priority":"high","assignee":"me","task_mode":"sequential","created_at":12345}`))
		case "/api/tasks/show-id-12345678/subtasks":
			w.Write([]byte(`{"subtasks":[]}`))
		case "/api/tasks/show-id-12345678/dependencies":
			w.Write([]byte(`{"blockers":[]}`))
		case "/api/tasks/show-id-12345678/dependents":
			w.Write([]byte(`{"dependents":[]}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.TaskShow(map[string]interface{}{"task_id": "show-id-12345678"})
	if err != nil {
		t.Fatalf("TaskShow failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_TaskShow_MissingID(t *testing.T) {
	c := &Client{}
	_, err := c.TaskShow(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing task_id")
	}
}

func TestClient_TaskStart_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/start-id-12345678/status" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"start-id-12345678","title":"Task","status":"in_progress"}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.TaskStart(map[string]interface{}{"task_id": "start-id-12345678"})
	if err != nil {
		t.Fatalf("TaskStart failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_TaskComplete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"done-id-12345678","title":"Task","status":"completed"}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.TaskComplete(map[string]interface{}{"task_id": "done-id-12345678"})
	if err != nil {
		t.Fatalf("TaskComplete failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_TaskFail_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"fail-id-12345678","title":"Task","status":"failed"}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.TaskFail(map[string]interface{}{"task_id": "fail-id-12345678", "reason": "network error"})
	if err != nil {
		t.Fatalf("TaskFail failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_TaskUpdate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/up-id-12345678" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"up-id-12345678","title":"Updated","status":"pending","priority":"high"}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.TaskUpdate(map[string]interface{}{"task_id": "up-id-12345678", "title": "Updated", "priority": "high"})
	if err != nil {
		t.Fatalf("TaskUpdate failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_TaskUpdate_MissingID(t *testing.T) {
	c := &Client{}
	_, err := c.TaskUpdate(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing task_id")
	}
}

func TestClient_TaskUpdate_NoFields(t *testing.T) {
	c := &Client{}
	_, err := c.TaskUpdate(map[string]interface{}{"task_id": "any-id"})
	if err == nil {
		t.Error("expected error for no update fields")
	}
}

func TestClient_TaskCancel_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"canc-id-12345678","title":"Task","status":"cancelled"}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.TaskCancel(map[string]interface{}{"task_id": "canc-id-12345678"})
	if err != nil {
		t.Fatalf("TaskCancel failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_TaskDelete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/del-id-12345678" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.TaskDelete(map[string]interface{}{"task_id": "del-id-12345678"})
	if err != nil {
		t.Fatalf("TaskDelete failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_TaskDelete_MissingID(t *testing.T) {
	c := &Client{}
	_, err := c.TaskDelete(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing task_id")
	}
}

func TestClient_ProjectList_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"projects":[]}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	_, err := c.ProjectList(map[string]interface{}{})
	if err != nil {
		t.Fatalf("ProjectList failed: %v", err)
	}
}

func TestClient_ProjectCreate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"proj-id-12345678","name":"my-proj","description":"test"}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.ProjectCreate(map[string]interface{}{"name": "my-proj", "description": "test"})
	if err != nil {
		t.Fatalf("ProjectCreate failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_ProjectCreate_MissingName(t *testing.T) {
	c := &Client{}
	_, err := c.ProjectCreate(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestClient_ProjectUpdate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/proj-id-12345678" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"proj-id-12345678","name":"updated"}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.ProjectUpdate(map[string]interface{}{"project_id": "proj-id-12345678", "name": "updated"})
	if err != nil {
		t.Fatalf("ProjectUpdate failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_ProjectUpdate_MissingID(t *testing.T) {
	c := &Client{}
	_, err := c.ProjectUpdate(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing project_id")
	}
}

func TestClient_ProjectDelete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.ProjectDelete(map[string]interface{}{"project_id": "proj-id-12345678"})
	if err != nil {
		t.Fatalf("ProjectDelete failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_SubtaskCreate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task":{"id":"child-id-12345678","title":"child","status":"pending"}}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.SubtaskCreate(map[string]interface{}{"task_id": "parent-id-12345678", "title": "child"})
	if err != nil {
		t.Fatalf("SubtaskCreate failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_SubtaskCreate_MissingTaskID(t *testing.T) {
	c := &Client{}
	_, err := c.SubtaskCreate(map[string]interface{}{"title": "child"})
	if err == nil {
		t.Error("expected error for missing task_id")
	}
}

func TestClient_SubtaskCreate_MissingTitle(t *testing.T) {
	c := &Client{}
	_, err := c.SubtaskCreate(map[string]interface{}{"task_id": "any"})
	if err == nil {
		t.Error("expected error for missing title")
	}
}

func TestClient_SubtaskList_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/parent-id-12345678/subtasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"subtasks":[]}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.SubtaskList(map[string]interface{}{"task_id": "parent-id-12345678"})
	if err != nil {
		t.Fatalf("SubtaskList failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_SubtaskReorder_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/parent-id-12345678/subtasks/child-id-12345678/position" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.SubtaskReorder(map[string]interface{}{
		"task_id": "parent-id-12345678", "child_id": "child-id-12345678", "position": 2,
	})
	if err != nil {
		t.Fatalf("SubtaskReorder failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_SubtaskReorder_MissingFields(t *testing.T) {
	c := &Client{}
	_, err := c.SubtaskReorder(map[string]interface{}{"task_id": "any"})
	if err == nil {
		t.Error("expected error for missing child_id")
	}
	_, err = c.SubtaskReorder(map[string]interface{}{"task_id": "any", "child_id": "any"})
	if err == nil {
		t.Error("expected error for missing position")
	}
}

func TestClient_DepAdd_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.DepAdd(map[string]interface{}{
		"task_id": "task-id-12345678", "blocker_id": "blocker-id-12345678",
	})
	if err != nil {
		t.Fatalf("DepAdd failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_DepAdd_MissingFields(t *testing.T) {
	c := &Client{}
	_, err := c.DepAdd(map[string]interface{}{"task_id": "any"})
	if err == nil {
		t.Error("expected error for missing blocker_id")
	}
	_, err = c.DepAdd(map[string]interface{}{"blocker_id": "any"})
	if err == nil {
		t.Error("expected error for missing task_id")
	}
}

func TestClient_DepList_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/tasks/task-id-12345678/dependencies" {
			w.Write([]byte(`{"blockers":[]}`))
		} else {
			w.Write([]byte(`{"dependents":[]}`))
		}
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.DepList(map[string]interface{}{"task_id": "task-id-12345678"})
	if err != nil {
		t.Fatalf("DepList failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_DepCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"can_start":true}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.DepCheck(map[string]interface{}{"task_id": "task-id-12345678"})
	if err != nil {
		t.Fatalf("DepCheck failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_DepCheck_Blocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"can_start":false,"blockers":[{"id":"b1","title":"Blocker","status":"in_progress"}]}`))
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.DepCheck(map[string]interface{}{"task_id": "task-id-12345678"})
	if err != nil {
		t.Fatalf("DepCheck failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_DepCheck_MissingID(t *testing.T) {
	c := &Client{}
	_, err := c.DepCheck(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing task_id")
	}
}

func TestClient_TaskShow_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not valid json`))
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	_, err := c.TaskShow(map[string]interface{}{"task_id": "any-id-12345678"})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestClient_TaskList_WithSearchAndSource(t *testing.T) {
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	_, err := c.TaskList(map[string]interface{}{
		"project_id": "proj-1", "status": "in_progress",
		"assignee":   "me", "search": "auth", "source": "claude-code",
	})
	if err != nil {
		t.Fatalf("TaskList failed: %v", err)
	}
	// source param should appear in query
	if receivedQuery == "" {
		t.Error("expected query params")
	}
}

func TestClient_TaskList_WithTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[
			{"id":"tid-abcdefgh","title":"My Task","status":"in_progress","priority":"high","assignee":"me"}
		],"total":1}`))
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	result, err := c.TaskList(map[string]interface{}{})
	if err != nil {
		t.Fatalf("TaskList failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_TaskList_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	_, err := c.TaskList(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestClient_TaskCreate_WithAllFields(t *testing.T) {
	taskID := "full-task-12345678"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/tasks":
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["title"] != "Full Task" {
				t.Errorf("unexpected body: %v", body)
			}
			w.Write([]byte(`{"id":"` + taskID + `","title":"Full Task","status":"pending","priority":"urgent"}`))
		case r.Method == "PATCH" && r.URL.Path == "/api/tasks/"+taskID+"/status":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	result, err := c.TaskCreate(map[string]interface{}{
		"title": "Full Task", "project_name": "my-proj",
		"description": "A long description", "priority": "urgent",
		"assignee": "bob", "task_mode": "sequential",
	})
	if err != nil {
		t.Fatalf("TaskCreate failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_TaskCreate_GitRepoDetection(t *testing.T) {
	// Simulate a git repo by creating .git in tmp dir
	tmpDir := t.TempDir()
	oldCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	defer os.Chdir(oldCwd)
	os.MkdirAll(tmpDir+"/.git", 0755)

	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/projects/by-repo":
			w.Write([]byte(`{"id":"git-proj-12345678"}`))
		case "/api/tasks":
			w.Write([]byte(`{"id":"git-task-12345678","title":"Git Task","status":"pending","priority":"medium"}`))
		default:
			if strings.HasPrefix(r.URL.Path, "/api/tasks/git-task") {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	_, err := c.TaskCreate(map[string]interface{}{"title": "Git Task"})
	if err != nil {
		t.Fatalf("TaskCreate failed: %v", err)
	}
	// Should have: GET /api/projects/by-repo, POST /api/tasks, PATCH /api/tasks/{id}/status
	if len(requests) < 2 {
		t.Errorf("expected at least 2 requests, got: %v", requests)
	}
}

func TestClient_TaskCreate_GitRepoDetectionError(t *testing.T) {
	tmpDir := t.TempDir()
	oldCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	defer os.Chdir(oldCwd)
	os.MkdirAll(tmpDir+"/.git", 0755)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	_, err := c.TaskCreate(map[string]interface{}{"title": "Git Task"})
	if err == nil {
		t.Error("expected error when git repo detection fails")
	}
}

func TestClient_TaskShow_WithSubtasksAndDeps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tasks/show2-12345678":
			w.Write([]byte(`{"id":"show2-12345678","title":"Parent","status":"pending","priority":"medium","assignee":"","task_mode":"sequential","created_at":123}`))
		case "/api/tasks/show2-12345678/subtasks":
			w.Write([]byte(`{"subtasks":[{"id":"child-12345678","title":"Child 1","status":"completed"}]}`))
		case "/api/tasks/show2-12345678/dependencies":
			w.Write([]byte(`{"blockers":[{"id":"block-12345678","title":"Blocker","status":"in_progress"}]}`))
		case "/api/tasks/show2-12345678/dependents":
			w.Write([]byte(`{"dependents":[{"id":"dep-12345678","title":"Dependent","status":"pending"}]}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	result, err := c.TaskShow(map[string]interface{}{"task_id": "show2-12345678"})
	if err != nil {
		t.Fatalf("TaskShow failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_TaskUpdate_WithDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/up2-12345678" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"up2-12345678","title":"Updated","status":"pending","priority":"high"}`))
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	result, err := c.TaskUpdate(map[string]interface{}{
		"task_id": "up2-12345678", "description": "new desc",
	})
	if err != nil {
		t.Fatalf("TaskUpdate failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_ProjectList_WithProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"projects":[
			{"id":"proj-12345678","name":"Alpha","description":"First project"},
			{"id":"proj-abcdefgh","name":"Beta","description":""}
		]}`))
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	result, err := c.ProjectList(map[string]interface{}{})
	if err != nil {
		t.Fatalf("ProjectList failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_ProjectUpdate_WithDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/p2-12345678" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"p2-12345678","name":"UpdatedProj","description":"New desc"}`))
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	result, err := c.ProjectUpdate(map[string]interface{}{
		"project_id": "p2-12345678", "name": "UpdatedProj", "description": "New desc",
	})
	if err != nil {
		t.Fatalf("ProjectUpdate failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_ProjectDelete_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	_, err := c.ProjectDelete(map[string]interface{}{"project_id": "missing-12345678"})
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestClient_SubtaskCreate_WithPriorityAssignee(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/par-12345678/subtasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task":{"id":"child-12345678","title":"subtask","status":"pending"}}`))
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	result, err := c.SubtaskCreate(map[string]interface{}{
		"task_id": "par-12345678", "title": "subtask",
		"priority": "high", "assignee": "alice",
	})
	if err != nil {
		t.Fatalf("SubtaskCreate failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_SubtaskList_WithItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/par-12345678/subtasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"subtasks":[
			{"id":"c1-12345678","title":"Child 1","status":"in_progress"},
			{"id":"c2-12345678","title":"Child 2","status":"pending"}
		]}`))
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	result, err := c.SubtaskList(map[string]interface{}{"task_id": "par-12345678"})
	if err != nil {
		t.Fatalf("SubtaskList failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_DepList_WithBlockers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/tasks/task-12345678/dependencies" {
			w.Write([]byte(`{"blockers":[{"id":"b1-12345678","title":"Blocker 1","status":"completed"}]}`))
		} else {
			w.Write([]byte(`{"dependents":[]}`))
		}
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	result, err := c.DepList(map[string]interface{}{"task_id": "task-12345678"})
	if err != nil {
		t.Fatalf("DepList failed: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestClient_TaskStart_ConnectionError(t *testing.T) {
	c := newTestClient("http://localhost:59999")
	_, err := c.TaskStart(map[string]interface{}{"task_id": "any-12345678"})
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestClient_do_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error":"server error"}`)
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	_, err := c.TaskList(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestClient_do_ConnectionError(t *testing.T) {
	c := newTestClient("http://localhost:59999")
	_, err := c.TaskList(map[string]interface{}{})
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestNew_MissingAPIKey(t *testing.T) {
	origURL := os.Getenv("TASKS_WATCHER_SERVER_URL")
	origKey := os.Getenv("TASKS_WATCHER_API_KEY")
	defer func() {
		if origURL != "" {
			os.Setenv("TASKS_WATCHER_SERVER_URL", origURL)
		} else {
			os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		}
		if origKey != "" {
			os.Setenv("TASKS_WATCHER_API_KEY", origKey)
		} else {
			os.Unsetenv("TASKS_WATCHER_API_KEY")
		}
	}()

	// Use a temp HOME with no key file
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpDir)
	os.Unsetenv("TASKS_WATCHER_SERVER_URL")
	os.Unsetenv("TASKS_WATCHER_API_KEY")

	_, err := New()
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestNew_Success(t *testing.T) {
	origURL := os.Getenv("TASKS_WATCHER_SERVER_URL")
	origKey := os.Getenv("TASKS_WATCHER_API_KEY")
	defer func() {
		if origURL != "" {
			os.Setenv("TASKS_WATCHER_SERVER_URL", origURL)
		} else {
			os.Unsetenv("TASKS_WATCHER_SERVER_URL")
		}
		if origKey != "" {
			os.Setenv("TASKS_WATCHER_API_KEY", origKey)
		} else {
			os.Unsetenv("TASKS_WATCHER_API_KEY")
		}
	}()

	os.Setenv("TASKS_WATCHER_SERVER_URL", "http://localhost:4242")
	os.Setenv("TASKS_WATCHER_API_KEY", "test-key-from-env")

	c, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if c.BaseURL != "http://localhost:4242" {
		t.Errorf("expected URL http://localhost:4242, got %s", c.BaseURL)
	}
	if c.APIKey != "test-key-from-env" {
		t.Errorf("expected API key test-key-from-env, got %s", c.APIKey)
	}
}

func TestClient_Close(t *testing.T) {
	c := &Client{}
	if err := c.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestGetOrCreateProjectByRepo_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":"not found"}`)
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	_, err := c.getOrCreateProjectByRepo("/some/repo")
	if err == nil {
		t.Error("expected error for API 404")
	}
}

func TestGetOrCreateProjectByRepo_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `not valid json`)
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	_, err := c.getOrCreateProjectByRepo("/some/repo")
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestGetOrCreateProjectByRepo_ConnectionError(t *testing.T) {
	c := newTestClient("http://localhost:59999")
	_, err := c.getOrCreateProjectByRepo("/some/repo")
	if err == nil {
		t.Error("expected connection error")
	}
}
