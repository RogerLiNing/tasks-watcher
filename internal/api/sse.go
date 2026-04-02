package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/rogerrlee/tasks-watcher/internal/models"
)

type SSEHandler struct {
	clients map[chan models.SSEEvent]struct{}
	mu      sync.RWMutex
}

func NewSSEHandler() *SSEHandler {
	return &SSEHandler{clients: make(map[chan models.SSEEvent]struct{})}
}

func (s *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	for ch := range s.clients {
		select {
		case ch <- event:
		default:
			// Drop if channel is full
		}
	}
}

// BroadcastTaskEvent is a helper to broadcast a task event
func BroadcastTaskEvent(sse *SSEHandler, eventType string, payload interface{}) {
	sse.Broadcast(models.SSEEvent{
		Type:    eventType,
		Payload: payload,
		Time:    models.Now(),
	})
}

func (s *SSEHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	s.ServeHTTP(w, r)
}
