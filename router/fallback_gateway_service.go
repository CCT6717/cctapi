package router

import (
	"encoding/json"
	"os"

	"github.com/songquanpeng/one-api/fallback"
)

func saveGatewayConfigPayload(merged fallback.Config) (string, error) {
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	backupPath, err := backupFallbackEditorConfig(fallbackEditorConfigPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(fallbackEditorConfigPath, data, 0644); err != nil {
		return "", err
	}
	if err := fallback.ReloadConfig(fallbackEditorConfigPath); err != nil {
		return "", err
	}
	return backupPath, nil
}
