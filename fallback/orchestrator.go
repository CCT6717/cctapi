package fallback

// DeploymentPlan describes the ordered deployment candidates prepared for one
// request, along with counts useful for request-side logging.
type DeploymentPlan struct {
	Deployments           []DeploymentConfig
	PreferredDeploymentID string
	StickyDeploymentID    string
	CapabilityBefore      int
	CapabilityAfter       int
	HealthBefore          int
	HealthAfter           int
}

// PrepareDeploymentsForRequest returns the deployment attempt order for a
// virtual model without depending on HTTP or Gin request state.
func PrepareDeploymentsForRequest(virtualModel string, caps RequestCapabilities) ([]DeploymentConfig, error) {
	plan, err := PrepareDeploymentPlanForRequest(virtualModel, caps)
	if err != nil {
		return nil, err
	}
	return plan.Deployments, nil
}

// PrepareDeploymentPlanForRequest combines virtual model lookup, capability
// filtering, health filtering, strategy sorting, and preferred/sticky
// promotion into one request planning step.
func PrepareDeploymentPlanForRequest(virtualModel string, caps RequestCapabilities) (DeploymentPlan, error) {
	var plan DeploymentPlan

	deployments, err := GetDeploymentsForVirtualModel(virtualModel)
	if err != nil {
		return plan, err
	}

	vmConfig, hasVMConfig := GetVirtualModel(virtualModel)
	if hasVMConfig {
		plan.PreferredDeploymentID = vmConfig.PreferredDeployment
		if plan.PreferredDeploymentID == "" {
			plan.PreferredDeploymentID = vmConfig.FixedDeployment
		}
	}
	plan.StickyDeploymentID = GetStickyDeployment(virtualModel)

	plan.CapabilityBefore = len(deployments)
	deployments = FilterByCapability(deployments, caps)
	plan.CapabilityAfter = len(deployments)

	plan.HealthBefore = len(deployments)
	deployments = FilterHealthyDeployments(deployments)
	plan.HealthAfter = len(deployments)

	if hasVMConfig && len(deployments) > 1 {
		deployments = SortByStrategy(deployments, vmConfig.Strategy)
	}

	if plan.PreferredDeploymentID != "" {
		deployments = PreferDeployment(deployments, plan.PreferredDeploymentID)
	} else if plan.StickyDeploymentID != "" {
		deployments = PreferDeployment(deployments, plan.StickyDeploymentID)
	}

	plan.Deployments = deployments
	return plan, nil
}

// FilterHealthyDeployments drops deployments that the background health checker
// has marked invalid or in a persistent error state. healthy/unknown pass through.
func FilterHealthyDeployments(deployments []DeploymentConfig) []DeploymentConfig {
	out := make([]DeploymentConfig, 0, len(deployments))
	for _, dep := range deployments {
		if IsDeploymentHealthy(dep.ID) {
			out = append(out, dep)
		}
	}
	return out
}
