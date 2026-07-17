package fallback

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	attemptFailureWindow     = time.Hour
	attemptAggregateLimit    = 5
	recentAttemptChainScope  = "process"
)

var attemptFailureOutcomes = []AttemptOutcome{
	AttemptOutcomeFailure,
	AttemptOutcomeModelRateLimited,
	AttemptOutcomeNonFallbackable,
	AttemptOutcomeModelCapabilityFalsePositive,
}

const safeErrorCategoryExpression = "CASE WHEN error_category IN ('none', 'client', 'quota', 'rate_limit', 'temporary', 'model_access') THEN error_category ELSE 'unknown' END"

func safeAttemptErrorCategory(category string) string {
	switch category {
	case "none", "client", "quota", "rate_limit", "temporary", "model_access":
		return category
	default:
		return "unknown"
	}
}

// AttemptAggregateItem is a safe, grouped summary of persisted attempt events.
type AttemptAggregateItem struct {
	Key          string    `json:"key"`
	DeploymentID string    `json:"deployment_id,omitempty"`
	Provider     string    `json:"provider,omitempty"`
	RealModel    string    `json:"real_model,omitempty"`
	Category     string    `json:"category,omitempty"`
	Outcome      string    `json:"outcome,omitempty"`
	Count        int64     `json:"count"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

// AttemptObservabilitySnapshot contains the current one-hour failure view and
// process-local request chains. Successful attempts are intentionally absent
// because they are not persisted in the attempt event store.
type AttemptObservabilitySnapshot struct {
	GeneratedAt          time.Time              `json:"generated_at"`
	FailureWindowSeconds int64                  `json:"failure_window_seconds"`
	FailureEventCount    int64                  `json:"failure_event_count"`
	SkipEventCount       int64                  `json:"skip_event_count"`
	TopDeployments       []AttemptAggregateItem `json:"top_deployments"`
	TopProviders         []AttemptAggregateItem `json:"top_providers"`
	TopModels            []AttemptAggregateItem `json:"top_models"`
	ErrorCategories      []AttemptAggregateItem `json:"error_categories"`
	Outcomes             []AttemptAggregateItem `json:"outcomes"`
	RecentChains         []AttemptRequestChain  `json:"recent_chains"`
	RecentChainScope     string                 `json:"recent_chain_scope"`
}

// SnapshotAttemptObservability returns the current one-hour attempt summary.
func SnapshotAttemptObservability() (AttemptObservabilitySnapshot, error) {
	return snapshotAttemptObservabilityAt(time.Now().UTC())
}

func snapshotAttemptObservabilityAt(now time.Time) (AttemptObservabilitySnapshot, error) {
	db, err := initAttemptEventStore()
	if err != nil {
		return AttemptObservabilitySnapshot{}, err
	}

	cutoff := now.Add(-attemptFailureWindow)
	failureCondition := "upstream_attempt_index > 0 AND outcome IN ?"
	snapshot := AttemptObservabilitySnapshot{
		GeneratedAt:          now,
		FailureWindowSeconds: int64(attemptFailureWindow / time.Second),
		TopDeployments:       make([]AttemptAggregateItem, 0),
		TopProviders:         make([]AttemptAggregateItem, 0),
		TopModels:            make([]AttemptAggregateItem, 0),
		ErrorCategories:      make([]AttemptAggregateItem, 0),
		Outcomes:             make([]AttemptAggregateItem, 0),
		RecentChains:         SnapshotRecentAttemptChains(0),
		RecentChainScope:     recentAttemptChainScope,
	}
	if snapshot.RecentChains == nil {
		snapshot.RecentChains = make([]AttemptRequestChain, 0)
	}

	failureEvents := db.Model(&AttemptEvent{}).
		Where("created_at >= ?", cutoff).
		Where(failureCondition, attemptFailureOutcomes)
	if err := failureEvents.Count(&snapshot.FailureEventCount).Error; err != nil {
		return AttemptObservabilitySnapshot{}, err
	}
	if err := db.Model(&AttemptEvent{}).
		Where("created_at >= ?", cutoff).
		Where("outcome LIKE ?", "skipped_%").
		Count(&snapshot.SkipEventCount).Error; err != nil {
		return AttemptObservabilitySnapshot{}, err
	}

	if snapshot.TopDeployments, err = scanAttemptAggregates(failureEvents, "deployment_id AS key, deployment_id, COUNT(*) AS count, MAX(created_at) AS last_seen_at", "deployment_id"); err != nil {
		return AttemptObservabilitySnapshot{}, err
	}
	if snapshot.TopProviders, err = scanAttemptAggregates(failureEvents, "provider AS key, provider, COUNT(*) AS count, MAX(created_at) AS last_seen_at", "provider"); err != nil {
		return AttemptObservabilitySnapshot{}, err
	}
	if snapshot.TopModels, err = scanAttemptAggregates(failureEvents, "real_model AS key, real_model, COUNT(*) AS count, MAX(created_at) AS last_seen_at", "real_model"); err != nil {
		return AttemptObservabilitySnapshot{}, err
	}
	if snapshot.ErrorCategories, err = scanAttemptAggregates(failureEvents, safeErrorCategoryExpression+" AS key, "+safeErrorCategoryExpression+" AS category, COUNT(*) AS count, MAX(created_at) AS last_seen_at", safeErrorCategoryExpression); err != nil {
		return AttemptObservabilitySnapshot{}, err
	}

	outcomeEvents := db.Model(&AttemptEvent{}).
		Where("created_at >= ?", cutoff).
		Where("("+failureCondition+") OR outcome LIKE ?", attemptFailureOutcomes, "skipped_%")
	if snapshot.Outcomes, err = scanAttemptAggregates(outcomeEvents, "outcome AS key, outcome, COUNT(*) AS count, MAX(created_at) AS last_seen_at", "outcome"); err != nil {
		return AttemptObservabilitySnapshot{}, err
	}

	return snapshot, nil
}

func scanAttemptAggregates(query *gorm.DB, selectClause, groupClause string) ([]AttemptAggregateItem, error) {
	rows := make([]attemptAggregateRow, 0)
	result := query.Select(selectClause).
		Group(groupClause).
		Order("count DESC, last_seen_at DESC, key ASC").
		Limit(attemptAggregateLimit).
		Scan(&rows)
	if result.Error != nil {
		return nil, result.Error
	}
	items := make([]AttemptAggregateItem, 0, len(rows))
	for _, row := range rows {
		lastSeenAt, err := parseAttemptAggregateTime(row.LastSeenAt)
		if err != nil {
			return nil, err
		}
		items = append(items, AttemptAggregateItem{
			Key:          row.Key,
			DeploymentID: row.DeploymentID,
			Provider:     row.Provider,
			RealModel:    row.RealModel,
			Category:     row.Category,
			Outcome:      row.Outcome,
			Count:        row.Count,
			LastSeenAt:   lastSeenAt,
		})
	}
	return items, nil
}

type attemptAggregateRow struct {
	Key          string
	DeploymentID string
	Provider     string
	RealModel    string
	Category     string
	Outcome      string
	Count        int64
	LastSeenAt   string
}

func parseAttemptAggregateTime(value string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999",
		time.DateTime,
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse aggregate attempt timestamp %q", value)
}
