package zhipu

import (
	"fmt"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestGetTokenGeneratesV5ParseableHMACToken(t *testing.T) {
	const apiKey = "zhipu-test-id.test-signing-secret"
	zhipuTokens.Delete(apiKey)

	tokenString := GetToken(apiKey)
	if tokenString == "" {
		t.Fatal("GetToken returned an empty token")
	}

	parsed, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return []byte("test-signing-secret"), nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if !parsed.Valid {
		t.Fatal("parsed token is not valid")
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims type = %T, want jwt.MapClaims", parsed.Claims)
	}
	if got := claims["api_key"]; got != "zhipu-test-id" {
		t.Errorf("api_key claim = %v, want zhipu-test-id", got)
	}
	if got := parsed.Header["sign_type"]; got != "SIGN" {
		t.Errorf("sign_type header = %v, want SIGN", got)
	}
	if _, ok := claims["exp"]; !ok {
		t.Error("exp claim is missing")
	}
	if _, ok := claims["timestamp"]; !ok {
		t.Error("timestamp claim is missing")
	}
}
