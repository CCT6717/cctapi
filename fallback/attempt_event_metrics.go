package fallback

import (
	"sync"
	"sync/atomic"
)

// ---------------------------------------------------------------------------
// In-memory attempt metrics — Phase 4: success events are aggregated here
// rather than persisted to SQLite, to avoid rapid database growth.
// ---------------------------------------------------------------------------

// AttemptMetrics holds aggregate counters and latency summaries for a single
// deployment. All fields are updated atomically so they can be read without locks.
type AttemptMetrics struct {
	SuccessCount    int64
	FailureCount    int64
	SkipCount       int64
	TotalDurationMs int64
	TotalSuccessMs  int64
	MinDurationMs   int64
	MaxDurationMs   int64
}

var (
	attemptMetricsMu    sync.RWMutex
	attemptMetricsStore map[string]*AttemptMetrics // deploymentID -> metrics
)

func ensureAttemptMetrics(deploymentID string) *AttemptMetrics {
	attemptMetricsMu.RLock()
	m, ok := attemptMetricsStore[deploymentID]
	attemptMetricsMu.RUnlock()
	if ok {
		return m
	}
	attemptMetricsMu.Lock()
	defer attemptMetricsMu.Unlock()
	if attemptMetricsStore == nil {
		attemptMetricsStore = make(map[string]*AttemptMetrics)
	}
	m, ok = attemptMetricsStore[deploymentID]
	if !ok {
		m = &AttemptMetrics{MinDurationMs: -1}
		attemptMetricsStore[deploymentID] = m
	}
	return m
}

// RecordAttemptSuccess updates in-memory metrics for a successful attempt.
func RecordAttemptSuccess(deploymentID string, durationMs int64) {
	m := ensureAttemptMetrics(deploymentID)
	atomic.AddInt64(&m.SuccessCount, 1)
	atomic.AddInt64(&m.TotalDurationMs, durationMs)
	atomic.AddInt64(&m.TotalSuccessMs, durationMs)

	for {
		oldMin := atomic.LoadInt64(&m.MinDurationMs)
		if oldMin == -1 || durationMs < oldMin {
			if atomic.CompareAndSwapInt64(&m.MinDurationMs, oldMin, durationMs) {
				break
			}
		} else {
			break
		}
	}

	for {
		oldMax := atomic.LoadInt64(&m.MaxDurationMs)
		if durationMs > oldMax {
			if atomic.CompareAndSwapInt64(&m.MaxDurationMs, oldMax, durationMs) {
				break
			}
		} else {
			break
		}
	}
}

// RecordAttemptFailure updates in-memory metrics for a failed attempt.
func RecordAttemptFailure(deploymentID string, durationMs int64) {
	m := ensureAttemptMetrics(deploymentID)
	atomic.AddInt64(&m.FailureCount, 1)
	atomic.AddInt64(&m.TotalDurationMs, durationMs)

	for {
		oldMin := atomic.LoadInt64(&m.MinDurationMs)
		if oldMin == -1 || durationMs < oldMin {
			if atomic.CompareAndSwapInt64(&m.MinDurationMs, oldMin, durationMs) {
				break
			}
		} else {
			break
		}
	}

	for {
		oldMax := atomic.LoadInt64(&m.MaxDurationMs)
		if durationMs > oldMax {
			if atomic.CompareAndSwapInt64(&m.MaxDurationMs, oldMax, durationMs) {
				break
			}
		} else {
			break
		}
	}
}

// RecordAttemptSkip updates in-memory metrics for a skipped attempt.
func RecordAttemptSkip(deploymentID string) {
	m := ensureAttemptMetrics(deploymentID)
	atomic.AddInt64(&m.SkipCount, 1)
}

// SnapshotAttemptMetrics returns a copy of the current metrics for a deployment.
func SnapshotAttemptMetrics(deploymentID string) AttemptMetrics {
	attemptMetricsMu.RLock()
	m, ok := attemptMetricsStore[deploymentID]
	attemptMetricsMu.RUnlock()
	if !ok {
		return AttemptMetrics{MinDurationMs: -1}
	}
	return AttemptMetrics{
		SuccessCount:    atomic.LoadInt64(&m.SuccessCount),
		FailureCount:    atomic.LoadInt64(&m.FailureCount),
		SkipCount:       atomic.LoadInt64(&m.SkipCount),
		TotalDurationMs: atomic.LoadInt64(&m.TotalDurationMs),
		TotalSuccessMs:  atomic.LoadInt64(&m.TotalSuccessMs),
		MinDurationMs:   atomic.LoadInt64(&m.MinDurationMs),
		MaxDurationMs:   atomic.LoadInt64(&m.MaxDurationMs),
	}
}

// AllAttemptMetricsSnapshot returns a snapshot of all in-memory metrics.
// It reads the store map under the lock but directly loads each metric's
// atomic fields without calling SnapshotAttemptMetrics, avoiding deadlocks.
func AllAttemptMetricsSnapshot() map[string]AttemptMetrics {
	attemptMetricsMu.RLock()
	defer attemptMetricsMu.RUnlock()
	out := make(map[string]AttemptMetrics, len(attemptMetricsStore))
	for id, m := range attemptMetricsStore {
		out[id] = AttemptMetrics{
			SuccessCount:    atomic.LoadInt64(&m.SuccessCount),
			FailureCount:    atomic.LoadInt64(&m.FailureCount),
			SkipCount:       atomic.LoadInt64(&m.SkipCount),
			TotalDurationMs: atomic.LoadInt64(&m.TotalDurationMs),
			TotalSuccessMs:  atomic.LoadInt64(&m.TotalSuccessMs),
			MinDurationMs:   atomic.LoadInt64(&m.MinDurationMs),
			MaxDurationMs:   atomic.LoadInt64(&m.MaxDurationMs),
		}
	}
	return out
}

// ResetAttemptMetrics clears in-memory metrics for a deployment (or all if empty).
func ResetAttemptMetrics(deploymentID string) {
	attemptMetricsMu.Lock()
	defer attemptMetricsMu.Unlock()
	if deploymentID == "" {
		attemptMetricsStore = make(map[string]*AttemptMetrics)
		return
	}
	delete(attemptMetricsStore, deploymentID)
}

// ---------------------------------------------------------------------------
// Error category counters (global, atomic)
// ---------------------------------------------------------------------------

var (
	errorCategoryCounters sync.Map // string -> *int64
)

// RecordErrorCategoryCounter increments a global counter for the given error category.
func RecordErrorCategoryCounter(category ErrorCategory) {
	key := category.String()
	if key == "" {
		key = "none"
	}
	v, _ := errorCategoryCounters.LoadOrStore(key, new(int64))
	atomic.AddInt64(v.(*int64), 1)
}

// SnapshotErrorCategoryCounters returns a copy of all error category counters.
func SnapshotErrorCategoryCounters() map[string]int64 {
	out := make(map[string]int64)
	errorCategoryCounters.Range(func(k, v interface{}) bool {
		out[k.(string)] = atomic.LoadInt64(v.(*int64))
		return true
	})
	return out
}

// ResetErrorCategoryCounters safely clears all category counters by deleting
// all keys from the sync.Map. This avoids the fatal "unlock of unlocked mutex"
// that can occur when replacing a sync.Map while concurrent operations are in
// flight (Go 1.24+ swiss table implementation).
func ResetErrorCategoryCounters() {
	var keys []string
	errorCategoryCounters.Range(func(k, v interface{}) bool {
		keys = append(keys, k.(string))
		return true
	})
	for _, k := range keys {
		errorCategoryCounters.Delete(k)
	}
}
