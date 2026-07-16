package fallback

import "strings"

// DeploymentModelAttempt is one request-scoped model candidate for a deployment.
// Its Deployment value is a copy and never mutates the configured deployment.
type DeploymentModelAttempt struct {
	Deployment                   DeploymentConfig
	ProviderName                 string
	ModelIndex                   int
	ModelCount                   int
	CompatibleCount              int
	CoolingCount                 int
	CapabilityFalsePositiveCount int
	Rotatable                    bool
	SkipReason                   string
}

func (attempt DeploymentModelAttempt) HasNextModel() bool {
	return attempt.Rotatable && attempt.RemainingModelAttempts() > 0
}

func (attempt DeploymentModelAttempt) RemainingModelAttempts() int {
	remaining := attempt.ModelCount - attempt.ModelIndex - 1
	if !attempt.Rotatable || remaining < 0 {
		return 0
	}
	return remaining
}

// DeploymentModelPlan groups the model candidates for one deployment.
type DeploymentModelPlan struct {
	Attempts                     []DeploymentModelAttempt
	CompatibleCount              int
	CoolingCount                 int
	CapabilityFalsePositiveCount int
	Rotatable                    bool
}

// PrepareDeploymentModelPlan builds the request-scoped model candidates for one
// deployment. Only automatically generated Kilo deployments rotate catalog
// models; every other deployment retains its existing one-attempt behavior.
func PrepareDeploymentModelPlan(dep DeploymentConfig, caps RequestCapabilities) DeploymentModelPlan {
	providerName, generated := FreeProviderNameFromDeploymentID(dep.ID)
	if !generated || providerName != "kilo" {
		return prepareSingleDeploymentModelPlan(dep, providerName, caps)
	}

	snapshot, ok := GetFreeProviderCatalogSnapshot(dep.ID)
	if !ok || len(snapshot.Models) == 0 {
		return prepareSingleDeploymentModelPlan(dep, providerName, caps)
	}

	compatible := filterKiloModelCandidates(kiloModelCandidates(dep, snapshot.Models), caps)
	plan := DeploymentModelPlan{
		CompatibleCount: len(compatible),
		Rotatable:       true,
	}
	available := make([]DeploymentConfig, 0, len(compatible))
	for _, candidate := range compatible {
		if IsFreeProviderModelCooling(dep.ID, candidate.RealModel) {
			plan.CoolingCount++
			continue
		}
		if caps.Tools && IsFreeProviderModelCapabilityFalsePositive(dep.ID, candidate.RealModel, "tools") {
			plan.CapabilityFalsePositiveCount++
			continue
		}
		available = append(available, candidate)
	}

	plan.Attempts = make([]DeploymentModelAttempt, 0, len(available))
	for index, candidate := range available {
		plan.Attempts = append(plan.Attempts, DeploymentModelAttempt{
			Deployment:                   candidate,
			ProviderName:                 providerName,
			ModelIndex:                   index,
			ModelCount:                   len(available),
			CompatibleCount:              plan.CompatibleCount,
			CoolingCount:                 plan.CoolingCount,
			CapabilityFalsePositiveCount: plan.CapabilityFalsePositiveCount,
			Rotatable:                    true,
		})
	}
	return plan
}

// PrepareDeploymentModelAttempts flattens request-scoped deployment plans in
// deployment order for the relay fallback loop.
func PrepareDeploymentModelAttempts(deployments []DeploymentConfig, caps RequestCapabilities) []DeploymentModelAttempt {
	attempts := make([]DeploymentModelAttempt, 0, len(deployments))
	for _, dep := range deployments {
		plan := PrepareDeploymentModelPlan(dep, caps)
		if len(plan.Attempts) == 0 && plan.CompatibleCount > 0 {
			providerName, _ := FreeProviderNameFromDeploymentID(dep.ID)
			attempts = append(attempts, DeploymentModelAttempt{
				Deployment:                   dep,
				ProviderName:                 providerName,
				CompatibleCount:              plan.CompatibleCount,
				CoolingCount:                 plan.CoolingCount,
				CapabilityFalsePositiveCount: plan.CapabilityFalsePositiveCount,
				Rotatable:                    plan.Rotatable,
				SkipReason:                   unavailableModelPlanReason(plan),
			})
			continue
		}
		attempts = append(attempts, plan.Attempts...)
	}
	return attempts
}

func unavailableModelPlanReason(plan DeploymentModelPlan) string {
	if plan.CoolingCount == plan.CompatibleCount {
		return "all compatible models cooling down"
	}
	if plan.CapabilityFalsePositiveCount == plan.CompatibleCount {
		return "tools capability temporarily unavailable"
	}
	return "all compatible models temporarily unavailable"
}

func prepareSingleDeploymentModelPlan(dep DeploymentConfig, providerName string, caps RequestCapabilities) DeploymentModelPlan {
	candidates := FilterByCapability([]DeploymentConfig{dep}, caps)
	plan := DeploymentModelPlan{CompatibleCount: len(candidates)}
	if len(candidates) == 0 {
		return plan
	}
	plan.Attempts = []DeploymentModelAttempt{{
		Deployment:      candidates[0],
		ProviderName:    providerName,
		ModelCount:      1,
		CompatibleCount: 1,
	}}
	return plan
}

func kiloModelCandidates(dep DeploymentConfig, models []FreeModelCatalogEntry) []DeploymentConfig {
	candidates := make([]DeploymentConfig, 0, len(models)+1)
	meta := BuiltinFreeProviders["kilo"]
	configuredModel := strings.TrimSpace(dep.RealModel)
	configuredFound := false
	for _, entry := range models {
		if entry.ID == configuredModel {
			candidates = append(candidates, applyFreeProviderCatalogModelCapabilities(dep, meta, entry))
			configuredFound = true
			break
		}
	}
	if !configuredFound && configuredModel != "" {
		candidates = append(candidates, dep)
	}
	for _, entry := range models {
		if entry.ID != configuredModel {
			candidates = append(candidates, applyFreeProviderCatalogModelCapabilities(dep, meta, entry))
		}
	}
	return candidates
}

func filterKiloModelCandidates(candidates []DeploymentConfig, caps RequestCapabilities) []DeploymentConfig {
	compatible := make([]DeploymentConfig, 0, len(candidates))
	for _, candidate := range candidates {
		filtered := FilterByCapability([]DeploymentConfig{candidate}, caps)
		if len(filtered) == 1 && filtered[0].RealModel == candidate.RealModel {
			compatible = append(compatible, candidate)
		}
	}
	return compatible
}
