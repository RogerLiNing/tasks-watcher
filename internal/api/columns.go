package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

type ColumnHandler struct {
	db  *db.DB
	sse *SSEHandler
}

func NewColumnHandler(database *db.DB, sse *SSEHandler) *ColumnHandler {
	return &ColumnHandler{db: database, sse: sse}
}

type createColumnReq struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Color    string `json:"color"`
	Position int    `json:"position"`
}

type updateColumnReq struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Color    string `json:"color"`
	Position int    `json:"position"`
}

func (h *ColumnHandler) List(w http.ResponseWriter, r *http.Request) {
	cols, err := h.db.ListColumns()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"columns": cols})
}

func (h *ColumnHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createColumnReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Label == "" {
		http.Error(w, `{"error":"label is required"}`, http.StatusBadRequest)
		return
	}
	if req.Color == "" {
		req.Color = "#86868b"
	}
	cols, _ := h.db.ListColumns()
	position := req.Position
	if position == 0 {
		position = len(cols)
	}
	key := req.Key
	if key == "" {
		key = slugify(req.Label)
	}
	// Deduplicate: append _2, _3, etc. if key already exists
	usedKeys := make(map[string]bool)
	for _, c := range cols {
		usedKeys[c.Key] = true
	}
	base := key
	counter := 2
	for usedKeys[key] {
		key = fmt.Sprintf("%s_%d", base, counter)
		counter++
	}
	c := &models.TaskColumn{
		Key:      key,
		Label:    req.Label,
		Color:   req.Color,
		Position: position,
	}
	if err := h.db.CreateColumn(c); err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	BroadcastTaskEvent(h.sse, models.EventColumnCreated, c)
	json.NewEncoder(w).Encode(c)
}

func (h *ColumnHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req updateColumnReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	cols, err := h.db.ListColumns()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	var col models.TaskColumn
	found := false
	for _, c := range cols {
		if c.ID == id {
			col = c
			found = true
			break
		}
	}
	if !found {
		http.Error(w, `{"error":"column not found"}`, http.StatusNotFound)
		return
	}
	if req.Label != "" {
		col.Label = req.Label
	}
	if req.Color != "" {
		col.Color = req.Color
	}
	col.Position = req.Position
	if err := h.db.UpdateColumn(&col); err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	BroadcastTaskEvent(h.sse, models.EventColumnUpdated, &col)
	json.NewEncoder(w).Encode(&col)
}

func (h *ColumnHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	// Check column exists
	cols, err := h.db.ListColumns()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	found := false
	for _, c := range cols {
		if c.ID == id {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, `{"error":"column not found"}`, http.StatusNotFound)
		return
	}
	if err := h.db.DeleteColumn(id); err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		return
	}
	BroadcastTaskEvent(h.sse, models.EventColumnDeleted, map[string]string{"id": id})
	w.WriteHeader(http.StatusNoContent)
}

func (h *ColumnHandler) Register(router *mux.Router) {
	r := router.PathPrefix("/columns").Subrouter()
	r.HandleFunc("", h.List).Methods("GET")
	r.HandleFunc("", h.Create).Methods("POST")
	r.HandleFunc("/{id}", h.Update).Methods("PUT")
	r.HandleFunc("/{id}", h.Delete).Methods("DELETE")
}

// slugify converts a label to a URL-safe slug key.
// Non-ASCII characters are preserved; spaces/special chars become underscores.
func slugify(label string) string {
	// Replace non-alphanumeric runs (except ASCII letters/numbers) with underscore
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug := strings.Trim(re.ReplaceAllString(strings.ToLower(label), "_"), "_")
	// If slug is empty (e.g. pure CJK), use a prefix so we still get a key
	if slug == "" {
		return "col"
	}
	return slug
}
