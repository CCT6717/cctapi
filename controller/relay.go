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
	"github.com/songquanpeng/one-api/common/claudeutil"
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

type fallbackRelayExecutor func(*gin.Context, int) *model.ErrorWithStatusCode

func relayWithFallback(c *gin.Context) {
	relayWithFallbackUsing(c, relayHelper)
}

func relayWithFallbackUsing(c *gin.Context, execute fallbackRelayExecutor) {
	ctx := c.Request.Context()
	requestId := c.GetString(helper.RequestIdKey)
	requestModelValue, exists := c.Get(ctxkey.RequestModel)
	if !exists {
		claudeutil.WriteClaudeOrOpenAIError(c, http.StatusInternalServerError, "one_api_error", "No request model found")
		return
	}

	virtualModel, ok := requestModelValue.(string)
	if !ok || virtualModel == "" {
		claudeutil.WriteClaudeOrOpenAIError(c, http.StatusInternalServerError, "one_api_error", "Invalid request model format")
		return
	}

	// Read the original request body once
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		claudeutil.WriteClaudeOrOpenAIError(c, http.StatusInternalServerError, "one_api_error", fmt.Sprintf("Failed to read request body: %s", err.Error()))
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

	plan, err := fallback.PrepareDeploymentPlanForRequest(virtualModel, caps)
	if err != nil {
		claudeutil.WriteClaudeOrOpenAIError(c, http.StatusServiceUnavailable, "one_api_error", fmt.Sprintf("No available deployments for virtual model %s: %s", virtualModel, err.Error()))
		return
	}
	deployments := plan.Deployments
	modelAttempts := fallback.PrepareDeploymentModelAttempts(deployments, caps)
	deploymentIndexes := make(map[string]int, len(deployments))
	for index, dep := range deployments {
		deploymentIndexes[dep.ID] = index
	}

	if plan.CapabilityAfter < plan.CapabilityBefore {
		logger.Infof(ctx, "[fallback] capability filter: %d -> %d deployments (vision=%v tools=%v json=%v stream=%v)",
			plan.CapabilityBefore, plan.CapabilityAfter, caps.Vision, caps.Tools, caps.JSON, caps.Stream)
	}

	if plan.HealthAfter < plan.HealthBefore {
		logger.Infof(ctx, "[fallback] health filter: %d -> %d deployments", plan.HealthBefore, plan.HealthAfter)
	}

	if plan.StickyDeploymentID != "" && plan.PreferredDeploymentID == "" {
		if len(deployments) > 0 && deployments[0].ID == plan.StickyDeploymentID {
			logger.Infof(ctx, "[fallback] sticky routing: virtual model %s pinned to deployment %s", virtualModel, plan.StickyDeploymentID)
		}
		logger.Infof(ctx, "[fallback] sticky active for %s -> %s", virtualModel, plan.StickyDeploymentID)
	} else if len(deployments) > 0 {
		logger.Infof(ctx, "[fallback] strategy-based start deployment for %s: %s", virtualModel, deployments[0].ID)
	}

	relayMode := relaymode.GetByPath(c.Request.URL.Path)
	var lastBizErr *model.ErrorWithStatusCode
	var upstreamAttemptCount int
	var prevDeployment string // track previous deployment for switch log
	var prevDurationMs int64
	deploymentCount := len(deployments)

	for i := 0; i < len(modelAttempts); i++ {
		attempt := modelAttempts[i]
		dep := attempt.Deployment
		deploymentIndex := deploymentIndexes[dep.ID]

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
		upstreamAttemptCount++

		// Log switch if this is not the first attempt
		if prevDeployment != "" && prevDeployment != dep.ID {
			fallbackSwitchLog(ctx, virtualModel, prevDeployment, dep.ID,
				getRelayErrorMessage(lastBizErr), lastBizErr.StatusCode, prevDurationMs)
			common.IncFallbackSwitch()
		}

		// Set fallback context keys in gin.Context
		c.Set(ctxkey.FallbackEnabled, true)
		c.Set(ctxkey.FallbackVirtualModel, virtualModel)
		c.Set(ctxkey.FallbackDeploymentID, dep.ID)
		c.Set(ctxkey.FallbackRealModel, dep.RealModel)
		freeProviderName, hasFreeProviderName := fallback.FreeProviderNameFromDeploymentID(dep.ID)
		if hasFreeProviderName {
			c.Set(ctxkey.FallbackFreeProviderName, freeProviderName)
		} else {
			c.Set(ctxkey.FallbackFreeProviderName, "")
		}
		c.Set(ctxkey.FallbackChannelID, dep.ChannelID)
		c.Set(ctxkey.FallbackDeploymentIndex, deploymentIndex)
		c.Set(ctxkey.FallbackAttemptCount, upstreamAttemptCount)
		// Refresh all channel-specific context for this deployment
		middleware.SetupContextForSelectedChannel(c, channel, virtualModel)

		logger.Infof(ctx, "[fallback] switched to channel id=%d name=%s model=%s",
			dep.ChannelID, channel.Name, dep.RealModel)

		// Set fallback context keys in context.Context for postConsumeQuota
		newCtx := context.WithValue(ctx, ctxkey.FallbackVirtualModel, virtualModel)
		newCtx = context.WithValue(newCtx, ctxkey.FallbackDeploymentID, dep.ID)
		newCtx = context.WithValue(newCtx, ctxkey.FallbackRealModel, dep.RealModel)
		if hasFreeProviderName {
			newCtx = context.WithValue(newCtx, ctxkey.FallbackFreeProviderName, freeProviderName)
		}
		newCtx = context.WithValue(newCtx, ctxkey.FallbackChannelID, dep.ChannelID)
		newCtx = context.WithValue(newCtx, ctxkey.FallbackDeploymentIndex, deploymentIndex)
		newCtx = context.WithValue(newCtx, ctxkey.FallbackAttemptCount, upstreamAttemptCount)
		c.Request = c.Request.WithContext(newCtx)

		// Reset request body for this attempt
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		attemptStart := time.Now()
		logger.Infof(ctx, "[fallback] attempt %d/%d virtual model %s deployment %s channel %d real model %s",
			upstreamAttemptCount, len(modelAttempts), virtualModel, dep.ID, dep.ChannelID, dep.RealModel)

		// Execute the relay helper
		bizErr := execute(c, relayMode)
		durationMs := time.Since(attemptStart).Milliseconds()
		releaseDeploymentSlot()
		// Debug: log full attempt details to help diagnose fallback behaviour
		if bizErr != nil {
			errInfo := fallback.FormatRelayErrorInfo(bizErr.StatusCode, getRelayErrorMessage(bizErr), bizErr.Error.Type, bizErr.Error.Code)
			errClass := fallback.ClassifyRelayError(errInfo)
			logger.Debugf(newCtx, "[fallback] attempt_result attempt=%d/%d deployment=%s status=%d msg=%q code=%q category=%v should_fallback=%v", upstreamAttemptCount, len(modelAttempts), dep.ID, bizErr.StatusCode, bizErr.Error.Message, bizErr.Error.Code, errClass.Category, errClass.ShouldFallback)
		} else {
			logger.Debugf(newCtx, "[fallback] attempt_result attempt=%d/%d deployment=%s status=success duration=%dms", upstreamAttemptCount, len(modelAttempts), dep.ID, durationMs)
		}

		if bizErr == nil {
			fallback.SetStickyDeployment(virtualModel, dep.ID)
			if attempt.ProviderName == "kilo" {
				fallback.RecordFreeProviderModelSuccess(dep.ID, dep.RealModel)
			}
			// Success - report to monitor and record for smart sorting
			monitor.Emit(dep.ChannelID, true)
			if !relayModeRecordsFallbackUsage(relayMode) {
				fallback.RecordFallbackDeploymentSuccess(dep.ID, dep.RealModel, fallback.UsageInfo{})
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
			dep.ID, upstreamAttemptCount, len(modelAttempts), durationMs, getRelayErrorMessage(bizErr))

		// Classify error using structured info (single-pass, replaces 4 separate string scans)
		errInfo := fallback.FormatRelayErrorInfo(bizErr.StatusCode, getRelayErrorMessage(bizErr), bizErr.Error.Type, bizErr.Error.Code)
		errClass := fallback.ClassifyRelayError(errInfo)

		// Only HTTP 429 responses are treated as confirmed rate limits for model rotation
		// and provider-level rate-limit score accounting. Other statuses with rate-limit-like
		// messages are handled as ordinary provider failures.
		isConfirmedHTTPRateLimit := errClass.Category == fallback.ErrorCategoryRateLimit &&
			bizErr.StatusCode == http.StatusTooManyRequests

		// Once any response bytes are written, replaying the request is unsafe.
		if c.Writer.Written() {
			logger.Infof(ctx, "[fallback] response already written for deployment %s, stopping attempts",
				dep.ID)
			return
		}

		if attempt.ProviderName == "kilo" && attempt.Rotatable && isConfirmedHTTPRateLimit {
			modelCooldown := fallback.MarkFreeProviderModelRateLimited(
				dep.ID, dep.RealModel, getRelayErrorMessage(bizErr), fallback.RelayCooldownInput{
					Category: errClass.Category, StatusCode: bizErr.StatusCode,
					RetryAfterSeconds: bizErr.RetryAfterSeconds, Attempt: 1,
				},
			)
			if attempt.HasNextModel() {
				logger.Infof(ctx, "[fallback] Kilo model %s cooling down for %.0fs; rotating within deployment %s",
					dep.RealModel, modelCooldown.Seconds(), dep.ID)
				continue
			}
		}

		// Record provider error state only after model-level rotation is exhausted.
		fallback.RecordDeploymentError(dep.ID, relayErr)
		fallback.RecordFailure(dep.ID, getRelayErrorMessage(bizErr), isConfirmedHTTPRateLimit)

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
			claudeutil.WriteClaudeOrOpenAIError(c, errCopy.StatusCode, errCopy.Error.Type, errCopy.Error.Message)
			return
		}
		// Report channel status to monitor for auto-disable tracking
		if monitor.ShouldDisableChannel(&bizErr.Error, bizErr.StatusCode) {
			monitor.DisableChannel(dep.ChannelID, channel.Name, getRelayErrorMessage(bizErr))
		} else {
			monitor.Emit(dep.ChannelID, false)
		}

		// Mark deployment state based on error category
		cooldownDuration, cooldownErr := fallback.ApplyRelayCooldown(dep.ID, getRelayErrorMessage(bizErr), fallback.RelayCooldownInput{
			Category:          errClass.Category,
			StatusCode:        bizErr.StatusCode,
			RetryAfterSeconds: bizErr.RetryAfterSeconds,
			Attempt:           upstreamAttemptCount,
		})
		if cooldownErr != nil {
			logger.SysError(fmt.Sprintf("[fallback] failed to apply cooldown state for %s: %v", dep.ID, cooldownErr))
		} else if errClass.Category == fallback.ErrorCategoryQuota {
			logger.Infof(ctx, "[fallback] deployment %s marked exhausted until end of day: %s",
				dep.ID, getRelayErrorMessage(bizErr))
		} else if cooldownDuration > 0 {
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

		if attempt.ProviderName == "kilo" && !isConfirmedHTTPRateLimit {
			i += attempt.RemainingModelAttempts()
		}

		// Continue to next deployment
		continue
	}

	// All deployments failed — fire alert event
	common.IncFallbackFailed()
	// Clear sticky since all deployments failed
	fallback.ClearStickyDeployment(virtualModel)
	logger.Infof(ctx, "[fallback] all %d deployments failed for virtual model %s",
		deploymentCount, virtualModel)

	// Fire a critical alert for total failure
	fallback.GlobalAlertManager.FireAlert(fallback.AlertEvent{
		DeploymentID: virtualModel,
		Level:        fallback.AlertCritical,
		Type:         fallback.AlertAllFailed,
		Message:      fmt.Sprintf("all %d deployments failed for virtual model %s", deploymentCount, virtualModel),
		CreatedAt:    time.Now(),
	})

	// Unified error response — never pass raw upstream errors to client
	claudeutil.WriteClaudeOrOpenAIError(c, http.StatusServiceUnavailable, "one_api_error", "所有上游均不可用，请稍后重试")
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
		claudeutil.WriteClaudeOrOpenAIError(c, errCopy.StatusCode, errCopy.Error.Type, errCopy.Error.Message)
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
