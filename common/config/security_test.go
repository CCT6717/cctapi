package config

import (
	"net/http"
	"os"
	"testing"
)

func TestResolveSessionCookiePolicy(t *testing.T) {
	tests := []struct {
		name          string
		mode          string
		serverAddress string
		wantSecure    bool
		wantError     bool
	}{
		{name: "explicit true", mode: "true", serverAddress: "http://localhost:3008", wantSecure: true},
		{name: "explicit false", mode: "false", serverAddress: "https://api.example.com"},
		{name: "auto https", mode: "auto", serverAddress: "https://api.example.com", wantSecure: true},
		{name: "auto local http", mode: "auto", serverAddress: "http://localhost:3008"},
		{name: "empty defaults to auto", serverAddress: "https://api.example.com", wantSecure: true},
		{name: "invalid mode", mode: "sometimes", serverAddress: "https://api.example.com", wantError: true},
		{name: "invalid address", mode: "auto", serverAddress: "://bad", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, err := ResolveSessionCookiePolicy(tt.mode, tt.serverAddress)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveSessionCookiePolicy() error = %v", err)
			}
			if policy.Path != "/" {
				t.Fatalf("Path = %q, want /", policy.Path)
			}
			if policy.MaxAge != 30*24*60*60 {
				t.Fatalf("MaxAge = %d, want 30 days", policy.MaxAge)
			}
			if !policy.HttpOnly {
				t.Fatal("HttpOnly must be enabled")
			}
			if policy.SameSite != http.SameSiteLaxMode {
				t.Fatalf("SameSite = %d, want Lax", policy.SameSite)
			}
			if policy.Secure != tt.wantSecure {
				t.Fatalf("Secure = %t, want %t", policy.Secure, tt.wantSecure)
			}
		})
	}
}

func TestEffectiveServerAddressUsesEnvironmentOverride(t *testing.T) {
	originalAddress := ServerAddress
	t.Cleanup(func() { ServerAddress = originalAddress })
	ServerAddress = "http://database-setting.example"
	t.Setenv("SERVER_ADDRESS", "https://public.example")

	if got := EffectiveServerAddress(); got != "https://public.example" {
		t.Fatalf("EffectiveServerAddress() = %q, want environment override", got)
	}

	if err := os.Unsetenv("SERVER_ADDRESS"); err != nil {
		t.Fatalf("unset SERVER_ADDRESS: %v", err)
	}
	if got := EffectiveServerAddress(); got != ServerAddress {
		t.Fatalf("EffectiveServerAddress() = %q, want configured fallback %q", got, ServerAddress)
	}
}
