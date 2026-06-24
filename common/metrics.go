package common

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// ——————— Fallback metrics (atomic counters, Prometheus text format) ———————

var (
	metricsMu          sync.Mutex
	metricsInitialized bool

	// fallback_requests_total — total requests handled by fallback system
	FallbackRequestsTotal int64

	// fallback_switch_total — deployment switch events
	FallbackSwitchTotal int64

	// fallback_failed_total — requests where all deployments failed
	FallbackFailedTotal int64

	// fallback_success_total — requests where a deployment succeeded
	FallbackSuccessTotal int64

	// claude_api_requests_total — requests via Claude API compatibility layer
	ClaudeAPIRequestsTotal int64

	// claude_api_tokens_total — tokens consumed via Claude API requests
	ClaudeAPITokensTotal int64

	// deployment_used_tokens — per-deployment token usage (label: deployment, virtual_model)
	deploymentUsedTokens      = make(map[string]int64)
	deploymentUsedTokensMu    sync.RWMutex
)

// IncFallbackRequests increments the total fallback request counter
func IncFallbackRequests() {
	atomic.AddInt64(&FallbackRequestsTotal, 1)
}

// IncFallbackSwitch increments the switch counter
func IncFallbackSwitch() {
	atomic.AddInt64(&FallbackSwitchTotal, 1)
}

// IncFallbackFailed increments the all-failed counter
func IncFallbackFailed() {
	atomic.AddInt64(&FallbackFailedTotal, 1)
}

// IncFallbackSuccess increments the success counter
func IncFallbackSuccess() {
	atomic.AddInt64(&FallbackSuccessTotal, 1)
}

// AddDeploymentTokens records token usage for a deployment
func AddDeploymentTokens(deploymentID string, tokens int64) {
	deploymentUsedTokensMu.Lock()
	deploymentUsedTokens[deploymentID] += tokens
	deploymentUsedTokensMu.Unlock()
}

// GetDeploymentTokens returns the recorded token usage for a deployment
func GetDeploymentTokens(deploymentID string) int64 {
	deploymentUsedTokensMu.RLock()
	defer deploymentUsedTokensMu.RUnlock()
	return deploymentUsedTokens[deploymentID]
}

// GetAllDeploymentTokens returns all deployment token usage
func GetAllDeploymentTokens() map[string]int64 {
	deploymentUsedTokensMu.RLock()
	defer deploymentUsedTokensMu.RUnlock()
	result := make(map[string]int64, len(deploymentUsedTokens))
	for k, v := range deploymentUsedTokens {
		result[k] = v
	}
	return result
}

// FormatPrometheusMetrics returns all metrics in Prometheus text format
func FormatPrometheusMetrics() string {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	reqTotal := atomic.LoadInt64(&FallbackRequestsTotal)
	swTotal := atomic.LoadInt64(&FallbackSwitchTotal)
	failTotal := atomic.LoadInt64(&FallbackFailedTotal)
	succTotal := atomic.LoadInt64(&FallbackSuccessTotal)
	claudeReq := atomic.LoadInt64(&ClaudeAPIRequestsTotal)
	claudeTok := atomic.LoadInt64(&ClaudeAPITokensTotal)

	var buf string

	// Counters
	buf += fmt.Sprintf("# HELP fallback_requests_total Total number of fallback requests\n")
	buf += fmt.Sprintf("# TYPE fallback_requests_total counter\n")
	buf += fmt.Sprintf("fallback_requests_total %d\n\n", reqTotal)

	buf += fmt.Sprintf("# HELP fallback_switch_total Total number of deployment switches\n")
	buf += fmt.Sprintf("# TYPE fallback_switch_total counter\n")
	buf += fmt.Sprintf("fallback_switch_total %d\n\n", swTotal)

	buf += fmt.Sprintf("# HELP fallback_failed_total Total requests where all deployments failed\n")
	buf += fmt.Sprintf("# TYPE fallback_failed_total counter\n")
	buf += fmt.Sprintf("fallback_failed_total %d\n\n", failTotal)

	buf += fmt.Sprintf("# HELP fallback_success_total Total requests where a deployment succeeded\n")
	buf += fmt.Sprintf("# TYPE fallback_success_total counter\n")
	buf += fmt.Sprintf("fallback_success_total %d\n\n", succTotal)

	buf += fmt.Sprintf("# HELP claude_api_requests_total Total requests via Claude API compatibility\n")
	buf += fmt.Sprintf("# TYPE claude_api_requests_total counter\n")
	buf += fmt.Sprintf("claude_api_requests_total %d\n\n", claudeReq)

	buf += fmt.Sprintf("# HELP claude_api_tokens_total Total tokens consumed via Claude API\n")
	buf += fmt.Sprintf("# TYPE claude_api_tokens_total counter\n")
	buf += fmt.Sprintf("claude_api_tokens_total %d\n\n", claudeTok)

	// Gauges with labels
	buf += fmt.Sprintf("# HELP deployment_used_tokens Token usage per deployment\n")
	buf += fmt.Sprintf("# TYPE deployment_used_tokens gauge\n")
	deploymentUsedTokensMu.RLock()
	for depID, tokens := range deploymentUsedTokens {
		buf += fmt.Sprintf("deployment_used_tokens{deployment=\"%s\"} %d\n", depID, tokens)
	}
	deploymentUsedTokensMu.RUnlock()

	return buf
}
