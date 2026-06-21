package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/fallback"
	"github.com/songquanpeng/one-api/middleware"
	dbmodel "github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/monitor"
	"github.com/songquanpeng/one-api/relay/controller"
	"github.com/songquanpeng/one-api/relay/model"
	"github.com/songquanpeng/one-api/relay/relaymode"
)

// https://platform.openai.com/docs/api-reference/chat

// getRelayErrorMessage extracts error message from ErrorWithStatusCode
func getRelayErrorMessage(bizErr *model.ErrorWithStatusCode) string {
	if bizErr == nil {
		return ""
	}
	if bizErr.Error.Message != "" {
		return bizErr.Error.Message
	}
	return fmt.Sprintf("relay error with status code %d", bizErr.StatusCode)
}

// calculateCooldownDuration determines the cooldown duration based on error type and attempt count.
//
// Rules:
//   - 429 with Retry-After header: use parsed value, capped at maxCooldown (300s)
//   - 503/502/504 without Retry-After: exponential backoff min(60*2^(attempt-1), 300)
//   - 429 without Retry-After: fall back to 60s default
//   - Other temporary errors: exponential backoff
func calculateCooldownDuration(bizErr *model.ErrorWithStatusCode, attempt int) time.Duration {
	const (
		maxCooldown    = 300 * time.Second
		defaultTimeout = 60 * time.Second
	)

	if bizErr == nil {
		return defaultTimeout
	}

	// If Retry-After header was provided by upstream, use it (capped at max)
	if bizErr.RetryAfterSeconds != nil && *bizErr.RetryAfterSeconds > 0 {
		duration := time.Duration(*bizErr.RetryAfterSeconds) * time.Second
		if duration > maxCooldown {
			duration = maxCooldown
		}
		return duration
	}

	// For 503/502/504 (temporary/server errors) without Retry-After, use exponential backoff
	if bizErr.StatusCode == http.StatusServiceUnavailable ||
		bizErr.StatusCode == http.StatusBadGateway ||
		bizErr.StatusCode == http.StatusGatewayTimeout {
		// min(60 * 2^(attempt-1), 300)
		expDuration := defaultTimeout * time.Duration(1<<uint(attempt-1))
		if expDuration > maxCooldown || expDuration <= 0 {
			expDuration = maxCooldown
		}
		return expDuration
	}

	// For all other errors (including 429 without Retry-After), use default
	return defaultTimeout
}

func relayHelper(c *gin.Context, relayMode int) *model.ErrorWithStatusCode {
	var err *model.ErrorWithStatusCode
	switch relayMode {
	case relaymode.ImagesGenerations:
		err = controller.RelayImageHelper(c, relayMode)
	case relaymode.AudioSpeech:
		fallthrough
	case relaymode.AudioTranslation:
		fallthrough
	case relaymode.AudioTranscription:
		err = controller.RelayAudioHelper(c, relayMode)
	case relaymode.Proxy:
		err = controller.RelayProxyHelper(c, relayMode)
	default:
		err = controller.RelayTextHelper(c)
	}
	return err
}

func relayModeRecordsFallbackUsage(relayMode int) bool {
	switch relayMode {
	case relaymode.ChatCompletions, relaymode.Completions, relaymode.Embeddings, relaymode.Moderations, relaymode.Edits:
		return true
	default:
		return false
	}
}

// fallbackSwitchLog writes a structured JSON log entry for deployment switches
func fallbackSwitchLog(ctx context.Context, virtualModel, fromDeployment, toDeployment, reason string, statusCode int, durationMs int64) {
	requestID := helper.GetRequestID(ctx)
	entry := map[string]interface{}{
		"event":           "fallback_switch",
		"virtual_model":   virtualModel,
		"from_deployment": fromDeployment,
		"to_deployment":   toDeployment,
		"reason":          reason,
		"status_code":     statusCode,
		"duration_ms":     durationMs,
		"request_id":      requestID,
	}
	data, _ := json.Marshal(entry)
	logger.Infof(ctx, "%s", string(data))
	if err := fallback.RecordSwitchEvent(fallback.SwitchEvent{
		CreatedAt:      time.Now().UTC(),
		VirtualModel:   virtualModel,
		FromDeployment: fromDeployment,
		ToDeployment:   toDeployment,
		Reason:         reason,
		StatusCode:     statusCode,
		DurationMs:     durationMs,
		RequestID:      requestID,
	}); err != nil {
		logger.SysError(fmt.Sprintf("[fallback] failed to persist switch event: %v", err))
	}
}

func relayWithFallback(c *gin.Context) {
	ctx := c.Request.Context()
	requestId := c.GetString(helper.RequestIdKey)
	requestModelValue, exists := c.Get(ctxkey.RequestModel)
	if !exists {
		err := model.Error{
			Message: "No request model found",
			Type:    "one_api_error",
			Param:   "",
			Code:    "no_request_model",
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}

	virtualModel, ok := requestModelValue.(string)
	if !ok || virtualModel == "" {
		err := model.Error{
			Message: "Invalid request model format",
			Type:    "one_api_error",
			Param:   "",
			Code:    "invalid_model_format",
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}

	// Get all deployments for this virtual model
	deployments, err := fallback.GetDeploymentsForVirtualModel(virtualModel)
	// Sticky routing: prefer last successful deployment; skip only when it hits soft limit, error, or becomes unavailable.
	stickyID := fallback.GetStickyDeployment(virtualModel)
	if stickyID != "" {
		for i, dep := range deployments {
			if dep.ID == stickyID {
				if i > 0 {
					deployments = append([]fallback.DeploymentConfig{dep}, append(deployments[:i], deployments[i+1:]...)...)
					logger.Infof(ctx, "[fallback] sticky routing: virtual model %s pinned to deployment %s", virtualModel, stickyID)
				}
				break
			}
		}
	}
	if err != nil {
		err := model.Error{
			Message: fmt.Sprintf("No available deployments for virtual model %s: %s", virtualModel, err.Error()),
			Type:    "one_api_error",
			Param:   "",
			Code:    "no_deployments",
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err})
		return
	}

	// Read the original request body once
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		err := model.Error{
			Message: fmt.Sprintf("Failed to read request body: %s", err.Error()),
			Type:    "one_api_error",
			Param:   "",
			Code:    "read_body_failed",
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}

	// Parse request body for context length estimation (pre-filtering)
	var estimatedTokens int
	var parsedRequest model.GeneralOpenAIRequest
	if err := json.Unmarshal(bodyBytes, &parsedRequest); err == nil && len(parsedRequest.Messages) > 0 {
		estimatedTokens = estimateTokenCount(&parsedRequest)
		if estimatedTokens > 0 {
			logger.Infof(ctx, "[fallback] estimated request tokens: %d", estimatedTokens)
		}
	}

	// Detect required capabilities (vision/tools/json/stream) and filter deployments.
	caps := fallback.DetectRequestCapabilities(&parsedRequest)
	if caps.MaxTokens == 0 && estimatedTokens > 0 {
		caps.MaxTokens = estimatedTokens
	}
	beforeCap := len(deployments)
	deployments = fallback.FilterByCapability(deployments, caps)
	if len(deployments) < beforeCap {
		logger.Infof(ctx, "[fallback] capability filter: %d -> %d deployments (vision=%v tools=%v json=%v stream=%v)",
			beforeCap, len(deployments), caps.Vision, caps.Tools, caps.JSON, caps.Stream)
	}

	// Health filter: drop deployments marked invalid or in error state by the
	// background health checker. healthy/unknown are allowed to route.
	beforeHealth := len(deployments)
	deployments = filterHealthyDeployments(deployments)
	if len(deployments) < beforeHealth {
		logger.Infof(ctx, "[fallback] health filter: %d -> %d deployments", beforeHealth, len(deployments))
	}

	// Strategy-aware sort (quality_first / cost_first / free_first).
	if vm, ok := fallback.GetVirtualModel(virtualModel); ok && len(deployments) > 1 {
		deployments = fallback.SortByStrategy(deployments, vm.Strategy)
	}

	if stickyID != "" {
		logger.Infof(ctx, "[fallback] sticky active for %s -> %s", virtualModel, stickyID)
	} else if len(deployments) > 0 {
		logger.Infof(ctx, "[fallback] strategy-based start deployment for %s: %s", virtualModel, deployments[0].ID)
	}





	relayMode := relaymode.GetByPath(c.Request.URL.Path)
	var lastBizErr *model.ErrorWithStatusCode
	var attempts int
	var prevDeployment string // track previous deployment for switch log
	var prevDurationMs int64
	deployCount := len(deployments)

	for i, dep := range deployments {
		attempts = i + 1

		// Check if deployment is available (with state filtering)
		available, reason := fallback.IsDeploymentAvailable(dep)
		if !available {
			logger.Infof(ctx, "[fallback] deployment %s unavailable: %s", dep.ID, reason)
			if fallback.IsDoubaoDeployment(dep) {
				if strings.Contains(strings.ToLower(reason), "soft daily token limit") ||
					strings.Contains(strings.ToLower(reason), "hard daily token limit") {
					if err := fallback.MarkDeploymentCooldownForDuration(dep.ID, reason, 24*time.Hour); err != nil {
						logger.SysError(fmt.Sprintf("[fallback] failed to mark 24h cooldown for %s: %v", dep.ID, err))
					} else {
						logger.Infof(ctx, "[fallback] deployment %s marked 24h cooldown after limit skip", dep.ID)
					}
				}
			}
			lastBizErr = &model.ErrorWithStatusCode{
				StatusCode: http.StatusServiceUnavailable,
				Error: model.Error{
					Message: reason,
					Type:    "fallback_unavailable",
					Code:    "deployment_unavailable",
				},
			}
			prevDeployment = dep.ID
			prevDurationMs = 0
			continue
		}


		// Get channel by deployment's channel ID
		channel, err := dbmodel.GetChannelById(dep.ChannelID, true)
		if err != nil {
			logger.Infof(ctx, "[fallback] deployment %s channel %d not found, skipping", dep.ID, dep.ChannelID)
			lastBizErr = &model.ErrorWithStatusCode{
				StatusCode: http.StatusServiceUnavailable,
				Error: model.Error{
					Message: fmt.Sprintf("channel %d not found", dep.ChannelID),
					Type:    "fallback_channel",
					Code:    "channel_not_found",
				},
			}
			fallback.MarkDeploymentCooldown(dep.ID, "channel not found", time.Now().Add(60*time.Second))
			prevDeployment = dep.ID
			prevDurationMs = 0
			continue
		}

		if channel.Status != dbmodel.ChannelStatusEnabled {
			logger.Infof(ctx, "[fallback] deployment %s channel %d is disabled, skipping", dep.ID, dep.ChannelID)
			fallback.MarkDeploymentCooldown(dep.ID, "channel disabled", time.Now().Add(60*time.Second))
			lastBizErr = &model.ErrorWithStatusCode{
				StatusCode: http.StatusForbidden,
				Error: model.Error{
					Message: fmt.Sprintf("channel %d is disabled", dep.ChannelID),
					Type:    "fallback_channel",
					Code:    "channel_disabled",
				},
			}
			prevDeployment = dep.ID
			prevDurationMs = 0
			continue
		}

		// Four-dimensional quota pre-check: RPM/RPD/TPM/TPD before sending the request.
		runtimeState := fallback.GetRuntimeState(dep.ID)
		if !fallback.PassQuotaCheck(dep, runtimeState, caps.MaxTokens) {
			logger.Infof(ctx, "[fallback] deployment %s quota pre-check failed (rpm=%d/%d rpd=%d/%d tpm=%d/%d tpd=%d/%d), skipping",
				dep.ID, runtimeState.MinuteRequests, dep.RPMLimit, runtimeState.DayRequests, dep.RPDLimit,
				runtimeState.MinuteTokens, dep.TPMLimit, runtimeState.DayTokens, dep.TPDLimit)
			lastBizErr = &model.ErrorWithStatusCode{
				StatusCode: http.StatusTooManyRequests,
				Error: model.Error{
					Message: fmt.Sprintf("deployment %s reached RPM/RPD/TPM/TPD limit", dep.ID),
					Type:    "fallback_quota",
					Code:    "deployment_quota_exceeded",
				},
			}
			prevDeployment = dep.ID
			prevDurationMs = 0
			continue
		}

		releaseDeploymentSlot, acquired, inFlight := fallback.TryAcquireDeploymentSlot(dep)
		if !acquired {
			logger.Infof(ctx, "[fallback] deployment %s concurrency limit reached: %d/%d, skipping",
				dep.ID, inFlight, dep.MaxConcurrentRequests)
			lastBizErr = &model.ErrorWithStatusCode{
				StatusCode: http.StatusTooManyRequests,
				Error: model.Error{
					Message: fmt.Sprintf("deployment %s concurrency limit reached: %d/%d", dep.ID, inFlight, dep.MaxConcurrentRequests),
					Type:    "fallback_concurrency",
					Code:    "deployment_concurrency_limit",
				},
			}
			prevDeployment = dep.ID
			prevDurationMs = 0
			continue
		}
		if dep.MaxConcurrentRequests > 0 {
			logger.Infof(ctx, "[fallback] deployment %s concurrency slot acquired: %d/%d",
				dep.ID, inFlight, dep.MaxConcurrentRequests)
		}

		// Log switch if this is not the first attempt
		if prevDeployment != "" {
			fallbackSwitchLog(ctx, virtualModel, prevDeployment, dep.ID,
				getRelayErrorMessage(lastBizErr), lastBizErr.StatusCode, prevDurationMs)
			common.IncFallbackSwitch()
		}

		// Set fallback context keys in gin.Context
		c.Set(ctxkey.FallbackEnabled, true)
		c.Set(ctxkey.FallbackVirtualModel, virtualModel)
		c.Set(ctxkey.FallbackDeploymentID, dep.ID)
		c.Set(ctxkey.FallbackRealModel, dep.RealModel)
		c.Set(ctxkey.FallbackChannelID, dep.ChannelID)
		c.Set(ctxkey.FallbackDeploymentIndex, i)
		c.Set(ctxkey.FallbackAttemptCount, attempts)
		// Refresh all channel-specific context for this deployment
		middleware.SetupContextForSelectedChannel(c, channel, virtualModel)

		logger.Infof(ctx, "[fallback] switched to channel id=%d name=%s model=%s",
			dep.ChannelID, channel.Name, dep.RealModel)

		// Set fallback context keys in context.Context for postConsumeQuota
		newCtx := context.WithValue(ctx, ctxkey.FallbackVirtualModel, virtualModel)
		newCtx = context.WithValue(newCtx, ctxkey.FallbackDeploymentID, dep.ID)
		newCtx = context.WithValue(newCtx, ctxkey.FallbackRealModel, dep.RealModel)
		newCtx = context.WithValue(newCtx, ctxkey.FallbackChannelID, dep.ChannelID)
		newCtx = context.WithValue(newCtx, ctxkey.FallbackDeploymentIndex, i)
		newCtx = context.WithValue(newCtx, ctxkey.FallbackAttemptCount, attempts)
		c.Request = c.Request.WithContext(newCtx)

		// Reset request body for this attempt
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		attemptStart := time.Now()
		logger.Infof(ctx, "[fallback] attempt %d/%d virtual model %s deployment %s channel %d real model %s",
			attempts, deployCount, virtualModel, dep.ID, dep.ChannelID, dep.RealModel)

		// Execute the relay helper
		bizErr := relayHelper(c, relayMode)
		durationMs := time.Since(attemptStart).Milliseconds()
		releaseDeploymentSlot()
		// Debug: log full attempt details to help diagnose fallback behaviour
		if bizErr != nil {
			errInfo := fallback.FormatRelayErrorInfo(bizErr.StatusCode, getRelayErrorMessage(bizErr), bizErr.Error.Type, bizErr.Error.Code)
			errClass := fallback.ClassifyRelayError(errInfo)
			logger.Debugf(newCtx, "[fallback] attempt_result attempt=%d/%d deployment=%s status=%d msg=%q code=%q category=%v should_fallback=%v", attempts, deployCount, dep.ID, bizErr.StatusCode, bizErr.Error.Message, bizErr.Error.Code, errClass.Category, errClass.ShouldFallback)
		} else {
			logger.Debugf(newCtx, "[fallback] attempt_result attempt=%d/%d deployment=%s status=success duration=%dms", attempts, deployCount, dep.ID, durationMs)
		}

		if bizErr == nil {
			fallback.SetStickyDeployment(virtualModel, dep.ID)
			// Success - report to monitor and record for smart sorting
			monitor.Emit(dep.ChannelID, true)
			if !relayModeRecordsFallbackUsage(relayMode) {
				fallback.RecordDeploymentSuccess(dep.ID, fallback.UsageInfo{})
			}
			// Record runtime usage for RPM/RPD/TPM/TPD tracking.
			// Use estimated tokens when upstream usage isn't reported via UsageInfo path.
			fallback.RecordUsage(dep.ID, effectiveTokenCount(caps.MaxTokens))
			fallback.RecordSuccess(dep.ID)
			common.IncFallbackSuccess()
			logger.Infof(ctx, "[fallback] deployment %s succeeded in %dms", dep.ID, durationMs)
			return
		}

		lastBizErr = bizErr
		prevDeployment = dep.ID
		prevDurationMs = durationMs
		relayErr := errors.New(getRelayErrorMessage(bizErr))
		logger.Infof(ctx, "[fallback] deployment %s failed (attempt %d/%d, %dms): %v",
			dep.ID, attempts, deployCount, durationMs, getRelayErrorMessage(bizErr))

		// Classify error using structured info (single-pass, replaces 4 separate string scans)
		errInfo := fallback.FormatRelayErrorInfo(bizErr.StatusCode, getRelayErrorMessage(bizErr), bizErr.Error.Type, bizErr.Error.Code)
		errClass := fallback.ClassifyRelayError(errInfo)

		// Record error state
		fallback.RecordDeploymentError(dep.ID, relayErr)
		fallback.RecordFailure(dep.ID, getRelayErrorMessage(bizErr), errClass.Category == fallback.ErrorCategoryRateLimit)

		shouldFallback := errClass.ShouldFallback
		if shouldFallback {
			logger.Infof(ctx, "[fallback] error classified as fallbackable (category=%v): %s",
				errClass.Category, getRelayErrorMessage(bizErr))
		} else {
			logger.Infof(ctx, "[fallback] error classified as non-fallbackable (category=%v): %s",
				errClass.Category, getRelayErrorMessage(bizErr))
			logger.Infof(ctx, "[fallback] deployment %s returned non-fallback error, stopping attempts: %v",
				dep.ID, getRelayErrorMessage(bizErr))
			errCopy := *bizErr
			errCopy.Error.Message = helper.MessageWithRequestId(errCopy.Error.Message, requestId)
			c.JSON(errCopy.StatusCode, gin.H{
				"error": errCopy.Error,
			})
			return
		}

		// If this is a stream response that has already started writing to client,
		// we cannot fallback anymore.
		if bizErr.StatusCode == http.StatusOK && c.Writer.Written() {
			logger.Infof(ctx, "[fallback] response already written for deployment %s, stopping attempts",
				dep.ID)
			return
		}
		// Report channel status to monitor for auto-disable tracking
		if monitor.ShouldDisableChannel(&bizErr.Error, bizErr.StatusCode) {
			monitor.DisableChannel(dep.ChannelID, channel.Name, getRelayErrorMessage(bizErr))
		} else {
			monitor.Emit(dep.ChannelID, false)
		}

		// Mark deployment state based on error category
		if errClass.Category == fallback.ErrorCategoryQuota {
			fallback.MarkDeploymentExhausted(dep.ID, getRelayErrorMessage(bizErr), fallback.EndOfToday())
			logger.Infof(ctx, "[fallback] deployment %s marked exhausted until end of day: %s",
				dep.ID, getRelayErrorMessage(bizErr))
		} else if errClass.Category == fallback.ErrorCategoryRateLimit || errClass.Category == fallback.ErrorCategoryTemporary {
			cooldownDuration := calculateCooldownDuration(bizErr, attempts)
			cooldownUntil := time.Now().Add(cooldownDuration)
			fallback.MarkDeploymentCooldown(dep.ID, getRelayErrorMessage(bizErr), cooldownUntil)
			logger.Infof(ctx, "[fallback] deployment %s marked cooling down for %.0fs: %s",
				dep.ID, cooldownDuration.Seconds(), getRelayErrorMessage(bizErr))
		}
		// Doubao-specific: quota errors get 24h cooldown
		if fallback.IsDoubaoDeployment(dep) && shouldFallback && errClass.Category == fallback.ErrorCategoryQuota {
			if err := fallback.MarkDeploymentCooldownForDuration(dep.ID, getRelayErrorMessage(bizErr), 24*time.Hour); err != nil {
				logger.SysError(fmt.Sprintf("[fallback] failed to mark doubao 24h cooldown for %s: %v", dep.ID, err))
			} else {
				logger.Infof(ctx, "[fallback] deployment %s marked 24h cooldown after doubao quota error", dep.ID)
			}
		}

		// Continue to next deployment
		continue
	}

	// All deployments failed — fire alert event
	common.IncFallbackFailed()
	// Clear sticky since all deployments failed
	fallback.ClearStickyDeployment(virtualModel)
	logger.Infof(ctx, "[fallback] all %d deployments failed for virtual model %s",
		deployCount, virtualModel)

	// Fire a critical alert for total failure
	fallback.GlobalAlertManager.FireAlert(fallback.AlertEvent{
		DeploymentID: virtualModel,
		Level:        fallback.AlertCritical,
		Type:         fallback.AlertAllFailed,
		Message:      fmt.Sprintf("all %d deployments failed for virtual model %s", deployCount, virtualModel),
		CreatedAt:    time.Now(),
	})

	// Unified error response — never pass raw upstream errors to client
	errResponse := model.Error{
		Message: "所有上游均不可用，请稍后重试",
		Type:    "one_api_error",
		Param:   "",
		Code:    "all_deployments_failed",
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": errResponse})
}

func Relay(c *gin.Context) {
	ctx := c.Request.Context()

	// Initialize fallback state store if not already done
	if common.IsFallbackEnabled {
		if err := fallback.InitStateStore(); err != nil {
			logger.SysError(fmt.Sprintf("failed to initialize fallback state store: %v", err))
		}
	}

	// Check if this is a fallback request
	if common.IsFallbackEnabled && fallback.IsVirtualModel(c.GetString(ctxkey.RequestModel)) {
		common.IncFallbackRequests()
		relayWithFallback(c)
		return
	}

	// Normal One API relay flow
	relayMode := relaymode.GetByPath(c.Request.URL.Path)
	if config.DebugEnabled {
		requestBody, _ := common.GetRequestBody(c)
		logger.Debugf(ctx, "request body: %s", string(requestBody))
	}
	channelId := c.GetInt(ctxkey.ChannelId)
	userId := c.GetInt(ctxkey.Id)
	bizErr := relayHelper(c, relayMode)
	if bizErr == nil {
		monitor.Emit(channelId, true)
		return
	}
	lastFailedChannelId := channelId
	channelName := c.GetString(ctxkey.ChannelName)
	group := c.GetString(ctxkey.Group)
	originalModel := c.GetString(ctxkey.OriginalModel)
	go processChannelRelayError(ctx, userId, channelId, channelName, *bizErr)
	requestId := c.GetString(helper.RequestIdKey)
	retryTimes := config.RetryTimes
	if !shouldRetry(c, bizErr.StatusCode) {
		logger.Errorf(ctx, "relay error happen, status code is %d, won't retry in this case", bizErr.StatusCode)
		retryTimes = 0
	}
	for i := retryTimes; i > 0; i-- {
		channel, err := dbmodel.CacheGetRandomSatisfiedChannel(group, originalModel, i != retryTimes)
		if err != nil {
			logger.Errorf(ctx, "CacheGetRandomSatisfiedChannel failed: %+v", err)
			break
		}
		logger.Infof(ctx, "using channel #%d to retry (remain times %d)", channel.Id, i)
		if channel.Id == lastFailedChannelId {
			continue
		}
		middleware.SetupContextForSelectedChannel(c, channel, originalModel)
		requestBody, err := common.GetRequestBody(c)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		bizErr = relayHelper(c, relayMode)
		if bizErr == nil {
			return
		}
		channelId := c.GetInt(ctxkey.ChannelId)
		lastFailedChannelId = channelId
		channelName := c.GetString(ctxkey.ChannelName)
		go processChannelRelayError(ctx, userId, channelId, channelName, *bizErr)
	}
	if bizErr != nil {
		// Copy before mutation to avoid race with goroutines from processChannelRelayError
		errCopy := *bizErr
		if errCopy.StatusCode == http.StatusTooManyRequests {
			errCopy.Error.Message = "当前分组上游负载已饱和，请稍后再试"
		}

		errCopy.Error.Message = helper.MessageWithRequestId(errCopy.Error.Message, requestId)
		c.JSON(errCopy.StatusCode, gin.H{
			"error": errCopy.Error,
		})
	}
}

func shouldRetry(c *gin.Context, statusCode int) bool {
	if _, ok := c.Get(ctxkey.SpecificChannelId); ok {
		return false
	}
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	if statusCode/100 == 5 {
		return true
	}
	if statusCode == http.StatusBadRequest {
		return false
	}
	if statusCode/100 == 2 {
		return false
	}
	return true
}

func processChannelRelayError(ctx context.Context, userId int, channelId int, channelName string, err model.ErrorWithStatusCode) {
	logger.Errorf(ctx, "relay error (channel id %d, user id: %d): %s", channelId, userId, err.Message)
	// https://platform.openai.com/docs/guides/error-codes/api-errors
	if monitor.ShouldDisableChannel(&err.Error, err.StatusCode) {
		monitor.DisableChannel(channelId, channelName, err.Message)
	} else {
		monitor.Emit(channelId, false)
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := model.Error{
		Message: "API not implemented",
		Type:    "one_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := model.Error{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

// estimateTokenCount estimates the token count of a request using character-based approximation.
// Roughly 3.5 characters per token — works well for mixed Chinese/English text.
// Adds max_tokens / max_completion_tokens from the request to account for expected output.
// Returns 0 if the request has no messages (e.g. image generation, audio, embedding).
// effectiveTokenCount returns a non-zero token estimate for runtime usage
// accounting. Falls back to a small default when estimation produced nothing
// (e.g. non-text relay modes) so the request still counts toward RPM/RPD.
func effectiveTokenCount(estimated int) int {
	if estimated > 0 {
		return estimated
	}
	return 1024
}

// filterHealthyDeployments drops deployments that the background health checker
// has marked invalid or in a persistent error state. healthy/unknown pass through.
func filterHealthyDeployments(deployments []fallback.DeploymentConfig) []fallback.DeploymentConfig {
	out := make([]fallback.DeploymentConfig, 0, len(deployments))
	for _, dep := range deployments {
		if fallback.IsDeploymentHealthy(dep.ID) {
			out = append(out, dep)
		}
	}
	return out
}

func estimateTokenCount(req *model.GeneralOpenAIRequest) int {
	if req == nil || len(req.Messages) == 0 {
		return 0
	}

	totalChars := 0
	for _, msg := range req.Messages {
		totalChars += len(msg.Role)
		switch content := msg.Content.(type) {
		case string:
			totalChars += len(content)
		case []any:
			for _, part := range content {
				if m, ok := part.(map[string]any); ok {
					if text, ok := m["text"].(string); ok {
						totalChars += len(text)
					}
				}
			}
		}
	}

	// Rough estimate: ~3.5 chars per token for mixed Chinese/English
	estimatedTokens := int(float64(totalChars) / 3.5)

	// Add max_tokens to account for expected output
	if req.MaxTokens > 0 {
		estimatedTokens += req.MaxTokens
	}
	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > 0 {
		estimatedTokens += *req.MaxCompletionTokens
	}

	return estimatedTokens
}
