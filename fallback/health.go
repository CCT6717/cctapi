package fallback

// 部署/供应商级健康状态（Deployment / Provider Level）
//
// 本文件管理部署级健康状态（HealthStatus），属于三层状态模型中的部署/供应商级：
//   - 通过后台探针或手动触发检测部署可用性
//   - 健康状态仅用于路由决策参考，不替代渠道级禁用逻辑
//
// 渠道级的启用/禁用由 controller/relay.go 检查 channel.Status 和 monitor.DisableChannel 控制。
// 本文件的健康检查不管理模型级状态（见 free_provider_model_runtime.go）。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/songquanpeng/one-api/common/logger"
	dbmodel "github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/channeltype"
)

// Health status per deployment, kept in memory alongside the cooldown state.
type HealthStatus string

const (
	HealthHealthy     HealthStatus = "healthy"
	HealthRateLimited HealthStatus = "rate_limited"
	HealthInvalid     HealthStatus = "invalid"
	HealthError       HealthStatus = "error"
	HealthUnknown     HealthStatus = "unknown"
)

const maxHealthProbeErrorBodyBytes = 4096
const maxHealthProbeErrorDetailRunes = 500

type HealthCheckConfig struct {
	Enabled     bool `json:"enabled"`
	IntervalSec int  `json:"interval_seconds"`
	TimeoutSec  int  `json:"timeout_seconds"`
}

type healthState struct {
	mu      sync.RWMutex
	status  map[string]HealthStatus
	stopCh  chan struct{}
	running bool
}

var globalHealth = &healthState{
	status: make(map[string]HealthStatus),
}

// StartHealthChecker launches a background goroutine that pings every enabled
// deployment every IntervalSec and updates its health status. Returns immediately.
func StartHealthChecker(cfg HealthCheckConfig) {
	if !cfg.Enabled {
		logger.SysLog("[health] health checker disabled by config")
		return
	}
	interval := time.Duration(cfg.IntervalSec) * time.Second
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	globalHealth.mu.Lock()
	if globalHealth.running {
		globalHealth.mu.Unlock()
		return
	}
	globalHealth.stopCh = make(chan struct{})
	globalHealth.running = true
	globalHealth.mu.Unlock()

	go runHealthChecker(interval, timeout)
	logger.SysLogf("[health] health checker started, interval %s, timeout %s", interval, timeout)
}

// StopHealthChecker stops the background checker.
func StopHealthChecker() {
	globalHealth.mu.Lock()
	defer globalHealth.mu.Unlock()
	if globalHealth.running && globalHealth.stopCh != nil {
		close(globalHealth.stopCh)
		globalHealth.running = false
	}
}

func runHealthChecker(interval, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// decay rate-limit penalties every 2 minutes alongside health checks
	decayTicker := time.NewTicker(2 * time.Minute)
	defer decayTicker.Stop()

	checkAllDeployments(timeout)
	for {
		select {
		case <-ticker.C:
			checkAllDeployments(timeout)
		case <-decayTicker.C:
			DecayRateLimitScores()
		case <-globalHealth.stopCh:
			return
		}
	}
}

func checkAllDeployments(timeout time.Duration) {
	cfg := GetConfig()
	if cfg == nil || !cfg.Enabled {
		return
	}
	var wg sync.WaitGroup
	for id, dep := range cfg.Deployments {
		if !dep.Enabled {
			continue
		}
		wg.Add(1)
		go func(deploymentID string, depCfg DeploymentConfig) {
			defer wg.Done()
			checkOneDeployment(deploymentID, depCfg, timeout)
		}(id, dep)
	}
	wg.Wait()
}

// checkOneDeployment sends a minimal ping to a deployment and maps the
// response to a health status. It also applies cooldown for transient issues.
func checkOneDeployment(deploymentID string, dep DeploymentConfig, timeout time.Duration) {
	channel, err := dbmodel.GetChannelById(dep.ChannelID, true)
	if err != nil || channel == nil {
		RecordFailure(deploymentID, fmt.Sprintf("health check channel %d not found", dep.ChannelID), false)
		setHealthStatus(deploymentID, HealthError)
		return
	}
	if channel.Status != dbmodel.ChannelStatusEnabled {
		RecordFailure(deploymentID, fmt.Sprintf("health check channel %d disabled", dep.ChannelID), false)
		setHealthStatus(deploymentID, HealthError)
		return
	}

	// Free deployments now go through the same ping path (max_tokens=1 in
	// pingDeployment keeps per-ping cost ~1 token). Previously skipped to
	// avoid quota consumption, but that left free deployments without any
	// active probing — failures were only discovered by real requests.
	statusCode, responseBody, err := pingDeployment(deploymentID, channel, dep, timeout)
	if err != nil {
		logger.SysError(fmt.Sprintf("[health] ping %s failed: %v", deploymentID, err))
		RecordFailure(deploymentID, fmt.Sprintf("health check failed: %v", err), false)
		setHealthStatus(deploymentID, HealthError)
		_ = MarkDeploymentCooldownForDuration(deploymentID, "health check timeout", 30*time.Second)
		return
	}

	switch {
	case statusCode == http.StatusOK:
		clearRuntimeError(deploymentID)
		setHealthStatus(deploymentID, HealthHealthy)
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		reason := healthProbeFailureReason("health check unauthorized", statusCode, responseBody)
		RecordFailure(deploymentID, reason, false)
		setHealthStatus(deploymentID, HealthInvalid)
		_ = MarkInvalid(deploymentID, reason)
	case statusCode == http.StatusTooManyRequests:
		reason := healthProbeFailureReason("health check rate limited", statusCode, responseBody)
		RecordFailure(deploymentID, reason, true)
		setHealthStatus(deploymentID, HealthRateLimited)
		_ = MarkDeploymentCooldownForDuration(deploymentID, reason, 60*time.Second)
	case statusCode >= 500:
		reason := healthProbeFailureReason("health check upstream error", statusCode, responseBody)
		RecordFailure(deploymentID, reason, false)
		setHealthStatus(deploymentID, HealthError)
		_ = MarkDeploymentCooldownForDuration(deploymentID, reason, 30*time.Second)
	default:
		reason := healthProbeFailureReason("health check rejected", statusCode, responseBody)
		RecordFailure(deploymentID, reason, false)
		setHealthStatus(deploymentID, HealthError)
	}
}

// pingDeployment builds a minimal chat completion against the deployment's
// channel and returns the HTTP status code plus a small response-body sample.
func pingDeployment(deploymentID string, channel *dbmodel.Channel, dep DeploymentConfig, timeout time.Duration) (int, []byte, error) {
	req, err := buildHealthProbeRequest(deploymentID, channel, dep)
	if err != nil {
		return 0, nil, err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxHealthProbeErrorBodyBytes))
	return resp.StatusCode, body, nil
}

func healthProbeFailureReason(prefix string, statusCode int, body []byte) string {
	reason := fmt.Sprintf("%s: HTTP %d", prefix, statusCode)
	if detail := parseHealthProbeErrorDetail(body); detail != "" {
		reason += ": " + detail
	}
	return reason
}

func parseHealthProbeErrorDetail(body []byte) string {
	raw := sanitizeHealthProbeErrorText(string(body))
	if raw == "" {
		return ""
	}

	var root map[string]interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return raw
	}

	if detail := parseHealthProbeJSONError(root["error"]); detail != "" {
		return detail
	}
	if detail := jsonValueString(root["message"]); detail != "" {
		return sanitizeHealthProbeErrorText(detail)
	}
	if detail := jsonValueString(root["detail"]); detail != "" {
		return sanitizeHealthProbeErrorText(detail)
	}
	return raw
}

func parseHealthProbeJSONError(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return sanitizeHealthProbeErrorText(typed)
	case map[string]interface{}:
		message := jsonValueString(typed["message"])
		var attrs []string
		if errorType := jsonValueString(typed["type"]); errorType != "" {
			attrs = append(attrs, "type="+errorType)
		}
		if code := jsonValueString(typed["code"]); code != "" {
			attrs = append(attrs, "code="+code)
		}
		if message != "" && len(attrs) > 0 {
			return sanitizeHealthProbeErrorText(fmt.Sprintf("%s (%s)", message, strings.Join(attrs, ", ")))
		}
		if message != "" {
			return sanitizeHealthProbeErrorText(message)
		}
		if len(attrs) > 0 {
			return sanitizeHealthProbeErrorText(strings.Join(attrs, ", "))
		}
	}
	return ""
}

func jsonValueString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64, bool:
		return strings.TrimSpace(fmt.Sprint(typed))
	default:
		return ""
	}
}

func sanitizeHealthProbeErrorText(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > maxHealthProbeErrorDetailRunes {
		return string(runes[:maxHealthProbeErrorDetailRunes]) + "..."
	}
	return value
}

func buildHealthProbeRequest(deploymentID string, channel *dbmodel.Channel, dep DeploymentConfig) (*http.Request, error) {
	baseURL := buildChannelBaseURL(channel)
	if baseURL == "" {
		return nil, fmt.Errorf("channel %d has empty base url", channel.Id)
	}
	providerName, _ := FreeProviderNameFromDeploymentID(deploymentID)
	quirks := freeProviderQuirks(providerName)
	body := buildHealthProbeBody(dep, quirks, 1)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/chat/completions", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(channel.Key) != "" {
		req.Header.Set("Authorization", "Bearer "+channel.Key)
	}
	if quirks != nil && quirks.DefaultUserAgent != "" {
		req.Header.Set("User-Agent", quirks.DefaultUserAgent)
	}
	return req, nil
}

func buildHealthProbeBody(dep DeploymentConfig, quirks *FreeProviderQuirks, requestedMaxTokens int) string {
	maxTokens := requestedMaxTokens
	if maxTokens <= 0 {
		maxTokens = 1
	}
	if quirks != nil && quirks.MaxOutputTokens > 0 && quirks.MaxOutputTokens < maxTokens {
		maxTokens = quirks.MaxOutputTokens
	}
	return fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"ping"}],"max_tokens":%d,"stream":false}`, dep.RealModel, maxTokens)
}

func buildChannelBaseURL(channel *dbmodel.Channel) string {
	if channel.BaseURL != nil && *channel.BaseURL != "" {
		u := strings.TrimRight(*channel.BaseURL, "/")
		// OpenAI-compatible channels need /v1 suffix
		if channel.Type == channeltype.OpenAICompatible && !strings.HasSuffix(u, "/v1") {
			u += "/v1"
		}
		return u
	}
	return ""
}

func setHealthStatus(deploymentID string, status HealthStatus) {
	globalHealth.mu.Lock()
	defer globalHealth.mu.Unlock()
	globalHealth.status[deploymentID] = status
}

// GetHealthStatus returns the current health status of a deployment.
func GetHealthStatus(deploymentID string) HealthStatus {
	globalHealth.mu.RLock()
	defer globalHealth.mu.RUnlock()
	if s, ok := globalHealth.status[deploymentID]; ok {
		return s
	}
	return HealthUnknown
}

// IsDeploymentHealthy reports whether a deployment is healthy or unknown
// (unknown = never checked, so allowed to route).
func IsDeploymentHealthy(deploymentID string) bool {
	switch GetHealthStatus(deploymentID) {
	case HealthInvalid, HealthError:
		return false
	default:
		return true
	}
}

// SnapshotAllHealth returns a map of deploymentID -> health status for API/UI.
func SnapshotAllHealth() map[string]HealthStatus {
	globalHealth.mu.RLock()
	defer globalHealth.mu.RUnlock()
	out := make(map[string]HealthStatus, len(globalHealth.status))
	for k, v := range globalHealth.status {
		out[k] = v
	}
	return out
}

// TriggerHealthCheckForDeployment runs a single synchronous health check for one
// deployment and returns the resulting status. Exposed so the admin API can
// offer a "manual health check" button without waiting for the background loop.
func TriggerHealthCheckForDeployment(deploymentID string) (HealthStatus, error) {
	cfg := GetConfig()
	if cfg == nil || !cfg.Enabled {
		return HealthUnknown, fmt.Errorf("fallback not enabled")
	}
	dep, ok := cfg.Deployments[deploymentID]
	if !ok {
		return HealthUnknown, fmt.Errorf("deployment %s not found", deploymentID)
	}
	if !dep.Enabled {
		return HealthError, fmt.Errorf("deployment %s disabled", deploymentID)
	}
	checkOneDeployment(deploymentID, dep, 10*time.Second)
	return GetHealthStatus(deploymentID), nil
}
