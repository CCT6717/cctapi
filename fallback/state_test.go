package fallback

import (
	"testing"
	"time"
)

func TestQuotaPeriodDateRefreshesAtNoonUTC8(t *testing.T) {
	beforeNoon := time.Date(2026, 6, 3, 3, 59, 59, 0, time.UTC)
	if got := quotaPeriodDate(beforeNoon); got != "2026-06-02" {
		t.Fatalf("expected previous quota date before noon UTC+8, got %s", got)
	}

	atNoon := time.Date(2026, 6, 3, 4, 0, 0, 0, time.UTC)
	if got := quotaPeriodDate(atNoon); got != "2026-06-03" {
		t.Fatalf("expected current quota date at noon UTC+8, got %s", got)
	}
}

func TestNextQuotaRefreshTime(t *testing.T) {
	beforeNoon := time.Date(2026, 6, 3, 3, 59, 59, 0, time.UTC)
	expectedSameDayNoon := time.Date(2026, 6, 3, 4, 0, 0, 0, time.UTC)
	if got := nextQuotaRefreshTime(beforeNoon); !got.Equal(expectedSameDayNoon) {
		t.Fatalf("expected next refresh %s, got %s", expectedSameDayNoon, got)
	}

	afterNoon := time.Date(2026, 6, 3, 4, 0, 1, 0, time.UTC)
	expectedNextDayNoon := time.Date(2026, 6, 4, 4, 0, 0, 0, time.UTC)
	if got := nextQuotaRefreshTime(afterNoon); !got.Equal(expectedNextDayNoon) {
		t.Fatalf("expected next refresh %s, got %s", expectedNextDayNoon, got)
	}
}
