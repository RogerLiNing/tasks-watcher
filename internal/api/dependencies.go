package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

type DepHandler struct {
	db  *db.DB
	sse *SSEHandler
}

func NewDepHandler(database *db.DB, sse *SSEHandler) *DepHandler {
	return &DepHandler{db: database, sse: sse}
}

type addDepReq struct {
	BlockerID string `json:"blocker_id"`
}

func (h *DepHandler) ListBlockers(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	tasks, err := h.db.GetDependencyTasks(id)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"blockers": tasks})
}

func (h *DepHandler) ListDependents(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	tasks, err := h.db.GetDependentTasks(id)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"dependents": tasks})
}

func (h *DepHandler) AddBlocker(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req addDepReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.BlockerID == "" {
		http.Error(w, `{"error":"blocker_id is required"}`, http.StatusBadRequest)
		return
	}

	dep, err := h.db.AddDependency(id, req.BlockerID)
	if err != nil {
		log.Printf("AddDependency(%s, %s) failed: %v", id, req.BlockerID, err)
		// Pass through user-meaningful errors; log and return generic for internal errors.
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			http.Error(w, `{"error":"`+msg+`"}`, http.StatusNotFound)
		case strings.Contains(msg, "circular"):
			http.Error(w, `{"error":"`+msg+`"}`, http.StatusBadRequest)
		case strings.Contains(msg, "cannot depend on itself"):
			http.Error(w, `{"error":"`+msg+`"}`, http.StatusBadRequest)
		default:
			http.Error(w, `{"error":"failed to add dependency"}`, http.StatusInternalServerError)
		}
		return
	}

	BroadcastTaskEvent(h.sse, models.EventDependencyAdded, dep)
	json.NewEncoder(w).Encode(dep)
}

func (h *DepHandler) RemoveBlocker(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	blockerID := mux.Vars(r)["blockerId"]
	if err := h.db.RemoveDependency(id, blockerID); err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	BroadcastTaskEvent(h.sse, models.EventDependencyRemoved, map[string]string{
		"task_id":    id,
		"blocker_id": blockerID,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *DepHandler) CanStart(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	result, err := h.db.CanStartTask(id)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	json.NewEncoder(w).Encode(result)
}

func (h *DepHandler) Register(router *mux.Router) {
	r := router.PathPrefix("/tasks/{id}").Subrouter()
	r.HandleFunc("/dependencies", h.ListBlockers).Methods("GET")
	r.HandleFunc("/dependencies", h.AddBlocker).Methods("POST")
	r.HandleFunc("/dependencies/{blockerId}", h.RemoveBlocker).Methods("DELETE")
	r.HandleFunc("/dependents", h.ListDependents).Methods("GET")
	r.HandleFunc("/can-start", h.CanStart).Methods("GET")
}
