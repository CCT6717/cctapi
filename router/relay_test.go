package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetRelayRouterRegistersResponsesRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	for _, route := range engine.Routes() {
		if route.Method == "POST" && route.Path == "/v1/responses" {
			return
		}
	}

	t.Fatalf("POST /v1/responses route was not registered")
}
