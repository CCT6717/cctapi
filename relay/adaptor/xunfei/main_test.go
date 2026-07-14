package xunfei

import "testing"

func TestBuildXunfeiAuthURLRejectsMalformedHostURL(t *testing.T) {
	if got := buildXunfeiAuthUrl("https://example.com/%zz", "key", "secret"); got != "" {
		t.Fatalf("buildXunfeiAuthUrl() = %q, want empty string", got)
	}
}
