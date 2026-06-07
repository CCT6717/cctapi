package fallback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/songquanpeng/one-api/common/logger"
)

// ——————— Alert types ———————

type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
)

type AlertType string

const (
	AlertSoftLimit AlertType = "soft_limit"
	AlertHardLimit AlertType = "hard_limit"
	AlertExhausted AlertType = "exhausted"
	AlertRecovered AlertType = "recovered"
	AlertCooldown  AlertType = "cooldown"
	AlertAllFailed AlertType = "all_failed"
)

type AlertEvent struct {
	DeploymentID string     `json:"deployment_id"`
	Level        AlertLevel `json:"level"`
	Type         AlertType  `json:"type"`
	Message      string     `json:"message"`
	UsedTokens   int64      `json:"used_tokens"`
	DailyLimit   int64      `json:"daily_limit"`
	Percentage   float64    `json:"percentage"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ——————— Alert notification config ———————

type AlertConfig struct {
	Enabled           bool   `json:"enabled"`
	CheckIntervalSec  int    `json:"check_interval_seconds"`
	WebhookURL        string `json:"webhook_url"`
	NotifyOnSoftLimit bool   `json:"notify_on_soft_limit"`
	NotifyOnHardLimit bool   `json:"notify_on_hard_limit"`
	NotifyOnExhausted bool   `json:"notify_on_exhausted"`
	NotifyOnRecovered bool   `json:"notify_on_recovered"`
	NotifyOnAllFailed bool   `json:"notify_on_all_failed"`
}

// ——————— Alert manager (in-memory state) ———————

// firedKey tracks a unique alert event to avoid duplicates
type firedKey struct {
	DeploymentID string
	AlertType    AlertType
}

type AlertManager struct {
	mu       sync.RWMutex
	fired    map[firedKey]time.Time // key → when it was last fired
	silenced map[string]bool        // deploymentID → silenced?
	config   AlertConfig
	stopCh   chan struct{}
	running  bool
}

var GlobalAlertManager = &AlertManager{
	fired:    make(map[firedKey]time.Time),
	silenced: make(map[string]bool),
	config: AlertConfig{
		Enabled:           false,
		CheckIntervalSec:  300,
		NotifyOnSoftLimit: true,
		NotifyOnHardLimit: true,
		NotifyOnExhausted: true,
		NotifyOnRecovered: true,
	},
}

// InitAlertManager reads alert config and starts background checker
func InitAlertManager() {
	if !IsEnabled() {
		return
	}

	configLock.RLock()
	alertCfg := config.Alert
	configLock.RUnlock()

	if !alertCfg.Enabled {
		logger.SysLog("[alert] alert manager disabled by config")
		return
	}

	GlobalAlertManager.mu.Lock()
	GlobalAlertManager.config = alertCfg
	GlobalAlertManager.stopCh = make(chan struct{})
	GlobalAlertManager.running = true
	GlobalAlertManager.mu.Unlock()

	go GlobalAlertManager.runChecker()

	logger.SysLog(fmt.Sprintf("[alert] alert manager started, check interval: %ds", alertCfg.CheckIntervalSec))
}

// StopAlertManager stops the background checker
func StopAlertManager() {
	GlobalAlertManager.mu.Lock()
	defer GlobalAlertManager.mu.Unlock()
	if GlobalAlertManager.running && GlobalAlertManager.stopCh != nil {
		close(GlobalAlertManager.stopCh)
		GlobalAlertManager.running = false
	}
}

func (am *AlertManager) runChecker() {
	am.mu.RLock()
	interval := time.Duration(am.config.CheckIntervalSec) * time.Second
	am.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once immediately
	am.checkAllDeployments()

	for {
		select {
		case <-ticker.C:
			am.checkAllDeployments()
		case <-am.stopCh:
			return
		}
	}
}

func (am *AlertManager) checkAllDeployments() {
	config := GetConfig()
	if config == nil || !config.Enabled {
		return
	}

	am.mu.RLock()
	alertCfg := am.config
	am.mu.RUnlock()

	var wg sync.WaitGroup

	for id, dep := range config.Deployments {
		if !dep.Enabled {
			continue
		}

		wg.Add(1)
		go func(deploymentID string, depCfg DeploymentConfig) {
			defer wg.Done()
			am.checkDeployment(deploymentID, depCfg, alertCfg)
		}(id, dep)
	}

	wg.Wait()
}

func (am *AlertManager) checkDeployment(id string, dep DeploymentConfig, alertCfg AlertConfig) {
	state, err := EnsureDeploymentState(id, todayString())
	if err != nil {
		return
	}

	if dep.DailyLimitTokens <= 0 {
		return // unlimited, skip
	}

	used := state.UsedTotalTokens
	limit := dep.DailyLimitTokens
	pct := float64(used) / float64(limit) * 100
	softThreshold := dep.SoftLimitRatio * 100
	hardThreshold := dep.HardLimitRatio * 100

	// Check hard limit
	if hardThreshold > 0 && pct >= hardThreshold {
		if alertCfg.NotifyOnHardLimit {
			am.fireAlert(AlertEvent{
				DeploymentID: id,
				Level:        AlertCritical,
				Type:         AlertHardLimit,
				Message:      fmt.Sprintf("deployment %s reached hard limit: %.1f%% (used=%d, limit=%d)", id, pct, used, limit),
				UsedTokens:   used,
				DailyLimit:   limit,
				Percentage:   pct,
				CreatedAt:    time.Now(),
			})
		}
		return // hard limit supersedes soft limit
	}

	// Check soft limit
	if softThreshold > 0 && pct >= softThreshold {
		if alertCfg.NotifyOnSoftLimit {
			am.fireAlert(AlertEvent{
				DeploymentID: id,
				Level:        AlertWarning,
				Type:         AlertSoftLimit,
				Message:      fmt.Sprintf("deployment %s reached soft limit: %.1f%% (used=%d, limit=%d)", id, pct, used, limit),
				UsedTokens:   used,
				DailyLimit:   limit,
				Percentage:   pct,
				CreatedAt:    time.Now(),
			})
		}
		return
	}

	// Check exhausted
	if state.ExhaustedUntil != nil && state.ExhaustedUntil.After(time.Now()) {
		if alertCfg.NotifyOnExhausted {
			am.fireAlert(AlertEvent{
				DeploymentID: id,
				Level:        AlertCritical,
				Type:         AlertExhausted,
				Message:      fmt.Sprintf("deployment %s exhausted until %s", id, state.ExhaustedUntil.Format(time.RFC3339)),
				UsedTokens:   used,
				DailyLimit:   limit,
				Percentage:   pct,
				CreatedAt:    time.Now(),
			})
		}
		return
	}

	// Check cooldown
	if state.CooldownUntil != nil && state.CooldownUntil.After(time.Now()) {
		if alertCfg.NotifyOnExhausted {
			am.fireAlert(AlertEvent{
				DeploymentID: id,
				Level:        AlertWarning,
				Type:         AlertCooldown,
				Message:      fmt.Sprintf("deployment %s cooling down until %s", id, state.CooldownUntil.Format(time.RFC3339)),
				UsedTokens:   used,
				DailyLimit:   limit,
				Percentage:   pct,
				CreatedAt:    time.Now(),
			})
		}
		return
	}

	// Check recovery: if we previously fired an alert for this deployment and now it's normal
	recoveries := am.collectRecoveries(id, pct, alertCfg)
	for _, evt := range recoveries {
		am.fireAlert(evt)
	}
}

// collectRecoveries finds previously-fired alerts that have now returned to normal.
// Returns alert events to fire, does NOT fire them itself.
func (am *AlertManager) collectRecoveries(id string, currentPct float64, alertCfg AlertConfig) []AlertEvent {
	am.mu.Lock()
	defer am.mu.Unlock()

	alertTypes := []AlertType{AlertSoftLimit, AlertHardLimit, AlertExhausted, AlertCooldown}
	var recoveries []AlertEvent
	for _, at := range alertTypes {
		key := firedKey{DeploymentID: id, AlertType: at}
		if _, exists := am.fired[key]; exists {
			delete(am.fired, key)
			if alertCfg.NotifyOnRecovered {
				recoveries = append(recoveries, AlertEvent{
					DeploymentID: id,
					Level:        AlertInfo,
					Type:         AlertRecovered,
					Message:      fmt.Sprintf("deployment %s recovered (current usage: %.1f%%)", id, currentPct),
					UsedTokens:   0,
					DailyLimit:   0,
					Percentage:   currentPct,
					CreatedAt:    time.Now(),
				})
			}
		}
	}
	return recoveries
}

func (am *AlertManager) fireAlert(event AlertEvent) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.fireAlertLocked(event)
}

func (am *AlertManager) fireAlertLocked(event AlertEvent) {
	// Check silence
	if am.silenced[event.DeploymentID] {
		return
	}

	// Dedup: don't fire the same alert type for the same deployment within cooldown period
	key := firedKey{DeploymentID: event.DeploymentID, AlertType: event.Type}
	if lastFired, exists := am.fired[key]; exists {
		if time.Since(lastFired) < 10*time.Minute {
			return
		}
	}
	am.fired[key] = time.Now()

	// Log
	logger.SysLog(fmt.Sprintf("[alert] [%s] [%s] %s", event.Level, event.Type, event.Message))
	if err := RecordAlertEvent(event); err != nil {
		logger.SysError(fmt.Sprintf("[alert] failed to persist alert event: %v", err))
	}

	// Webhook
	if am.config.WebhookURL != "" {
		go am.sendWebhook(event)
	}
}

func (am *AlertManager) sendWebhook(event AlertEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		logger.SysError(fmt.Sprintf("[alert] failed to marshal webhook payload: %v", err))
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(am.config.WebhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		logger.SysError(fmt.Sprintf("[alert] webhook request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		logger.SysError(fmt.Sprintf("[alert] webhook returned %d", resp.StatusCode))
	}
}

// ——————— Public API ———————

// GetAlertEvents returns recent alert events
func (am *AlertManager) GetAlertEvents() []AlertEvent {
	records, err := GetAlertHistory(100)
	if err != nil {
		return nil
	}
	events := make([]AlertEvent, 0, len(records))
	for _, record := range records {
		events = append(events, AlertEvent{
			DeploymentID: record.DeploymentID,
			Level:        AlertLevel(record.Level),
			Type:         AlertType(record.Type),
			Message:      record.Message,
			UsedTokens:   record.UsedTokens,
			DailyLimit:   record.DailyLimit,
			Percentage:   record.Percentage,
			CreatedAt:    record.CreatedAt,
		})
	}
	return events
}

// FireAlert publicly fires an alert event (respects dedup and silence rules)
func (am *AlertManager) FireAlert(event AlertEvent) {
	am.mu.RLock()
	notifyOnAllFailed := am.config.NotifyOnAllFailed
	am.mu.RUnlock()

	if !notifyOnAllFailed {
		// Still log but don't fire webhook
		logger.SysLog(fmt.Sprintf("[alert] [%s] [%s] %s", event.Level, event.Type, event.Message))
		return
	}

	am.fireAlert(event)
}

// SilencedDeployments returns currently silenced deployment IDs
func (am *AlertManager) GetSilencedDeployments() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	ids := make([]string, 0, len(am.silenced))
	for id := range am.silenced {
		ids = append(ids, id)
	}
	return ids
}

// SilenceDeployment silences alerts for a deployment
func (am *AlertManager) SilenceDeployment(id string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.silenced[id] = true
}

// UnsilenceDeployment removes silence for a deployment
func (am *AlertManager) UnsilenceDeployment(id string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.silenced, id)
}

func (am *AlertManager) MarkAlertFired(id string, alertType AlertType) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.fired[firedKey{DeploymentID: id, AlertType: alertType}] = time.Now()
}

func (am *AlertManager) ClearFiredAlerts(id string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	alertTypes := []AlertType{AlertSoftLimit, AlertHardLimit, AlertExhausted, AlertCooldown}
	for _, alertType := range alertTypes {
		delete(am.fired, firedKey{DeploymentID: id, AlertType: alertType})
	}
}

// IsDeploymentSilenced checks if a deployment is silenced
func (am *AlertManager) IsDeploymentSilenced(id string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.silenced[id]
}

// GetAlertConfig returns a copy of the alert config
func (am *AlertManager) GetAlertConfig() AlertConfig {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.config
}

// GetAlertStatus returns alert status summary for all deployments
func GetAlertStatus() []map[string]interface{} {
	config := GetConfig()
	if config == nil || !config.Enabled {
		return nil
	}

	var result []map[string]interface{}

	for id, dep := range config.Deployments {
		if !dep.Enabled {
			continue
		}

		state, err := EnsureDeploymentState(id, todayString())
		if err != nil {
			continue
		}

		pct := 0.0
		if dep.DailyLimitTokens > 0 {
			pct = float64(state.UsedTotalTokens) / float64(dep.DailyLimitTokens) * 100
		}

		alertLevel := "normal"
		alertType := ""
		if state.ExhaustedUntil != nil && state.ExhaustedUntil.After(time.Now()) {
			alertLevel = "critical"
			alertType = "exhausted"
		} else if state.CooldownUntil != nil && state.CooldownUntil.After(time.Now()) {
			alertLevel = "warning"
			alertType = "cooldown"
		} else if dep.DailyLimitTokens > 0 && dep.HardLimitRatio > 0 && pct >= dep.HardLimitRatio*100 {
			alertLevel = "critical"
			alertType = "hard_limit"
		} else if dep.DailyLimitTokens > 0 && dep.SoftLimitRatio > 0 && pct >= dep.SoftLimitRatio*100 {
			alertLevel = "warning"
			alertType = "soft_limit"
		}

		silenced := GlobalAlertManager.IsDeploymentSilenced(id)

		entry := map[string]interface{}{
			"deployment_id":           id,
			"channel_id":              dep.ChannelID,
			"real_model":              dep.RealModel,
			"enabled":                 dep.Enabled,
			"weight":                  dep.Weight,
			"max_concurrent_requests": dep.MaxConcurrentRequests,
			"in_flight_requests":      GetDeploymentInFlight(id),
			"used_tokens":             state.UsedTotalTokens,
			"daily_limit":             dep.DailyLimitTokens,
			"usage_percent":           fmt.Sprintf("%.1f%%", pct),
			"alert_level":             alertLevel,
			"alert_type":              alertType,
			"silenced":                silenced,
			"exhausted_until":         state.ExhaustedUntil,
			"cooldown_until":          state.CooldownUntil,
		}
		result = append(result, entry)
	}

	return result
}
