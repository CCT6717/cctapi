package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSchedulePostConsumeDefersAndSettlesOnce(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("fallback_defer_post_consume", true)
	committed := 0
	rolledBack := 0

	schedulePostConsume(c, func() { committed++ }, func() { rolledBack++ })
	if committed != 0 || rolledBack != 0 {
		t.Fatalf("settlement ran before validation: committed=%d rolled_back=%d", committed, rolledBack)
	}
	value, exists := c.Get("fallback_deferred_post_consume")
	settle, ok := value.(func(bool))
	if !exists || !ok {
		t.Fatal("deferred settlement callback was not stored")
	}
	settle(false)
	settle(true)
	if committed != 0 || rolledBack != 1 {
		t.Fatalf("settlement was not one-shot rollback: committed=%d rolled_back=%d", committed, rolledBack)
	}
}

func TestSchedulePostConsumeCommitsImmediatelyByDefault(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	committed := 0
	rolledBack := 0

	schedulePostConsume(c, func() { committed++ }, func() { rolledBack++ })
	if committed != 1 || rolledBack != 0 {
		t.Fatalf("default settlement committed=%d rolled_back=%d, want 1/0", committed, rolledBack)
	}
	if _, exists := c.Get("fallback_deferred_post_consume"); exists {
		t.Fatal("ordinary request stored a deferred settlement")
	}
}
