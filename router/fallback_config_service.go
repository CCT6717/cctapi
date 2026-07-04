package router

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/songquanpeng/one-api/fallback"
)

func saveFallbackEditorConfig(payload fallbackEditorConfig) (fallbackEditorConfig, string, error) {
	virtualModels, deployments, err := normalizeFallbackEditorPayload(payload)
	if err != nil {
		return fallbackEditorConfig{}, "", err
	}

	deployments, err = upsertFallbackEditorChannels(deployments)
	if err != nil {
		return fallbackEditorConfig{}, "", err
	}

	cfg := buildFallbackConfigFromEditor(payload, virtualModels, deployments)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fallbackEditorConfig{}, "", err
	}
	data = append(data, '\n')

	backupPath, err := backupFallbackEditorConfig(fallbackEditorConfigPath)
	if err != nil {
		return fallbackEditorConfig{}, "", err
	}

	if err := os.WriteFile(fallbackEditorConfigPath, data, 0644); err != nil {
		return fallbackEditorConfig{}, "", err
	}

	if err := fallback.ReloadConfig(fallbackEditorConfigPath); err != nil {
		return fallbackEditorConfig{}, "", err
	}

	return buildFallbackEditorConfig(&cfg), backupPath, nil
}

func backupFallbackEditorConfig(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read old fallback config for backup: %w", err)
	}
	data, err = sanitizeFallbackBackupData(data)
	if err != nil {
		return "", fmt.Errorf("failed to sanitize fallback config backup: %w", err)
	}

	ext := filepath.Ext(configPath)
	base := strings.TrimSuffix(filepath.Base(configPath), ext)
	backupDir := filepath.Join(filepath.Dir(configPath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create fallback config backup directory: %w", err)
	}

	now := time.Now()
	backupStem := fmt.Sprintf("%s.%s-%09d", base, now.Format("20060102-150405"), now.Nanosecond())
	backupPath := filepath.Join(backupDir, backupStem+ext)
	for index := 1; ; index++ {
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			break
		} else if err != nil {
			return "", fmt.Errorf("failed to inspect fallback config backup path: %w", err)
		}
		backupPath = filepath.Join(backupDir, fmt.Sprintf("%s.%d%s", backupStem, index, ext))
	}
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write fallback config backup: %w", err)
	}
	return backupPath, nil
}

func sanitizeFallbackBackupData(data []byte) ([]byte, error) {
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	rawProviders, ok := root["free_providers"].(map[string]interface{})
	if !ok {
		return data, nil
	}

	changed := false
	for _, rawProvider := range rawProviders {
		provider, ok := rawProvider.(map[string]interface{})
		if !ok {
			continue
		}
		rawKeys, ok := provider["keys"].([]interface{})
		if !ok || len(rawKeys) == 0 {
			continue
		}

		keyHashes := make([]string, 0, len(rawKeys))
		for _, rawKey := range rawKeys {
			key, ok := rawKey.(string)
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			keyHashes = append(keyHashes, fallback.SafeKeyHash(key))
		}

		provider["keys"] = []interface{}{}
		provider["key_hashes"] = keyHashes
		changed = true
	}

	if !changed {
		return data, nil
	}

	sanitized, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(sanitized, '\n'), nil
}
