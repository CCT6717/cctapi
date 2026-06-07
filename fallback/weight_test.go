package fallback

import "testing"

func resetWeightedRoundRobinStoreForTest() {
	weightedRoundRobinStore.Lock()
	defer weightedRoundRobinStore.Unlock()
	weightedRoundRobinStore.current = make(map[string]map[string]int64)
}

func TestOrderDeploymentsForRequestWeightedRatio(t *testing.T) {
	resetWeightedRoundRobinStoreForTest()

	deployments := []DeploymentConfig{
		{ID: "primary", Weight: 70},
		{ID: "backup", Weight: 30},
	}
	counts := map[string]int{}
	for i := 0; i < 10; i++ {
		ordered := OrderDeploymentsForRequest("weighted/auto", deployments)
		counts[ordered[0].ID]++
	}

	if counts["primary"] != 7 || counts["backup"] != 3 {
		t.Fatalf("expected 70/30 distribution over 10 requests, got primary=%d backup=%d", counts["primary"], counts["backup"])
	}
}

func TestOrderDeploymentsForRequestPreservesFallbackOrder(t *testing.T) {
	resetWeightedRoundRobinStoreForTest()

	deployments := []DeploymentConfig{
		{ID: "first", Weight: 10},
		{ID: "second", Weight: 90},
		{ID: "third", Weight: 10},
	}
	ordered := OrderDeploymentsForRequest("order/auto", deployments)

	expected := []string{"second", "first", "third"}
	for i, deploymentID := range expected {
		if ordered[i].ID != deploymentID {
			t.Fatalf("expected index %d to be %s, got %s", i, deploymentID, ordered[i].ID)
		}
	}
}
