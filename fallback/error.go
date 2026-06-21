package fallback

import (
	"fmt"
	"net/http"
	"strings"
)

// ErrorCategory classifies the type of relay error for fallback decision-making.
type ErrorCategory int

const (
	ErrorCategoryNone        ErrorCategory = iota // no error or unclassified
	ErrorCategoryClient                           // client/request error — do NOT fallback
	ErrorCategoryQuota                            // quota/billing exhausted
	ErrorCategoryRateLimit                        // rate limited (429)
	ErrorCategoryTemporary                        // network/server issue (5xx, timeout)
	ErrorCategoryModelAccess                      // model not found / no access
)

// ErrorClassification holds the result of classifying a relay error.
type ErrorClassification struct {
	Category       ErrorCategory
	ShouldFallback bool
}

// RelayErrorInfo carries structured error information from the relay layer.
// Instead of parsing error messages as strings, callers should populate this
// from the structured bizErr (ErrorWithStatusCode) to enable accurate classification.
type RelayErrorInfo struct {
	StatusCode int    // HTTP status code (0 if unavailable)
	ErrMsg     string // Error message text
	ErrType    string // Error type field (e.g. "zhipu_error")
	ErrCode    string // Error code field (e.g. "1211")
}

// ClassifyRelayError performs a single-pass classification of a relay error.
// It uses structured fields (StatusCode, ErrCode, ErrType) first for precision,
// then falls back to message-based matching for cases where structured info is absent.
//
// This replaces the previous pattern of calling ShouldFallback + IsQuotaError +
// IsRateLimitError + IsTemporaryError separately, which scanned the same message
// four times and was prone to false positives (e.g. "429" matching inside random text).
func ClassifyRelayError(info RelayErrorInfo) ErrorClassification {
	msg := strings.ToLower(info.ErrMsg)
	code := strings.ToLower(info.ErrCode)
	etype := strings.ToLower(info.ErrType)

	// Some providers wrap upstream server failures into 400 responses with
	// internal_server_error. Treat that provider signal as a temporary upstream error.
	if code == "internal_server_error" {
		return ErrorClassification{Category: ErrorCategoryTemporary, ShouldFallback: true}
	}

	// ── Phase 1: Structured classification by HTTP status code ──
	switch info.StatusCode {
	case http.StatusBadRequest: // 400
		return ErrorClassification{Category: ErrorCategoryClient, ShouldFallback: false}
	case http.StatusUnauthorized: // 401
		return ErrorClassification{Category: ErrorCategoryModelAccess, ShouldFallback: true}
	case http.StatusPaymentRequired: // 402
		return ErrorClassification{Category: ErrorCategoryQuota, ShouldFallback: true}
	case http.StatusForbidden: // 403
		return ErrorClassification{Category: ErrorCategoryModelAccess, ShouldFallback: true}
	case http.StatusNotFound: // 404
		return ErrorClassification{Category: ErrorCategoryModelAccess, ShouldFallback: true}
	case http.StatusTooManyRequests: // 429
		return ErrorClassification{Category: ErrorCategoryRateLimit, ShouldFallback: true}
	case http.StatusBadGateway: // 502
		return ErrorClassification{Category: ErrorCategoryTemporary, ShouldFallback: true}
	case http.StatusServiceUnavailable: // 503
		return ErrorClassification{Category: ErrorCategoryTemporary, ShouldFallback: true}
	case http.StatusGatewayTimeout: // 504
		return ErrorClassification{Category: ErrorCategoryTemporary, ShouldFallback: true}
	}

	// ── Phase 2: Structured classification by error type + code ──
	// Provider-specific error codes that have precise meanings
	if etype == "zhipu_error" && code == "1211" {
		// 智谱: model rate limit
		return ErrorClassification{Category: ErrorCategoryRateLimit, ShouldFallback: true}
	}

	// ── Phase 3: Client-error patterns — do NOT fallback ──
	nonFallbackPatterns := []string{
		"invalid_request",
		"invalid parameter",
		"unsupported parameter",
		"context_length_exceeded",
		"messages is required",
		"invalid role",
	}
	for _, p := range nonFallbackPatterns {
		if strings.Contains(msg, p) {
			return ErrorClassification{Category: ErrorCategoryClient, ShouldFallback: false}
		}
	}

	// ── Phase 4: Message-based classification (fallback for missing status codes) ──

	// Quota/billing errors
	quotaPatterns := []string{
		"setlimitexceeded",
		"inference limit",
		"model service has been paused",
		"safe experience mode",
		"quota",
		"insufficient quota",
		"exceeded quota",
		"balance not enough",
		"insufficient balance",
		"billing",
		"payment required",
	}
	for _, p := range quotaPatterns {
		if strings.Contains(msg, p) {
			return ErrorClassification{Category: ErrorCategoryQuota, ShouldFallback: true}
		}
	}

	// Rate limit errors (without relying on bare "429" substring)
	rateLimitPatterns := []string{
		"rate limit",
		"too many requests",
		"rate exceeded",
	}
	for _, p := range rateLimitPatterns {
		if strings.Contains(msg, p) {
			return ErrorClassification{Category: ErrorCategoryRateLimit, ShouldFallback: true}
		}
	}

	// Model/access errors
	modelAccessPatterns := []string{
		"模型不存在",
		"model not found",
		"does not exist",
		"does not support",
		"no access",
		"not have access",
		"unauthorized",
		"invalid model",
		"unknown model",
		"not_found",
		"model_not_found",
		"model_not_available",
	}
	for _, p := range modelAccessPatterns {
		if strings.Contains(msg, p) {
			return ErrorClassification{Category: ErrorCategoryModelAccess, ShouldFallback: true}
		}
	}

	// Temporary/network errors
	temporaryPatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"eof",
		"stream error",
		"error decoding",
		"unexpected eof",
		"broken pipe",
		"connection closed",
		"connection has been closed",
		"stream interrupted",
		"stream closed",
		"write error",
		"read error",
		"reset by peer",
		"unexpected end of stream",
		"client disconnected",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
		"connection error",
	}
	for _, p := range temporaryPatterns {
		if strings.Contains(msg, p) {
			return ErrorClassification{Category: ErrorCategoryTemporary, ShouldFallback: true}
		}
	}

	return ErrorClassification{Category: ErrorCategoryNone, ShouldFallback: false}
}

// ClassifyRelayErrorWithConfig extends ClassifyRelayError with config-driven
// error code overrides. If a blocked error code is configured in the fallback
// config and matches the relay error's ErrCode, the classification is overridden
// to Client (no fallback), even if the base classifier would have triggered one.
//
// This allows operators to suppress fallback for specific error codes without
// modifying Go code, by adding them to blocked_error_codes in fallback.json.
func ClassifyRelayErrorWithConfig(info RelayErrorInfo, cfg *Config) ErrorClassification {
	cls := ClassifyRelayError(info)
	if !cls.ShouldFallback || info.ErrCode == "" || cfg == nil || len(cfg.BlockedErrorCodes) == 0 {
		return cls
	}
	for _, blocked := range cfg.BlockedErrorCodes {
		if strings.EqualFold(info.ErrCode, blocked) {
			return ErrorClassification{Category: ErrorCategoryClient, ShouldFallback: false}
		}
	}
	return cls
}

// ── Legacy compatibility wrappers ──
// These preserve the existing function signatures so call sites can migrate gradually.

// ShouldFallback determines if an error should trigger fallback to next deployment.
// Deprecated: Use ClassifyRelayError for new code. This wrapper loses structured info.
func ShouldFallback(err error) bool {
	if err == nil {
		return false
	}
	info := RelayErrorInfo{ErrMsg: err.Error()}
	return ClassifyRelayError(info).ShouldFallback
}

// IsQuotaError checks if error is related to quota/billing.
// Deprecated: Use ClassifyRelayError instead.
func IsQuotaError(err error) bool {
	if err == nil {
		return false
	}
	info := RelayErrorInfo{ErrMsg: err.Error()}
	return ClassifyRelayError(info).Category == ErrorCategoryQuota
}

// IsRateLimitError checks if error is related to rate limiting.
// Deprecated: Use ClassifyRelayError instead.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	info := RelayErrorInfo{ErrMsg: err.Error()}
	return ClassifyRelayError(info).Category == ErrorCategoryRateLimit
}

// IsTemporaryError checks if error is temporary (network issues, server problems).
// Deprecated: Use ClassifyRelayError instead.
func IsTemporaryError(err error) bool {
	if err == nil {
		return false
	}
	info := RelayErrorInfo{ErrMsg: err.Error()}
	return ClassifyRelayError(info).Category == ErrorCategoryTemporary
}

// IsConfigError checks if error is configuration-related.
// Deprecated: Use ClassifyRelayError instead.
func IsConfigError(err error) bool {
	if err == nil {
		return false
	}
	info := RelayErrorInfo{ErrMsg: err.Error()}
	return ClassifyRelayError(info).Category == ErrorCategoryClient
}

// FormatRelayErrorInfo is a convenience function to build RelayErrorInfo
// from a ErrorWithStatusCode struct without the caller needing to know field names.
func FormatRelayErrorInfo(statusCode int, errMsg string, errType string, errCode any) RelayErrorInfo {
	codeStr := ""
	if errCode != nil {
		codeStr = fmt.Sprintf("%v", errCode)
	}
	return RelayErrorInfo{
		StatusCode: statusCode,
		ErrMsg:     errMsg,
		ErrType:    errType,
		ErrCode:    codeStr,
	}
}
