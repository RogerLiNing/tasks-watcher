package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

type ProjectHandler struct {
	db  *db.DB
	sse *SSEHandler
}

func NewProjectHandler(database *db.DB, sse *SSEHandler) *ProjectHandler {
	return &ProjectHandler{db: database, sse: sse}
}

type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RepoPath    string `json:"repo_path"`
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.db.ListProjects()
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if projects == nil {
		projects = []models.Project{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"projects": projects})
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	p, err := h.db.GetProject(id)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(p)
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	// Get or auto-create
	p, err := h.db.GetOrCreateProject(req.Name)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Set repo_path if provided and project is new or has no repo_path
	if req.RepoPath != "" && p.RepoPath == "" {
		p.RepoPath = req.RepoPath
		if err := h.db.UpdateProject(p); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
	}

	BroadcastTaskEvent(h.sse, models.EventProjectCreated, p)
	json.NewEncoder(w).Encode(p)
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	p, err := h.db.GetProject(id)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.RepoPath != "" {
		p.RepoPath = req.RepoPath
	}

	if err := h.db.UpdateProject(p); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	BroadcastTaskEvent(h.sse, models.EventProjectUpdated, p)
	json.NewEncoder(w).Encode(p)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.db.DeleteProject(id); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	BroadcastTaskEvent(h.sse, models.EventProjectDeleted, map[string]string{"id": id})
	w.WriteHeader(http.StatusNoContent)
}

// GetOrCreateByRepo looks up or creates a project by repo_path.
// GET /projects/by-repo?repo_path=/path/to/repo
func (h *ProjectHandler) GetOrCreateByRepo(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo_path")
	if repoPath == "" {
		http.Error(w, `{"error":"repo_path query parameter is required"}`, http.StatusBadRequest)
		return
	}
	p, err := h.db.GetOrCreateByRepoPath(repoPath)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(p)
}

func (h *ProjectHandler) Register(router *mux.Router) {
	router.HandleFunc("/projects", h.List).Methods("GET")
	router.HandleFunc("/projects", h.Create).Methods("POST")
	router.HandleFunc("/projects/by-repo", h.GetOrCreateByRepo).Methods("GET")
	router.HandleFunc("/projects/{id}", h.Get).Methods("GET")
	router.HandleFunc("/projects/{id}", h.Update).Methods("PUT")
	router.HandleFunc("/projects/{id}", h.Delete).Methods("DELETE")
}
