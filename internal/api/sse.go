package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/rogerrlee/tasks-watcher/internal/models"
)

type SSEHandler struct {
	clients map[chan models.SSEEvent]struct{}
	mu      sync.RWMutex
	apiKey  string
}

func NewSSEHandler(apiKey string) *SSEHandler {
	return &SSEHandler{clients: make(map[chan models.SSEEvent]struct{}), apiKey: apiKey}
}

func (s *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If request passed through AuthMiddleware, trust its session or CLI auth.
	// This handles both web UI (session cookie → ContextKeyUserID) and
	// CLI (?api_key= → ContextKeyIsCLI). Direct callers bypassing the
	// middleware still need to provide a valid API key.
	if r.Context().Value(ContextKeyUserID) != nil || r.Context().Value(ContextKeyIsCLI) != nil {
		// Authenticated via middleware — pass through
	} else {
		key := r.URL.Query().Get("api_key")
		if key == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if key != s.apiKey {
			http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
			return
		}
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	eventChan := make(chan models.SSEEvent, 100)
	s.mu.Lock()
	s.clients[eventChan] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, eventChan)
		s.mu.Unlock()
		close(eventChan)
	}()

	// Send initial ping
	ping := "data: ping\n\n"
	if _, err := w.Write([]byte(ping)); err == nil {
		flusher.Flush()
	}

	clientGone := r.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			msg := "data: " + string(data) + "\n\n"
			if _, err := w.Write([]byte(msg)); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *SSEHandler) Broadcast(event models.SSEEvent) {
	// Snapshot channels under read lock, then send outside the lock
	// to avoid holding the lock while sending (which could block).
	s.mu.RLock()
	channels := make([]chan models.SSEEvent, 0, len(s.clients))
	for ch := range s.clients {
		channels = append(channels, ch)
	}
	s.mu.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- event:
		default:
			// Drop if channel buffer is full (slow consumer)
		}
	}
}

// BroadcastTaskEvent is a helper to broadcast a task event
func BroadcastTaskEvent(sse *SSEHandler, eventType string, payload interface{}) {
	if sse == nil {
		return
	}
	sse.Broadcast(models.SSEEvent{
		Type:    eventType,
		Payload: payload,
		Time:    models.Now(),
	})
}

func (s *SSEHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	s.ServeHTTP(w, r)
}
