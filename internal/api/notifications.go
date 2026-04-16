package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

type NotificationHandler struct {
	db *db.DB
}

func NewNotificationHandler(database *db.DB) *NotificationHandler {
	return &NotificationHandler{db: database}
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	notifs, err := h.db.ListNotifications(100)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	if notifs == nil {
		notifs = []models.Notification{}
	}
	count, err := h.db.GetUnreadNotificationCount()
	if err != nil {
		log.Printf("GetUnreadNotificationCount failed: %v", err)
		count = 0
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"notifications": notifs,
		"unread_count":   count,
	})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.db.MarkNotificationRead(id); err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	if err := h.db.MarkAllNotificationsRead(); err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) Clear(w http.ResponseWriter, r *http.Request) {
	if err := h.db.ClearNotifications(); err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) Register(router *mux.Router) {
	router.HandleFunc("/notifications", h.List).Methods("GET")
	router.HandleFunc("/notifications/read", h.MarkAllRead).Methods("POST")
	router.HandleFunc("/notifications/{id}/read", h.MarkRead).Methods("PATCH")
	router.HandleFunc("/notifications", h.Clear).Methods("DELETE")
}
