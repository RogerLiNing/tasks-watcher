package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/rogerrlee/tasks-watcher/internal/api"
	"github.com/rogerrlee/tasks-watcher/internal/config"
	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/notifications"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	sse := api.NewSSEHandler(cfg.APIKey)
	dispatcher := notifications.NewDispatcher(database, sse)

	router := mux.NewRouter()
	auth := api.NewAuthMiddleware(cfg, database)

	// SSE endpoint (auth via query param ?api_key=... OR via session cookie — both handled by auth middleware)

	// Health check (no auth)
	router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")

	// API key endpoint (no auth — local tool, key already on disk)
	router.HandleFunc("/api/key", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"api_key": cfg.APIKey})
	}).Methods("GET")

	// Protected API routes
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(auth.Authenticate)

	// SSE endpoint inside protected router (auth via ?api_key= OR cookie)
	apiRouter.HandleFunc("/events", sse.ServeHTTP).Methods("GET")

	// API handlers
	projectHandler := api.NewProjectHandler(database, sse)
	taskHandler := api.NewTaskHandler(database, sse, dispatcher)
	notifHandler := api.NewNotificationHandler(database)
	notifConfigHandler := api.NewNotificationConfigHandler(database)
	agentHandler := api.NewAgentHandler(database)
	webhookHandler := api.NewWebhookHandler(database)
	depHandler := api.NewDepHandler(database, sse)
	subtaskHandler := api.NewSubtaskHandler(database, sse)
	columnHandler := api.NewColumnHandler(database, sse)
	commentHandler := api.NewCommentHandler(database, sse)
	authHandler := api.NewAuthAPIHandler(database, cfg.JWTSecret)

	// Register on subrouter — handlers use paths WITHOUT /api prefix (subrouter handles it)
	authHandler.Register(apiRouter)
	projectHandler.Register(apiRouter)
	taskHandler.Register(apiRouter)
	notifHandler.Register(apiRouter)
	notifConfigHandler.Register(apiRouter)
	agentHandler.Register(apiRouter)
	webhookHandler.Register(apiRouter)
	depHandler.Register(apiRouter)
	subtaskHandler.Register(apiRouter)
	columnHandler.Register(apiRouter)
	commentHandler.Register(apiRouter)

	// Config endpoint
	apiRouter.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"api_key_hint": "Use your API key from ~/.tasks-watcher/api.key",
		})
	}).Methods("GET")

	// Export endpoint
	apiRouter.HandleFunc("/export", func(w http.ResponseWriter, r *http.Request) {
		data, err := database.ExportAll()
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}).Methods("GET")

	// Serve embedded web app
	webDist := findWebDist()
	if webDist != "" {
		router.PathPrefix("/").Handler(http.FileServer(http.Dir(webDist)))
	}

	addr := ":" + cfg.Port
	fmt.Printf("\n[Tasks Watcher] Server running at http://localhost:%s\n", cfg.Port)
	fmt.Printf("[Tasks Watcher] Data directory: %s\n", cfg.DataDir)
	fmt.Printf("[Tasks Watcher] API key: %s… [first 8 chars]\n\n", cfg.APIKey[:8])

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func findWebDist() string {
	paths := []string{
		"web/dist",
		"../web/dist",
		filepath.Join(os.Getenv("PWD"), "web/dist"),
	}
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return ""
}
