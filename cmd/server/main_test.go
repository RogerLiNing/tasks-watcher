package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindWebDist_NotFound(t *testing.T) {
	// When no web/dist exists, should return ""
	os.Unsetenv("PWD")
	got := findWebDist()
	if got != "" {
		t.Errorf("findWebDist() = %q, want empty string", got)
	}
}

func TestFindWebDist_Found(t *testing.T) {
	tmpDir := t.TempDir()
	webDist := filepath.Join(tmpDir, "web", "dist")
	if err := os.MkdirAll(webDist, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a dummy index.html so it's recognized as a dir with content
	f, err := os.Create(filepath.Join(webDist, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Change to tmpDir so relative path "web/dist" resolves correctly
	oldPwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	defer os.Chdir(oldPwd)

	got := findWebDist()
	if got != "web/dist" {
		t.Errorf("findWebDist() = %q, want web/dist", got)
	}
}
