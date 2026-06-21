package fallback

import (
	"net/http"
	"testing"
)

// ── Structured classification tests ──

func TestClassifyByStatusCode(t *testing.T) {
	tests := []struct {
		statusCode   int
		wantCategory ErrorCategory
		wantFallback bool
	}{
		{http.StatusBadRequest, ErrorCategoryClient, false},           // 400
		{http.StatusUnauthorized, ErrorCategoryModelAccess, true},     // 401
		{http.StatusPaymentRequired, ErrorCategoryQuota, true},        // 402
		{http.StatusForbidden, ErrorCategoryModelAccess, true},        // 403
		{http.StatusNotFound, ErrorCategoryModelAccess, true},         // 404
		{http.StatusTooManyRequests, ErrorCategoryRateLimit, true},    // 429
		{http.StatusBadGateway, ErrorCategoryTemporary, true},         // 502
		{http.StatusServiceUnavailable, ErrorCategoryTemporary, true}, // 503
		{http.StatusGatewayTimeout, ErrorCategoryTemporary, true},     // 504
	}
	for _, tt := range tests {
		info := RelayErrorInfo{StatusCode: tt.statusCode, ErrMsg: "some error"}
		cls := ClassifyRelayError(info)
		if cls.Category != tt.wantCategory {
			t.Errorf("statusCode=%d: got category=%v, want %v", tt.statusCode, cls.Category, tt.wantCategory)
		}
		if cls.ShouldFallback != tt.wantFallback {
			t.Errorf("statusCode=%d: got ShouldFallback=%v, want %v", tt.statusCode, cls.ShouldFallback, tt.wantFallback)
		}
	}
}

func TestClassifyZhipuError(t *testing.T) {
	info := RelayErrorInfo{ErrType: "zhipu_error", ErrCode: "1211", ErrMsg: "rate limited"}
	cls := ClassifyRelayError(info)
	if cls.Category != ErrorCategoryRateLimit {
		t.Errorf("zhipu_error 1211: got category=%v, want RateLimit", cls.Category)
	}
	if !cls.ShouldFallback {
		t.Error("zhipu_error 1211 should trigger fallback")
	}
}

func TestClassifyQuotaByMessage(t *testing.T) {
	info := RelayErrorInfo{ErrMsg: "insufficient quota for model"}
	cls := ClassifyRelayError(info)
	if cls.Category != ErrorCategoryQuota {
		t.Errorf("got category=%v, want Quota", cls.Category)
	}
}

func TestClassifySetLimitExceeded(t *testing.T) {
	info := RelayErrorInfo{ErrMsg: `{"error":{"code":"SetLimitExceeded","message":"Your account has reached the set inference limit and the model service has been paused. Close Safe Experience Mode to continue."}}`}
	cls := ClassifyRelayError(info)
	if !cls.ShouldFallback {
		t.Fatal("expected SetLimitExceeded error to trigger fallback")
	}
	if cls.Category != ErrorCategoryQuota {
		t.Errorf("expected Quota category, got %v", cls.Category)
	}
}

func TestClassifyClientError(t *testing.T) {
	info := RelayErrorInfo{ErrMsg: "invalid_request: bad request"}
	cls := ClassifyRelayError(info)
	if cls.ShouldFallback {
		t.Fatal("expected client error to NOT trigger fallback")
	}
	if cls.Category != ErrorCategoryClient {
		t.Errorf("expected Client category, got %v", cls.Category)
	}
}

func TestClassifyContextLengthExceeded(t *testing.T) {
	info := RelayErrorInfo{ErrMsg: "context_length_exceeded: maximum context length is 4096"}
	cls := ClassifyRelayError(info)
	if cls.ShouldFallback {
		t.Fatal("context_length_exceeded should NOT trigger fallback")
	}
}

func TestClassifyNetworkError(t *testing.T) {
	info := RelayErrorInfo{ErrMsg: "connection refused: dial tcp 10.0.0.1:443: connect: connection refused"}
	cls := ClassifyRelayError(info)
	if !cls.ShouldFallback {
		t.Fatal("connection refused should trigger fallback")
	}
	if cls.Category != ErrorCategoryTemporary {
		t.Errorf("expected Temporary category, got %v", cls.Category)
	}
}

func TestClassifyModelNotFound(t *testing.T) {
	info := RelayErrorInfo{ErrMsg: "model not found: gpt-5"}
	cls := ClassifyRelayError(info)
	if !cls.ShouldFallback {
		t.Fatal("model not found should trigger fallback")
	}
	if cls.Category != ErrorCategoryModelAccess {
		t.Errorf("expected ModelAccess category, got %v", cls.Category)
	}
}

func TestStatusCodeTakesPrecedenceOverMessage(t *testing.T) {
	// A 400 status code should classify as Client error even if message contains "timeout"
	info := RelayErrorInfo{StatusCode: 400, ErrMsg: "timeout waiting for input validation"}
	cls := ClassifyRelayError(info)
	if cls.ShouldFallback {
		t.Fatal("400 with 'timeout' in message should NOT fallback — status code takes precedence")
	}
}

func TestClassifyRateLimitByMessage(t *testing.T) {
	info := RelayErrorInfo{ErrMsg: "rate limit exceeded for this model"}
	cls := ClassifyRelayError(info)
	if !cls.ShouldFallback {
		t.Fatal("rate limit error should trigger fallback")
	}
	if cls.Category != ErrorCategoryRateLimit {
		t.Errorf("expected RateLimit category, got %v", cls.Category)
	}
}

func TestClassifyInternalServerError400FallsBack(t *testing.T) {
	info := RelayErrorInfo{StatusCode: http.StatusBadRequest, ErrCode: "internal_server_error", ErrMsg: "wrapped upstream error"}
	cls := ClassifyRelayError(info)
	if !cls.ShouldFallback {
		t.Fatal("400 + internal_server_error should trigger fallback")
	}
	if cls.Category != ErrorCategoryTemporary {
		t.Errorf("expected Temporary category, got %v", cls.Category)
	}
}

func TestClassifyRateLimit429FallsBack(t *testing.T) {
	info := RelayErrorInfo{StatusCode: http.StatusTooManyRequests, ErrCode: "rate_limit", ErrMsg: "rate limited"}
	cls := ClassifyRelayError(info)
	if !cls.ShouldFallback {
		t.Fatal("429 should trigger fallback")
	}
	if cls.Category != ErrorCategoryRateLimit {
		t.Errorf("expected RateLimit category, got %v", cls.Category)
	}
}

func TestClassifyBadGateway502FallsBack(t *testing.T) {
	info := RelayErrorInfo{StatusCode: http.StatusBadGateway, ErrMsg: "bad gateway"}
	cls := ClassifyRelayError(info)
	if !cls.ShouldFallback {
		t.Fatal("502 should trigger fallback")
	}
	if cls.Category != ErrorCategoryTemporary {
		t.Errorf("expected Temporary category, got %v", cls.Category)
	}
}

func TestClassifyServiceUnavailableAndGatewayTimeoutFallBack(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "503", statusCode: http.StatusServiceUnavailable},
		{name: "504", statusCode: http.StatusGatewayTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := RelayErrorInfo{StatusCode: tt.statusCode, ErrMsg: "temporary upstream failure"}
			cls := ClassifyRelayError(info)
			if !cls.ShouldFallback {
				t.Fatalf("%d should trigger fallback", tt.statusCode)
			}
			if cls.Category != ErrorCategoryTemporary {
				t.Errorf("expected Temporary category, got %v", cls.Category)
			}
		})
	}
}

// ── Legacy wrapper compatibility tests ──

func TestShouldFallbackMatchesSetLimitExceeded(t *testing.T) {
	err := &testError{msg: `{"error":{"code":"SetLimitExceeded","message":"Your account has reached the set inference limit and the model service has been paused. Close Safe Experience Mode to continue."}}`}
	if !ShouldFallback(err) {
		t.Fatal("expected SetLimitExceeded error to trigger fallback")
	}
}

func TestShouldFallbackRejectsClientBadRequest(t *testing.T) {
	err := &testError{msg: "invalid_request: bad request"}
	if ShouldFallback(err) {
		t.Fatal("expected client bad request to not trigger fallback")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// ── Blocked error codes (ClassifyRelayErrorWithConfig) tests ──

func TestClassifyWithConfigBlockedCodes_Default(t *testing.T) {
	// Default: no blocked codes → internal_server_error keeps ShouldFallback=true
	info := RelayErrorInfo{ErrCode: "internal_server_error", ErrMsg: "wrapped upstream failure"}
	cls := ClassifyRelayErrorWithConfig(info, nil)
	if !cls.ShouldFallback {
		t.Fatal("default (no config): internal_server_error should still trigger fallback")
	}
}

func TestClassifyWithConfigBlockedCodes_BlocksWhenConfigured(t *testing.T) {
	// With blocked_error_codes=["internal_server_error"] → ShouldFallback=false
	info := RelayErrorInfo{ErrCode: "internal_server_error", ErrMsg: "wrapped upstream failure"}
	cfg := &Config{BlockedErrorCodes: []string{"internal_server_error"}}
	cls := ClassifyRelayErrorWithConfig(info, cfg)
	if cls.ShouldFallback {
		t.Fatal("blocked error code should NOT trigger fallback when configured")
	}
	if cls.Category != ErrorCategoryClient {
		t.Errorf("expected Client category when blocked, got %v", cls.Category)
	}
}

func TestClassifyWithConfigBlockedCodes_UnrelatedCodeUnaffected(t *testing.T) {
	// A different error code (1211) should be unaffected by blocking internal_server_error
	info := RelayErrorInfo{ErrType: "zhipu_error", ErrCode: "1211", ErrMsg: "rate limited"}
	cfg := &Config{BlockedErrorCodes: []string{"internal_server_error"}}
	cls := ClassifyRelayErrorWithConfig(info, cfg)
	if !cls.ShouldFallback {
		t.Fatal("unrelated error code 1211 should still trigger fallback")
	}
}
