package config

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const sessionCookieMaxAge = 30 * 24 * 60 * 60

type SessionCookiePolicy struct {
	Path     string
	MaxAge   int
	HttpOnly bool
	Secure   bool
	SameSite http.SameSite
}

func ResolveSessionCookiePolicy(mode string, serverAddress string) (SessionCookiePolicy, error) {
	policy := SessionCookiePolicy{
		Path:     "/",
		MaxAge:   sessionCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "auto":
		address, err := url.Parse(serverAddress)
		if err != nil || address.Host == "" || (address.Scheme != "http" && address.Scheme != "https") {
			return SessionCookiePolicy{}, fmt.Errorf("invalid SERVER_ADDRESS %q", serverAddress)
		}
		policy.Secure = address.Scheme == "https"
	case "true":
		policy.Secure = true
	case "false":
		policy.Secure = false
	default:
		return SessionCookiePolicy{}, fmt.Errorf("invalid SESSION_COOKIE_SECURE value %q", mode)
	}

	return policy, nil
}
