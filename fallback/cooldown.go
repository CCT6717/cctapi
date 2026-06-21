package fallback

import (
	"time"
)

// CooldownDurations centralises the cooldown policy per error category.
// Lower-coupling shim over the existing MarkDeploymentCooldown / state machine.
type CooldownPolicy struct {
	RateLimitShort time.Duration // minute-window 429
	RateLimitDay   time.Duration // daily quota 429 -> next reset
	TemporaryShort time.Duration // 5xx / timeout
	GatewayError   time.Duration // 502/503/504 with retry-after absent
}

var DefaultCooldownPolicy = CooldownPolicy{
	RateLimitShort: 60 * time.Second,
	RateLimitDay:   24 * time.Hour,
	TemporaryShort: 30 * time.Second,
	GatewayError:   30 * time.Second,
}

// ApplyCooldown marks a deployment cooled down based on the error category.
// It returns the duration used so callers can log it.
func ApplyCooldown(deploymentID, reason string, category ErrorCategory) time.Duration {
	switch category {
	case ErrorCategoryQuota:
		if err := MarkDeploymentExhausted(deploymentID, reason, EndOfToday()); err == nil {
			return time.Until(EndOfToday())
		}
		return 0
	case ErrorCategoryRateLimit:
		d := DefaultCooldownPolicy.RateLimitShort
		if err := MarkDeploymentCooldownForDuration(deploymentID, reason, d); err == nil {
			return d
		}
		return 0
	case ErrorCategoryTemporary:
		d := DefaultCooldownPolicy.TemporaryShort
		if err := MarkDeploymentCooldownForDuration(deploymentID, reason, d); err == nil {
			return d
		}
		return 0
	default:
		return 0
	}
}

// MarkInvalid marks a deployment invalid (401/403). Invalid deployments are
// cooled down for a long time so they drop out of routing until manually recovered.
func MarkInvalid(deploymentID, reason string) error {
	return MarkDeploymentCooldownForDuration(deploymentID, reason, 24*time.Hour)
}
