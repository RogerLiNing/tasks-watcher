package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
)

type AgentHandler struct {
	db *db.DB
}

func NewAgentHandler(database *db.DB) *AgentHandler {
	return &AgentHandler{db: database}
}

type AgentOverview struct {
	Name         string `json:"name"`
	ActiveTasks  int    `json:"active_tasks"`
	PendingTasks int    `json:"pending_tasks"`
	CompletedTasks int  `json:"completed_tasks"`
	FailedTasks  int    `json:"failed_tasks"`
	TotalTasks   int    `json:"total_tasks"`
}

func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	agents, err := h.db.ListAgents()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	if agents == nil {
		agents = []string{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"agents": agents})
}

func (h *AgentHandler) Overview(w http.ResponseWriter, r *http.Request) {
	tasks, _, err := h.db.ListTasks("", "", "", "", "", 0, 0)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}

	// Group by assignee
	type counts struct{ active, pending, completed, failed, total int }
	byAgent := make(map[string]*counts)
	var noAgent bool

	for _, t := range tasks {
		assignees := t.Assignees
		if len(assignees) == 0 {
			noAgent = true
			continue
		}
		for _, name := range assignees {
			if _, ok := byAgent[name]; !ok {
				byAgent[name] = &counts{}
			}
			c := byAgent[name]
			c.total++
			switch t.Status {
			case "in_progress":
				c.active++
			case "pending":
				c.pending++
			case "completed":
				c.completed++
			case "failed", "cancelled":
				c.failed++
			}
		}
	}

	overviews := []map[string]interface{}{}
	for name, c := range byAgent {
		overviews = append(overviews, map[string]interface{}{
			"name":            name,
			"active_tasks":    c.active,
			"pending_tasks":   c.pending,
			"completed_tasks": c.completed,
			"failed_tasks":    c.failed,
			"total_tasks":     c.total,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents":    overviews,
		"no_agent":  noAgent,
	})
}

func (h *AgentHandler) Register(router *mux.Router) {
	router.HandleFunc("/agents", h.List).Methods("GET")
	router.HandleFunc("/agents/overview", h.Overview).Methods("GET")
}
