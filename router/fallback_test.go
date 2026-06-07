package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupFallbackEditorConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "fallback.json")
	oldContent := []byte(`{"enabled":true}`)
	if err := os.WriteFile(configPath, oldContent, 0644); err != nil {
		t.Fatalf("failed to write source config: %v", err)
	}

	backupPath, err := backupFallbackEditorConfig(configPath)
	if err != nil {
		t.Fatalf("expected backup to succeed, got %v", err)
	}
	if !strings.Contains(backupPath, filepath.Join("backups", "fallback.")) {
		t.Fatalf("expected backup path under backups directory, got %s", backupPath)
	}

	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup config: %v", err)
	}
	if string(backupContent) != string(oldContent) {
		t.Fatalf("expected backup content %s, got %s", oldContent, backupContent)
	}
}

func TestBackupFallbackEditorConfigCreatesUniquePaths(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "fallback.json")
	if err := os.WriteFile(configPath, []byte(`{"enabled":true}`), 0644); err != nil {
		t.Fatalf("failed to write source config: %v", err)
	}

	firstPath, err := backupFallbackEditorConfig(configPath)
	if err != nil {
		t.Fatalf("expected first backup to succeed, got %v", err)
	}
	secondPath, err := backupFallbackEditorConfig(configPath)
	if err != nil {
		t.Fatalf("expected second backup to succeed, got %v", err)
	}
	if firstPath == secondPath {
		t.Fatalf("expected unique backup paths, got %s", firstPath)
	}
}

func TestBackupFallbackEditorConfigMissingFile(t *testing.T) {
	backupPath, err := backupFallbackEditorConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("expected missing config to be ignored, got %v", err)
	}
	if backupPath != "" {
		t.Fatalf("expected no backup path for missing config, got %s", backupPath)
	}
}
