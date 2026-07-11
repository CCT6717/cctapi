package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

type smokeFixture struct {
	metricCalls int64
	usageCalls  int64
	options     smokeFixtureOptions
}

type smokeFixtureOptions struct {
	metricBefore       string
	metricAfter        string
	usageRequestBefore int64
	usageSuccessBefore int64
	usageRequestDelta  int64
	usageSuccessDelta  int64
	redirectPage       bool
	redirectFollowed   *int64
}

type smokeSummary struct {
	Pass                       bool    `json:"pass"`
	FallbackRequestsDelta      float64 `json:"fallbackRequestsDelta"`
	UsageRequestCount          int64   `json:"usageRequestCount"`
	UsageSuccessCount          int64   `json:"usageSuccessCount"`
	UsageRequestDelta          int64   `json:"usageRequestDelta"`
	UsageSuccessDelta          int64   `json:"usageSuccessDelta"`
	PageReachable              bool    `json:"pageReachable"`
	PageContainsOpenRouterAuto bool    `json:"pageContainsOpenRouterAuto"`
}

func defaultSmokeFixtureOptions() smokeFixtureOptions {
	return smokeFixtureOptions{
		metricBefore:       "fallback_requests_total 10\n",
		metricAfter:        "fallback_requests_total 12\n",
		usageRequestBefore: 5,
		usageSuccessBefore: 4,
		usageRequestDelta:  2,
		usageSuccessDelta:  2,
	}
}

func TestOpenRouterAutoSmokeRejectsZeroFallbackMetricDelta(t *testing.T) {
	options := defaultSmokeFixtureOptions()
	options.metricAfter = options.metricBefore
	server := newSmokeServer(t, options)
	defer server.Close()

	expectSmokeFailure(t, server.URL, "zero fallback metric delta")
}

func TestOpenRouterAutoSmokeReportsSPAReachabilitySeparately(t *testing.T) {
	server := newSmokeServer(t, defaultSmokeFixtureOptions())
	defer server.Close()

	output, err := runOpenRouterAutoSmoke(t, server.URL)
	if err != nil {
		t.Fatalf("smoke script failed with positive traffic deltas: %v\noutput:\n%s", err, output)
	}

	summary := parseSmokeSummary(t, output)

	if !summary.Pass {
		t.Fatal("summary pass=false, want true")
	}
	if summary.FallbackRequestsDelta != 2 {
		t.Fatalf("fallbackRequestsDelta=%v, want 2", summary.FallbackRequestsDelta)
	}
	if summary.UsageRequestDelta != 2 {
		t.Fatalf("usageRequestDelta=%d, want 2", summary.UsageRequestDelta)
	}
	if summary.UsageSuccessDelta != 2 {
		t.Fatalf("usageSuccessDelta=%d, want 2", summary.UsageSuccessDelta)
	}
	if !summary.PageReachable {
		t.Fatal("pageReachable=false, want true for a 200 SPA shell")
	}
	if summary.PageContainsOpenRouterAuto {
		t.Fatal("pageContainsOpenRouterAuto=true, want false for a marker-free SPA shell")
	}
}

func TestOpenRouterAutoSmokeSupportsInt64UsageCounters(t *testing.T) {
	options := defaultSmokeFixtureOptions()
	options.usageRequestBefore = int64(1<<53) + 101
	options.usageSuccessBefore = int64(1<<53) + 51
	server := newSmokeServer(t, options)
	defer server.Close()

	output, err := runOpenRouterAutoSmoke(t, server.URL)
	if err != nil {
		t.Fatalf("smoke script rejected valid int64 usage counters: %v\noutput:\n%s", err, output)
	}

	summary := parseSmokeSummary(t, output)
	if summary.UsageRequestCount != options.usageRequestBefore+options.usageRequestDelta {
		t.Fatalf("usageRequestCount=%d, want %d", summary.UsageRequestCount, options.usageRequestBefore+options.usageRequestDelta)
	}
	if summary.UsageSuccessCount != options.usageSuccessBefore+options.usageSuccessDelta {
		t.Fatalf("usageSuccessCount=%d, want %d", summary.UsageSuccessCount, options.usageSuccessBefore+options.usageSuccessDelta)
	}
	if summary.UsageRequestDelta != options.usageRequestDelta {
		t.Fatalf("usageRequestDelta=%d, want %d", summary.UsageRequestDelta, options.usageRequestDelta)
	}
	if summary.UsageSuccessDelta != options.usageSuccessDelta {
		t.Fatalf("usageSuccessDelta=%d, want %d", summary.UsageSuccessDelta, options.usageSuccessDelta)
	}
}

func TestOpenRouterAutoSmokeRejectsZeroUsageRequestDelta(t *testing.T) {
	options := defaultSmokeFixtureOptions()
	options.usageRequestDelta = 0
	server := newSmokeServer(t, options)
	defer server.Close()

	expectSmokeFailure(t, server.URL, "zero usage request delta")
}

func TestOpenRouterAutoSmokeRejectsZeroUsageSuccessDelta(t *testing.T) {
	options := defaultSmokeFixtureOptions()
	options.usageSuccessDelta = 0
	server := newSmokeServer(t, options)
	defer server.Close()

	expectSmokeFailure(t, server.URL, "zero usage success delta")
}

func TestOpenRouterAutoSmokeRejectsInvalidFallbackMetricSamples(t *testing.T) {
	tests := []struct {
		name         string
		metricBefore string
		metricAfter  string
	}{
		{name: "NaN", metricAfter: "fallback_requests_total NaN\n"},
		{name: "Infinity", metricAfter: "fallback_requests_total Infinity\n"},
		{
			name:         "infinite aggregate",
			metricBefore: "fallback_requests_total 1.7e308\n",
			metricAfter:  "fallback_requests_total 1.7e308\nfallback_requests_total 1.7e308\n",
		},
		{name: "malformed", metricAfter: "fallback_requests_total 12\nfallback_requests_total malformed\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := defaultSmokeFixtureOptions()
			if test.metricBefore != "" {
				options.metricBefore = test.metricBefore
			}
			options.metricAfter = test.metricAfter
			server := newSmokeServer(t, options)
			defer server.Close()

			expectSmokeFailure(t, server.URL, test.name+" fallback metric sample")
		})
	}
}

func TestOpenRouterAutoSmokeRejectsRedirectedSPAReachability(t *testing.T) {
	var redirectFollowed int64
	options := defaultSmokeFixtureOptions()
	options.redirectPage = true
	options.redirectFollowed = &redirectFollowed
	server := newSmokeServer(t, options)
	defer server.Close()

	expectSmokeFailure(t, server.URL, "redirected free-pool page")
	if got := atomic.LoadInt64(&redirectFollowed); got != 0 {
		t.Fatalf("redirect target was fetched %d times, want 0", got)
	}
}

func newSmokeServer(t *testing.T, options smokeFixtureOptions) *httptest.Server {
	t.Helper()

	fixture := &smokeFixture{options: options}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/fallback/gateway/config":
			writeJSON(t, w, map[string]any{
				"success": true,
				"data": map[string]any{
					"free_provider_catalog": []map[string]any{{
						"name":         "openrouter",
						"enabled":      true,
						"keyless":      false,
						"key_count":    1,
						"requires_key": true,
					}},
				},
			})
		case "/api/fallback/config/reload", "/api/fallback/free-pool/sync":
			writeJSON(t, w, map[string]any{"success": true, "data": map[string]any{}})
		case "/api/fallback/deployments/runtime-status":
			writeJSON(t, w, map[string]any{
				"success": true,
				"data": []map[string]any{{
					"deployment_id": "free:openrouter-test",
					"enabled":       true,
					"health":        "healthy",
				}},
			})
		case "/metrics":
			w.Header().Set("Content-Type", "text/plain")
			call := atomic.AddInt64(&fixture.metricCalls, 1)
			samples := fixture.options.metricBefore
			if call > 1 {
				samples = fixture.options.metricAfter
			}
			fmt.Fprintf(w, "# TYPE fallback_requests_total counter\n%s", samples)
		case "/api/fallback/free-pool/usage":
			call := atomic.AddInt64(&fixture.usageCalls, 1)
			requestCount := fixture.options.usageRequestBefore
			successCount := fixture.options.usageSuccessBefore
			if call > 1 {
				requestCount += fixture.options.usageRequestDelta
				successCount += fixture.options.usageSuccessDelta
			}
			writeJSON(t, w, map[string]any{
				"success": true,
				"data": []map[string]any{{
					"provider":      "openrouter",
					"request_count": requestCount,
					"success_count": successCount,
				}},
			})
		case "/v1/chat/completions":
			var request struct {
				Stream bool `json:"stream"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if request.Stream {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")
				return
			}
			writeJSON(t, w, map[string]any{
				"model":   "openrouter/auto",
				"choices": []map[string]any{{"message": map[string]any{"content": "ok"}}},
			})
		case "/fallback/free-pool":
			if fixture.options.redirectPage {
				http.Redirect(w, r, "/fallback/free-pool-shell", http.StatusFound)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<!doctype html><html><body><div id=\"root\"></div></body></html>")
		case "/fallback/free-pool-shell":
			if fixture.options.redirectFollowed != nil {
				atomic.AddInt64(fixture.options.redirectFollowed, 1)
			}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<!doctype html><html><body><div id=\"root\"></div></body></html>")
		default:
			http.NotFound(w, r)
		}
	}))
}

func expectSmokeFailure(t *testing.T, baseURL, reason string) {
	t.Helper()
	output, err := runOpenRouterAutoSmoke(t, baseURL)
	if err == nil {
		t.Fatalf("smoke script accepted %s; output:\n%s", reason, output)
	}
}

func parseSmokeSummary(t *testing.T, output []byte) smokeSummary {
	t.Helper()
	var summary smokeSummary
	if err := json.Unmarshal(trailingJSON(t, output), &summary); err != nil {
		t.Fatalf("parse trailing smoke JSON: %v\noutput:\n%s", err, output)
	}
	return summary
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode fixture response: %v", err)
	}
}

func runOpenRouterAutoSmoke(t *testing.T, baseURL string) ([]byte, error) {
	t.Helper()

	powerShell := findPowerShell(t)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate smoke test source file")
	}
	script := filepath.Join(filepath.Dir(filename), "fallback-openrouter-auto-smoke.ps1")
	cmd := exec.Command(
		powerShell,
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-File", script,
		"-BaseUrl", baseURL,
		"-ApiToken", "fake-api-token",
		"-AdminToken", "fake-admin-token",
		"-TimeoutSec", "5",
		"-OutputJson",
	)
	cmd.Env = withoutEnvironmentVariables(os.Environ(), "CCT_API_TOKEN", "CCT_ADMIN_TOKEN")
	return cmd.CombinedOutput()
}

func findPowerShell(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"powershell.exe", "pwsh"} {
		path, err := exec.LookPath(name)
		if err == nil {
			return path
		}
	}
	t.Skip("neither powershell.exe nor pwsh is available")
	return ""
}

func withoutEnvironmentVariables(environment []string, names ...string) []string {
	blocked := make(map[string]struct{}, len(names))
	for _, name := range names {
		blocked[strings.ToUpper(name)] = struct{}{}
	}

	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if _, found := blocked[strings.ToUpper(name)]; !found {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func trailingJSON(t *testing.T, output []byte) []byte {
	t.Helper()

	normalized := bytes.ReplaceAll(output, []byte("\r\n"), []byte("\n"))
	lines := bytes.Split(normalized, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(string(lines[i])) == "{" {
			return bytes.Join(lines[i:], []byte("\n"))
		}
	}
	t.Fatalf("trailing JSON object not found in output:\n%s", output)
	return nil
}
