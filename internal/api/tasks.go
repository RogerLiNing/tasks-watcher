package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
	"github.com/rogerrlee/tasks-watcher/internal/notifications"
)

type TaskHandler struct {
	db          *db.DB
	sse         *SSEHandler
	dispatcher  *notifications.Dispatcher
}

func NewTaskHandler(database *db.DB, sse *SSEHandler, disp *notifications.Dispatcher) *TaskHandler {
	return &TaskHandler{db: database, sse: sse, dispatcher: disp}
}

type CreateTaskRequest struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	RepoPath    string `json:"repo_path"`
	Title       string `json:"title"`
	Description any    `json:"description"`
	Locale      string `json:"locale"`
	Priority    string `json:"priority"`
	Assignee    string `json:"assignee"`
	Source      string `json:"source"`
	TaskMode    string `json:"task_mode"`
}

type UpdateTaskRequest struct {
	ProjectID   string `json:"project_id"`
	Title       string `json:"title"`
	Description any    `json:"description"`
	Locale      string `json:"locale"`
	Priority    string `json:"priority"`
	Assignee    string `json:"assignee"`
	Source      string `json:"source"`
	TaskMode    string `json:"task_mode"`
}

type StatusUpdateRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	status := r.URL.Query().Get("status")
	assignee := r.URL.Query().Get("assignee")
	search := r.URL.Query().Get("search")
	source := r.URL.Query().Get("source")

	limit := 100
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed := parseInt(l); parsed > 0 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed := parseInt(o); parsed >= 0 {
			offset = parsed
		}
	}

	tasks, total, err := h.db.ListTasks(projectID, status, assignee, search, source, limit, offset)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks, "total": total, "limit": limit, "offset": offset})
}

func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	t, err := h.db.GetTask(id)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if t == nil {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(t)
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
		return
	}

	// Resolve project
	projectID := req.ProjectID
	if projectID == "" && req.ProjectName != "" {
		p, err := h.db.GetOrCreateProject(req.ProjectName)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		projectID = p.ID
	}
	if projectID == "" && req.RepoPath != "" {
		// Auto-associate project by git repo path
		p, err := h.db.GetOrCreateByRepoPath(req.RepoPath)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		projectID = p.ID
	}
	if projectID == "" {
		// Use or create "default" project
		p, err := h.db.GetOrCreateProject("default")
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		projectID = p.ID
	}

	priority := models.PriorityMedium
	if models.ValidPriority(req.Priority) {
		priority = models.Priority(req.Priority)
	}

	locale := req.Locale
	if locale == "" {
		locale = "en"
	}

	desc := map[string]string{locale: ""}
	if req.Description != nil {
		desc = models.MergeDescription(nil, req.Description)
	}

	t := &models.Task{
		ProjectID:   projectID,
		Title:       req.Title,
		Description: desc,
		Status:      models.TaskStatusPending,
		Priority:    priority,
		Assignee:    req.Assignee,
		Source:      req.Source,
	}

	if t.Source == "" {
		t.Source = "manual"
	}
	if models.ValidTaskMode(req.TaskMode) {
		t.TaskMode = models.TaskMode(req.TaskMode)
	}

	if err := h.db.CreateTask(t); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Notify
	if h.dispatcher != nil {
		h.dispatcher.Notify(models.EventTaskCreated, t)
	}
	BroadcastTaskEvent(h.sse, models.EventTaskCreated, t)

	json.NewEncoder(w).Encode(t)
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	t, err := h.db.GetTask(id)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if t == nil {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}

	if req.Title != "" {
		t.Title = req.Title
	}
	if req.Description != nil {
		if t.Description == nil {
			t.Description = make(map[string]string)
		}
		t.Description = models.MergeDescription(t.Description, req.Description)
	}
	if models.ValidPriority(req.Priority) {
		t.Priority = models.Priority(req.Priority)
	}
	if req.ProjectID != "" {
		t.ProjectID = req.ProjectID
	}
	if req.Assignee != "" {
		t.Assignee = req.Assignee
	}
	if req.Source != "" {
		t.Source = req.Source
	}
	if models.ValidTaskMode(req.TaskMode) {
		t.TaskMode = models.TaskMode(req.TaskMode)
	}

	if err := h.db.UpdateTask(t); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	BroadcastTaskEvent(h.sse, models.EventTaskUpdated, t)
	json.NewEncoder(w).Encode(t)
}

func (h *TaskHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req StatusUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if !models.ValidTaskStatus(req.Status) {
		http.Error(w, `{"error":"invalid status"}`, http.StatusBadRequest)
		return
	}

	t, err := h.db.GetTask(id)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if t == nil {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}

	// Validate transition
	from := t.Status
	to := models.TaskStatus(req.Status)
	if err := validateTransition(from, to); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	// Block transition to in_progress if task has incomplete blockers
	if to == models.TaskStatusInProgress {
		result, err := h.db.CanStartTask(id)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if !result.CanStart {
			resp := map[string]interface{}{
				"error":                  "task is blocked",
				"can_start":              false,
				"blockers":               result.Blockers,
				"child_titles":           result.ChildTitles,
				"blocked_by_sequential":  result.BlockedBySequential,
				"sequential_blocker":     result.SequentialBlocker,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}

	// Update status
	now := models.Now()
	completedAt := int64(0)
	if to == models.TaskStatusCompleted || to == models.TaskStatusFailed || to == models.TaskStatusCancelled {
		completedAt = now
	}

	t.Status = to
	t.UpdatedAt = now
	t.CompletedAt = completedAt
	if req.Reason != "" {
		t.ErrorMessage = req.Reason
	}

	if err := h.db.UpdateTask(t); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Propagate status to parent if this is a subtask and the new status is relevant
	h.propagateToParent(t)

	// Determine event type
	eventType := fmt.Sprintf("task.%s", to)

	if h.dispatcher != nil {
		h.dispatcher.Notify(eventType, t)
	}
	BroadcastTaskEvent(h.sse, eventType, t)

	json.NewEncoder(w).Encode(t)
}

// propagateToParent updates the parent task's status based on child statuses.
// - If any child becomes in_progress, parent auto-starts if pending.
// - If parent is sequential and any child reaches failed/cancelled, propagate that status.
// - If all children reach terminal state, parent auto-completes.
func (h *TaskHandler) propagateToParent(child *models.Task) {
	parentID, err := h.db.GetParentID(child.ID)
	if err != nil || parentID == "" {
		return
	}

	parent, err := h.db.GetTask(parentID)
	if err != nil || parent == nil {
		return
	}

	// If parent is already in a terminal state, skip propagation
	if parent.IsTerminal() {
		return
	}

	// If parent is sequential and child reached a terminal state, propagate it
	if parent.TaskMode == models.TaskModeSequential && child.IsTerminal() {
		newStatus := child.Status
		if err := h.db.UpdateTaskStatus(parentID, newStatus, ""); err != nil {
			return
		}
		BroadcastTaskEvent(h.sse, models.EventSubtaskStatusChanged, map[string]interface{}{
			"parent":       parent,
			"child_id":     child.ID,
			"child_status": child.Status,
		})
		return
	}

	// Get all child statuses
	childStatuses, err := h.db.GetChildStatuses(parentID)
	if err != nil {
		return
	}

	newStatus := db.ComputeParentStatus(childStatuses)
	if newStatus == parent.Status {
		return // No change needed
	}

	if err := h.db.UpdateTaskStatus(parentID, newStatus, ""); err != nil {
		return
	}

	// Broadcast parent status change
	BroadcastTaskEvent(h.sse, models.EventSubtaskStatusChanged, map[string]interface{}{
		"parent":       parent,
		"child_id":     child.ID,
		"child_status": child.Status,
	})
}

func (h *TaskHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.db.HeartbeatTask(id); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.db.DeleteTask(id); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	BroadcastTaskEvent(h.sse, "task.deleted", map[string]string{"id": id})
	w.WriteHeader(http.StatusNoContent)
}

func validateTransition(from, to models.TaskStatus) error {
	// Define allowed transitions
	valid := map[models.TaskStatus][]models.TaskStatus{
		models.TaskStatusPending:    {models.TaskStatusInProgress, models.TaskStatusCancelled},
		models.TaskStatusInProgress: {models.TaskStatusCompleted, models.TaskStatusFailed, models.TaskStatusCancelled},
		models.TaskStatusCompleted:  {},
		models.TaskStatusFailed:     {models.TaskStatusInProgress}, // Can retry
		models.TaskStatusCancelled:  {models.TaskStatusInProgress}, // Can restart
	}

	allowed, ok := valid[from]
	if !ok {
		return fmt.Errorf("unknown status: %s", from)
	}

	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %s to %s", from, to)
}

func parseInt(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return 0
}

func (h *TaskHandler) Register(router *mux.Router) {
	router.HandleFunc("/tasks", h.List).Methods("GET")
	router.HandleFunc("/tasks", h.Create).Methods("POST")
	router.HandleFunc("/tasks/{id}", h.Get).Methods("GET")
	router.HandleFunc("/tasks/{id}", h.Update).Methods("PUT")
	router.HandleFunc("/tasks/{id}", h.Delete).Methods("DELETE")
	router.HandleFunc("/tasks/{id}/status", h.UpdateStatus).Methods("PATCH")
	router.HandleFunc("/tasks/{id}/heartbeat", h.Heartbeat).Methods("POST")
}
