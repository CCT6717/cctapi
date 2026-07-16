package fallback

import "time"

const (
	providerRateLimitObservationWindow  = 15 * time.Minute
	providerRateLimitRecoveryInterval   = 10 * time.Minute
	providerRateLimitDegradationMax     = 3
	providerRateLimitDegradationPenalty = 25
	providerRateLimitDegradationReason  = "repeated rate limits"
	providerRateLimitRecoverySuccesses  = 3
)

type ProviderRateLimitDegradationSnapshot struct {
	Active                       bool       `json:"active"`
	Level                        int        `json:"level"`
	EpisodeCount                 int        `json:"episode_count"`
	Reason                       string     `json:"reason,omitempty"`
	LastRateLimitedAt            *time.Time `json:"last_rate_limited_at,omitempty"`
	NextRecoveryAt               *time.Time `json:"next_recovery_at,omitempty"`
	ConsecutiveRecoverySuccesses int        `json:"consecutive_recovery_successes"`
}

// RecordProviderRateLimitEpisode records one provider-level rate-limit episode.
func RecordProviderRateLimitEpisode(deploymentID string, cooldownDuration time.Duration) {
	now := time.Now()
	cooldownUntil := now.Add(cooldownDuration)

	runtimeStatesMu.Lock()
	defer runtimeStatesMu.Unlock()
	recordProviderRateLimitEpisodeAtLocked(ensureRuntimeStateLocked(deploymentID, now), now, cooldownUntil)
}

// ResetProviderRateLimitDegradation clears only the in-memory degradation state.
func ResetProviderRateLimitDegradation(deploymentID string) {
	runtimeStatesMu.Lock()
	defer runtimeStatesMu.Unlock()
	if state, ok := runtimeStates[deploymentID]; ok {
		resetProviderRateLimitDegradationLocked(state)
	}
}

// SnapshotProviderRateLimitDegradation returns a safe snapshot of provider degradation state.
func SnapshotProviderRateLimitDegradation(deploymentID string) ProviderRateLimitDegradationSnapshot {
	runtimeStatesMu.RLock()
	defer runtimeStatesMu.RUnlock()
	state, ok := runtimeStates[deploymentID]
	if !ok {
		return ProviderRateLimitDegradationSnapshot{}
	}
	return snapshotProviderRateLimitDegradationLocked(state)
}

func ensureRuntimeStateLocked(deploymentID string, now time.Time) *DeploymentRuntimeState {
	if state, ok := runtimeStates[deploymentID]; ok {
		return state
	}
	state := &DeploymentRuntimeState{
		DeploymentID:    deploymentID,
		LastResetMinute: now.Truncate(time.Minute),
		LastResetDay:    truncateToDay(now),
	}
	runtimeStates[deploymentID] = state
	return state
}

func recordProviderRateLimitEpisodeAtLocked(state *DeploymentRuntimeState, now, cooldownUntil time.Time) {
	if !state.LastProviderRateLimitedAt.IsZero() && now.Sub(state.LastProviderRateLimitedAt) >= providerRateLimitObservationWindow {
		resetProviderRateLimitDegradationLocked(state)
	}

	if !state.RateLimitEpisodeCooldownUntil.IsZero() {
		if cooldownUntil.Equal(state.RateLimitEpisodeCooldownUntil) || !now.After(state.RateLimitEpisodeCooldownUntil) {
			return
		}
	}

	state.RateLimitEpisodeCount++
	state.LastProviderRateLimitedAt = now
	state.RateLimitEpisodeCooldownUntil = cooldownUntil
	state.ConsecutiveRecoverySuccesses = 0
	if state.RateLimitEpisodeCount == 1 {
		return
	}
	if state.RateLimitDegradationLevel < providerRateLimitDegradationMax {
		state.RateLimitDegradationLevel++
	}
	state.NextDegradationRecoveryAt = now.Add(providerRateLimitRecoveryInterval)
}

func recordProviderRateLimitDegradationSuccessAtLocked(state *DeploymentRuntimeState, now time.Time) {
	if state.RateLimitDegradationLevel == 0 {
		resetProviderRateLimitDegradationLocked(state)
		return
	}

	state.ConsecutiveRecoverySuccesses++
	if state.ConsecutiveRecoverySuccesses < providerRateLimitRecoverySuccesses {
		return
	}

	state.RateLimitDegradationLevel--
	state.ConsecutiveRecoverySuccesses = 0
	if state.RateLimitDegradationLevel == 0 {
		resetProviderRateLimitDegradationLocked(state)
		return
	}
	state.NextDegradationRecoveryAt = now.Add(providerRateLimitRecoveryInterval)
}

func decayProviderRateLimitDegradationAtLocked(state *DeploymentRuntimeState, now time.Time) {
	if state.RateLimitDegradationLevel == 0 || state.NextDegradationRecoveryAt.IsZero() || now.Before(state.NextDegradationRecoveryAt) {
		return
	}

	state.RateLimitDegradationLevel--
	state.ConsecutiveRecoverySuccesses = 0
	if state.RateLimitDegradationLevel == 0 {
		resetProviderRateLimitDegradationLocked(state)
		return
	}
	state.NextDegradationRecoveryAt = now.Add(providerRateLimitRecoveryInterval)
}

func resetProviderRateLimitDegradationLocked(state *DeploymentRuntimeState) {
	state.RateLimitEpisodeCount = 0
	state.RateLimitDegradationLevel = 0
	state.LastProviderRateLimitedAt = time.Time{}
	state.RateLimitEpisodeCooldownUntil = time.Time{}
	state.NextDegradationRecoveryAt = time.Time{}
	state.ConsecutiveRecoverySuccesses = 0
}

func snapshotProviderRateLimitDegradationLocked(state *DeploymentRuntimeState) ProviderRateLimitDegradationSnapshot {
	snapshot := ProviderRateLimitDegradationSnapshot{
		Active:                       state.RateLimitDegradationLevel > 0,
		Level:                        state.RateLimitDegradationLevel,
		EpisodeCount:                 state.RateLimitEpisodeCount,
		ConsecutiveRecoverySuccesses: state.ConsecutiveRecoverySuccesses,
	}
	if snapshot.Active {
		snapshot.Reason = providerRateLimitDegradationReason
	}
	if !state.LastProviderRateLimitedAt.IsZero() {
		lastRateLimitedAt := state.LastProviderRateLimitedAt
		snapshot.LastRateLimitedAt = &lastRateLimitedAt
	}
	if !state.NextDegradationRecoveryAt.IsZero() {
		nextRecoveryAt := state.NextDegradationRecoveryAt
		snapshot.NextRecoveryAt = &nextRecoveryAt
	}
	return snapshot
}
