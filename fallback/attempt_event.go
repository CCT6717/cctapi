package fallback

import (
	"fmt"
	"sync"
	"time"

	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
	"gorm.io/gorm"
)

// AttemptOutcome describes the final result of a single upstream attempt.
type AttemptOutcome string

const (
	AttemptOutcomeSuccess                      AttemptOutcome = "success"
	AttemptOutcomeFailure                      AttemptOutcome = "failure"
	AttemptOutcomeSkippedUnavailable           AttemptOutcome = "skipped_unavailable"
	AttemptOutcomeSkippedQuota                 AttemptOutcome = "skipped_quota"
	AttemptOutcomeSkippedConcurrency           AttemptOutcome = "skipped_concurrency"
	AttemptOutcomeSkippedChannel               AttemptOutcome = "skipped_channel"
	AttemptOutcomeSkippedModelState            AttemptOutcome = "skipped_model_state"
	AttemptOutcomeModelRateLimited             AttemptOutcome = "model_rate_limited"
	AttemptOutcomeNonFallbackable              AttemptOutcome = "non_fallbackable"
	AttemptOutcomeModelCapabilityFalsePositive AttemptOutcome = "model_capability_false_positive"
)

// AttemptEvent records a single attempt at an upstream deployment.
// It is designed to be lightweight and does NOT store raw error messages
// or sensitive credentials.
//
// Phase 4 design: success events are aggregated into memory metrics rather
// than persisted to SQLite, to avoid rapid database growth. Failures,
// skips, and model_rate_limited are persisted for post-mortem analysis.
type AttemptEvent struct {
	ID                   int            `json:"id" gorm:"primaryKey"`
	CreatedAt            time.Time      `json:"created_at" gorm:"index"`
	RequestID            string         `json:"request_id" gorm:"index"`
	VirtualModel         string         `json:"virtual_model" gorm:"index"`
	Provider             string         `json:"provider" gorm:"index"`
	DeploymentID         string         `json:"deployment_id" gorm:"index"`
	RealModel            string         `json:"real_model" gorm:"index"`
	Outcome              AttemptOutcome `json:"outcome" gorm:"index"`
	StatusCode           int            `json:"status_code"`
	ErrorCategory        string         `json:"error_category"`
	DurationMs           int64          `json:"duration_ms"`
	StreamWritten        bool           `json:"stream_written"`
	PlanIndex            int            `json:"plan_index"`             // 1-based index in modelAttempts plan
	UpstreamAttemptIndex int            `json:"upstream_attempt_index"` // 1-based for real upstream calls; 0 for skips
}

// AttemptEventRetention is the default retention duration for attempt events.
// Only failure, skip, and model_rate_limited events are persisted.
const AttemptEventRetention = 7 * 24 * time.Hour // 7 days

var attemptEventStore = struct {
	sync.Mutex
	activeDB *gorm.DB
}{}

func initAttemptEventStore() (*gorm.DB, error) {
	db := model.DB
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	attemptEventStore.Lock()
	defer attemptEventStore.Unlock()
	if attemptEventStore.activeDB == db {
		return db, nil
	}
	if err := db.AutoMigrate(&AttemptEvent{}); err != nil {
		return nil, err
	}
	attemptEventStore.activeDB = db
	return db, nil
}

// InitAttemptEventStore creates the attempt_events table for the active
// database. Replacing model.DB automatically triggers migration for the new
// connection, matching the other fallback stores.
func InitAttemptEventStore() error {
	_, err := initAttemptEventStore()
	return err
}

// RecordAttemptEvent persists an attempt event to the database.
// Callers should prefer RecordAttemptEventIfWorthy to follow the
// success-aggregation / failure-persistence rule.
func RecordAttemptEvent(event AttemptEvent) error {
	db, err := initAttemptEventStore()
	if err != nil {
		return err
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if err := db.Create(&event).Error; err != nil {
		logger.SysError(fmt.Sprintf("[fallback] failed to record attempt event: %v", err))
		return err
	}
	return nil
}

// RecordAttemptEventIfWorthy decides whether to persist an attempt event based on
// the Phase 4 principle: persist failures, skips, and model_rate_limited; do NOT persist
// routine successes (they are aggregated into memory metrics instead).
func RecordAttemptEventIfWorthy(event AttemptEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	recordRecentAttempt(event)

	switch event.Outcome {
	case AttemptOutcomeSuccess:
		// Success events are aggregated into memory metrics, not persisted
		// to avoid SQLite growth. They can still be logged at debug level.
		return nil
	default:
		return RecordAttemptEvent(event)
	}
}

// GetAttemptEvents returns recent attempt events ordered by time descending.
func GetAttemptEvents(limit int) ([]AttemptEvent, error) {
	db, err := initAttemptEventStore()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	events := make([]AttemptEvent, 0)
	err = db.Order("created_at desc, id desc").Limit(limit).Find(&events).Error
	return events, err
}

// GetAttemptEventsByRequestID returns all attempt events for a specific request.
func GetAttemptEventsByRequestID(requestID string, limit int) ([]AttemptEvent, error) {
	db, err := initAttemptEventStore()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	events := make([]AttemptEvent, 0)
	err = db.Where("request_id = ?", requestID).Order("created_at desc, id desc").Limit(limit).Find(&events).Error
	return events, err
}

// GetAttemptEventsByDeploymentID returns recent attempt events for a deployment.
func GetAttemptEventsByDeploymentID(deploymentID string, limit int) ([]AttemptEvent, error) {
	db, err := initAttemptEventStore()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	events := make([]AttemptEvent, 0)
	err = db.Where("deployment_id = ?", deploymentID).Order("created_at desc, id desc").Limit(limit).Find(&events).Error
	return events, err
}

// DeleteAttemptEventsOlderThan removes attempt events older than the given time.
// Call this periodically to enforce retention.
func DeleteAttemptEventsOlderThan(cutoff time.Time) (int64, error) {
	db, err := initAttemptEventStore()
	if err != nil {
		return 0, err
	}
	result := db.Where("created_at < ?", cutoff).Delete(&AttemptEvent{})
	return result.RowsAffected, result.Error
}

// CleanupAttemptEvents removes attempt events older than AttemptEventRetention.
// It logs the number of deleted rows for observability.
func CleanupAttemptEvents() (int64, error) {
	cutoff := time.Now().UTC().Add(-AttemptEventRetention)
	deleted, err := DeleteAttemptEventsOlderThan(cutoff)
	if err != nil {
		logger.SysError(fmt.Sprintf("[fallback] failed to cleanup attempt events: %v", err))
		return 0, err
	}
	if deleted > 0 {
		logger.SysLogf("[fallback] cleaned up %d attempt events older than %s", deleted, cutoff.Format(time.RFC3339))
	}
	return deleted, nil
}
