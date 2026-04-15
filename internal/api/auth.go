package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/auth"
	"github.com/rogerrlee/tasks-watcher/internal/config"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

type contextKey string

const (
	ContextKeyUserID contextKey = "user_id"
	ContextKeyIsCLI  contextKey = "is_cli"
)

type AuthMiddleware struct {
	apiKey    string
	jwtSecret string
	db        *db.DB
}

func NewAuthMiddleware(cfg *config.Config, database *db.DB) *AuthMiddleware {
	return &AuthMiddleware{apiKey: cfg.APIKey, jwtSecret: cfg.JWTSecret, db: database}
}

// Authenticate returns a mux middleware function that supports two auth modes:
//   - CLI / Agent: Authorization: Bearer <api_key>
//   - Web UI:       HttpOnly cookie "session_token" = <jwt>
func (a *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Skip auth entirely for public endpoints
		if path == "/api/key" || path == "/api/health" ||
			strings.HasPrefix(path, "/static") ||
			(path == "/api/auth/login" || path == "/api/auth/register") {
			next.ServeHTTP(w, r)
			return
		}

		// SSE endpoint: check cookie (web UI) or ?api_key= (CLI)
		if path == "/api/events" {
			if token := r.URL.Query().Get("api_key"); token != "" && token == a.apiKey {
				ctx := context.WithValue(r.Context(), ContextKeyIsCLI, true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			if token := getSessionToken(r); token != "" {
				userID, ok := a.validateSessionToken(token)
				if ok {
					ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// /auth/me requires a valid session cookie
		if path == "/api/auth/me" {
			if token := getSessionToken(r); token != "" {
				userID, ok := a.validateSessionToken(token)
				if ok {
					ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Check Bearer token (CLI / Agent mode)
		authHdr := r.Header.Get("Authorization")
		if authHdr != "" && strings.HasPrefix(authHdr, "Bearer ") {
			token := strings.TrimPrefix(authHdr, "Bearer ")
			if token == a.apiKey {
				ctx := context.WithValue(r.Context(), ContextKeyIsCLI, true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Check ?api_key= query param (CLI / Agent mode, for non-SSE paths)
		if apiKey := r.URL.Query().Get("api_key"); apiKey != "" && apiKey == a.apiKey {
			ctx := context.WithValue(r.Context(), ContextKeyIsCLI, true)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Check session cookie (Web UI mode)
		token := getSessionToken(r)
		if token != "" {
			userID, ok := a.validateSessionToken(token)
			if ok {
				ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	})
}

func getSessionToken(r *http.Request) string {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (a *AuthMiddleware) validateSessionToken(token string) (string, bool) {
	userID, ok := auth.ValidateToken(token, a.jwtSecret)
	if !ok {
		return "", false
	}
	tokenHash := auth.HashToken(token)
	if a.db.IsSessionDenied(tokenHash) {
		return "", false
	}
	return userID, true
}

// GetUserID extracts the authenticated user ID from context.
func GetUserID(r *http.Request) string {
	if v := r.Context().Value(ContextKeyUserID); v != nil {
		return v.(string)
	}
	return ""
}

// GetIsCLI returns true if the request used CLI/Agent API key auth.
func GetIsCLI(r *http.Request) bool {
	if v := r.Context().Value(ContextKeyIsCLI); v != nil {
		return v.(bool)
	}
	return false
}

// --- Auth API Handler ---

type AuthAPIHandler struct {
	db        *db.DB
	jwtSecret string
}

func NewAuthAPIHandler(database *db.DB, jwtSecret string) *AuthAPIHandler {
	return &AuthAPIHandler{db: database, jwtSecret: jwtSecret}
}

func (h *AuthAPIHandler) Register(router *mux.Router) {
	r := router.PathPrefix("/auth").Subrouter()
	r.HandleFunc("/register", h.HandleRegister).Methods("POST")
	r.HandleFunc("/login", h.HandleLogin).Methods("POST")
	r.HandleFunc("/logout", h.HandleLogout).Methods("POST")
	r.HandleFunc("/me", h.HandleMe).Methods("GET")
}

type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthAPIHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if len(req.Username) < 3 || len(req.Username) > 32 {
		http.Error(w, `{"error":"username must be 3-32 characters"}`, http.StatusBadRequest)
		return
	}
	if !isValidUsername(req.Username) {
		http.Error(w, `{"error":"username must be alphanumeric or underscore only"}`, http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 128 {
		http.Error(w, `{"error":"password must be 8-128 characters"}`, http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, `{"error":"failed to hash password"}`, http.StatusInternalServerError)
		return
	}

	u := &models.User{Username: req.Username, PasswordHash: hash}
	if err := h.db.CreateUser(u); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			http.Error(w, `{"error":"username already taken"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"failed to create user"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": map[string]interface{}{"id": u.ID, "username": u.Username, "created_at": u.CreatedAt},
	})
}

func (h *AuthAPIHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, `{"error":"username and password are required"}`, http.StatusBadRequest)
		return
	}

	u, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if u == nil || !auth.VerifyPassword(req.Password, u.PasswordHash) {
		// Constant-time response to prevent timing attacks
		auth.VerifyPassword("dummy", "$2a$12$000000000000000000000uQkO00000000000000000000000000")
		http.Error(w, `{"error":"invalid username or password"}`, http.StatusUnauthorized)
		return
	}

	session, token, err := auth.GenerateSession(u.ID, h.jwtSecret)
	if err != nil {
		http.Error(w, `{"error":"failed to create session"}`, http.StatusInternalServerError)
		return
	}
	if err := h.db.CreateSession(session); err != nil {
		http.Error(w, `{"error":"failed to save session"}`, http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, r, token)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": map[string]interface{}{"id": u.ID, "username": u.Username},
	})
}

func (h *AuthAPIHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	token := getSessionToken(r)
	if token != "" {
		tokenHash := auth.HashToken(token)
		h.db.DenySession(tokenHash)
		h.db.CleanExpiredSessions()
	}
	clearSessionCookie(w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *AuthAPIHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r)
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	u, err := h.db.GetUserByID(userID)
	if err != nil || u == nil {
		// Treat lookup errors as unauthenticated so the frontend
		// correctly redirects to the login page instead of showing the app.
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": map[string]interface{}{"id": u.ID, "username": u.Username, "created_at": u.CreatedAt},
	})
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		MaxAge:   60 * 60 * 24, // 24 hours
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func isValidUsername(s string) bool {
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}
