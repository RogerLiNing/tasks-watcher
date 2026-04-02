package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

type WebhookHandler struct {
	db *db.DB
}

func NewWebhookHandler(database *db.DB) *WebhookHandler {
	return &WebhookHandler{db: database}
}

type CreateWebhookRequest struct {
	URL    string `json:"url"`
	Events string `json:"events"`
	Active bool   `json:"active"`
}

func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	webhooks, err := h.db.ListWebhooks()
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	type webhookList struct {
		Webhooks []models.WebhookConfig `json:"webhooks"`
	}
	if webhooks == nil {
		webhooks = []models.WebhookConfig{}
	}
	json.NewEncoder(w).Encode(webhookList{Webhooks: webhooks})
}

func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, `{"error":"url is required"}`, http.StatusBadRequest)
		return
	}
	if req.Events == "" {
		req.Events = "task.*"
	}

	wh := &models.WebhookConfig{
		URL:    req.URL,
		Events: req.Events,
		Active: req.Active,
	}
	if err := h.db.CreateWebhook(wh); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(wh)
}

func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.db.DeleteWebhook(id); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WebhookHandler) Register(router *mux.Router) {
	router.HandleFunc("/webhooks", h.List).Methods("GET")
	router.HandleFunc("/webhooks", h.Create).Methods("POST")
	router.HandleFunc("/webhooks/{id}", h.Delete).Methods("DELETE")
}
