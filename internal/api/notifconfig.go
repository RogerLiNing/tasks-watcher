package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

type NotificationConfigHandler struct {
	db *db.DB
}

func NewNotificationConfigHandler(database *db.DB) *NotificationConfigHandler {
	return &NotificationConfigHandler{db: database}
}

type UpsertNotifConfigRequest struct {
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

func (h *NotificationConfigHandler) List(w http.ResponseWriter, r *http.Request) {
	configs, err := h.db.ListNotificationConfigs()
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if configs == nil {
		configs = []models.NotificationConfig{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"configs": configs})
}

func (h *NotificationConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	notifType := mux.Vars(r)["type"]
	c, err := h.db.GetNotificationConfig(notifType)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.Error(w, `{"error":"config not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(c)
}

func (h *NotificationConfigHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req UpsertNotifConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		http.Error(w, `{"error":"type is required"}`, http.StatusBadRequest)
		return
	}
	if req.Type != "macos" && req.Type != "email" {
		http.Error(w, `{"error":"type must be macos or email"}`, http.StatusBadRequest)
		return
	}

	c := &models.NotificationConfig{
		Type:      req.Type,
		Enabled:   req.Enabled,
		Config:    req.Config,
		CreatedAt: models.Now(),
	}

	if err := h.db.UpsertNotificationConfig(c); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	updated, _ := h.db.GetNotificationConfig(req.Type)
	json.NewEncoder(w).Encode(updated)
}

func (h *NotificationConfigHandler) Register(router *mux.Router) {
	r := router.PathPrefix("/notifications/configs").Subrouter()
	r.HandleFunc("", h.List).Methods("GET")
	r.HandleFunc("", h.Upsert).Methods("POST")
	r.HandleFunc("/{type}", h.Get).Methods("GET")
}
