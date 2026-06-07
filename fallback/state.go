package fallback

import (
	"fmt"
	"sync"
	"time"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/logger"
	"gorm.io/gorm"

	"github.com/songquanpeng/one-api/model"
)

type DeploymentState struct {
	Id                   int        `gorm:"primaryKey"`
	DeploymentID         string     `gorm:"uniqueIndex:idx_deployment_date"`
	Date                 string     `gorm:"uniqueIndex:idx_deployment_date"`
	UsedPromptTokens     int        `gorm:"default:0"`
	UsedCompletionTokens int        `gorm:"default:0"`
	UsedTotalTokens      int64      `gorm:"default:0"`
	RequestCount         int        `gorm:"default:0"`
	SuccessCount         int        `gorm:"default:0"`
	ErrorCount           int        `gorm:"default:0"`
	ExhaustedUntil       *time.Time `gorm:"column:exhausted_until"`
	CooldownUntil        *time.Time `gorm:"column:cooldown_until"`
	LastErrorCode        string     `gorm:"column:last_error_code"`
	LastErrorMessage     string     `gorm:"column:last_error_message"`
	CreatedAt            time.Time  `gorm:"column:created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
}

type UsageInfo struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

var quotaRefreshLocation = time.FixedZone("UTC+8", 8*60*60)

const quotaRefreshHour = 12

// InitStateStore initializes the fallback state store and creates table if not exists
func InitStateStore() error {
	// Run migration first (dedup before AutoMigrate to ensure unique index can be created)
	if err := MigrateStateStore(); err != nil {
		logger.SysError(fmt.Sprintf("state migration failed: %v", err))
	}
	if err := model.DB.AutoMigrate(&DeploymentState{}); err != nil {
		return err
	}
	if err := InitAlertHistoryStore(); err != nil {
		return err
	}
	return InitScoreHistoryStore()
}

// MigrateStateStore cleans duplicate records and ensures DB unique constraint.
// 1. Removes duplicate records, keeping the most recent (max id) per (deployment_id, date)
// 2. Ensures the unique index idx_deployment_date exists
// Safe to run multiple times (idempotent, skip if table not yet created)
func MigrateStateStore() error {
	if !model.DB.Migrator().HasTable(&DeploymentState{}) {
		return nil
	}

	// Remove duplicates — keep the record with highest id per (deployment_id, date)
	// Double-nested subquery works around SQLite's "can't modify same table" limitation
	result := model.DB.Exec(`
		DELETE FROM deployment_states WHERE id IN (
			SELECT id FROM (
				SELECT id FROM deployment_states
				WHERE id NOT IN (
					SELECT MAX(id) FROM deployment_states GROUP BY deployment_id, date
				)
			) AS _tmp
		)
	`)
	if result.Error != nil {
		return fmt.Errorf("failed to deduplicate deployment_states: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		logger.SysLog(fmt.Sprintf("[migration] removed %d duplicate records from deployment_states (keeping most recent per deployment_id+date)", result.RowsAffected))
	}

	// Ensure unique index exists
	if !model.DB.Migrator().HasIndex(&DeploymentState{}, "idx_deployment_date") {
		if err := model.DB.Migrator().CreateIndex(&DeploymentState{}, "idx_deployment_date"); err != nil {
			return fmt.Errorf("failed to create unique index idx_deployment_date: %w", err)
		}
		logger.SysLog("[migration] created unique index idx_deployment_date on deployment_states")
	}

	return nil
}

// GetDeploymentState retrieves existing deployment state or creates new one
func GetDeploymentState(deploymentID string, date string) (*DeploymentState, error) {
	if model.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	var state DeploymentState
	err := model.DB.Where("deployment_id = ? AND date = ?", deploymentID, date).First(&state).Error
	if err != nil {
		return nil, err
	}
	return &state, nil
}

// EnsureDeploymentState ensures deployment state exists, creating if necessary
func EnsureDeploymentState(deploymentID string, date string) (*DeploymentState, error) {
	state, err := GetDeploymentState(deploymentID, date)
	if err == gorm.ErrRecordNotFound {
		state = &DeploymentState{
			DeploymentID:         deploymentID,
			Date:                 date,
			UsedPromptTokens:     0,
			UsedCompletionTokens: 0,
			UsedTotalTokens:      0,
			RequestCount:         0,
			SuccessCount:         0,
			ErrorCount:           0,
			ExhaustedUntil:       nil,
			CooldownUntil:        nil,
			LastErrorCode:        "",
			LastErrorMessage:     "",
			CreatedAt:            time.Now().UTC(),
			UpdatedAt:            time.Now().UTC(),
		}
		err = model.DB.Create(state).Error
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return state, nil
}

// IsDeploymentAvailable checks if deployment is available for use
func IsDeploymentAvailable(dep DeploymentConfig) (bool, string) {
	if !dep.Enabled {
		return false, "deployment disabled"
	}

	state, err := EnsureDeploymentState(dep.ID, todayString())
	if err != nil {
		logger.SysError(fmt.Sprintf("failed to ensure deployment state for %s: %v", dep.ID, err))
		return false, fmt.Sprintf("failed to check state: %v", err)
	}

	now := time.Now().UTC()

	// Check exhausted state
	if state.ExhaustedUntil != nil && state.ExhaustedUntil.After(now) {
		return false, fmt.Sprintf("deployment exhausted until %s", state.ExhaustedUntil.Format(time.RFC3339))
	}

	// Check cooldown state
	if state.CooldownUntil != nil && state.CooldownUntil.After(now) {
		return false, fmt.Sprintf("deployment cooling down until %s", state.CooldownUntil.Format(time.RFC3339))
	}

	// Check soft limit (preemptive skip: redirect to next deployment before hitting hard limit)
	if dep.DailyLimitTokens > 0 && dep.SoftLimitRatio > 0 {
		softLimit := int64(float64(dep.DailyLimitTokens) * dep.SoftLimitRatio)
		if state.UsedTotalTokens >= softLimit {
			logger.SysLog(fmt.Sprintf("[fallback] deployment %s soft-limited (skip to next): %d/%d (%.1f%%)",
				dep.ID, state.UsedTotalTokens, dep.DailyLimitTokens,
				float64(state.UsedTotalTokens)/float64(dep.DailyLimitTokens)*100))
			return false, fmt.Sprintf("deployment reached soft daily token limit: %d/%d", state.UsedTotalTokens, dep.DailyLimitTokens)
		}
	}

	// Check hard limit
	if dep.DailyLimitTokens > 0 && dep.HardLimitRatio > 0 {
		hardLimit := int64(float64(dep.DailyLimitTokens) * dep.HardLimitRatio)
		if state.UsedTotalTokens >= hardLimit {
			return false, fmt.Sprintf("deployment reached hard daily token limit: %d/%d", state.UsedTotalTokens, dep.DailyLimitTokens)
		}
	}

	return true, ""
}

// RecordDeploymentUsage records token usage for deployment
func RecordDeploymentUsage(deploymentID string, usage UsageInfo) error {
	state, err := EnsureDeploymentState(deploymentID, todayString())
	if err != nil {
		return err
	}

	state.UsedPromptTokens += usage.PromptTokens
	state.UsedCompletionTokens += usage.CompletionTokens
	state.UsedTotalTokens += int64(usage.TotalTokens)
	state.RequestCount++
	state.UpdatedAt = time.Now().UTC()

	return model.DB.Save(state).Error
}

// RecordDeploymentSuccess records successful deployment request
func RecordDeploymentSuccess(deploymentID string, usage UsageInfo) error {
	err := RecordDeploymentUsage(deploymentID, usage)
	if err != nil {
		return err
	}

	// Also increment success count for smart sorting
	return model.DB.Model(&DeploymentState{}).
		Where("deployment_id = ? AND date = ?", deploymentID, todayString()).
		UpdateColumn("success_count", gorm.Expr("success_count + 1")).
		Error
}

// RecordDeploymentError records deployment error
func RecordDeploymentError(deploymentID string, originalErr error) error {
	state, dbErr := EnsureDeploymentState(deploymentID, todayString())
	if dbErr != nil {
		return dbErr
	}

	state.ErrorCount++
	state.RequestCount++
	state.LastErrorCode = "unknown"
	state.LastErrorMessage = originalErr.Error()
	state.UpdatedAt = time.Now().UTC()

	return model.DB.Save(state).Error
}

// MarkDeploymentExhausted marks deployment as exhausted until specific time
func MarkDeploymentExhausted(deploymentID string, reason string, until time.Time) error {
	state, err := EnsureDeploymentState(deploymentID, todayString())
	if err != nil {
		return err
	}

	state.ExhaustedUntil = &until
	state.LastErrorCode = "exhausted"
	state.LastErrorMessage = fmt.Sprintf("%s: %s", reason, until.Format(time.RFC3339))
	state.UpdatedAt = time.Now().UTC()

	return model.DB.Save(state).Error
}

// MarkDeploymentCooldown marks deployment as cooling down until specific time
func MarkDeploymentCooldown(deploymentID string, reason string, until time.Time) error {
	state, err := EnsureDeploymentState(deploymentID, todayString())
	if err != nil {
		return err
	}

	state.CooldownUntil = &until
	state.LastErrorCode = "cooldown"
	state.LastErrorMessage = fmt.Sprintf("%s: %s", reason, until.Format(time.RFC3339))
	state.UpdatedAt = time.Now().UTC()

	return model.DB.Save(state).Error
}

// todayString returns the quota period date. A period starts at 12:00 UTC+8
// and ends at 12:00 UTC+8 the next day, matching the upstream quota reset.
func todayString() string {
	return quotaPeriodDate(time.Now())
}

func quotaPeriodDate(now time.Time) string {
	localNow := now.In(quotaRefreshLocation)
	if localNow.Hour() < quotaRefreshHour {
		localNow = localNow.AddDate(0, 0, -1)
	}
	return localNow.Format("2006-01-02")
}

// EndOfToday returns the next upstream quota refresh time.
func EndOfToday() time.Time {
	return nextQuotaRefreshTime(time.Now())
}

func nextQuotaRefreshTime(now time.Time) time.Time {
	localNow := now.In(quotaRefreshLocation)
	nextRefresh := time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day(),
		quotaRefreshHour,
		0,
		0,
		0,
		quotaRefreshLocation,
	)
	if !localNow.Before(nextRefresh) {
		nextRefresh = nextRefresh.AddDate(0, 0, 1)
	}
	return nextRefresh.UTC()
}

// GetDeploymentStats returns usage statistics for a deployment
func GetDeploymentStats(deploymentID string) (UsageInfo, int, int, error) {
	state, err := EnsureDeploymentState(deploymentID, todayString())
	if err != nil {
		return UsageInfo{}, 0, 0, err
	}

	return UsageInfo{
		PromptTokens:     state.UsedPromptTokens,
		CompletionTokens: state.UsedCompletionTokens,
		TotalTokens:      int(state.UsedTotalTokens),
	}, state.RequestCount, state.ErrorCount, nil
}

// ResetDeploymentState resets deployment state fields and current-period usage.
func ResetDeploymentState(deploymentID string) error {
	state, err := EnsureDeploymentState(deploymentID, todayString())
	if err != nil {
		return err
	}

	state.UsedPromptTokens = 0
	state.UsedCompletionTokens = 0
	state.UsedTotalTokens = 0
	state.RequestCount = 0
	state.SuccessCount = 0
	state.ErrorCount = 0
	state.ExhaustedUntil = nil
	state.CooldownUntil = nil
	state.LastErrorCode = ""
	state.LastErrorMessage = ""
	state.UpdatedAt = time.Now().UTC()

	return model.DB.Save(state).Error
}

// ClearDeploymentExhausted clears only the exhausted_until field
func ClearDeploymentExhausted(deploymentID string) error {
	state, err := EnsureDeploymentState(deploymentID, todayString())
	if err != nil {
		return err
	}

	state.ExhaustedUntil = nil
	state.UpdatedAt = time.Now().UTC()

	return model.DB.Save(state).Error
}

// ClearDeploymentCooldown clears only the cooldown_until field
func ClearDeploymentCooldown(deploymentID string) error {
	state, err := EnsureDeploymentState(deploymentID, todayString())
	if err != nil {
		return err
	}

	state.CooldownUntil = nil
	state.UpdatedAt = time.Now().UTC()

	return model.DB.Save(state).Error
}

// GetAllDeploymentStates returns all deployment states for a virtual model
func GetAllDeploymentStates(virtualModel string) (map[string]*DeploymentState, error) {
	if !common.IsFallbackEnabled {
		return nil, fmt.Errorf("fallback feature is disabled")
	}

	deployments, err := GetDeploymentsForVirtualModel(virtualModel)
	if err != nil {
		return nil, err
	}

	states := make(map[string]*DeploymentState)
	for _, dep := range deployments {
		state, err := GetDeploymentState(dep.ID, todayString())
		if err != nil {
			logger.SysError(fmt.Sprintf("failed to get state for deployment %s: %v", dep.ID, err))
			continue
		}
		states[dep.ID] = state
	}

	return states, nil
}

// Sticky routing: remember the last successful deployment per virtual model
var (
	stickyDepMu sync.RWMutex
	stickyDep   map[string]string // virtualModel -> deploymentID
)

// GetStickyDeployment returns the currently sticky deployment for a virtual model.
func GetStickyDeployment(virtualModel string) string {
	stickyDepMu.RLock()
	defer stickyDepMu.RUnlock()
	if stickyDep == nil {
		return ""
	}
	return stickyDep[virtualModel]
}

// SetStickyDeployment pins a deployment as the preferred one for a virtual model.
func SetStickyDeployment(virtualModel, deploymentID string) {
	stickyDepMu.Lock()
	defer stickyDepMu.Unlock()
	if stickyDep == nil {
		stickyDep = make(map[string]string)
	}
	stickyDep[virtualModel] = deploymentID
}

// ClearStickyDeployment removes the sticky preference.
func ClearStickyDeployment(virtualModel string) {
	stickyDepMu.Lock()
	defer stickyDepMu.Unlock()
	delete(stickyDep, virtualModel)
}
