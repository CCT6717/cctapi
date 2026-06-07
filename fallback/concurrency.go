package fallback

import "sync"

var deploymentConcurrency = struct {
	sync.Mutex
	inFlight map[string]int
}{
	inFlight: make(map[string]int),
}

func TryAcquireDeploymentSlot(dep DeploymentConfig) (func(), bool, int) {
	limit := dep.MaxConcurrentRequests
	if limit <= 0 {
		return func() {}, true, 0
	}

	deploymentConcurrency.Lock()
	current := deploymentConcurrency.inFlight[dep.ID]
	if current >= limit {
		deploymentConcurrency.Unlock()
		return func() {}, false, current
	}
	deploymentConcurrency.inFlight[dep.ID] = current + 1
	deploymentConcurrency.Unlock()

	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			deploymentConcurrency.Lock()
			defer deploymentConcurrency.Unlock()
			current := deploymentConcurrency.inFlight[dep.ID]
			if current <= 1 {
				delete(deploymentConcurrency.inFlight, dep.ID)
				return
			}
			deploymentConcurrency.inFlight[dep.ID] = current - 1
		})
	}

	return release, true, current + 1
}

func GetDeploymentInFlight(deploymentID string) int {
	deploymentConcurrency.Lock()
	defer deploymentConcurrency.Unlock()
	return deploymentConcurrency.inFlight[deploymentID]
}
