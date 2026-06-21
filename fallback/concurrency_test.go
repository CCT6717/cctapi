package fallback

import "testing"

func resetDeploymentConcurrencyForTest() {
	deploymentConcurrency.Lock()
	defer deploymentConcurrency.Unlock()
	deploymentConcurrency.locks = make(map[string]*perDeploymentLock)
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

func TestTryAcquireDifferentDeploymentsDoNotBlock(t *testing.T) {
	resetDeploymentConcurrencyForTest()

	dep1 := DeploymentConfig{ID: "dep-a", MaxConcurrentRequests: 1}
	dep2 := DeploymentConfig{ID: "dep-b", MaxConcurrentRequests: 1}

	release1, acquired1, inFlight1 := TryAcquireDeploymentSlot(dep1)
	if !acquired1 || inFlight1 != 1 {
		t.Fatalf("dep-a first acquire should succeed, got acquired=%v inFlight=%d", acquired1, inFlight1)
	}
	defer release1()

	// dep-b should succeed even though dep-a is at its limit — no cross-deployment blocking
	release2, acquired2, inFlight2 := TryAcquireDeploymentSlot(dep2)
	if !acquired2 || inFlight2 != 1 {
		t.Fatalf("dep-b should not be blocked by dep-a, got acquired=%v inFlight=%d", acquired2, inFlight2)
	}
	release2()
}
