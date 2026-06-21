package fallback

import (
	"sync"
	"time"
)

// DeploymentRuntimeState tracks live usage counters for RPM/RPD/TPM/TPD enforcement.
// One row per deployment, kept in memory; reset on minute/day boundaries.
type DeploymentRuntimeState struct {
	DeploymentID string

	MinuteRequests int
	DayRequests    int
	MinuteTokens   int
	DayTokens      int

	LastResetMinute time.Time
	LastResetDay    time.Time

	SuccessCount int
	FailureCount int
	RateLimitScore int
	LastError     string
	LastErrorAt   time.Time
}

var (
	runtimeStates   = make(map[string]*DeploymentRuntimeState)
	runtimeStatesMu sync.RWMutex
)

// GetRuntimeState returns the runtime state for a deployment, creating one if missing.
func GetRuntimeState(deploymentID string) *DeploymentRuntimeState {
	runtimeStatesMu.RLock()
	s, ok := runtimeStates[deploymentID]
	runtimeStatesMu.RUnlock()
	if ok {
		return s
	}

	runtimeStatesMu.Lock()
	defer runtimeStatesMu.Unlock()
	// double-check after acquiring write lock
	if s, ok := runtimeStates[deploymentID]; ok {
		return s
	}
	now := time.Now()
	s = &DeploymentRuntimeState{
		DeploymentID:   deploymentID,
		LastResetMinute: now.Truncate(time.Minute),
		LastResetDay:    truncateToDay(now),
	}
	runtimeStates[deploymentID] = s
	return s
}

func truncateToDay(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// maybeResetWindows rolls over the minute and day counters when their windows expire.
// Callers must hold no lock; this function locks internally.
func (s *DeploymentRuntimeState) maybeResetWindows() {
	now := time.Now()
	minuteStart := now.Truncate(time.Minute)
	if s.LastResetMinute.Before(minuteStart) {
		s.MinuteRequests = 0
		s.MinuteTokens = 0
		s.LastResetMinute = minuteStart
	}
	dayStart := truncateToDay(now)
	if s.LastResetDay.Before(dayStart) {
		s.DayRequests = 0
		s.DayTokens = 0
		s.LastResetDay = dayStart
	}
}

// PassQuotaCheck returns true if a request of estimatedTokens tokens would fit
// within the deployment's RPM/RPD/TPM/TPD limits. A limit of 0 means unchecked.
// It does NOT mutate the state; use RecordUsage after the request succeeds.
func PassQuotaCheck(dep DeploymentConfig, state *DeploymentRuntimeState, estimatedTokens int) bool {
	if state == nil {
		return true
	}
	if dep.RPMLimit > 0 && state.MinuteRequests+1 > dep.RPMLimit {
		return false
	}
	if dep.RPDLimit > 0 && state.DayRequests+1 > dep.RPDLimit {
		return false
	}
	if dep.TPMLimit > 0 && state.MinuteTokens+estimatedTokens > dep.TPMLimit {
		return false
	}
	if dep.TPDLimit > 0 && state.DayTokens+estimatedTokens > dep.TPDLimit {
		return false
	}
	return true
}

// RecordUsage increments the request and token counters after a successful relay.
// totalTokens should come from upstream usage; if absent, pass the estimated value.
func RecordUsage(deploymentID string, totalTokens int) {
	runtimeStatesMu.Lock()
	defer runtimeStatesMu.Unlock()
	s, ok := runtimeStates[deploymentID]
	if !ok {
		// create lazily without nested lock
		now := time.Now()
		s = &DeploymentRuntimeState{
			DeploymentID:   deploymentID,
			LastResetMinute: now.Truncate(time.Minute),
			LastResetDay:    truncateToDay(now),
		}
		runtimeStates[deploymentID] = s
	}
	s.maybeResetWindows()
	s.MinuteRequests++
	s.DayRequests++
	s.MinuteTokens += totalTokens
	s.DayTokens += totalTokens
}

// RecordSuccess bumps success counter and clears the recent-error flag.
func RecordSuccess(deploymentID string) {
	runtimeStatesMu.Lock()
	defer runtimeStatesMu.Unlock()
	if s, ok := runtimeStates[deploymentID]; ok {
		s.SuccessCount++
		s.LastError = ""
	}
}

// RecordFailure bumps failure counter, stamps last error, and bumps the
// rate-limit penalty score (capped at 10). Creates the state if missing.
func RecordFailure(deploymentID, errMsg string, isRateLimit bool) {
	runtimeStatesMu.Lock()
	defer runtimeStatesMu.Unlock()
	s, ok := runtimeStates[deploymentID]
	if !ok {
		now := time.Now()
		s = &DeploymentRuntimeState{
			DeploymentID:   deploymentID,
			LastResetMinute: now.Truncate(time.Minute),
			LastResetDay:    truncateToDay(now),
		}
		runtimeStates[deploymentID] = s
	}
	s.FailureCount++
	s.LastError = errMsg
	s.LastErrorAt = time.Now()
	if isRateLimit {
		s.RateLimitScore++
		if s.RateLimitScore > 10 {
			s.RateLimitScore = 10
		}
	}
}

// DecayRateLimitScores is meant to be called periodically (e.g. every 2 minutes)
// to age out stale rate-limit penalties so a deployment can recover.
func DecayRateLimitScores() {
	runtimeStatesMu.Lock()
	defer runtimeStatesMu.Unlock()
	cutoff := time.Now().Add(-2 * time.Minute)
	for _, s := range runtimeStates {
		if s.RateLimitScore > 0 && s.LastErrorAt.Before(cutoff) {
			s.RateLimitScore--
		}
	}
}

// SnapshotRuntimeState returns a safe copy for API responses / UI.
type RuntimeStateSnapshot struct {
	DeploymentID    string    `json:"deployment_id"`
	MinuteRequests  int       `json:"minute_requests"`
	DayRequests     int       `json:"day_requests"`
	MinuteTokens    int       `json:"minute_tokens"`
	DayTokens       int       `json:"day_tokens"`
	SuccessCount    int       `json:"success_count"`
	FailureCount    int       `json:"failure_count"`
	RateLimitScore  int       `json:"rate_limit_score"`
	LastError       string    `json:"last_error"`
	LastErrorAt     time.Time `json:"last_error_at"`
}

func SnapshotRuntimeState(deploymentID string) RuntimeStateSnapshot {
	runtimeStatesMu.RLock()
	defer runtimeStatesMu.RUnlock()
	s, ok := runtimeStates[deploymentID]
	if !ok {
		return RuntimeStateSnapshot{DeploymentID: deploymentID}
	}
	return RuntimeStateSnapshot{
		DeploymentID:   s.DeploymentID,
		MinuteRequests: s.MinuteRequests,
		DayRequests:    s.DayRequests,
		MinuteTokens:   s.MinuteTokens,
		DayTokens:      s.DayTokens,
		SuccessCount:   s.SuccessCount,
		FailureCount:   s.FailureCount,
		RateLimitScore: s.RateLimitScore,
		LastError:      s.LastError,
		LastErrorAt:    s.LastErrorAt,
	}
}
