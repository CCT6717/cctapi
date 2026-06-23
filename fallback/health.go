package fallback

import (
	"context"
	"fmt"
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

type HealthCheckConfig struct {
	Enabled         bool `json:"enabled"`
	IntervalSec     int  `json:"interval_seconds"`
	TimeoutSec      int  `json:"timeout_seconds"`
}

type healthState struct {
	mu     sync.RWMutex
	status map[string]HealthStatus
	stopCh chan struct{}
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
	if err != nil || channel == nil || channel.Status != dbmodel.ChannelStatusEnabled {
		setHealthStatus(deploymentID, HealthError)
		return
	}

	// Free deployments now go through the same ping path (max_tokens=1 in
	// pingDeployment keeps per-ping cost ~1 token). Previously skipped to
	// avoid quota consumption, but that left free deployments without any
	// active probing — failures were only discovered by real requests.
	statusCode, err := pingDeployment(channel, dep, timeout)
	if err != nil {
		logger.SysError(fmt.Sprintf("[health] ping %s failed: %v", deploymentID, err))
		setHealthStatus(deploymentID, HealthError)
		_ = MarkDeploymentCooldownForDuration(deploymentID, "health check timeout", 30*time.Second)
		return
	}

	switch {
	case statusCode == http.StatusOK:
		setHealthStatus(deploymentID, HealthHealthy)
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		setHealthStatus(deploymentID, HealthInvalid)
		_ = MarkInvalid(deploymentID, "health check unauthorized")
	case statusCode == http.StatusTooManyRequests:
		setHealthStatus(deploymentID, HealthRateLimited)
		_ = MarkDeploymentCooldownForDuration(deploymentID, "health check rate limited", 60*time.Second)
	case statusCode >= 500:
		setHealthStatus(deploymentID, HealthError)
		_ = MarkDeploymentCooldownForDuration(deploymentID, "health check 5xx", 30*time.Second)
	default:
		setHealthStatus(deploymentID, HealthHealthy)
	}
}

// pingDeployment builds a minimal chat completion against the deployment's
// channel and returns the HTTP status code. It does NOT parse the body.
func pingDeployment(channel *dbmodel.Channel, dep DeploymentConfig, timeout time.Duration) (int, error) {
	baseURL := buildChannelBaseURL(channel)
	if baseURL == "" {
		return 0, fmt.Errorf("channel %d has empty base url", channel.Id)
	}

	// ponytail: minimal ping body, max_tokens=1 to keep cost near zero
	body := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"ping"}],"max_tokens":1,"stream":false}`, dep.RealModel)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/chat/completions", strings.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+channel.Key)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
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
