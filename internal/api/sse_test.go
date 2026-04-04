package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rogerrlee/tasks-watcher/internal/models"
)

func TestSSEHandler_ServeHTTP_InvalidAPIKey(t *testing.T) {
	handler := NewSSEHandler("correct-key")

	req := httptest.NewRequest("GET", "/events?api_key=wrong-key", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSSEHandler_ServeHTTP_MissingAPIKey(t *testing.T) {
	handler := NewSSEHandler("correct-key")

	req := httptest.NewRequest("GET", "/events", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSSEHandler_ServeHTTP_BearerToken_Wrong(t *testing.T) {
	handler := NewSSEHandler("correct-key")

	req := httptest.NewRequest("GET", "/events", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSSEHandler_ServeHTTP_BearerToken_Correct(t *testing.T) {
	handler := NewSSEHandler("correct-key")

	req := httptest.NewRequest("GET", "/events", nil)
	req.Header.Set("Authorization", "Bearer correct-key")
	// Set a deadline so ServeHTTP doesn't block forever (SSE loop)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", ct)
	}
}

func TestSSEHandler_ServeHTTP_ApiKeyQueryParam_Correct(t *testing.T) {
	handler := NewSSEHandler("correct-key")

	req := httptest.NewRequest("GET", "/events?api_key=correct-key", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestSSEHandler_Broadcast_DoesNotPanic(t *testing.T) {
	handler := NewSSEHandler("key")
	handler.Broadcast(models.SSEEvent{Type: "task.created", Payload: nil})
}

func TestBroadcastTaskEvent_NilSSE(t *testing.T) {
	BroadcastTaskEvent(nil, "task.created", nil)
}

func TestSSEHandler_HandleSSE(t *testing.T) {
	handler := NewSSEHandler("test-key")

	req := httptest.NewRequest("GET", "/events?api_key=wrong", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.HandleSSE(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSSEHandler_Broadcast_DropsSlowConsumer(t *testing.T) {
	handler := NewSSEHandler("test-key")

	blockedChan := make(chan models.SSEEvent)
	handler.mu.Lock()
	handler.clients[blockedChan] = struct{}{}
	handler.mu.Unlock()

	handler.Broadcast(models.SSEEvent{Type: "task.created"})

	handler.mu.Lock()
	delete(handler.clients, blockedChan)
	handler.mu.Unlock()
}
