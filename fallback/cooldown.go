package fallback

import (
	"net/http"
	"time"
)

// CooldownDurations centralises the cooldown policy per error category.
// Lower-coupling shim over the existing MarkDeploymentCooldown / state machine.
type CooldownPolicy struct {
	RateLimitShort time.Duration // minute-window 429
	RateLimitDay   time.Duration // daily quota 429 -> next reset
	TemporaryShort time.Duration // 5xx / timeout
	GatewayError   time.Duration // 502/503/504 with retry-after absent
	Max            time.Duration // hard cap for relay-driven cooldowns
}

var DefaultCooldownPolicy = CooldownPolicy{
	RateLimitShort: 60 * time.Second,
	RateLimitDay:   24 * time.Hour,
	TemporaryShort: 30 * time.Second,
	GatewayError:   30 * time.Second,
	Max:            300 * time.Second,
}

type RelayCooldownInput struct {
	Category          ErrorCategory
	StatusCode        int
	RetryAfterSeconds *int
	Attempt           int
}

func CalculateRelayCooldownDuration(input RelayCooldownInput) time.Duration {
	if input.RetryAfterSeconds != nil && *input.RetryAfterSeconds > 0 {
		duration := time.Duration(*input.RetryAfterSeconds) * time.Second
		if duration > DefaultCooldownPolicy.Max {
			return DefaultCooldownPolicy.Max
		}
		return duration
	}

	if input.StatusCode == http.StatusBadGateway ||
		input.StatusCode == http.StatusServiceUnavailable ||
		input.StatusCode == http.StatusGatewayTimeout {
		attempt := input.Attempt
		if attempt < 1 {
			attempt = 1
		}
		duration := DefaultCooldownPolicy.RateLimitShort * time.Duration(1<<uint(attempt-1))
		if duration <= 0 || duration > DefaultCooldownPolicy.Max {
			return DefaultCooldownPolicy.Max
		}
		return duration
	}

	return DefaultCooldownPolicy.RateLimitShort
}

func ApplyRelayCooldown(deploymentID, reason string, input RelayCooldownInput) (time.Duration, error) {
	switch input.Category {
	case ErrorCategoryQuota:
		until := EndOfToday()
		if err := MarkDeploymentExhausted(deploymentID, reason, until); err != nil {
			return 0, err
		}
		return time.Until(until), nil
	case ErrorCategoryRateLimit, ErrorCategoryTemporary:
		duration := CalculateRelayCooldownDuration(input)
		if err := MarkDeploymentCooldown(deploymentID, reason, time.Now().Add(duration)); err != nil {
			return 0, err
		}
		return duration, nil
	default:
		return 0, nil
	}
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
