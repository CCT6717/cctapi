package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/songquanpeng/one-api/fallback"
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

func TestSplitFallbackEditorChannelModels(t *testing.T) {
	models := splitFallbackEditorChannelModels(" deepseek-v3,deepseek-reasoner,, deepseek-v3 , claude-3-5-sonnet ")

	expected := []string{"deepseek-v3", "deepseek-reasoner", "claude-3-5-sonnet"}
	if len(models) != len(expected) {
		t.Fatalf("expected %d models, got %d: %v", len(expected), len(models), models)
	}
	for i := range expected {
		if models[i] != expected[i] {
			t.Fatalf("expected model %d to be %s, got %s", i, expected[i], models[i])
		}
	}
}

func TestBuildFallbackConfigFromEditorPreservesFixedDeployment(t *testing.T) {
	payload := fallbackEditorConfig{
		Enabled: true,
	}
	virtualModels := []fallbackEditorVirtualModel{
		{
			Name:            "core/auto",
			Enabled:         true,
			Description:     "core fixed model",
			RoutingMode:     "fixed",
			FixedDeployment: "core-primary",
			FallbackOrder:   []string{"core-primary", "core-backup"},
		},
	}
	deployments := []fallbackEditorDeployment{
		{ID: "core-primary", Enabled: true, ChannelID: 1, RealModel: "deepseek-v3"},
		{ID: "core-backup", Enabled: true, ChannelID: 2, RealModel: "deepseek-reasoner"},
	}

	cfg := buildFallbackConfigFromEditor(payload, virtualModels, deployments)

	vm := cfg.VirtualModels["core/auto"]
	if vm.RoutingMode != fallback.RoutingModeFixed {
		t.Fatalf("expected routing mode fixed, got %s", vm.RoutingMode)
	}
	if vm.FixedDeployment != "core-primary" {
		t.Fatalf("expected fixed deployment core-primary, got %q", vm.FixedDeployment)
	}
}

func TestNormalizeFallbackEditorPayloadRejectsInvalidFixedDeployment(t *testing.T) {
	basePayload := func() fallbackEditorConfig {
		return fallbackEditorConfig{
			Enabled: true,
			VirtualModels: []fallbackEditorVirtualModel{
				{
					Name:            "core/auto",
					Enabled:         true,
					RoutingMode:     "fixed",
					FixedDeployment: "core-primary",
					FallbackOrder:   []string{"core-primary", "core-backup"},
				},
			},
			Deployments: []fallbackEditorDeployment{
				{ID: "core-primary", Enabled: true, ChannelID: 1, RealModel: "deepseek-v3"},
				{ID: "core-backup", Enabled: true, ChannelID: 2, RealModel: "deepseek-reasoner"},
			},
		}
	}

	tests := []struct {
		name string
		edit func(*fallbackEditorConfig)
	}{
		{
			name: "empty fixed deployment",
			edit: func(payload *fallbackEditorConfig) {
				payload.VirtualModels[0].FixedDeployment = ""
			},
		},
		{
			name: "fixed deployment outside fallback order",
			edit: func(payload *fallbackEditorConfig) {
				payload.VirtualModels[0].FixedDeployment = "missing"
			},
		},
		{
			name: "disabled fixed deployment",
			edit: func(payload *fallbackEditorConfig) {
				payload.Deployments[0].Enabled = false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := basePayload()
			tt.edit(&payload)
			if _, _, err := normalizeFallbackEditorPayload(payload); err == nil {
				t.Fatalf("expected normalizeFallbackEditorPayload to reject %s", tt.name)
			}
		})
	}
}

func TestBuildFallbackEditorConfigIncludesFixedDeployment(t *testing.T) {
	cfg := &fallback.Config{
		Enabled: true,
		VirtualModels: map[string]fallback.VirtualModelConfig{
			"core/auto": {
				Enabled:         true,
				RoutingMode:     fallback.RoutingModeFixed,
				FixedDeployment: "core-primary",
				FallbackOrder:   []string{"core-primary"},
			},
		},
		Deployments: map[string]fallback.DeploymentConfig{
			"core-primary": {Enabled: true, ChannelID: 0, RealModel: "deepseek-v3"},
		},
	}

	editorCfg := buildFallbackEditorConfig(cfg)

	if len(editorCfg.VirtualModels) != 1 {
		t.Fatalf("expected one virtual model, got %d", len(editorCfg.VirtualModels))
	}
	vm := editorCfg.VirtualModels[0]
	if vm.RoutingMode != fallback.RoutingModeFixed {
		t.Fatalf("expected routing mode fixed, got %s", vm.RoutingMode)
	}
	if vm.FixedDeployment != "core-primary" {
		t.Fatalf("expected fixed deployment core-primary, got %q", vm.FixedDeployment)
	}
}
