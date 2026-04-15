package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

// CommentHandler handles task comment endpoints.
type CommentHandler struct {
	db  *db.DB
	sse *SSEHandler
}

// NewCommentHandler creates a new CommentHandler.
func NewCommentHandler(database *db.DB, sse *SSEHandler) *CommentHandler {
	return &CommentHandler{db: database, sse: sse}
}

// Register registers comment routes under /tasks/{id}/comments.
func (h *CommentHandler) Register(router *mux.Router) {
	r := router.PathPrefix("/tasks/{id}").Subrouter()
	r.HandleFunc("/comments", h.ListComments).Methods("GET")
	r.HandleFunc("/comments", h.CreateComment).Methods("POST")
	r.HandleFunc("/comments/{commentId}", h.UpdateComment).Methods("PUT")
	r.HandleFunc("/comments/{commentId}", h.DeleteComment).Methods("DELETE")
}

type createCommentReq struct {
	Content string `json:"content"`
}

type updateCommentReq struct {
	Content string `json:"content"`
}

func (h *CommentHandler) ListComments(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["id"]
	if taskID == "" {
		http.Error(w, `{"error":"task_id required"}`, http.StatusBadRequest)
		return
	}

	comments, err := h.db.ListComments(taskID)
	if err != nil {
		log.Printf("ListComments(%s) failed: %v", taskID, err)
		http.Error(w, `{"error":"failed to list comments"}`, http.StatusInternalServerError)
		return
	}
	if comments == nil {
		comments = []models.TaskComment{}
	}

	// Enrich with author usernames in a single batch query
	authorIDs := make([]string, len(comments))
	for i, c := range comments {
		authorIDs[i] = c.Author
	}
	if users, err := h.db.GetUsersByIDs(authorIDs); err == nil {
		for i := range comments {
			if u, ok := users[comments[i].Author]; ok {
				comments[i].AuthorUsername = u.Username
			}
		}
	} else {
		log.Printf("ListComments: GetUsersByIDs failed: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"comments": comments})
}

func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["id"]
	if taskID == "" {
		http.Error(w, `{"error":"task_id required"}`, http.StatusBadRequest)
		return
	}

	var req createCommentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		http.Error(w, `{"error":"content is required"}`, http.StatusBadRequest)
		return
	}

	author := GetUserID(r)
	if author == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	c := &models.TaskComment{
		TaskID:  taskID,
		Author:  author,
		Content: req.Content,
	}
	if err := h.db.CreateComment(c); err != nil {
		log.Printf("CreateComment(%s) failed: %v", taskID, err)
		http.Error(w, `{"error":"failed to create comment"}`, http.StatusInternalServerError)
		return
	}

	// Enrich with author username for the response
	if u, err := h.db.GetUserByID(author); err == nil && u != nil {
		c.AuthorUsername = u.Username
	}

	BroadcastTaskEvent(h.sse, models.EventCommentAdded, c)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

func (h *CommentHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	commentID := mux.Vars(r)["commentId"]
	if commentID == "" {
		http.Error(w, `{"error":"comment_id required"}`, http.StatusBadRequest)
		return
	}

	var req updateCommentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		http.Error(w, `{"error":"content is required"}`, http.StatusBadRequest)
		return
	}

	c, err := h.db.GetComment(commentID)
	if err != nil {
		log.Printf("GetComment(%s) failed: %v", commentID, err)
		http.Error(w, `{"error":"failed to get comment"}`, http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.Error(w, `{"error":"comment not found"}`, http.StatusNotFound)
		return
	}

	// Only content is updatable; author is preserved from creation.
	c.Content = req.Content
	if err := h.db.UpdateComment(c); err != nil {
		log.Printf("UpdateComment(%s) failed: %v", commentID, err)
		http.Error(w, `{"error":"failed to update comment"}`, http.StatusInternalServerError)
		return
	}

	// Re-fetch for broadcast (enrich with username)
	if u, err := h.db.GetUserByID(c.Author); err == nil && u != nil {
		c.AuthorUsername = u.Username
	}

	BroadcastTaskEvent(h.sse, models.EventCommentUpdated, c)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

func (h *CommentHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	commentID := mux.Vars(r)["commentId"]
	if commentID == "" {
		http.Error(w, `{"error":"comment_id required"}`, http.StatusBadRequest)
		return
	}

	c, err := h.db.GetComment(commentID)
	if err != nil {
		log.Printf("GetComment(%s) in DeleteComment failed: %v", commentID, err)
		http.Error(w, `{"error":"failed to get comment"}`, http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.Error(w, `{"error":"comment not found"}`, http.StatusNotFound)
		return
	}

	if err := h.db.DeleteComment(commentID); err != nil {
		log.Printf("DeleteComment(%s) failed: %v", commentID, err)
		http.Error(w, `{"error":"failed to delete comment"}`, http.StatusInternalServerError)
		return
	}

	BroadcastTaskEvent(h.sse, models.EventCommentDeleted, map[string]string{"id": commentID, "task_id": mux.Vars(r)["id"]})
	w.WriteHeader(http.StatusNoContent)
}
