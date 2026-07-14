package fallback

// 部署/供应商级冷却策略（Deployment / Provider Level）
//
// 本文件实现部署级冷却策略，属于三层状态模型中的部署/供应商级：
//   - CooldownPolicy：按错误类别定义冷却时长
//   - ApplyRelayCooldown / ApplyCooldown：根据错误分类驱动冷却决策
//
// 冷却决策由 controller/relay.go 的错误分类结果驱动，最终作用于部署/供应商级。
// 不管理模型级冷却（见 free_provider_model_runtime.go）或渠道级禁用（见 monitor.DisableChannel）。

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
	case ErrorCategoryModelAccess:
		if input.StatusCode == http.StatusUnauthorized || input.StatusCode == http.StatusForbidden {
			if err := MarkInvalid(deploymentID, reason); err != nil {
				return 0, err
			}
			return 24 * time.Hour, nil
		}
		return 0, nil
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
