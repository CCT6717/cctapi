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
	metricDelta float64
}

func TestOpenRouterAutoSmokeRejectsZeroFallbackMetricDelta(t *testing.T) {
	server := newSmokeServer(t, 0)
	defer server.Close()

	output, err := runOpenRouterAutoSmoke(t, server.URL)
	if err == nil {
		t.Fatalf("smoke script accepted zero fallback metric delta; output:\n%s", output)
	}
}

func TestOpenRouterAutoSmokeReportsSPAReachabilitySeparately(t *testing.T) {
	server := newSmokeServer(t, 2)
	defer server.Close()

	output, err := runOpenRouterAutoSmoke(t, server.URL)
	if err != nil {
		t.Fatalf("smoke script failed with positive traffic deltas: %v\noutput:\n%s", err, output)
	}

	var summary struct {
		Pass                       bool    `json:"pass"`
		FallbackRequestsDelta      float64 `json:"fallbackRequestsDelta"`
		UsageRequestDelta          int     `json:"usageRequestDelta"`
		UsageSuccessDelta          int     `json:"usageSuccessDelta"`
		PageReachable              bool    `json:"pageReachable"`
		PageContainsOpenRouterAuto bool    `json:"pageContainsOpenRouterAuto"`
	}
	if err := json.Unmarshal(trailingJSON(t, output), &summary); err != nil {
		t.Fatalf("parse trailing smoke JSON: %v\noutput:\n%s", err, output)
	}

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

func newSmokeServer(t *testing.T, metricDelta float64) *httptest.Server {
	t.Helper()

	fixture := &smokeFixture{metricDelta: metricDelta}
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
			value := 10.0
			if call > 1 {
				value += fixture.metricDelta
			}
			fmt.Fprintf(w, "# TYPE fallback_requests_total counter\nfallback_requests_total %v\n", value)
		case "/api/fallback/free-pool/usage":
			call := atomic.AddInt64(&fixture.usageCalls, 1)
			requestCount, successCount := 5, 4
			if call > 1 {
				requestCount, successCount = 7, 6
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
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<!doctype html><html><body><div id=\"root\"></div></body></html>")
		default:
			http.NotFound(w, r)
		}
	}))
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
