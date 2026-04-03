package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetAPIKeyPath(t *testing.T) {
	path := GetAPIKeyPath("/data/dir")
	if path != filepath.Join("/data/dir", "api.key") {
		t.Errorf("expected /data/dir/api.key, got %s", path)
	}
}

func TestRegenerateAPIKey(t *testing.T) {
	tmpDir := t.TempDir()

	key1, err := RegenerateAPIKey(tmpDir)
	if err != nil {
		t.Fatalf("RegenerateAPIKey failed: %v", err)
	}
	if len(key1) != 64 { // 32 bytes hex-encoded = 64 chars
		t.Errorf("expected 64-char hex key, got %d chars", len(key1))
	}

	// Verify file was written
	keyPath := filepath.Join(tmpDir, "api.key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("failed to read key file: %v", err)
	}
	if string(data) != key1 {
		t.Errorf("key file content mismatch")
	}

	// Calling again should generate a different key
	key2, err := RegenerateAPIKey(tmpDir)
	if err != nil {
		t.Fatalf("RegenerateAPIKey second call failed: %v", err)
	}
	if key1 == key2 {
		t.Error("expected different key on second call")
	}
}

func TestRegenerateAPIKey_WriteError(t *testing.T) {
	// Try to write to a non-existent path to trigger error
	_, err := RegenerateAPIKey("/nonexistent/path/that/cannot/be/created")
	if err == nil {
		t.Error("expected error for non-writable path")
	}
}

func TestLoad_WithEnvVars(t *testing.T) {
	tmpDir := t.TempDir()

	// Set env vars so Load uses our temp directory
	os.Setenv("TASKS_WATCHER_DATA_DIR", tmpDir)
	os.Setenv("TASKS_WATCHER_PORT", "9999")
	os.Setenv("TASKS_WATCHER_DB_PATH", filepath.Join(tmpDir, "test.db"))
	os.Setenv("TASKS_WATCHER_NOTIFY", "false")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_DATA_DIR")
		os.Unsetenv("TASKS_WATCHER_PORT")
		os.Unsetenv("TASKS_WATCHER_DB_PATH")
		os.Unsetenv("TASKS_WATCHER_NOTIFY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Port != "9999" {
		t.Errorf("expected port 9999, got %s", cfg.Port)
	}
	if cfg.DBPath != filepath.Join(tmpDir, "test.db") {
		t.Errorf("expected db path %s, got %s", filepath.Join(tmpDir, "test.db"), cfg.DBPath)
	}
	if cfg.Notify != false {
		t.Error("expected Notify=false")
	}
	if cfg.APIKey == "" {
		t.Error("expected non-empty API key")
	}
	if cfg.DataDir != tmpDir {
		t.Errorf("expected data dir %s, got %s", tmpDir, cfg.DataDir)
	}
}

func TestLoad_DefaultValues(t *testing.T) {
	tmpDir := t.TempDir()

	// Only set DATA_DIR; port, db_path, notify should use defaults
	os.Setenv("TASKS_WATCHER_DATA_DIR", tmpDir)
	os.Unsetenv("TASKS_WATCHER_PORT")
	os.Unsetenv("TASKS_WATCHER_DB_PATH")
	os.Unsetenv("TASKS_WATCHER_NOTIFY")
	defer os.Unsetenv("TASKS_WATCHER_DATA_DIR")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Port != "4242" {
		t.Errorf("expected default port 4242, got %s", cfg.Port)
	}
	if cfg.DBPath != filepath.Join(tmpDir, "tasks.db") {
		t.Errorf("expected default db path, got %s", cfg.DBPath)
	}
	if cfg.Notify != true {
		t.Error("expected default Notify=true")
	}
}

func TestLoad_UsesGetDataDir(t *testing.T) {
	// Do NOT set TASKS_WATCHER_DATA_DIR so Load falls through to getDataDir()
	os.Unsetenv("TASKS_WATCHER_DATA_DIR")
	os.Unsetenv("TASKS_WATCHER_PORT")
	os.Unsetenv("TASKS_WATCHER_DB_PATH")
	os.Unsetenv("TASKS_WATCHER_NOTIFY")
	defer func() {
		os.Unsetenv("TASKS_WATCHER_DATA_DIR")
		os.Unsetenv("TASKS_WATCHER_PORT")
		os.Unsetenv("TASKS_WATCHER_DB_PATH")
		os.Unsetenv("TASKS_WATCHER_NOTIFY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".tasks-watcher")
	if cfg.DataDir != expected {
		t.Errorf("expected data dir %s, got %s", expected, cfg.DataDir)
	}
	if cfg.DBPath != filepath.Join(expected, "tasks.db") {
		t.Errorf("expected db path under %s, got %s", expected, cfg.DBPath)
	}
}
