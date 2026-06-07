package fallback

import "sync"

const defaultDeploymentWeight = 100

var weightedRoundRobinStore = struct {
	sync.Mutex
	current map[string]map[string]int64
}{
	current: make(map[string]map[string]int64),
}

// OrderDeploymentsForRequest rotates the first deployment by smooth weighted
// round-robin, while keeping the remaining deployments in their existing
// smart-sort/priority order for fallback attempts.
func OrderDeploymentsForRequest(virtualModel string, deployments []DeploymentConfig) []DeploymentConfig {
	if len(deployments) <= 1 {
		return deployments
	}

	selectedIndex := nextWeightedDeploymentIndex(virtualModel, deployments)
	if selectedIndex <= 0 {
		return deployments
	}

	ordered := make([]DeploymentConfig, 0, len(deployments))
	ordered = append(ordered, deployments[selectedIndex])
	for i, dep := range deployments {
		if i == selectedIndex {
			continue
		}
		ordered = append(ordered, dep)
	}
	return ordered
}

func nextWeightedDeploymentIndex(virtualModel string, deployments []DeploymentConfig) int {
	totalWeight := int64(0)
	activeIDs := make(map[string]bool, len(deployments))
	for _, dep := range deployments {
		weight := deploymentWeight(dep)
		if weight <= 0 {
			continue
		}
		activeIDs[dep.ID] = true
		totalWeight += weight
	}
	if totalWeight <= 0 {
		return 0
	}

	weightedRoundRobinStore.Lock()
	defer weightedRoundRobinStore.Unlock()

	current, ok := weightedRoundRobinStore.current[virtualModel]
	if !ok {
		current = make(map[string]int64)
		weightedRoundRobinStore.current[virtualModel] = current
	}
	for deploymentID := range current {
		if !activeIDs[deploymentID] {
			delete(current, deploymentID)
		}
	}

	selectedIndex := -1
	selectedCurrent := int64(0)
	for i, dep := range deployments {
		weight := deploymentWeight(dep)
		if weight <= 0 {
			continue
		}
		current[dep.ID] += weight
		if selectedIndex < 0 || current[dep.ID] > selectedCurrent {
			selectedIndex = i
			selectedCurrent = current[dep.ID]
		}
	}

	if selectedIndex >= 0 {
		current[deployments[selectedIndex].ID] -= totalWeight
	}
	return selectedIndex
}

func deploymentWeight(dep DeploymentConfig) int64 {
	if dep.Weight <= 0 {
		return defaultDeploymentWeight
	}
	return int64(dep.Weight)
}
