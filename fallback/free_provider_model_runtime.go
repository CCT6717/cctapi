package fallback

// 本文件管理模型级运行时状态（model-level runtime state）。
// 只有 Kilo 供应商支持模型级轮换，状态通过 freeProviderModelRuntime 内存映射维护。
// 模型级状态包括：短期冷却、连续429计数、成功/失败计数。
// 模型级状态独立于部署级状态：模型429不触发部署级冷却或 RateLimitScore，
// 只有所有兼容模型耗尽或遇到非429错误时，才按部署级处理。
// 该层级的状态是进程级内存状态，由多个请求共享，进程重启后清空。部署恢复时通过 ResetFreeProviderModelRuntime 重置。

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

// MarkFreeProviderModelRateLimited 属于模型级状态操作，记录单个模型的429冷却与连续计数，不修改部署级状态。
func MarkFreeProviderModelRateLimited(deploymentID, modelID, reason string, input RelayCooldownInput) time.Duration {
	_ = reason // Reserved for the planned interface; runtime state stores only the safe category.
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
	entry.reason = "rate limited"
	entry.lastRateLimitedAt = cloneTime(&now)
	entry.lastAttemptedAt = cloneTime(&now)
	entry.consecutive429Count++
	entry.failureCount++
	return duration
}

// RecordFreeProviderModelSuccess 属于模型级状态操作，记录单个模型的成功并清除其短期冷却，不修改部署级状态。
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

// IsFreeProviderModelCooling 属于模型级状态操作，检查单个模型是否处于短期冷却，不修改部署级状态。
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
