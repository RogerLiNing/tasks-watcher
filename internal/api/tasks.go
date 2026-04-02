package api

import (
	"encoding/json"
	"fmt"
	"net/http"

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
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Assignee    string `json:"assignee"`
	Source      string `json:"source"`
}

type UpdateTaskRequest struct {
	ProjectID   string `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Assignee    string `json:"assignee"`
	Source      string `json:"source"`
}

type StatusUpdateRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	status := r.URL.Query().Get("status")
	assignee := r.URL.Query().Get("assignee")

	tasks, err := h.db.ListTasks(projectID, status, assignee)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
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

	t := &models.Task{
		ProjectID:   projectID,
		Title:       req.Title,
		Description: req.Description,
		Status:      models.TaskStatusPending,
		Priority:    priority,
		Assignee:    req.Assignee,
		Source:      req.Source,
	}

	if t.Source == "" {
		t.Source = "manual"
	}

	if err := h.db.CreateTask(t); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Notify
	h.dispatcher.Notify(models.EventTaskCreated, t)
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
	if err != nil || t == nil {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}

	if req.Title != "" {
		t.Title = req.Title
	}
	if req.Description != "" {
		t.Description = req.Description
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

	if err := h.db.UpdateTask(t); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	BroadcastTaskEvent(h.sse, models.EventTaskCreated, t)
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
	if err != nil || t == nil {
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

	// Determine event type
	eventType := fmt.Sprintf("task.%s", to)

	h.dispatcher.Notify(eventType, t)
	BroadcastTaskEvent(h.sse, eventType, t)

	json.NewEncoder(w).Encode(t)
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

func (h *TaskHandler) Register(router *mux.Router) {
	router.HandleFunc("/tasks", h.List).Methods("GET")
	router.HandleFunc("/tasks", h.Create).Methods("POST")
	router.HandleFunc("/tasks/{id}", h.Get).Methods("GET")
	router.HandleFunc("/tasks/{id}", h.Update).Methods("PUT")
	router.HandleFunc("/tasks/{id}", h.Delete).Methods("DELETE")
	router.HandleFunc("/tasks/{id}/status", h.UpdateStatus).Methods("PATCH")
	router.HandleFunc("/tasks/{id}/heartbeat", h.Heartbeat).Methods("POST")
}
