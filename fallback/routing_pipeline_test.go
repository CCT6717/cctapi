package fallback

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
)

func TestDecideDeploymentModelAttempt(t *testing.T) {
	tests := []struct {
		name          string
		attempt       DeploymentModelAttempt
		statusCode    int
		rateLimit     bool
		wantAction    DeploymentModelAction
		wantConfirmed bool
		wantModel429  bool
	}{
		{
			name: "Kilo 429 rotates when another model remains",
			attempt: DeploymentModelAttempt{
				ProviderName: "kilo", Rotatable: true, ModelIndex: 0, ModelCount: 2,
			},
			statusCode: http.StatusTooManyRequests, rateLimit: true,
			wantAction: DeploymentModelActionRotate, wantConfirmed: true, wantModel429: true,
		},
		{
			name: "last Kilo 429 completes the deployment",
			attempt: DeploymentModelAttempt{
				ProviderName: "kilo", Rotatable: true, ModelIndex: 1, ModelCount: 2,
			},
			statusCode: http.StatusTooManyRequests, rateLimit: true,
			wantAction: DeploymentModelActionComplete, wantConfirmed: true, wantModel429: true,
		},
		{
			name: "rate-limit-shaped 500 skips remaining Kilo models",
			attempt: DeploymentModelAttempt{
				ProviderName: "kilo", Rotatable: true, ModelIndex: 0, ModelCount: 2,
			},
			statusCode: http.StatusInternalServerError, rateLimit: true,
			wantAction: DeploymentModelActionSkipRemaining,
		},
		{
			name: "ordinary Kilo failure skips remaining models",
			attempt: DeploymentModelAttempt{
				ProviderName: "kilo", Rotatable: true, ModelIndex: 0, ModelCount: 2,
			},
			statusCode: http.StatusBadGateway,
			wantAction: DeploymentModelActionSkipRemaining,
		},
		{
			name: "non-Kilo deployment keeps provider-level behavior",
			attempt: DeploymentModelAttempt{
				ProviderName: "pollinations", ModelIndex: 0, ModelCount: 1,
			},
			statusCode: http.StatusTooManyRequests, rateLimit: true,
			wantAction: DeploymentModelActionComplete, wantConfirmed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecideDeploymentModelAttempt(tt.attempt, tt.statusCode, tt.rateLimit)
			if got.Action != tt.wantAction ||
				got.ConfirmedHTTPRateLimit != tt.wantConfirmed ||
				got.RecordModelRateLimit != tt.wantModel429 {
				t.Fatalf("decision = %+v, want action=%v confirmed=%v model429=%v", got, tt.wantAction, tt.wantConfirmed, tt.wantModel429)
			}
		})
	}
}

func TestSnapshotVirtualModelRoutingConfigIsRequestScoped(t *testing.T) {
	resetFallbackPlanningStateForTest(t, &Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {
				Enabled:             true,
				Strategy:            StrategyFreeFirst,
				Pools:               []string{"free"},
				FallbackOrder:       []string{"dep-a", "dep-b"},
				PreferredDeployment: "dep-b",
			},
		},
		Deployments: map[string]DeploymentConfig{
			"dep-a": {Enabled: true, Pool: "free", RealModel: "model-a", Priority: 1},
			"dep-b": {Enabled: true, Pool: "free", RealModel: "model-b", Priority: 2},
		},
	})

	snapshot, err := SnapshotVirtualModelRoutingConfig("cct/free")
	if err != nil {
		t.Fatalf("SnapshotVirtualModelRoutingConfig failed: %v", err)
	}
	if snapshot.VirtualModel.PreferredDeployment != "dep-b" {
		t.Fatalf("preferred deployment = %q, want dep-b", snapshot.VirtualModel.PreferredDeployment)
	}
	if len(snapshot.Deployments) != 2 || snapshot.Deployments[0].ID != "dep-b" {
		t.Fatalf("snapshot deployments = %#v, want dep-b first", snapshot.Deployments)
	}

	snapshot.VirtualModel.Pools[0] = "mutated"
	snapshot.VirtualModel.FallbackOrder[0] = "mutated"
	snapshot.Deployments[0].RealModel = "mutated"

	live := GetConfig()
	if live.VirtualModels["cct/free"].Pools[0] != "free" ||
		live.VirtualModels["cct/free"].FallbackOrder[0] != "dep-a" ||
		live.Deployments["dep-b"].RealModel != "model-b" {
		t.Fatalf("request snapshot mutated live config: %#v", live)
	}
}

func TestSnapshotVirtualModelRoutingConfigDoesNotMixConfigGenerations(t *testing.T) {
	configA := &Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {Enabled: true, Pools: []string{"pool-a"}, PreferredDeployment: "dep-a"},
		},
		Deployments: map[string]DeploymentConfig{
			"dep-a": {Enabled: true, Pool: "pool-a", RealModel: "model-a"},
		},
	}
	configB := &Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {Enabled: true, Pools: []string{"pool-b"}, PreferredDeployment: "dep-b"},
		},
		Deployments: map[string]DeploymentConfig{
			"dep-b": {Enabled: true, Pool: "pool-b", RealModel: "model-b"},
		},
	}

	publish := func(cfg *Config) {
		configLock.Lock()
		config = cloneConfig(cfg)
		configLock.Unlock()
	}
	publish(configA)
	t.Cleanup(func() { resetConfigForTest(nil) })

	const iterations = 500
	errCh := make(chan error, iterations)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if i%2 == 0 {
				publish(configB)
			} else {
				publish(configA)
			}
		}
	}()
	for reader := 0; reader < 2; reader++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				snapshot, err := SnapshotVirtualModelRoutingConfig("cct/free")
				if err != nil {
					errCh <- err
					continue
				}
				preferred := snapshot.VirtualModel.PreferredDeployment
				if len(snapshot.Deployments) != 1 || snapshot.Deployments[0].ID != preferred {
					errCh <- fmt.Errorf("mixed routing snapshot: preferred=%s deployments=%#v", preferred, snapshot.Deployments)
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestGetVirtualModelReturnsRequestScopedClone(t *testing.T) {
	resetFallbackPlanningStateForTest(t, &Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {
				Enabled:       true,
				Pools:         []string{"free"},
				FallbackOrder: []string{"dep-a"},
			},
		},
	})

	vm, ok := GetVirtualModel("cct/free")
	if !ok {
		t.Fatal("expected virtual model")
	}
	vm.Pools[0] = "mutated"
	vm.FallbackOrder[0] = "mutated"

	live := GetConfig().VirtualModels["cct/free"]
	if live.Pools[0] != "free" || live.FallbackOrder[0] != "dep-a" {
		t.Fatalf("GetVirtualModel leaked mutation into live config: %#v", live)
	}
}
