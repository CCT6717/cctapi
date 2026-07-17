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

	// =========================================================================
	// Phase 2: 记账契约总览
	// =========================================================================
	// 本循环遵循三级状态边界和五条核心记账契约：
	// 1. 中间模型 429 只更新模型级状态（MarkFreeProviderModelRateLimited），不触发部署级处罚。
	// 2. 所有模型 429 耗尽后，供应商失败和限流分只增加一次（RecordDeploymentError + RecordFailure）。
	// 3. 非 429 错误（500、认证失败等）直接跳过剩余 Kilo 模型（i += RemainingModelAttempts）。
	// 4. 只有 deployment 变化才记录供应商切换事件（fallbackSwitchLog），模型级轮换不触发。
	// 5. 响应已写出后（c.Writer.Written()），立即终止，不再轮换或回退。
	// 详见 fallback/state_model.go 的完整文档。
	// =========================================================================
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
			recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeSkippedUnavailable,
				http.StatusServiceUnavailable, fallback.ErrorCategoryTemporary, 0, false, i+1, 0)
			continue
		}
		if attempt.SkipReason != "" {
			logger.Infof(ctx, "[fallback] deployment %s skipped before upstream attempt: %s", dep.ID, attempt.SkipReason)
			lastBizErr = &model.ErrorWithStatusCode{
				StatusCode: http.StatusServiceUnavailable,
				Error: model.Error{
					Message: attempt.SkipReason,
					Type:    "fallback_model_state",
					Code:    "models_temporarily_unavailable",
				},
			}
			prevDeployment = dep.ID
			prevDurationMs = 0
			recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeSkippedModelState,
				http.StatusServiceUnavailable, fallback.ErrorCategoryTemporary, 0, false, i+1, 0)
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
			recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeSkippedChannel,
				http.StatusServiceUnavailable, fallback.ErrorCategoryTemporary, 0, false, i+1, 0)
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
			recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeSkippedChannel,
				http.StatusForbidden, fallback.ErrorCategoryModelAccess, 0, false, i+1, 0)
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
			recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeSkippedQuota,
				http.StatusTooManyRequests, fallback.ErrorCategoryRateLimit, 0, false, i+1, 0)
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
			recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeSkippedConcurrency,
				http.StatusTooManyRequests, fallback.ErrorCategoryRateLimit, 0, false, i+1, 0)
			continue
		}
		if dep.MaxConcurrentRequests > 0 {
			logger.Infof(ctx, "[fallback] deployment %s concurrency slot acquired: %d/%d",
				dep.ID, inFlight, dep.MaxConcurrentRequests)
		}
		upstreamAttemptCount++

		// 契约 4：只有 deployment 发生变化才记录供应商切换事件（fallbackSwitchLog），模型级轮换不触发。
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

		// For non-streaming tools requests, buffer the response so we can validate
		// tool-call presence and arguments before committing them to the client.
		needBufferResponse := !caps.Stream && caps.Tools && attempt.ProviderName == "kilo"
		toolChoice := parseToolChoice(bodyBytes)
		var bufWriter *bufferedResponseWriter
		var originalWriter gin.ResponseWriter
		if needBufferResponse {
			originalWriter = c.Writer
			bufWriter = newBufferedResponseWriter(originalWriter)
			c.Writer = bufWriter
			c.Set(ctxkey.FallbackDeferPostConsume, true)
			c.Set(ctxkey.FallbackDeferredPostConsume, nil)
		}

		// Execute the relay helper
		bizErr := execute(c, relayMode)
		if needBufferResponse {
			c.Set(ctxkey.FallbackDeferPostConsume, false)
		}
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
			// Validate non-streaming tool calls before writing to client.
			if needBufferResponse && bufWriter != nil {
				c.Writer = originalWriter
				if !validateToolCallsForChoice(bufWriter.buf.Bytes(), toolChoice) {
					settleDeferredPostConsume(c, false)
					if attempt.ProviderName == "kilo" {
						fallback.MarkFreeProviderModelCapabilityFalsePositive(dep.ID, dep.RealModel, "tools")
						logger.Infof(ctx, "[fallback] model %s tools capability false-positive detected, rotating", dep.RealModel)
					}
					lastBizErr = &model.ErrorWithStatusCode{
						StatusCode: http.StatusBadGateway,
						Error: model.Error{
							Message: "invalid tool arguments",
							Type:    "invalid_upstream_response",
							Code:    "model_capability_false_positive",
						},
					}
					prevDeployment = dep.ID
					prevDurationMs = durationMs
					recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeModelCapabilityFalsePositive,
						http.StatusBadGateway, fallback.ErrorCategoryTemporary, durationMs, false, i+1, upstreamAttemptCount)
					continue
				}
				settleDeferredPostConsume(c, true)
				bufWriter.flushTo(originalWriter)
			}

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
			recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeSuccess,
				0, fallback.ErrorCategoryNone, durationMs, c.Writer.Written(), i+1, upstreamAttemptCount)
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

		// 契约 5：响应已写出后停止。一旦 c.Writer.Written() 返回 true，立即 return，不再尝试轮换或回退。
		// Once any response bytes are written, replaying the request is unsafe.
		if needBufferResponse && bufWriter != nil {
			// The buffered response was never committed to the client. Discard it
			// and let the existing fallback decision handle the upstream error.
			c.Writer = originalWriter
		}
		if c.Writer.Written() {
			logger.Infof(ctx, "[fallback] response already written for deployment %s, stopping attempts",
				dep.ID)
			recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeNonFallbackable,
				bizErr.StatusCode, errClass.Category, durationMs, true, i+1, upstreamAttemptCount)
			return
		}

		attemptDecision := fallback.DecideDeploymentModelAttempt(
			attempt,
			bizErr.StatusCode,
			errClass.Category == fallback.ErrorCategoryRateLimit,
		)
		isConfirmedHTTPRateLimit := attemptDecision.ConfirmedHTTPRateLimit

		// 契约 1：中间模型 429 只更新模型级状态（MarkFreeProviderModelRateLimited），不处罚部署级。
		if attemptDecision.RecordModelRateLimit {
			modelCooldown := fallback.MarkFreeProviderModelRateLimited(
				dep.ID, dep.RealModel, getRelayErrorMessage(bizErr), fallback.RelayCooldownInput{
					Category: errClass.Category, StatusCode: bizErr.StatusCode,
					RetryAfterSeconds: bizErr.RetryAfterSeconds, Attempt: 1,
				},
			)
			if attemptDecision.Action == fallback.DeploymentModelActionRotate {
				logger.Infof(ctx, "[fallback] Kilo model %s cooling down for %.0fs; rotating within deployment %s",
					dep.RealModel, modelCooldown.Seconds(), dep.ID)
				recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeModelRateLimited,
					bizErr.StatusCode, fallback.ErrorCategoryRateLimit, durationMs, false, i+1, upstreamAttemptCount)
				// Model-level 429: record attempt failure and category counter,
				// but do NOT call RecordDeploymentError or RecordFailure (supplier-level).
				fallback.RecordAttemptFailure(attempt.Deployment.ID, durationMs)
				fallback.RecordErrorCategoryCounter(fallback.ErrorCategoryRateLimit)
				continue
			}
		}

		// 契约 2：所有模型 429 耗尽后，供应商失败和限流分只增加一次（RecordDeploymentError + RecordFailure）。
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
			recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeNonFallbackable,
				bizErr.StatusCode, errClass.Category, durationMs, false, i+1, upstreamAttemptCount)
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
		if isConfirmedHTTPRateLimit && cooldownErr == nil && cooldownDuration > 0 {
			fallback.RecordProviderRateLimitEpisode(dep.ID, cooldownDuration)
		}
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

		recordFallbackAttempt(ctx, requestId, virtualModel, attempt, fallback.AttemptOutcomeFailure,
			bizErr.StatusCode, errClass.Category, durationMs, false, i+1, upstreamAttemptCount)

		// 契约 3：非 429 错误（500、认证失败等）直接跳过剩余 Kilo 模型（i += attempt.RemainingModelAttempts()）。
		if attemptDecision.Action == fallback.DeploymentModelActionSkipRemaining {
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

// ---------------------------------------------------------------------------
// Phase 4: Structured attempt event recording
// ---------------------------------------------------------------------------

// recordFallbackAttempt builds and emits a structured AttemptEvent.
// Success events update in-memory metrics only (to avoid SQLite growth).
// Failure / skip / switch events are persisted for post-mortem analysis.
func recordFallbackAttempt(
	ctx context.Context,
	requestID, virtualModel string,
	attempt fallback.DeploymentModelAttempt,
	outcome fallback.AttemptOutcome,
	statusCode int,
	errCat fallback.ErrorCategory,
	durationMs int64,
	streamWritten bool,
	planIndex, upstreamAttemptIndex int,
) {
	event := fallback.AttemptEvent{
		RequestID:            requestID,
		VirtualModel:         virtualModel,
		Provider:             attempt.ProviderName,
		DeploymentID:         attempt.Deployment.ID,
		RealModel:            attempt.Deployment.RealModel,
		Outcome:              outcome,
		StatusCode:           statusCode,
		ErrorCategory:        errCat.String(),
		DurationMs:           durationMs,
		StreamWritten:        streamWritten,
		PlanIndex:            planIndex,
		UpstreamAttemptIndex: upstreamAttemptIndex,
	}
	switch outcome {
	case fallback.AttemptOutcomeSuccess:
		fallback.RecordAttemptSuccess(attempt.Deployment.ID, durationMs)
	default:
		if outcome == fallback.AttemptOutcomeFailure ||
			outcome == fallback.AttemptOutcomeModelCapabilityFalsePositive {
			fallback.RecordAttemptFailure(attempt.Deployment.ID, durationMs)
			fallback.RecordErrorCategoryCounter(errCat)
		} else if outcome == fallback.AttemptOutcomeSkippedUnavailable ||
			outcome == fallback.AttemptOutcomeSkippedQuota ||
			outcome == fallback.AttemptOutcomeSkippedConcurrency ||
			outcome == fallback.AttemptOutcomeSkippedChannel ||
			outcome == fallback.AttemptOutcomeSkippedModelState {
			fallback.RecordAttemptSkip(attempt.Deployment.ID)
		}
	}
	fallback.RecordAttemptEventIfWorthy(event)
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

type bufferedResponseWriter struct {
	gin.ResponseWriter
	header     http.Header
	buf        bytes.Buffer
	statusCode int
	size       int
	written    bool
}

func newBufferedResponseWriter(writer gin.ResponseWriter) *bufferedResponseWriter {
	return &bufferedResponseWriter{
		ResponseWriter: writer,
		header:         writer.Header().Clone(),
		statusCode:     http.StatusOK,
		size:           -1,
	}
}

func (writer *bufferedResponseWriter) Header() http.Header {
	return writer.header
}

func (writer *bufferedResponseWriter) WriteHeader(statusCode int) {
	if writer.written {
		return
	}
	writer.statusCode = statusCode
	writer.written = true
	writer.size = 0
}

func (writer *bufferedResponseWriter) WriteHeaderNow() {
	if !writer.written {
		writer.WriteHeader(writer.statusCode)
	}
}

func (writer *bufferedResponseWriter) Write(data []byte) (int, error) {
	writer.WriteHeaderNow()
	written, err := writer.buf.Write(data)
	writer.size += written
	return written, err
}

func (writer *bufferedResponseWriter) WriteString(value string) (int, error) {
	return writer.Write([]byte(value))
}

func (writer *bufferedResponseWriter) Status() int { return writer.statusCode }
func (writer *bufferedResponseWriter) Size() int   { return writer.size }
func (writer *bufferedResponseWriter) Written() bool {
	return writer.written
}

// Flush intentionally does not commit a buffered non-streaming response.
func (writer *bufferedResponseWriter) Flush() {}

func (writer *bufferedResponseWriter) flushTo(destination gin.ResponseWriter) {
	if writer == nil || destination == nil || !writer.written {
		return
	}
	for key := range destination.Header() {
		delete(destination.Header(), key)
	}
	for key, values := range writer.header {
		destination.Header()[key] = append([]string(nil), values...)
	}
	destination.WriteHeader(writer.statusCode)
	if writer.buf.Len() > 0 {
		_, _ = destination.Write(writer.buf.Bytes())
	}
}

func settleDeferredPostConsume(c *gin.Context, accepted bool) {
	value, exists := c.Get(ctxkey.FallbackDeferredPostConsume)
	c.Set(ctxkey.FallbackDeferredPostConsume, nil)
	if !exists {
		return
	}
	if settle, ok := value.(func(bool)); ok {
		settle(accepted)
	}
}

type toolChoiceContract struct {
	required     bool
	prohibited   bool
	selectedName string
}

func parseToolChoice(body []byte) toolChoiceContract {
	var request struct {
		ToolChoice json.RawMessage `json:"tool_choice"`
	}
	if json.Unmarshal(body, &request) != nil || len(request.ToolChoice) == 0 {
		return toolChoiceContract{}
	}

	var choice string
	if json.Unmarshal(request.ToolChoice, &choice) == nil {
		switch choice {
		case "required":
			return toolChoiceContract{required: true}
		case "none":
			return toolChoiceContract{prohibited: true}
		default:
			return toolChoiceContract{}
		}
	}

	var selected struct {
		Type     string `json:"type"`
		Function *struct {
			Name string `json:"name"`
		} `json:"function"`
		Name string `json:"name"`
	}
	if json.Unmarshal(request.ToolChoice, &selected) != nil {
		return toolChoiceContract{}
	}
	switch selected.Type {
	case "function":
		name := selected.Name
		if selected.Function != nil && selected.Function.Name != "" {
			name = selected.Function.Name
		}
		if name != "" {
			return toolChoiceContract{required: true, selectedName: name}
		}
	case "tool":
		if selected.Name != "" {
			return toolChoiceContract{required: true, selectedName: selected.Name}
		}
	case "any":
		return toolChoiceContract{required: true}
	case "none":
		return toolChoiceContract{prohibited: true}
	}
	return toolChoiceContract{}
}

func requiresToolCall(body []byte) bool {
	return parseToolChoice(body).required
}

// validateToolCalls accepts the response schemas emitted by Chat Completions,
// Responses, and Anthropic Messages. Unless the request explicitly requires a
// tool call, responses without one are valid. Every emitted tool call must
// contain a non-empty JSON object as arguments.
func validateToolCalls(body []byte, requireToolCall bool) bool {
	return validateToolCallsForChoice(body, toolChoiceContract{required: requireToolCall})
}

func validateToolCallsForChoice(body []byte, toolChoice toolChoiceContract) bool {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	if choices, ok := payload["choices"]; ok {
		return validateChatCompletionToolCalls(choices, toolChoice)
	}
	if output, ok := payload["output"]; ok {
		return validateResponsesToolCalls(output, toolChoice)
	}
	if content, ok := payload["content"]; ok {
		return validateAnthropicToolCalls(content, toolChoice)
	}
	return false
}

func validateChatCompletionToolCalls(raw json.RawMessage, toolChoice toolChoiceContract) bool {
	var choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			ToolCalls []struct {
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	}
	if err := json.Unmarshal(raw, &choices); err != nil || len(choices) == 0 {
		return false
	}
	for _, choice := range choices {
		toolCallCount := len(choice.Message.ToolCalls)
		if toolChoice.prohibited && toolCallCount > 0 {
			return false
		}
		if (toolChoice.required || choice.FinishReason == "tool_calls") && toolCallCount == 0 {
			return false
		}
		for _, toolCall := range choice.Message.ToolCalls {
			if toolChoice.selectedName != "" && toolCall.Function.Name != toolChoice.selectedName {
				return false
			}
			if !validToolArguments(toolCall.Function.Arguments) {
				return false
			}
		}
	}
	return true
}

func validateResponsesToolCalls(raw json.RawMessage, toolChoice toolChoiceContract) bool {
	var output []struct {
		Type      string `json:"type"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &output); err != nil || len(output) == 0 {
		return false
	}
	foundToolCall := false
	for _, item := range output {
		if item.Type == "function_call" {
			if toolChoice.prohibited {
				return false
			}
			foundToolCall = true
			if toolChoice.selectedName != "" && item.Name != toolChoice.selectedName {
				return false
			}
			if !validToolArguments(item.Arguments) {
				return false
			}
		}
	}
	return !toolChoice.required || foundToolCall
}

func validateAnthropicToolCalls(raw json.RawMessage, toolChoice toolChoiceContract) bool {
	var content []struct {
		Type  string          `json:"type"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(raw, &content); err != nil || len(content) == 0 {
		return false
	}
	foundToolCall := false
	for _, block := range content {
		if block.Type != "tool_use" {
			continue
		}
		if toolChoice.prohibited {
			return false
		}
		foundToolCall = true
		if toolChoice.selectedName != "" && block.Name != toolChoice.selectedName {
			return false
		}
		var arguments map[string]any
		if len(block.Input) == 0 || json.Unmarshal(block.Input, &arguments) != nil || arguments == nil {
			return false
		}
	}
	return !toolChoice.required || foundToolCall
}

func validToolArguments(value string) bool {
	if value == "" {
		return false
	}
	var arguments map[string]any
	return json.Unmarshal([]byte(value), &arguments) == nil && arguments != nil
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
