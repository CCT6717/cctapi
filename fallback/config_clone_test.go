package fallback

import "testing"

func TestCloneConfigMutationIsolation(t *testing.T) {
	rpm := 10
	resetConfigForTest(&Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/high": {
				Enabled:       true,
				Pools:         []string{"paid_high"},
				FallbackOrder: []string{"dep-a"},
			},
		},
		Deployments: map[string]DeploymentConfig{
			"dep-a": {Enabled: true, RealModel: "gpt-4", Pool: "paid_high"},
		},
		FreeProviders: map[string]FreeProviderConfig{
			"groq": {
				Enabled:        true,
				Keys:           []string{"original-key"},
				Models:         []string{"original-model"},
				LimitsOverride: &FreeProviderLimits{RPMLimit: &rpm},
			},
		},
		BlockedErrorCodes: []string{"blocked-a"},
	})
	t.Cleanup(func() { resetConfigForTest(nil) })

	cloned := CloneConfig()
	if cloned == nil {
		t.Fatal("expected cloned config")
	}

	vm := cloned.VirtualModels["cct/high"]
	vm.Pools[0] = "mutated_pool"
	vm.FallbackOrder[0] = "mutated-dep"
	cloned.VirtualModels["cct/high"] = vm

	dep := cloned.Deployments["dep-a"]
	dep.RealModel = "mutated-model"
	cloned.Deployments["dep-a"] = dep

	fp := cloned.FreeProviders["groq"]
	fp.Keys[0] = "mutated-key"
	fp.Models[0] = "mutated-model"
	*fp.LimitsOverride.RPMLimit = 999
	cloned.FreeProviders["groq"] = fp

	cloned.BlockedErrorCodes[0] = "mutated-block"

	live := GetConfig()
	if live.VirtualModels["cct/high"].Pools[0] != "paid_high" {
		t.Fatalf("live virtual model pool mutated: %v", live.VirtualModels["cct/high"].Pools)
	}
	if live.VirtualModels["cct/high"].FallbackOrder[0] != "dep-a" {
		t.Fatalf("live fallback order mutated: %v", live.VirtualModels["cct/high"].FallbackOrder)
	}
	if live.Deployments["dep-a"].RealModel != "gpt-4" {
		t.Fatalf("live deployment mutated: %#v", live.Deployments["dep-a"])
	}
	if live.FreeProviders["groq"].Keys[0] != "original-key" {
		t.Fatalf("live free provider key mutated: %v", live.FreeProviders["groq"].Keys)
	}
	if live.FreeProviders["groq"].Models[0] != "original-model" {
		t.Fatalf("live free provider models mutated: %v", live.FreeProviders["groq"].Models)
	}
	if *live.FreeProviders["groq"].LimitsOverride.RPMLimit != 10 {
		t.Fatalf("live free provider limits mutated: %d", *live.FreeProviders["groq"].LimitsOverride.RPMLimit)
	}
	if live.BlockedErrorCodes[0] != "blocked-a" {
		t.Fatalf("live blocked error codes mutated: %v", live.BlockedErrorCodes)
	}
}

func TestCloneVirtualModelMutationIsolation(t *testing.T) {
	resetConfigForTest(&Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {
				Enabled:       true,
				Pools:         []string{"free"},
				FallbackOrder: []string{"free-a"},
			},
		},
	})
	t.Cleanup(func() { resetConfigForTest(nil) })

	vm, ok := CloneVirtualModel("cct/free")
	if !ok {
		t.Fatal("expected virtual model clone")
	}
	vm.Pools[0] = "mutated"
	vm.FallbackOrder[0] = "mutated"

	live := GetConfig().VirtualModels["cct/free"]
	if live.Pools[0] != "free" {
		t.Fatalf("live pools mutated: %v", live.Pools)
	}
	if live.FallbackOrder[0] != "free-a" {
		t.Fatalf("live fallback order mutated: %v", live.FallbackOrder)
	}
}

func TestCloneDeploymentMutationIsolation(t *testing.T) {
	resetConfigForTest(&Config{
		Deployments: map[string]DeploymentConfig{
			"dep-a": {Enabled: true, RealModel: "original"},
		},
	})
	t.Cleanup(func() { resetConfigForTest(nil) })

	dep, ok := CloneDeployment("dep-a")
	if !ok {
		t.Fatal("expected deployment clone")
	}
	dep.RealModel = "mutated"

	if live := GetConfig().Deployments["dep-a"]; live.RealModel != "original" {
		t.Fatalf("live deployment mutated: %#v", live)
	}
}
