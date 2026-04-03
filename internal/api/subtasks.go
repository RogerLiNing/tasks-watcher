package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

type SubtaskHandler struct {
	db  *db.DB
	sse *SSEHandler
}

func NewSubtaskHandler(database *db.DB, sse *SSEHandler) *SubtaskHandler {
	return &SubtaskHandler{db: database, sse: sse}
}

type createSubtaskReq struct {
	Title       string `json:"title"`
	Description any    `json:"description"`
	Locale      string `json:"locale"`
	Priority    string `json:"priority"`
	Assignee    string `json:"assignee"`
}

type addSubtaskReq struct {
	ChildID     string `json:"child_id"`
	Title       string `json:"title"`
	Description any    `json:"description"`
	Locale      string `json:"locale"`
	Priority    string `json:"priority"`
	Assignee    string `json:"assignee"`
	Position    int    `json:"position"` // optional; auto-assigned if not provided
}

type reorderSubtaskReq struct {
	Position int `json:"position"`
}

func (h *SubtaskHandler) ListSubtasks(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	tasks, err := h.db.GetSubtaskTasks(id)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	posMap, err := h.db.GetSubtaskPositions(id)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"subtasks": tasks})
		return
	}
	type taskWithPos struct {
		models.Task
		Position int `json:"position"`
	}
	taskList := make([]taskWithPos, len(tasks))
	for i, t := range tasks {
		taskList[i] = taskWithPos{Task: t, Position: posMap[t.ID]}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"subtasks": taskList})
}

func (h *SubtaskHandler) AddSubtask(w http.ResponseWriter, r *http.Request) {
	parentID := mux.Vars(r)["id"]
	var req addSubtaskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.ChildID != "" {
		// Link an existing task as a subtask
		child, err := h.db.AddSubtask(parentID, req.ChildID)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		BroadcastTaskEvent(h.sse, models.EventSubtaskAdded, map[string]interface{}{
			"parent_id": parentID,
			"child":     child,
		})
		json.NewEncoder(w).Encode(map[string]interface{}{"task": child})
		return
	}

	if req.Title == "" {
		http.Error(w, `{"error":"title or child_id is required"}`, http.StatusBadRequest)
		return
	}

	// Create a new task as a subtask
	// Use parent's project
	parent, err := h.db.GetTask(parentID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if parent == nil {
		http.Error(w, `{"error":"parent task not found"}`, http.StatusNotFound)
		return
	}

	priority := models.PriorityMedium
	if models.ValidPriority(req.Priority) {
		priority = models.Priority(req.Priority)
	}

	locale := req.Locale
	if locale == "" {
		locale = "en"
	}
	desc := map[string]string{}
	if req.Description != nil {
		desc = models.MergeDescription(nil, req.Description)
	}

	t := &models.Task{
		ProjectID:   parent.ProjectID,
		Title:       req.Title,
		Description: desc,
		Status:      models.TaskStatusPending,
		Priority:    priority,
		Assignee:    req.Assignee,
		Source:      "manual",
	}
	if err := h.db.CreateTask(t); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_, err = h.db.AddSubtask(parentID, t.ID)
	if err != nil {
		// Rollback task creation
		h.db.DeleteTask(t.ID)
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	BroadcastTaskEvent(h.sse, models.EventSubtaskAdded, map[string]interface{}{
		"parent_id": parentID,
		"child":     t,
	})
	json.NewEncoder(w).Encode(map[string]interface{}{"task": t})
}

func (h *SubtaskHandler) RemoveSubtask(w http.ResponseWriter, r *http.Request) {
	parentID := mux.Vars(r)["id"]
	childID := mux.Vars(r)["childId"]
	if err := h.db.RemoveSubtask(parentID, childID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	BroadcastTaskEvent(h.sse, models.EventSubtaskRemoved, map[string]string{
		"parent_id": parentID,
		"child_id":  childID,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *SubtaskHandler) GetParent(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	task, err := h.db.GetParentTask(id)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if task == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"parent": nil})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"parent": task})
}

// ReorderSubtask moves a subtask to a new position within its parent.
func (h *SubtaskHandler) ReorderSubtask(w http.ResponseWriter, r *http.Request) {
	parentID := mux.Vars(r)["id"]
	childID := mux.Vars(r)["childId"]
	var req reorderSubtaskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.SetSubtaskPosition(parentID, childID, req.Position); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	BroadcastTaskEvent(h.sse, "task.subtask.reordered", map[string]string{
		"parent_id": parentID,
		"child_id":  childID,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *SubtaskHandler) Register(router *mux.Router) {
	r := router.PathPrefix("/tasks/{id}").Subrouter()
	r.HandleFunc("/subtasks", h.ListSubtasks).Methods("GET")
	r.HandleFunc("/subtasks", h.AddSubtask).Methods("POST")
	r.HandleFunc("/subtasks/{childId}", h.RemoveSubtask).Methods("DELETE")
	r.HandleFunc("/subtasks/{childId}/position", h.ReorderSubtask).Methods("PATCH")
	r.HandleFunc("/parent", h.GetParent).Methods("GET")
}
