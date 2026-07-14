package fallback

import (
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

func TestAttemptMetricsSuccessAggregation(t *testing.T) {
	ResetAttemptMetrics("")
	defer ResetAttemptMetrics("")

	deploymentID := "free:test-metrics-success-001"
	RecordAttemptSuccess(deploymentID, 100)
	RecordAttemptSuccess(deploymentID, 200)
	RecordAttemptSuccess(deploymentID, 50)

	metrics := SnapshotAttemptMetrics(deploymentID)
	if metrics.SuccessCount != 3 {
		t.Fatalf("expected success count 3, got %d", metrics.SuccessCount)
	}
	if metrics.TotalDurationMs != 350 {
		t.Fatalf("expected total duration 350, got %d", metrics.TotalDurationMs)
	}
	if metrics.TotalSuccessMs != 350 {
		t.Fatalf("expected total success ms 350, got %d", metrics.TotalSuccessMs)
	}
	if metrics.MinDurationMs != 50 {
		t.Fatalf("expected min duration 50, got %d", metrics.MinDurationMs)
	}
	if metrics.MaxDurationMs != 200 {
		t.Fatalf("expected max duration 200, got %d", metrics.MaxDurationMs)
	}
	if metrics.FailureCount != 0 {
		t.Fatalf("expected failure count 0, got %d", metrics.FailureCount)
	}
	if metrics.SkipCount != 0 {
		t.Fatalf("expected skip count 0, got %d", metrics.SkipCount)
	}
}

func TestAttemptMetricsFailureAggregation(t *testing.T) {
	ResetAttemptMetrics("")
	defer ResetAttemptMetrics("")

	deploymentID := "free:test-metrics-failure-001"
	RecordAttemptFailure(deploymentID, 150)
	RecordAttemptFailure(deploymentID, 250)

	metrics := SnapshotAttemptMetrics(deploymentID)
	if metrics.FailureCount != 2 {
		t.Fatalf("expected failure count 2, got %d", metrics.FailureCount)
	}
	if metrics.TotalDurationMs != 400 {
		t.Fatalf("expected total duration 400, got %d", metrics.TotalDurationMs)
	}
	if metrics.MinDurationMs != 150 {
		t.Fatalf("expected min duration 150, got %d", metrics.MinDurationMs)
	}
	if metrics.MaxDurationMs != 250 {
		t.Fatalf("expected max duration 250, got %d", metrics.MaxDurationMs)
	}
	if metrics.SuccessCount != 0 {
		t.Fatalf("expected success count 0, got %d", metrics.SuccessCount)
	}
}

func TestAttemptMetricsSkipAggregation(t *testing.T) {
	ResetAttemptMetrics("")
	defer ResetAttemptMetrics("")

	deploymentID := "free:test-metrics-skip-001"
	RecordAttemptSkip(deploymentID)
	RecordAttemptSkip(deploymentID)
	RecordAttemptSkip(deploymentID)

	metrics := SnapshotAttemptMetrics(deploymentID)
	if metrics.SkipCount != 3 {
		t.Fatalf("expected skip count 3, got %d", metrics.SkipCount)
	}
	if metrics.SuccessCount != 0 || metrics.FailureCount != 0 {
		t.Fatalf("expected only skips, got success=%d failure=%d", metrics.SuccessCount, metrics.FailureCount)
	}
}

func TestAttemptMetricsAllSnapshotNoDeadlock(t *testing.T) {
	ResetAttemptMetrics("")
	defer ResetAttemptMetrics("")

	deploymentID := "free:test-metrics-deadlock-001"
	RecordAttemptSuccess(deploymentID, 100)

	var wg sync.WaitGroup
	done := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			RecordAttemptSuccess(deploymentID, int64(i))
			select {
			case <-done:
				return
			default:
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = AllAttemptMetricsSnapshot()
			select {
			case <-done:
				return
			default:
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = SnapshotAttemptMetrics(deploymentID)
			select {
			case <-done:
				return
			default:
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	close(done)
	wg.Wait()

	all := AllAttemptMetricsSnapshot()
	if len(all) == 0 {
		t.Fatal("expected at least one deployment in snapshot")
	}
}

func TestAttemptMetricsConcurrentRecordNoRace(t *testing.T) {
	ResetAttemptMetrics("")
	defer ResetAttemptMetrics("")

	deploymentID := "free:test-metrics-concurrent-001"
	const numGoroutines = 20
	const opsPerGoroutine = 100

	var g errgroup.Group
	for i := 0; i < numGoroutines; i++ {
		g.Go(func() error {
			for j := 0; j < opsPerGoroutine; j++ {
				switch j % 3 {
				case 0:
					RecordAttemptSuccess(deploymentID, int64(j))
				case 1:
					RecordAttemptFailure(deploymentID, int64(j))
				case 2:
					RecordAttemptSkip(deploymentID)
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("concurrent recording failed: %v", err)
	}

	metrics := SnapshotAttemptMetrics(deploymentID)
	expectedSuccess := 20 * 34 // j%3==0 occurs 34 times in 0..99
	expectedFailure := 20 * 33 // j%3==1 occurs 33 times
	expectedSkip := 20 * 33    // j%3==2 occurs 33 times

	if metrics.SuccessCount != int64(expectedSuccess) {
		t.Fatalf("expected success count %d, got %d", expectedSuccess, metrics.SuccessCount)
	}
	if metrics.FailureCount != int64(expectedFailure) {
		t.Fatalf("expected failure count %d, got %d", expectedFailure, metrics.FailureCount)
	}
	if metrics.SkipCount != int64(expectedSkip) {
		t.Fatalf("expected skip count %d, got %d", expectedSkip, metrics.SkipCount)
	}

	total := metrics.SuccessCount + metrics.FailureCount + metrics.SkipCount
	expectedTotal := int64(numGoroutines * opsPerGoroutine)
	if total != expectedTotal {
		t.Fatalf("expected total %d, got %d", expectedTotal, total)
	}
}

func TestErrorCategoryCountersRecordAndSnapshot(t *testing.T) {
	ResetErrorCategoryCounters()
	defer ResetErrorCategoryCounters()

	RecordErrorCategoryCounter(ErrorCategoryRateLimit)
	RecordErrorCategoryCounter(ErrorCategoryRateLimit)
	RecordErrorCategoryCounter(ErrorCategoryTemporary)
	RecordErrorCategoryCounter(ErrorCategoryQuota)
	RecordErrorCategoryCounter(ErrorCategoryNone)

	counters := SnapshotErrorCategoryCounters()
	if counters["rate_limit"] != 2 {
		t.Fatalf("expected rate_limit counter 2, got %d", counters["rate_limit"])
	}
	if counters["temporary"] != 1 {
		t.Fatalf("expected temporary counter 1, got %d", counters["temporary"])
	}
	if counters["quota"] != 1 {
		t.Fatalf("expected quota counter 1, got %d", counters["quota"])
	}
	if counters["none"] != 1 {
		t.Fatalf("expected none counter 1, got %d", counters["none"])
	}
	if counters["client"] != 0 {
		t.Fatalf("expected client counter 0, got %d", counters["client"])
	}
}

func TestErrorCategoryCountersResetSafe(t *testing.T) {
	ResetErrorCategoryCounters()
	defer ResetErrorCategoryCounters()

	RecordErrorCategoryCounter(ErrorCategoryRateLimit)
	RecordErrorCategoryCounter(ErrorCategoryTemporary)

	var wg sync.WaitGroup
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				RecordErrorCategoryCounter(ErrorCategoryRateLimit)
				select {
				case <-done:
					return
				default:
				}
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			ResetErrorCategoryCounters()
			select {
			case <-done:
				return
			default:
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = SnapshotErrorCategoryCounters()
			select {
			case <-done:
				return
			default:
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	close(done)
	wg.Wait()

	_ = SnapshotErrorCategoryCounters()
}

func TestAttemptMetricsResetDeployment(t *testing.T) {
	ResetAttemptMetrics("")
	defer ResetAttemptMetrics("")

	deploymentID := "free:test-metrics-reset-001"
	RecordAttemptSuccess(deploymentID, 100)
	RecordAttemptFailure(deploymentID, 200)
	RecordAttemptSkip(deploymentID)

	metrics := SnapshotAttemptMetrics(deploymentID)
	if metrics.SuccessCount != 1 || metrics.FailureCount != 1 || metrics.SkipCount != 1 {
		t.Fatalf("expected pre-reset counts 1/1/1, got %d/%d/%d",
			metrics.SuccessCount, metrics.FailureCount, metrics.SkipCount)
	}

	ResetAttemptMetrics(deploymentID)

	metrics = SnapshotAttemptMetrics(deploymentID)
	if metrics.SuccessCount != 0 || metrics.FailureCount != 0 || metrics.SkipCount != 0 {
		t.Fatalf("expected post-reset counts 0/0/0, got %d/%d/%d",
			metrics.SuccessCount, metrics.FailureCount, metrics.SkipCount)
	}
	if metrics.MinDurationMs != -1 {
		t.Fatalf("expected MinDurationMs -1 after reset, got %d", metrics.MinDurationMs)
	}
}

func TestAttemptMetricsResetAll(t *testing.T) {
	ResetAttemptMetrics("")
	defer ResetAttemptMetrics("")

	deploymentA := "free:test-metrics-resetall-a"
	deploymentB := "free:test-metrics-resetall-b"
	RecordAttemptSuccess(deploymentA, 100)
	RecordAttemptFailure(deploymentB, 200)

	all := AllAttemptMetricsSnapshot()
	if len(all) != 2 {
		t.Fatalf("expected 2 deployments pre-reset, got %d", len(all))
	}

	ResetAttemptMetrics("")

	all = AllAttemptMetricsSnapshot()
	if len(all) != 0 {
		t.Fatalf("expected 0 deployments post-reset, got %d", len(all))
	}
}

func TestAttemptMetricsMinDurationUpdatesCorrectly(t *testing.T) {
	ResetAttemptMetrics("")
	defer ResetAttemptMetrics("")

	deploymentID := "free:test-metrics-min-001"
	RecordAttemptSuccess(deploymentID, 300)
	RecordAttemptSuccess(deploymentID, 100)
	RecordAttemptSuccess(deploymentID, 200)

	metrics := SnapshotAttemptMetrics(deploymentID)
	if metrics.MinDurationMs != 100 {
		t.Fatalf("expected min duration 100, got %d", metrics.MinDurationMs)
	}
	if metrics.MaxDurationMs != 300 {
		t.Fatalf("expected max duration 300, got %d", metrics.MaxDurationMs)
	}
}

func TestAttemptMetricsSnapshotMetricsReturnsZeroForUnknownDeployment(t *testing.T) {
	ResetAttemptMetrics("")
	defer ResetAttemptMetrics("")

	metrics := SnapshotAttemptMetrics("nonexistent-deployment")
	if metrics.SuccessCount != 0 || metrics.FailureCount != 0 || metrics.SkipCount != 0 {
		t.Fatalf("expected zeroed metrics for unknown deployment, got %+v", metrics)
	}
	if metrics.MinDurationMs != -1 {
		t.Fatalf("expected MinDurationMs -1 for unknown deployment, got %d", metrics.MinDurationMs)
	}
}
