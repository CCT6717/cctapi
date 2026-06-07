package fallback

import (
	"strings"
)

// ShouldFallback determines if an error should trigger fallback to next deployment
func ShouldFallback(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	// Patterns that should NOT trigger fallback (these are client/request errors)
	nonFallbackPatterns := []string{
		"invalid_request",
		"invalid parameter",
		"unsupported parameter",
		"context_length_exceeded",
		"messages is required",
		"invalid role",
		"bad request",
	}

	for _, p := range nonFallbackPatterns {
		if strings.Contains(msg, p) {
			return false
		}
	}

	// Patterns that SHOULD trigger fallback
	fallbackPatterns := []string{
		// Model/access errors — the channel can't serve this model, try next
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
		"429",
		"rate limit",
		"too many requests",
		"quota",
		"insufficient quota",
		"exceeded quota",
		"balance not enough",
		"insufficient balance",
		"billing",
		"payment required",
		"402",
		"502",
		"503",
		"504",
		"timeout",
		"connection refused",
		"connection reset",
		"eof",
		"stream error",
		"error decoding",
		"unexpected eof",
		"broken pipe",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
		"connection error",
	}

	for _, p := range fallbackPatterns {
		if strings.Contains(msg, p) {
			return true
		}
	}

	return false
}

// IsQuotaError checks if error is related to quota/billing
func IsQuotaError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	quotaPatterns := []string{
		"quota",
		"insufficient quota",
		"exceeded quota",
		"balance not enough",
		"insufficient balance",
		"billing",
		"payment required",
		"402",
	}

	for _, p := range quotaPatterns {
		if strings.Contains(msg, p) {
			return true
		}
	}

	return false
}

// IsRateLimitError checks if error is related to rate limiting
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	rateLimitPatterns := []string{
		"429",
		"rate limit",
		"too many requests",
		"rate exceeded",
	}

	for _, p := range rateLimitPatterns {
		if strings.Contains(msg, p) {
			return true
		}
	}

	return false
}

// IsTemporaryError checks if error is temporary (network issues, server problems)
func IsTemporaryError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	temporaryPatterns := []string{
		"timeout",
		"502",
		"503",
		"504",
		"connection refused",
		"connection reset",
		"connection error",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
		"eof",
	}

	for _, p := range temporaryPatterns {
		if strings.Contains(msg, p) {
			return true
		}
	}

	return false
}

// IsConfigError checks if error is configuration-related
func IsConfigError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	configPatterns := []string{
		"invalid request",
		"bad request",
		"invalid parameter",
		"unsupported parameter",
		"messages is required",
		"invalid role",
		"bad request",
	}

	for _, p := range configPatterns {
		if strings.Contains(msg, p) {
			return true
		}
	}

	return false
}
