package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DBPath      string
	Port        string
	APIKey      string
	JWTSecret   string
	Notify      bool
	DataDir     string
	WebhookDir  string
}

func getDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tasks-watcher")
}

func Load() (*Config, error) {
	dataDir := os.Getenv("TASKS_WATCHER_DATA_DIR")
	if dataDir == "" {
		dataDir = getDataDir()
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	dbPath := os.Getenv("TASKS_WATCHER_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "tasks.db")
	}

	port := os.Getenv("TASKS_WATCHER_PORT")
	if port == "" {
		port = "4242"
	}

	notify := os.Getenv("TASKS_WATCHER_NOTIFY")
	if notify == "" {
		notify = "true"
	}

	apiKey, err := loadOrCreateAPIKey(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	jwtSecret, err := loadOrCreateJWTSecret(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWT secret: %w", err)
	}

	return &Config{
		DBPath:     dbPath,
		Port:       port,
		APIKey:     apiKey,
		JWTSecret:  jwtSecret,
		Notify:     notify == "true",
		DataDir:    dataDir,
		WebhookDir: filepath.Join(dataDir, "webhooks"),
	}, nil
}

func loadOrCreateAPIKey(dataDir string) (string, error) {
	keyPath := filepath.Join(dataDir, "api.key")

	// Check if key file exists
	if data, err := os.ReadFile(keyPath); err == nil {
		return string(data), nil
	}

	// Generate a new API key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}
	key := hex.EncodeToString(keyBytes)

	// Write to file with restricted permissions
	if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
		return "", fmt.Errorf("failed to write API key: %w", err)
	}

	fmt.Printf("\n[Tasks Watcher] API key generated: %s\n", key)
	fmt.Printf("[Tasks Watcher] API key saved to: %s\n", keyPath)
	fmt.Printf("[Tasks Watcher] Key (for reference): %s...\n\n", key[:16])

	return key, nil
}

func GetAPIKeyPath(dataDir string) string {
	return filepath.Join(dataDir, "api.key")
}

func RegenerateAPIKey(dataDir string) (string, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}
	key := hex.EncodeToString(keyBytes)

	keyPath := filepath.Join(dataDir, "api.key")
	if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
		return "", fmt.Errorf("failed to write API key: %w", err)
	}

	return key, nil
}

func loadOrCreateJWTSecret(dataDir string) (string, error) {
	secretPath := filepath.Join(dataDir, "jwt.secret")
	if data, err := os.ReadFile(secretPath); err == nil && len(data) >= 32 {
		return strings.TrimSpace(string(data)), nil
	}
	// Generate a new secret
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", fmt.Errorf("failed to generate JWT secret: %w", err)
	}
	secret := hex.EncodeToString(secretBytes)
	if err := os.WriteFile(secretPath, []byte(secret), 0600); err != nil {
		return "", fmt.Errorf("failed to write JWT secret: %w", err)
	}
	return secret, nil
}
