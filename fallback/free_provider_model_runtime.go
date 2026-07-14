package fallback

import (
	"sort"
	"strings"
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
	entry.reason = sanitizeFreeProviderModelRuntimeReason(reason)
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
			if entry.lastSuccessAt == nil && entry.successCount == 0 {
				delete(models, modelID)
				continue
			}
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
	if len(models) == 0 {
		delete(freeProviderModelRuntime, deploymentID)
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

const freeProviderModelRuntimeReasonMaxLength = 128

func sanitizeFreeProviderModelRuntimeReason(reason string) string {
	fields := strings.Fields(reason)
	parts := make([]string, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if strings.EqualFold(field, "bearer") {
			parts = append(parts, "Bearer [REDACTED]")
			if i+1 < len(fields) {
				i++
			}
			continue
		}
		parts = append(parts, redactFreeProviderModelRuntimeToken(field))
	}

	sanitized := strings.Join(parts, " ")
	if sanitized == "" {
		return "rate limited"
	}
	if !strings.Contains(strings.ToLower(sanitized), "rate") {
		sanitized = "rate limited: " + sanitized
	}
	runes := []rune(sanitized)
	if len(runes) > freeProviderModelRuntimeReasonMaxLength {
		sanitized = string(runes[:freeProviderModelRuntimeReasonMaxLength])
	}
	return sanitized
}

func redactFreeProviderModelRuntimeToken(token string) string {
	lower := strings.ToLower(token)
	for _, key := range []string{
		"api_key=", "api-key=", "access_token=", "access-token=", "authorization=", "token=",
	} {
		if index := strings.Index(lower, key); index >= 0 {
			return token[:index+len(key)] + "[REDACTED]"
		}
	}
	return token
}
