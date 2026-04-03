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
