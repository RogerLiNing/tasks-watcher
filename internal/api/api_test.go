package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

func setupTaskTestDB(t *testing.T) *db.DB {
	// Resolve project root from this test file's location.
	// api_test.go is at: $PROJECT_ROOT/internal/api/api_test.go
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

func newTestTaskRouter(t *testing.T) (*mux.Router, *db.DB) {
	database := setupTaskTestDB(t)
	sse := NewSSEHandler("test-api-key")
	handler := NewTaskHandler(database, sse, nil)

	router := mux.NewRouter()
	handler.Register(router)
	return router, database
}

func newJSONBody(v interface{}) *bytes.Buffer {
	data, _ := json.Marshal(v)
	return bytes.NewBuffer(data)
}

// makeProject creates a project in the database and returns its ID.
func makeProject(t *testing.T, database *db.DB, name string) string {
	p := &models.Project{Name: name}
	if err := database.CreateProject(p); err != nil {
		t.Fatalf("CreateProject(%q) failed: %v", name, err)
	}
	return p.ID
}

// makeTask creates a task in the database and returns its ID.
func makeTask(t *testing.T, database *db.DB, projectID, title string, status models.TaskStatus) string {
	task := &models.Task{
		ProjectID: projectID,
		Title:    title,
		Status:   status,
		Priority: models.PriorityMedium,
	}
	if err := database.CreateTask(task); err != nil {
		t.Fatalf("CreateTask(%q) failed: %v", title, err)
	}
	return task.ID
}

func TestTaskHandler_List(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	req := httptest.NewRequest("GET", "/tasks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	tasks, ok := resp["tasks"].([]interface{})
	if !ok {
		t.Fatalf("expected tasks to be an array, got %T", resp["tasks"])
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
	if total, ok := resp["total"].(float64); !ok || int(total) != 0 {
		t.Errorf("expected total 0, got %v", resp["total"])
	}
}

func TestTaskHandler_Create_TitleRequired(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	body := `{"title": ""}`
	req := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if err, ok := resp["error"].(string); !ok || err != "title is required" {
		t.Errorf("expected 'title is required', got %q", err)
	}
}

func TestTaskHandler_Create_Success(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	body := `{"title": "Test task", "priority": "high"}`
	req := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var task models.Task
	if err := json.NewDecoder(w.Body).Decode(&task); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if task.ID == "" {
		t.Error("expected task ID to be set")
	}
	if task.Title != "Test task" {
		t.Errorf("expected title 'Test task', got %q", task.Title)
	}
	if task.Priority != models.PriorityHigh {
		t.Errorf("expected priority 'high', got %q", task.Priority)
	}
	if task.Status != models.TaskStatusPending {
		t.Errorf("expected status 'pending', got %q", task.Status)
	}
	if task.Source != "manual" {
		t.Errorf("expected source 'manual', got %q", task.Source)
	}
}

func TestTaskHandler_Create_WithSource(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	body := `{"title": "Claude task", "source": "claude-code"}`
	req := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var task models.Task
	json.NewDecoder(w.Body).Decode(&task)
	if task.Source != "claude-code" {
		t.Errorf("expected source 'claude-code', got %q", task.Source)
	}
}

func TestTaskHandler_Create_WithTaskMode(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	body := `{"title": "Sequential task", "task_mode": "sequential"}`
	req := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var task models.Task
	json.NewDecoder(w.Body).Decode(&task)
	if task.TaskMode != models.TaskModeSequential {
		t.Errorf("expected task_mode 'sequential', got %q", task.TaskMode)
	}
}

func TestTaskHandler_Create_WithDescription(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	body := `{"title": "Task with desc", "description": "hello world", "locale": "en"}`
	req := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var task models.Task
	json.NewDecoder(w.Body).Decode(&task)
	if task.Description == nil {
		t.Fatal("expected description to be set")
	}
	if task.Description["en"] != "hello world" {
		t.Errorf("expected description['en']='hello world', got %v", task.Description)
	}
}

func TestTaskHandler_Create_WithDescriptionMap(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	body := `{"title": "Task with map desc", "description": {"zh": "你好", "en": "hello"}}`
	req := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var task models.Task
	json.NewDecoder(w.Body).Decode(&task)
	if task.Description["en"] != "hello" {
		t.Errorf("expected description['en']='hello', got %v", task.Description)
	}
	if task.Description["zh"] != "你好" {
		t.Errorf("expected description['zh']='你好', got %v", task.Description)
	}
}

func TestTaskHandler_Create_InvalidJSON(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	body := `{invalid json}`
	req := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestTaskHandler_Get_NotFound(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	req := httptest.NewRequest("GET", "/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestTaskHandler_Get_Success(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// Create a task first
	createBody := `{"title": "Get test task"}`
	createReq := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)

	var created models.Task
	json.NewDecoder(createW.Body).Decode(&created)

	// Get the task
	req := httptest.NewRequest("GET", "/tasks/"+created.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var task models.Task
	json.NewDecoder(w.Body).Decode(&task)
	if task.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, task.ID)
	}
	if task.Title != "Get test task" {
		t.Errorf("expected title 'Get test task', got %q", task.Title)
	}
}

func TestTaskHandler_Update_Success(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// Create
	createBody := `{"title": "Original title"}`
	createReq := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	var created models.Task
	json.NewDecoder(createW.Body).Decode(&created)

	// Update title
	updateBody := `{"title": "Updated title"}`
	req := httptest.NewRequest("PUT", "/tasks/"+created.ID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var task models.Task
	json.NewDecoder(w.Body).Decode(&task)
	if task.Title != "Updated title" {
		t.Errorf("expected title 'Updated title', got %q", task.Title)
	}
}

func TestTaskHandler_Update_Priority(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// Create
	createReq := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(`{"title": "Priority test"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	var created models.Task
	json.NewDecoder(createW.Body).Decode(&created)

	// Update priority
	req := httptest.NewRequest("PUT", "/tasks/"+created.ID, bytes.NewBufferString(`{"priority": "urgent"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var task models.Task
	json.NewDecoder(w.Body).Decode(&task)
	if task.Priority != models.PriorityUrgent {
		t.Errorf("expected priority 'urgent', got %q", task.Priority)
	}
}

func TestTaskHandler_Update_NotFound(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	req := httptest.NewRequest("PUT", "/tasks/nonexistent", bytes.NewBufferString(`{"title": "X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestTaskHandler_Update_InvalidJSON(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// Create
	createReq := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(`{"title": "JSON test"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	var created models.Task
	json.NewDecoder(createW.Body).Decode(&created)

	req := httptest.NewRequest("PUT", "/tasks/"+created.ID, bytes.NewBufferString(`{bad}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestTaskHandler_Delete_Success(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// Create
	createReq := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(`{"title": "Delete me"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	var created models.Task
	json.NewDecoder(createW.Body).Decode(&created)

	// Delete
	req := httptest.NewRequest("DELETE", "/tasks/"+created.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify it's gone
	getReq := httptest.NewRequest("GET", "/tasks/"+created.ID, nil)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getW.Code)
	}
}

func TestTaskHandler_Delete_NotFound(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// DELETE on non-existent resource returns 204 (idempotent — result is the same)
	req := httptest.NewRequest("DELETE", "/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestTaskHandler_List_Pagination(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// Create 3 tasks
	for i := 0; i < 3; i++ {
		body := bytes.NewBufferString(`{"title": "Page task ` + string(rune('a'+i)) + `"}`)
		req := httptest.NewRequest("POST", "/tasks", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// List with limit=2
	req := httptest.NewRequest("GET", "/tasks?limit=2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	tasks := resp["tasks"].([]interface{})
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
	if total, ok := resp["total"].(float64); !ok || int(total) != 3 {
		t.Errorf("expected total 3, got %v", resp["total"])
	}
	if limit, ok := resp["limit"].(float64); !ok || int(limit) != 2 {
		t.Errorf("expected limit 2, got %v", resp["limit"])
	}
}

func TestTaskHandler_List_Offset(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// Create 3 tasks
	for i := 0; i < 3; i++ {
		body := bytes.NewBufferString(`{"title": "Offset task ` + string(rune('A'+i)) + `"}`)
		req := httptest.NewRequest("POST", "/tasks", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// List with offset=1
	req := httptest.NewRequest("GET", "/tasks?offset=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	tasks := resp["tasks"].([]interface{})
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks with offset=1, got %d", len(tasks))
	}
	if offset, ok := resp["offset"].(float64); !ok || int(offset) != 1 {
		t.Errorf("expected offset 1, got %v", resp["offset"])
	}
}

func TestTaskHandler_List_FilterByStatus(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// Create 2 tasks
	for i := 0; i < 2; i++ {
		body := bytes.NewBufferString(`{"title": "Status filter task ` + string(rune('0'+i)) + `"}`)
		req := httptest.NewRequest("POST", "/tasks", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	req := httptest.NewRequest("GET", "/tasks?status=completed", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	tasks := resp["tasks"].([]interface{})
	if len(tasks) != 0 {
		t.Errorf("expected 0 completed tasks, got %d", len(tasks))
	}
}

func TestTaskHandler_List_FilterBySource(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// Create tasks with different sources
	for _, src := range []string{"claude-code", "claude-code", "manual"} {
		body := bytes.NewBufferString(`{"title": "Source test", "source": "` + src + `"}`)
		req := httptest.NewRequest("POST", "/tasks", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	req := httptest.NewRequest("GET", "/tasks?source=claude-code", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	tasks := resp["tasks"].([]interface{})
	if len(tasks) != 2 {
		t.Errorf("expected 2 claude-code tasks, got %d", len(tasks))
	}
}

func TestTaskHandler_UpdateStatus(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// Create task
	createReq := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(`{"title": "Status update test"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	var created models.Task
	json.NewDecoder(createW.Body).Decode(&created)

	// Update status to in_progress
	statusReq := httptest.NewRequest("PATCH", "/tasks/"+created.ID+"/status",
		bytes.NewBufferString(`{"status": "in_progress"}`))
	statusReq.Header.Set("Content-Type", "application/json")
	statusW := httptest.NewRecorder()
	router.ServeHTTP(statusW, statusReq)

	if statusW.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", statusW.Code)
	}

	var updated models.Task
	json.NewDecoder(statusW.Body).Decode(&updated)
	if updated.Status != models.TaskStatusInProgress {
		t.Errorf("expected status 'in_progress', got %q", updated.Status)
	}
}

func TestTaskHandler_UpdateStatus_InvalidStatus(t *testing.T) {
	router, _ := newTestTaskRouter(t)

	// Create task
	createReq := httptest.NewRequest("POST", "/tasks", bytes.NewBufferString(`{"title": "Invalid status test"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	var created models.Task
	json.NewDecoder(createW.Body).Decode(&created)

	// Try to set invalid status
	statusReq := httptest.NewRequest("PATCH", "/tasks/"+created.ID+"/status",
		bytes.NewBufferString(`{"status": "invalid_status"}`))
	statusReq.Header.Set("Content-Type", "application/json")
	statusW := httptest.NewRecorder()
	router.ServeHTTP(statusW, statusReq)

	// Should return 400 or silently ignore invalid status (depends on implementation)
	// Check task is still pending
	getReq := httptest.NewRequest("GET", "/tasks/"+created.ID, nil)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	var task models.Task
	json.NewDecoder(getW.Body).Decode(&task)
	if task.Status != models.TaskStatusPending {
		t.Errorf("expected status 'pending' (invalid status ignored), got %q", task.Status)
	}
}

func TestAuthMiddleware_MissingAuth(t *testing.T) {
	database := setupTaskTestDB(t)
	sse := NewSSEHandler("my-secret-key")
	handler := NewTaskHandler(database, sse, nil)

	router := mux.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				key := r.URL.Query().Get("api_key")
				if key != "my-secret-key" {
					http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
					return
				}
			} else if auth != "Bearer my-secret-key" {
				http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	handler.Register(router)

	// No auth header
	req := httptest.NewRequest("GET", "/tasks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	database := setupTaskTestDB(t)
	sse := NewSSEHandler("my-secret-key")
	handler := NewTaskHandler(database, sse, nil)

	router := mux.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				key := r.URL.Query().Get("api_key")
				if key != "my-secret-key" {
					http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
					return
				}
			} else if auth != "Bearer my-secret-key" {
				http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	handler.Register(router)

	// Wrong bearer token
	req := httptest.NewRequest("GET", "/tasks", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong token, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	database := setupTaskTestDB(t)
	sse := NewSSEHandler("my-secret-key")
	handler := NewTaskHandler(database, sse, nil)

	router := mux.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				key := r.URL.Query().Get("api_key")
				if key != "my-secret-key" {
					http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
					return
				}
			} else if auth != "Bearer my-secret-key" {
				http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	handler.Register(router)

	req := httptest.NewRequest("GET", "/tasks", nil)
	req.Header.Set("Authorization", "Bearer my-secret-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d", w.Code)
	}
}

func TestAuthMiddleware_QueryParamAuth(t *testing.T) {
	database := setupTaskTestDB(t)
	sse := NewSSEHandler("my-secret-key")
	handler := NewTaskHandler(database, sse, nil)

	router := mux.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				key := r.URL.Query().Get("api_key")
				if key != "my-secret-key" {
					http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
					return
				}
			} else if auth != "Bearer my-secret-key" {
				http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	handler.Register(router)

	req := httptest.NewRequest("GET", "/tasks?api_key=my-secret-key", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with api_key query param, got %d", w.Code)
	}
}

// --- ProjectHandler tests ---

func newTestProjectRouter(t *testing.T) (*mux.Router, *db.DB) {
	database := setupTaskTestDB(t)
	sse := NewSSEHandler("test-api-key")
	handler := NewProjectHandler(database, sse)

	router := mux.NewRouter()
	handler.Register(router)
	return router, database
}

func TestProjectHandler_List(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	req := httptest.NewRequest("GET", "/projects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	projects, ok := resp["projects"].([]interface{})
	if !ok {
		t.Fatalf("expected projects array, got %T", resp["projects"])
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestProjectHandler_Create_NameRequired(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/projects", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProjectHandler_Create_Success(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	payload := map[string]string{"name": "my-project", "description": "A test project"}
	body := newJSONBody(payload)
	req := httptest.NewRequest("POST", "/projects", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var proj models.Project
	if err := json.NewDecoder(w.Body).Decode(&proj); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if proj.Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", proj.Name)
	}
	if proj.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestProjectHandler_Create_InvalidJSON(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	body := bytes.NewBufferString(`not json`)
	req := httptest.NewRequest("POST", "/projects", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProjectHandler_Get_NotFound(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	req := httptest.NewRequest("GET", "/projects/nonexistent-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProjectHandler_Get_Success(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	// Create first
	payload := map[string]string{"name": "get-test-project"}
	body := newJSONBody(payload)
	req := httptest.NewRequest("POST", "/projects", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var created models.Project
	json.NewDecoder(w.Body).Decode(&created)

	req = httptest.NewRequest("GET", "/projects/"+created.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var fetched models.Project
	json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetched.ID)
	}
}

func TestProjectHandler_Update_Success(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	// Create first
	body := newJSONBody(map[string]string{"name": "orig-name"})
	req := httptest.NewRequest("POST", "/projects", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var created models.Project
	json.NewDecoder(w.Body).Decode(&created)

	// Update
	updateBody := newJSONBody(map[string]string{"name": "new-name", "description": "updated desc"})
	req = httptest.NewRequest("PUT", "/projects/"+created.ID, updateBody)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var updated models.Project
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Name != "new-name" {
		t.Errorf("expected name 'new-name', got %q", updated.Name)
	}
	if updated.Description != "updated desc" {
		t.Errorf("expected description 'updated desc', got %q", updated.Description)
	}
}

func TestProjectHandler_Update_NotFound(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	body := newJSONBody(map[string]string{"name": "some-name"})
	req := httptest.NewRequest("PUT", "/projects/nonexistent-id", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProjectHandler_Delete_Success(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	// Create first
	body := newJSONBody(map[string]string{"name": "to-delete"})
	req := httptest.NewRequest("POST", "/projects", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var created models.Project
	json.NewDecoder(w.Body).Decode(&created)

	// Delete
	req = httptest.NewRequest("DELETE", "/projects/"+created.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	// Verify it's gone
	req = httptest.NewRequest("GET", "/projects/"+created.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w.Code)
	}
}

func TestProjectHandler_GetOrCreateByRepo_MissingParam(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	req := httptest.NewRequest("GET", "/projects/by-repo", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProjectHandler_GetOrCreateByRepo_Success(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	req := httptest.NewRequest("GET", "/projects/by-repo?repo_path=/tmp/test-repo", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var proj models.Project
	json.NewDecoder(w.Body).Decode(&proj)
	if proj.RepoPath != "/tmp/test-repo" {
		t.Errorf("expected repo_path '/tmp/test-repo', got %q", proj.RepoPath)
	}
}

func TestProjectHandler_List_AfterCreate(t *testing.T) {
	router, _ := newTestProjectRouter(t)

	// Create two projects
	for _, name := range []string{"proj-a", "proj-b"} {
		body := newJSONBody(map[string]string{"name": name})
		req := httptest.NewRequest("POST", "/projects", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("create failed for %s: %d", name, w.Code)
		}
	}

	// List
	req := httptest.NewRequest("GET", "/projects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	projects := resp["projects"].([]interface{})
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

// --- ColumnHandler tests ---

func newTestColumnRouter(t *testing.T) (*mux.Router, *db.DB) {
	database := setupTaskTestDB(t)
	sse := NewSSEHandler("test-api-key")
	handler := NewColumnHandler(database, sse)

	router := mux.NewRouter()
	handler.Register(router)
	return router, database
}

func TestColumnHandler_List_Empty(t *testing.T) {
	router, database := newTestColumnRouter(t)

	// Delete any default columns seeded by migration 005
	cols, _ := database.ListColumns()
	for _, c := range cols {
		database.DeleteColumn(c.ID)
	}

	req := httptest.NewRequest("GET", "/columns", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	colsVal := resp["columns"]
	if colsVal == nil {
		colsVal = []interface{}{}
	}
	if cols := colsVal.([]interface{}); len(cols) != 0 {
		t.Errorf("expected 0 columns, got %d", len(cols))
	}
}

func TestColumnHandler_Create_LabelRequired(t *testing.T) {
	router, _ := newTestColumnRouter(t)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/columns", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestColumnHandler_Create_Success(t *testing.T) {
	router, _ := newTestColumnRouter(t)

	payload := map[string]interface{}{"label": "In Review", "color": "#ff0000", "position": 0}
	body := newJSONBody(payload)
	req := httptest.NewRequest("POST", "/columns", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var col models.TaskColumn
	json.NewDecoder(w.Body).Decode(&col)
	if col.Label != "In Review" {
		t.Errorf("expected label 'In Review', got %q", col.Label)
	}
	if col.Color != "#ff0000" {
		t.Errorf("expected color '#ff0000', got %q", col.Color)
	}
	if col.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestColumnHandler_Create_InvalidJSON(t *testing.T) {
	router, _ := newTestColumnRouter(t)

	body := bytes.NewBufferString(`{invalid}`)
	req := httptest.NewRequest("POST", "/columns", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestColumnHandler_Create_DefaultColor(t *testing.T) {
	router, _ := newTestColumnRouter(t)

	// Omit color — should default to #86868b
	payload := map[string]string{"label": "Backlog"}
	body := newJSONBody(payload)
	req := httptest.NewRequest("POST", "/columns", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var col models.TaskColumn
	json.NewDecoder(w.Body).Decode(&col)
	if col.Color != "#86868b" {
		t.Errorf("expected default color '#86868b', got %q", col.Color)
	}
}

func TestColumnHandler_Create_KeyDeduplication(t *testing.T) {
	router, database := newTestColumnRouter(t)

	// Delete default columns seeded by migration so we test in isolation
	cols, _ := database.ListColumns()
	for _, c := range cols {
		database.DeleteColumn(c.ID)
	}

	// Create first column — auto-generates key from label
	payload := map[string]string{"label": "In Progress"}
	body := newJSONBody(payload)
	req := httptest.NewRequest("POST", "/columns", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var first models.TaskColumn
	json.NewDecoder(w.Body).Decode(&first)

	// Create second column with same label — key should be deduplicated
	req = httptest.NewRequest("POST", "/columns", newJSONBody(payload))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var second models.TaskColumn
	json.NewDecoder(w.Body).Decode(&second)

	if first.Key == second.Key {
		t.Errorf("expected different keys, got same: %q", first.Key)
	}
	if second.Key != "in_progress_2" {
		t.Errorf("expected key 'in_progress_2', got %q", second.Key)
	}
}

func TestColumnHandler_Get_NotFound(t *testing.T) {
	router, _ := newTestColumnRouter(t)

	req := httptest.NewRequest("PUT", "/columns/nonexistent-id", newJSONBody(map[string]string{"label": "x"}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestColumnHandler_Update_Success(t *testing.T) {
	router, _ := newTestColumnRouter(t)

	// Create first
	body := newJSONBody(map[string]string{"label": "To Do", "color": "#0000ff"})
	req := httptest.NewRequest("POST", "/columns", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var created models.TaskColumn
	json.NewDecoder(w.Body).Decode(&created)

	// Update
	updateBody := newJSONBody(map[string]interface{}{"label": "Done", "color": "#00ff00", "position": 5})
	req = httptest.NewRequest("PUT", "/columns/"+created.ID, updateBody)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var updated models.TaskColumn
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Label != "Done" {
		t.Errorf("expected label 'Done', got %q", updated.Label)
	}
	if updated.Color != "#00ff00" {
		t.Errorf("expected color '#00ff00', got %q", updated.Color)
	}
	if updated.Position != 5 {
		t.Errorf("expected position 5, got %d", updated.Position)
	}
}

func TestColumnHandler_Update_NotFound(t *testing.T) {
	router, _ := newTestColumnRouter(t)

	body := newJSONBody(map[string]string{"label": "x"})
	req := httptest.NewRequest("PUT", "/columns/nonexistent-id", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestColumnHandler_Delete_Success(t *testing.T) {
	router, _ := newTestColumnRouter(t)

	// Create first
	body := newJSONBody(map[string]string{"label": "temp-col"})
	req := httptest.NewRequest("POST", "/columns", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var created models.TaskColumn
	json.NewDecoder(w.Body).Decode(&created)

	// Delete
	req = httptest.NewRequest("DELETE", "/columns/"+created.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	// Verify 404
	req = httptest.NewRequest("PUT", "/columns/"+created.ID, newJSONBody(map[string]string{"label": "x"}))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w.Code)
	}
}

func TestColumnHandler_Delete_NotFound(t *testing.T) {
	router, _ := newTestColumnRouter(t)

	req := httptest.NewRequest("DELETE", "/columns/nonexistent-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- slugify tests ---

func TestSlugify(t *testing.T) {
	cases := []struct {
		label string
		want  string
	}{
		{"In Progress", "in_progress"},
		{"To-Do List!", "to_do_list"},
		{"  spaces  ", "spaces"},
		{"ALL_CAPS", "all_caps"},
		{"hello123world", "hello123world"},
		{"Mixed!@#$%Case", "mixed_case"},
		{"CJK任务", "cjk"},
		{"日本語テスト", "col"},
		{"한국어", "col"},
		{"emoji 🚀 test", "emoji_test"},
		{"", "col"},
		{"   ", "col"},
		{"---dashes---", "dashes"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := slugify(tc.label)
			if got != tc.want {
				t.Errorf("slugify(%q) = %q, want %q", tc.label, got, tc.want)
			}
		})
	}
}

// --- SubtaskHandler tests ---

func newTestSubtaskRouter(t *testing.T) (*mux.Router, *db.DB) {
	database := setupTaskTestDB(t)
	sse := NewSSEHandler("test-api-key")
	handler := NewSubtaskHandler(database, sse)

	router := mux.NewRouter()
	handler.Register(router)
	return router, database
}

// createTaskViaAPI creates a task via the DB and returns it as a map.
func createTaskViaAPI(t *testing.T, database *db.DB, title string) map[string]interface{} {
	task := &models.Task{
		ProjectID: "", // will be set by DB or use default project
		Title:     title,
		Status:    models.TaskStatusPending,
		Priority:  models.PriorityMedium,
	}
	// Ensure a default project exists
	p := &models.Project{Name: "default"}
	database.CreateProject(p)
	task.ProjectID = p.ID
	if err := database.CreateTask(task); err != nil {
		t.Fatalf("CreateTask(%q) failed: %v", title, err)
	}
	return map[string]interface{}{
		"id":         task.ID,
		"title":      task.Title,
		"status":     string(task.Status),
		"priority":   string(task.Priority),
		"project_id": task.ProjectID,
	}
}

func TestSubtaskHandler_ListSubtasks_Empty(t *testing.T) {
	router, database := newTestSubtaskRouter(t)

	// Create parent task
	parent := createTaskViaAPI(t, database, "parent-task")
	pid := parent["id"].(string)
	database.UpdateTaskStatus(pid, models.TaskStatusPending, "")

	req := httptest.NewRequest("GET", "/tasks/"+pid+"/subtasks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	subtasks := resp["subtasks"].([]interface{})
	if len(subtasks) != 0 {
		t.Errorf("expected 0 subtasks, got %d", len(subtasks))
	}
}

func TestSubtaskHandler_ListSubtasks_WithChildren(t *testing.T) {
	router, database := newTestSubtaskRouter(t)

	// Create parent and two children
	parent := createTaskViaAPI(t, database, "parent")
	pid := parent["id"].(string)
	c1 := createTaskViaAPI(t, database, "child1")
	c2 := createTaskViaAPI(t, database, "child2")

	// Add subtasks via API
	for _, child := range []map[string]interface{}{c1, c2} {
		body := newJSONBody(map[string]interface{}{"child_id": child["id"]})
		req := httptest.NewRequest("POST", "/tasks/"+pid+"/subtasks", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("add subtask failed: %d", w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/tasks/"+pid+"/subtasks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	subtasks := resp["subtasks"].([]interface{})
	if len(subtasks) != 2 {
		t.Errorf("expected 2 subtasks, got %d", len(subtasks))
	}
}

func TestSubtaskHandler_AddSubtask_LinkExisting(t *testing.T) {
	router, database := newTestSubtaskRouter(t)

	parent := createTaskViaAPI(t, database, "parent")
	pid := parent["id"].(string)
	child := createTaskViaAPI(t, database, "existing-child")

	body := newJSONBody(map[string]interface{}{"child_id": child["id"]})
	req := httptest.NewRequest("POST", "/tasks/"+pid+"/subtasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	task := resp["task"].(map[string]interface{})
	if task["id"] != child["id"] {
		t.Errorf("expected child id %q, got %q", child["id"], task["id"])
	}
}

func TestSubtaskHandler_AddSubtask_CreateNew(t *testing.T) {
	router, database := newTestSubtaskRouter(t)

	parent := createTaskViaAPI(t, database, "parent")
	pid := parent["id"].(string)

	body := newJSONBody(map[string]interface{}{
		"title":       "brand-new-subtask",
		"description": "a description",
		"priority":    "high",
	})
	req := httptest.NewRequest("POST", "/tasks/"+pid+"/subtasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	task := resp["task"].(map[string]interface{})
	if task["title"] != "brand-new-subtask" {
		t.Errorf("expected title 'brand-new-subtask', got %q", task["title"])
	}
	if task["priority"] != "high" {
		t.Errorf("expected priority 'high', got %q", task["priority"])
	}
}

func TestSubtaskHandler_AddSubtask_NeitherChildNorTitle(t *testing.T) {
	router, database := newTestSubtaskRouter(t)

	parent := createTaskViaAPI(t, database, "parent")
	pid := parent["id"].(string)

	body := newJSONBody(map[string]interface{}{})
	req := httptest.NewRequest("POST", "/tasks/"+pid+"/subtasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSubtaskHandler_AddSubtask_InvalidJSON(t *testing.T) {
	router, database := newTestSubtaskRouter(t)

	parent := createTaskViaAPI(t, database, "parent")
	pid := parent["id"].(string)

	body := bytes.NewBufferString(`not json`)
	req := httptest.NewRequest("POST", "/tasks/"+pid+"/subtasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSubtaskHandler_RemoveSubtask(t *testing.T) {
	router, database := newTestSubtaskRouter(t)

	parent := createTaskViaAPI(t, database, "parent")
	pid := parent["id"].(string)
	child := createTaskViaAPI(t, database, "child-to-remove")

	// Add subtask
	body := newJSONBody(map[string]interface{}{"child_id": child["id"]})
	req := httptest.NewRequest("POST", "/tasks/"+pid+"/subtasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Remove
	req = httptest.NewRequest("DELETE", "/tasks/"+pid+"/subtasks/"+child["id"].(string), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	// Verify it's gone
	req = httptest.NewRequest("GET", "/tasks/"+pid+"/subtasks", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	subtasks := resp["subtasks"].([]interface{})
	if len(subtasks) != 0 {
		t.Errorf("expected 0 subtasks after removal, got %d", len(subtasks))
	}
}

func TestSubtaskHandler_ReorderSubtask(t *testing.T) {
	router, database := newTestSubtaskRouter(t)

	parent := createTaskViaAPI(t, database, "parent")
	pid := parent["id"].(string)
	child := createTaskViaAPI(t, database, "child-to-reorder")

	// Add subtask
	body := newJSONBody(map[string]interface{}{"child_id": child["id"]})
	req := httptest.NewRequest("POST", "/tasks/"+pid+"/subtasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Reorder to position 5
	body = newJSONBody(map[string]interface{}{"position": 5})
	req = httptest.NewRequest("PATCH", "/tasks/"+pid+"/subtasks/"+child["id"].(string)+"/position", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	// Verify position
	req = httptest.NewRequest("GET", "/tasks/"+pid+"/subtasks", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	subtasks := resp["subtasks"].([]interface{})
	if len(subtasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(subtasks))
	}
	pos := subtasks[0].(map[string]interface{})["position"].(float64)
	if int(pos) != 5 {
		t.Errorf("expected position 5, got %v", int(pos))
	}
}

func TestSubtaskHandler_ReorderSubtask_InvalidJSON(t *testing.T) {
	router, database := newTestSubtaskRouter(t)

	parent := createTaskViaAPI(t, database, "parent")
	pid := parent["id"].(string)
	child := createTaskViaAPI(t, database, "child")

	body := newJSONBody(map[string]interface{}{"child_id": child["id"]})
	req := httptest.NewRequest("POST", "/tasks/"+pid+"/subtasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body = bytes.NewBufferString(`{invalid}`)
	req = httptest.NewRequest("PATCH", "/tasks/"+pid+"/subtasks/"+child["id"].(string)+"/position", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSubtaskHandler_GetParent_NoParent(t *testing.T) {
	router, database := newTestSubtaskRouter(t)

	child := createTaskViaAPI(t, database, "orphan")
	cid := child["id"].(string)

	req := httptest.NewRequest("GET", "/tasks/"+cid+"/parent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["parent"] != nil {
		t.Errorf("expected nil parent, got %v", resp["parent"])
	}
}

func TestSubtaskHandler_GetParent_WithParent(t *testing.T) {
	router, database := newTestSubtaskRouter(t)

	parent := createTaskViaAPI(t, database, "real-parent")
	pid := parent["id"].(string)
	child := createTaskViaAPI(t, database, "child-with-parent")

	// Make child a subtask
	body := newJSONBody(map[string]interface{}{"child_id": child["id"]})
	req := httptest.NewRequest("POST", "/tasks/"+pid+"/subtasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Get parent
	req = httptest.NewRequest("GET", "/tasks/"+child["id"].(string)+"/parent", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	parentData := resp["parent"].(map[string]interface{})
	if parentData["id"] != pid {
		t.Errorf("expected parent id %q, got %q", pid, parentData["id"])
	}
}

// --- DepHandler tests ---

func newTestDepRouter(t *testing.T) (*mux.Router, *db.DB) {
	database := setupTaskTestDB(t)
	sse := NewSSEHandler("test-api-key")
	handler := NewDepHandler(database, sse)
	router := mux.NewRouter()
	handler.Register(router)
	return router, database
}

func TestDepHandler_ListBlockers_Empty(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	tid := makeTask(t, database, pid, "task", models.TaskStatusPending)

	req := httptest.NewRequest("GET", "/tasks/"+tid+"/dependencies", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	blockers := resp["blockers"].([]interface{})
	if len(blockers) != 0 {
		t.Errorf("expected 0 blockers, got %d", len(blockers))
	}
}

func TestDepHandler_ListBlockers_WithBlockers(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	t1 := makeTask(t, database, pid, "task1", models.TaskStatusPending)
	t2 := makeTask(t, database, pid, "blocker", models.TaskStatusInProgress)
	database.AddDependency(t1, t2)

	req := httptest.NewRequest("GET", "/tasks/"+t1+"/dependencies", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	blockers := resp["blockers"].([]interface{})
	if len(blockers) != 1 {
		t.Errorf("expected 1 blocker, got %d", len(blockers))
	}
}

func TestDepHandler_ListDependents_Empty(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	tid := makeTask(t, database, pid, "task", models.TaskStatusPending)

	req := httptest.NewRequest("GET", "/tasks/"+tid+"/dependents", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	deps := resp["dependents"].([]interface{})
	if len(deps) != 0 {
		t.Errorf("expected 0 dependents, got %d", len(deps))
	}
}

func TestDepHandler_ListDependents_WithDependents(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	blocker := makeTask(t, database, pid, "blocker", models.TaskStatusInProgress)
	dependent := makeTask(t, database, pid, "dependent", models.TaskStatusPending)
	database.AddDependency(dependent, blocker)

	req := httptest.NewRequest("GET", "/tasks/"+blocker+"/dependents", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	deps := resp["dependents"].([]interface{})
	if len(deps) != 1 {
		t.Errorf("expected 1 dependent, got %d", len(deps))
	}
}

func TestDepHandler_AddBlocker_Success(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	t1 := makeTask(t, database, pid, "task1", models.TaskStatusPending)
	t2 := makeTask(t, database, pid, "blocker", models.TaskStatusInProgress)

	body := newJSONBody(map[string]string{"blocker_id": t2})
	req := httptest.NewRequest("POST", "/tasks/"+t1+"/dependencies", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDepHandler_AddBlocker_MissingBlockerID(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	tid := makeTask(t, database, pid, "task", models.TaskStatusPending)

	body := newJSONBody(map[string]string{"blocker_id": ""})
	req := httptest.NewRequest("POST", "/tasks/"+tid+"/dependencies", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDepHandler_AddBlocker_SelfLoop(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	tid := makeTask(t, database, pid, "task", models.TaskStatusPending)

	body := newJSONBody(map[string]string{"blocker_id": tid})
	req := httptest.NewRequest("POST", "/tasks/"+tid+"/dependencies", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for self-loop, got %d", w.Code)
	}
}

func TestDepHandler_AddBlocker_Circular(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	t1 := makeTask(t, database, pid, "t1", models.TaskStatusPending)
	t2 := makeTask(t, database, pid, "t2", models.TaskStatusPending)
	t3 := makeTask(t, database, pid, "t3", models.TaskStatusPending)
	database.AddDependency(t1, t2)
	database.AddDependency(t2, t3)

	body := newJSONBody(map[string]string{"blocker_id": t1})
	req := httptest.NewRequest("POST", "/tasks/"+t3+"/dependencies", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for circular dep, got %d", w.Code)
	}
}

func TestDepHandler_AddBlocker_InvalidJSON(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	tid := makeTask(t, database, pid, "task", models.TaskStatusPending)

	req := httptest.NewRequest("POST", "/tasks/"+tid+"/dependencies", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDepHandler_RemoveBlocker_Success(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	t1 := makeTask(t, database, pid, "task1", models.TaskStatusPending)
	t2 := makeTask(t, database, pid, "blocker", models.TaskStatusInProgress)
	database.AddDependency(t1, t2)

	req := httptest.NewRequest("DELETE", "/tasks/"+t1+"/dependencies/"+t2, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestDepHandler_CanStart_NoBlockers(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	tid := makeTask(t, database, pid, "task", models.TaskStatusPending)

	req := httptest.NewRequest("GET", "/tasks/"+tid+"/can-start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["can_start"] != true {
		t.Errorf("expected can_start=true, got %v", result["can_start"])
	}
}

func TestDepHandler_CanStart_Blocked(t *testing.T) {
	router, database := newTestDepRouter(t)
	pid := makeProject(t, database, "proj")
	t1 := makeTask(t, database, pid, "task1", models.TaskStatusPending)
	t2 := makeTask(t, database, pid, "blocker", models.TaskStatusInProgress)
	database.AddDependency(t1, t2)

	req := httptest.NewRequest("GET", "/tasks/"+t1+"/can-start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["can_start"] != false {
		t.Errorf("expected can_start=false, got %v", result["can_start"])
	}
}

// --- NotificationHandler tests ---

func newTestNotifRouter(t *testing.T) *mux.Router {
	database := setupTaskTestDB(t)
	handler := NewNotificationHandler(database)
	router := mux.NewRouter()
	handler.Register(router)
	return router
}

func TestNotificationHandler_List_Empty(t *testing.T) {
	router := newTestNotifRouter(t)

	req := httptest.NewRequest("GET", "/notifications", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["unread_count"].(float64) != 0 {
		t.Errorf("expected unread_count=0, got %v", resp["unread_count"])
	}
}

func TestNotificationHandler_List_WithNotifications(t *testing.T) {
	database := setupTaskTestDB(t)
	handler := NewNotificationHandler(database)
	database.CreateNotification(&models.Notification{
		TaskID:  "task-1",
		Type:    "task.completed",
		Message: "Task completed: Test",
		Read:    false,
	})
	database.CreateNotification(&models.Notification{
		TaskID:  "task-2",
		Type:    "task.failed",
		Message: "Task failed: Oops",
		Read:    true,
	})

	router := mux.NewRouter()
	handler.Register(router)
	req := httptest.NewRequest("GET", "/notifications", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	notifs := resp["notifications"].([]interface{})
	if len(notifs) != 2 {
		t.Errorf("expected 2 notifications, got %d", len(notifs))
	}
	if resp["unread_count"].(float64) != 1 {
		t.Errorf("expected unread_count=1, got %v", resp["unread_count"])
	}
}

func TestNotificationHandler_MarkRead(t *testing.T) {
	database := setupTaskTestDB(t)
	handler := NewNotificationHandler(database)
	database.CreateNotification(&models.Notification{
		TaskID:  "task-1",
		Type:    "task.completed",
		Message: "Done",
		Read:    false,
	})
	notifs, _ := database.ListNotifications(10)
	unreadID := notifs[0].ID

	router := mux.NewRouter()
	handler.Register(router)
	req := httptest.NewRequest("PATCH", "/notifications/"+unreadID+"/read", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestNotificationHandler_MarkAllRead(t *testing.T) {
	database := setupTaskTestDB(t)
	handler := NewNotificationHandler(database)
	database.CreateNotification(&models.Notification{TaskID: "t1", Type: "t", Message: "m1", Read: false})
	database.CreateNotification(&models.Notification{TaskID: "t2", Type: "t", Message: "m2", Read: false})

	router := mux.NewRouter()
	handler.Register(router)
	req := httptest.NewRequest("POST", "/notifications/read", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	count, _ := database.GetUnreadNotificationCount()
	if count != 0 {
		t.Errorf("expected 0 unread after mark-all-read, got %d", count)
	}
}

func TestNotificationHandler_Clear(t *testing.T) {
	database := setupTaskTestDB(t)
	handler := NewNotificationHandler(database)
	database.CreateNotification(&models.Notification{TaskID: "t1", Type: "t", Message: "m1", Read: false})
	database.CreateNotification(&models.Notification{TaskID: "t2", Type: "t", Message: "m2", Read: false})

	router := mux.NewRouter()
	handler.Register(router)
	req := httptest.NewRequest("DELETE", "/notifications", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	count, _ := database.GetUnreadNotificationCount()
	if count != 0 {
		t.Errorf("expected 0 notifications after clear, got %d", count)
	}
}

// --- AgentHandler tests ---

func newTestAgentRouter(t *testing.T) *mux.Router {
	database := setupTaskTestDB(t)
	handler := NewAgentHandler(database)
	router := mux.NewRouter()
	handler.Register(router)
	return router
}

func TestAgentHandler_List_Empty(t *testing.T) {
	router := newTestAgentRouter(t)

	req := httptest.NewRequest("GET", "/agents", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	agents := resp["agents"].([]interface{})
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestAgentHandler_Overview_Empty(t *testing.T) {
	router := newTestAgentRouter(t)

	req := httptest.NewRequest("GET", "/agents/overview", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	agents := resp["agents"].([]interface{})
	if len(agents) != 0 {
		t.Errorf("expected 0 agents in overview, got %d", len(agents))
	}
}

func TestAgentHandler_Overview_WithTasks(t *testing.T) {
	database := setupTaskTestDB(t)
	handler := NewAgentHandler(database)
	p := &models.Project{Name: "proj"}
	database.CreateProject(p)
	database.CreateTask(&models.Task{ProjectID: p.ID, Title: "Task 1", Status: models.TaskStatusInProgress, Priority: models.PriorityMedium, Assignee: "agent-a"})
	database.CreateTask(&models.Task{ProjectID: p.ID, Title: "Task 2", Status: models.TaskStatusCompleted, Priority: models.PriorityMedium, Assignee: "agent-a"})
	database.CreateTask(&models.Task{ProjectID: p.ID, Title: "Task 3", Status: models.TaskStatusPending, Priority: models.PriorityMedium, Assignee: "agent-b"})

	router := mux.NewRouter()
	handler.Register(router)
	req := httptest.NewRequest("GET", "/agents/overview", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	agents := resp["agents"].([]interface{})
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

// --- WebhookHandler tests ---

func newTestWebhookRouter(t *testing.T) *mux.Router {
	database := setupTaskTestDB(t)
	handler := NewWebhookHandler(database)
	router := mux.NewRouter()
	handler.Register(router)
	return router
}

func TestWebhookHandler_List_Empty(t *testing.T) {
	router := newTestWebhookRouter(t)

	req := httptest.NewRequest("GET", "/webhooks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	hooks := resp["webhooks"].([]interface{})
	if len(hooks) != 0 {
		t.Errorf("expected 0 webhooks, got %d", len(hooks))
	}
}

func TestWebhookHandler_Create_Success(t *testing.T) {
	router := newTestWebhookRouter(t)

	body := newJSONBody(map[string]interface{}{"url": "https://example.com/hook", "events": "task.completed", "active": true})
	req := httptest.NewRequest("POST", "/webhooks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var wh models.WebhookConfig
	json.NewDecoder(w.Body).Decode(&wh)
	if wh.URL != "https://example.com/hook" {
		t.Errorf("expected URL 'https://example.com/hook', got %q", wh.URL)
	}
	if wh.Events != "task.completed" {
		t.Errorf("expected events 'task.completed', got %q", wh.Events)
	}
}

func TestWebhookHandler_Create_DefaultEvents(t *testing.T) {
	router := newTestWebhookRouter(t)

	body := newJSONBody(map[string]string{"url": "https://example.com/hook"})
	req := httptest.NewRequest("POST", "/webhooks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var wh models.WebhookConfig
	json.NewDecoder(w.Body).Decode(&wh)
	if wh.Events != "task.*" {
		t.Errorf("expected default events 'task.*', got %q", wh.Events)
	}
}

func TestWebhookHandler_Create_URLRequired(t *testing.T) {
	router := newTestWebhookRouter(t)

	body := newJSONBody(map[string]string{"url": ""})
	req := httptest.NewRequest("POST", "/webhooks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestWebhookHandler_Create_InvalidJSON(t *testing.T) {
	router := newTestWebhookRouter(t)

	req := httptest.NewRequest("POST", "/webhooks", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestWebhookHandler_Delete_Success(t *testing.T) {
	router := newTestWebhookRouter(t)

	// Create first
	body := newJSONBody(map[string]string{"url": "https://example.com/hook"})
	req := httptest.NewRequest("POST", "/webhooks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var wh models.WebhookConfig
	json.NewDecoder(w.Body).Decode(&wh)

	// Delete
	req = httptest.NewRequest("DELETE", "/webhooks/"+wh.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

// --- NotificationConfigHandler tests ---

func newTestNotifConfigRouter(t *testing.T) *mux.Router {
	database := setupTaskTestDB(t)
	handler := NewNotificationConfigHandler(database)
	router := mux.NewRouter()
	handler.Register(router)
	return router
}

func TestNotificationConfigHandler_List(t *testing.T) {
	router := newTestNotifConfigRouter(t)

	req := httptest.NewRequest("GET", "/notifications/configs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	cfgs := resp["configs"].([]interface{})
	if len(cfgs) != 2 {
		t.Errorf("expected 2 configs (macos+email from migration seed), got %d", len(cfgs))
	}
}

func TestNotificationConfigHandler_Get_NotFound(t *testing.T) {
	router := newTestNotifConfigRouter(t)

	req := httptest.NewRequest("GET", "/notifications/configs/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestNotificationConfigHandler_Get_Success(t *testing.T) {
	database := setupTaskTestDB(t)
	handler := NewNotificationConfigHandler(database)
	database.UpsertNotificationConfig(&models.NotificationConfig{
		Type:    "macos",
		Enabled: true,
		Config:  map[string]interface{}{"enabled": true},
	})

	router := mux.NewRouter()
	handler.Register(router)
	req := httptest.NewRequest("GET", "/notifications/configs/macos", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var cfg models.NotificationConfig
	json.NewDecoder(w.Body).Decode(&cfg)
	if cfg.Type != "macos" {
		t.Errorf("expected type 'macos', got %q", cfg.Type)
	}
}

func TestNotificationConfigHandler_Upsert_Success(t *testing.T) {
	router := newTestNotifConfigRouter(t)

	body := newJSONBody(map[string]interface{}{
		"type":    "email",
		"enabled": true,
		"config":  map[string]interface{}{"smtp_host": "smtp.example.com"},
	})
	req := httptest.NewRequest("POST", "/notifications/configs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotificationConfigHandler_Upsert_InvalidJSON(t *testing.T) {
	router := newTestNotifConfigRouter(t)

	req := httptest.NewRequest("POST", "/notifications/configs", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
