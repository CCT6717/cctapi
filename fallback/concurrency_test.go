package fallback

import "testing"

func resetDeploymentConcurrencyForTest() {
	deploymentConcurrency.Lock()
	defer deploymentConcurrency.Unlock()
	deploymentConcurrency.inFlight = make(map[string]int)
}

func TestTryAcquireDeploymentSlotSkipsWhenLimitReached(t *testing.T) {
	resetDeploymentConcurrencyForTest()

	dep := DeploymentConfig{ID: "limited", MaxConcurrentRequests: 1}
	release, acquired, inFlight := TryAcquireDeploymentSlot(dep)
	if !acquired || inFlight != 1 {
		t.Fatalf("expected first acquire to succeed with inFlight=1, got acquired=%v inFlight=%d", acquired, inFlight)
	}
	defer release()

	_, acquired, inFlight = TryAcquireDeploymentSlot(dep)
	if acquired || inFlight != 1 {
		t.Fatalf("expected second acquire to be skipped with inFlight=1, got acquired=%v inFlight=%d", acquired, inFlight)
	}
}

func TestTryAcquireDeploymentSlotReleaseIsIdempotent(t *testing.T) {
	resetDeploymentConcurrencyForTest()

	dep := DeploymentConfig{ID: "limited", MaxConcurrentRequests: 1}
	release, acquired, _ := TryAcquireDeploymentSlot(dep)
	if !acquired {
		t.Fatal("expected acquire to succeed")
	}

	release()
	release()

	if got := GetDeploymentInFlight(dep.ID); got != 0 {
		t.Fatalf("expected in-flight count to return to 0, got %d", got)
	}
}

func TestTryAcquireDeploymentSlotUnlimited(t *testing.T) {
	resetDeploymentConcurrencyForTest()

	dep := DeploymentConfig{ID: "unlimited", MaxConcurrentRequests: 0}
	release, acquired, inFlight := TryAcquireDeploymentSlot(dep)
	if !acquired || inFlight != 0 {
		t.Fatalf("expected unlimited acquire to succeed without tracking, got acquired=%v inFlight=%d", acquired, inFlight)
	}
	release()
}
