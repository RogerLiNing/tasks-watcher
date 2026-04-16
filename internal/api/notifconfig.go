package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

// maskSensitiveFields replaces sensitive config values (passwords, secrets)
// with a placeholder before sending to the client.
func maskSensitiveFields(c models.NotificationConfig) models.NotificationConfig {
	if c.Config == nil {
		return c
	}
	cfg := make(map[string]interface{}, len(c.Config))
	for k, v := range c.Config {
		switch k {
		case "smtp_password", "password", "secret", "api_key":
			if v != "" {
				cfg[k] = "******"
			} else {
				cfg[k] = ""
			}
		default:
			cfg[k] = v
		}
	}
	c.Config = cfg
	return c
}

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
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	if configs == nil {
		configs = []models.NotificationConfig{}
	}
	out := make([]models.NotificationConfig, len(configs))
	for i := range configs {
		out[i] = maskSensitiveFields(configs[i])
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"configs": out})
}

func (h *NotificationConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	notifType := mux.Vars(r)["type"]
	c, err := h.db.GetNotificationConfig(notifType)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	if c == nil {
		http.Error(w, `{"error":"config not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(maskSensitiveFields(*c))
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
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}

	updated, err := h.db.GetNotificationConfig(req.Type)
	if err != nil {
		log.Printf("GetNotificationConfig(%s) failed after upsert: %v", req.Type, err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(updated)
}

func (h *NotificationConfigHandler) Register(router *mux.Router) {
	r := router.PathPrefix("/notifications/configs").Subrouter()
	r.HandleFunc("", h.List).Methods("GET")
	r.HandleFunc("", h.Upsert).Methods("POST")
	r.HandleFunc("/{type}", h.Get).Methods("GET")
}
