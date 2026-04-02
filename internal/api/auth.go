package api

import (
	"net/http"
	"strings"

	"github.com/rogerrlee/tasks-watcher/internal/config"
)

type AuthMiddleware struct {
	apiKey string
}

func NewAuthMiddleware(cfg *config.Config) *AuthMiddleware {
	return &AuthMiddleware{apiKey: cfg.APIKey}
}

// Authenticate returns a mux middleware function
func (a *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for SSE endpoint, health, static files, and key endpoint
		path := r.URL.Path
		if path == "/api/key" || path == "/api/events" || path == "/api/health" || strings.HasPrefix(path, "/static") {
			next.ServeHTTP(w, r)
			return
		}

		// Check Bearer token
		auth := r.Header.Get("Authorization")
		if auth == "" {
			// Also check query param for SSE reconnection
			token := r.URL.Query().Get("api_key")
			if token != "" && token == a.apiKey {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, `{"error":"invalid Authorization format"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if token != a.apiKey {
			http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
