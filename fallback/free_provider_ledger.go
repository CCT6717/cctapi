package fallback

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/songquanpeng/one-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FreeProviderUsageLedger struct {
	ID               int       `json:"id" gorm:"primaryKey"`
	Provider         string    `json:"provider" gorm:"size:64;uniqueIndex:idx_free_provider_usage_period;index"`
	KeyHash          string    `json:"key_hash" gorm:"size:16;uniqueIndex:idx_free_provider_usage_period;index"`
	ModelName        string    `json:"model_name" gorm:"size:191;uniqueIndex:idx_free_provider_usage_period;index"`
	Period           string    `json:"period" gorm:"size:16;uniqueIndex:idx_free_provider_usage_period;index"`
	PromptTokens     int64     `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int64     `json:"completion_tokens" gorm:"default:0"`
	TotalTokens      int64     `json:"total_tokens" gorm:"default:0"`
	RequestCount     int64     `json:"request_count" gorm:"default:0"`
	SuccessCount     int64     `json:"success_count" gorm:"default:0"`
	CreatedAt        time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func InitFreeProviderLedgerStore() error {
	if model.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	return model.DB.AutoMigrate(&FreeProviderUsageLedger{})
}

func parseAutoFreeDeploymentID(deploymentID string) (provider string, keyHash string, ok bool) {
	for providerName := range knownFreeProviders {
		prefix := "free:" + providerName + "-"
		if !strings.HasPrefix(deploymentID, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(deploymentID, prefix)
		if !isAutoDeploymentSuffix(suffix) {
			return "", "", false
		}
		return providerName, suffix, true
	}
	return "", "", false
}

func RecordFreeProviderUsage(deploymentID string, modelName string, usage UsageInfo) error {
	provider, keyHash, ok := parseAutoFreeDeploymentID(deploymentID)
	if !ok {
		return nil
	}
	if model.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	if err := InitFreeProviderLedgerStore(); err != nil {
		return err
	}

	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		modelName = "unknown"
	}
	totalTokens := int64(usage.TotalTokens)
	if totalTokens == 0 {
		totalTokens = int64(usage.PromptTokens + usage.CompletionTokens)
	}

	now := time.Now().UTC()
	row := FreeProviderUsageLedger{
		Provider:         provider,
		KeyHash:          keyHash,
		ModelName:        modelName,
		Period:           todayString(),
		PromptTokens:     int64(usage.PromptTokens),
		CompletionTokens: int64(usage.CompletionTokens),
		TotalTokens:      totalTokens,
		RequestCount:     1,
		SuccessCount:     1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	return model.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "provider"},
			{Name: "key_hash"},
			{Name: "model_name"},
			{Name: "period"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"prompt_tokens":     gorm.Expr("prompt_tokens + ?", row.PromptTokens),
			"completion_tokens": gorm.Expr("completion_tokens + ?", row.CompletionTokens),
			"total_tokens":      gorm.Expr("total_tokens + ?", row.TotalTokens),
			"request_count":     gorm.Expr("request_count + 1"),
			"success_count":     gorm.Expr("success_count + 1"),
			"updated_at":        now,
		}),
	}).Create(&row).Error
}

func GetFreeProviderUsage(provider string, keyHash string, modelName string, period string) (*FreeProviderUsageLedger, error) {
	if model.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if period == "" {
		period = todayString()
	}

	var row FreeProviderUsageLedger
	err := model.DB.Where("provider = ? AND key_hash = ? AND model_name = ? AND period = ?",
		provider, keyHash, modelName, period).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
