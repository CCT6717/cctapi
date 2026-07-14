package fallback

import (
	"sort"
	"sync"
	"time"
)

type FreeProviderModelRuntimeEntrySnapshot struct {
	ModelID             string     `json:"model_id"`
	CooldownUntil       *time.Time `json:"cooldown_until,omitempty"`
	CooldownActive      bool       `json:"cooldown_active"`
	Reason              string     `json:"reason,omitempty"`
	LastRateLimitedAt   *time.Time `json:"last_rate_limited_at,omitempty"`
	Consecutive429Count int        `json:"consecutive_429_count"`
	SuccessCount        int        `json:"success_count"`
	FailureCount        int        `json:"failure_count"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
}

type FreeProviderModelRuntimeSummary struct {
	ActiveCooldownCount int                                     `json:"active_cooldown_count"`
	LastAttemptedModel  string                                  `json:"last_attempted_model,omitempty"`
	LastSuccessfulModel string                                  `json:"last_successful_model,omitempty"`
	Models              []FreeProviderModelRuntimeEntrySnapshot `json:"models"`
}

type freeProviderModelRuntimeEntry struct {
	modelID             string
	cooldownUntil       *time.Time
	reason              string
	lastRateLimitedAt   *time.Time
	consecutive429Count int
	successCount        int
	failureCount        int
	lastSuccessAt       *time.Time
	lastAttemptedAt     *time.Time
}

var (
	freeProviderModelRuntimeMu  sync.RWMutex
	freeProviderModelRuntime    = make(map[string]map[string]*freeProviderModelRuntimeEntry)
	freeProviderModelRuntimeNow = time.Now
)

func MarkFreeProviderModelRateLimited(deploymentID, modelID, reason string, input RelayCooldownInput) time.Duration {
	now := freeProviderModelRuntimeNow()
	duration := CalculateRelayCooldownDuration(input)
	until := now.Add(duration)

	freeProviderModelRuntimeMu.Lock()
	defer freeProviderModelRuntimeMu.Unlock()
	models := freeProviderModelRuntime[deploymentID]
	if models == nil {
		models = make(map[string]*freeProviderModelRuntimeEntry)
		freeProviderModelRuntime[deploymentID] = models
	}
	entry := models[modelID]
	if entry == nil {
		entry = &freeProviderModelRuntimeEntry{modelID: modelID}
		models[modelID] = entry
	}
	entry.cooldownUntil = cloneTime(&until)
	entry.reason = reason
	entry.lastRateLimitedAt = cloneTime(&now)
	entry.lastAttemptedAt = cloneTime(&now)
	entry.consecutive429Count++
	entry.failureCount++
	return duration
}

func RecordFreeProviderModelSuccess(deploymentID, modelID string) {
	now := freeProviderModelRuntimeNow()

	freeProviderModelRuntimeMu.Lock()
	defer freeProviderModelRuntimeMu.Unlock()
	models := freeProviderModelRuntime[deploymentID]
	if models == nil {
		models = make(map[string]*freeProviderModelRuntimeEntry)
		freeProviderModelRuntime[deploymentID] = models
	}
	entry := models[modelID]
	if entry == nil {
		entry = &freeProviderModelRuntimeEntry{modelID: modelID}
		models[modelID] = entry
	}
	entry.cooldownUntil = nil
	entry.reason = ""
	entry.consecutive429Count = 0
	entry.successCount++
	entry.lastSuccessAt = cloneTime(&now)
	entry.lastAttemptedAt = cloneTime(&now)
}

func IsFreeProviderModelCooling(deploymentID, modelID string) bool {
	now := freeProviderModelRuntimeNow()
	freeProviderModelRuntimeMu.RLock()
	defer freeProviderModelRuntimeMu.RUnlock()
	entry := freeProviderModelRuntime[deploymentID][modelID]
	return entry != nil && entry.cooldownUntil != nil && entry.cooldownUntil.After(now)
}

func SnapshotFreeProviderModelRuntime(deploymentID string) FreeProviderModelRuntimeSummary {
	now := freeProviderModelRuntimeNow()
	freeProviderModelRuntimeMu.Lock()
	defer freeProviderModelRuntimeMu.Unlock()

	summary := FreeProviderModelRuntimeSummary{}
	models := freeProviderModelRuntime[deploymentID]
	var latestAttemptedAt *time.Time
	var latestSuccessfulAt *time.Time
	for modelID, entry := range models {
		if entry.cooldownUntil != nil && !entry.cooldownUntil.After(now) {
			entry.cooldownUntil = nil
			entry.reason = ""
		}
		if entry.lastAttemptedAt != nil {
			if latestAttemptedAt == nil || entry.lastAttemptedAt.After(*latestAttemptedAt) {
				summary.LastAttemptedModel = modelID
				latestAttemptedAt = entry.lastAttemptedAt
			}
		}
		if entry.lastSuccessAt != nil && (latestSuccessfulAt == nil || entry.lastSuccessAt.After(*latestSuccessfulAt)) {
			summary.LastSuccessfulModel = modelID
			latestSuccessfulAt = entry.lastSuccessAt
		}
		if entry.cooldownUntil == nil && entry.lastRateLimitedAt == nil && entry.lastSuccessAt == nil {
			delete(models, modelID)
			continue
		}
		copy := FreeProviderModelRuntimeEntrySnapshot{
			ModelID: entry.modelID, CooldownUntil: cloneTime(entry.cooldownUntil),
			CooldownActive: entry.cooldownUntil != nil && entry.cooldownUntil.After(now),
			Reason:         entry.reason, LastRateLimitedAt: cloneTime(entry.lastRateLimitedAt),
			Consecutive429Count: entry.consecutive429Count, SuccessCount: entry.successCount,
			FailureCount: entry.failureCount, LastSuccessAt: cloneTime(entry.lastSuccessAt),
		}
		if copy.CooldownActive {
			summary.ActiveCooldownCount++
		}
		summary.Models = append(summary.Models, copy)
	}
	sort.Slice(summary.Models, func(i, j int) bool { return summary.Models[i].ModelID < summary.Models[j].ModelID })
	return summary
}

func ResetFreeProviderModelRuntime(deploymentID string) {
	freeProviderModelRuntimeMu.Lock()
	delete(freeProviderModelRuntime, deploymentID)
	freeProviderModelRuntimeMu.Unlock()
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
