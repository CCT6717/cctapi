package fallback

import (
	"time"

	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
)

const (
	// SwitchEventRetentionDays is how long switch event logs are kept.
	SwitchEventRetentionDays = 7
	// AlertHistoryRetentionDays is how long alert history records are kept.
	AlertHistoryRetentionDays = 30
	// ScoreSnapshotRetentionDays is how long score snapshots are kept.
	// Score snapshots are high-volume and have short diagnostic value.
	ScoreSnapshotRetentionDays = 3
	// CleanupInterval is how often the background cleanup runs.
	CleanupInterval = 1 * time.Hour
)

// StartHistoryCleanup launches a background goroutine that periodically deletes
// old records from the fallback event tables. It returns immediately.
func StartHistoryCleanup() {
	go func() {
		// Run once immediately on start
		runCleanup()

		ticker := time.NewTicker(CleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			runCleanup()
		}
	}()
	logger.SysLog("[fallback] history cleanup started")
}

func runCleanup() {
	if model.DB == nil {
		return
	}

	cutoffSwitch := time.Now().UTC().AddDate(0, 0, -SwitchEventRetentionDays)
	cutoffAlert := time.Now().UTC().AddDate(0, 0, -AlertHistoryRetentionDays)
	cutoffScore := time.Now().UTC().AddDate(0, 0, -ScoreSnapshotRetentionDays)

	// Clean switch events
	if model.DB.Migrator().HasTable(&SwitchEvent{}) {
		result := model.DB.Where("created_at < ?", cutoffSwitch).Delete(&SwitchEvent{})
		if result.RowsAffected > 0 {
			logger.SysLogf("[fallback] cleanup: deleted %d switch events older than %d days", result.RowsAffected, SwitchEventRetentionDays)
		}
	}

	// Clean alert history
	if model.DB.Migrator().HasTable(&AlertHistoryEvent{}) {
		result := model.DB.Where("created_at < ?", cutoffAlert).Delete(&AlertHistoryEvent{})
		if result.RowsAffected > 0 {
			logger.SysLogf("[fallback] cleanup: deleted %d alert history records older than %d days", result.RowsAffected, AlertHistoryRetentionDays)
		}
	}

	// Clean score snapshots
	if model.DB.Migrator().HasTable(&ScoreSnapshot{}) {
		result := model.DB.Where("created_at < ?", cutoffScore).Delete(&ScoreSnapshot{})
		if result.RowsAffected > 0 {
			logger.SysLogf("[fallback] cleanup: deleted %d score snapshots older than %d days", result.RowsAffected, ScoreSnapshotRetentionDays)
		}
	}
}
