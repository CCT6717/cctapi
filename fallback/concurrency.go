package fallback

import "sync"

// perDeploymentLock holds a dedicated mutex for each deployment's in-flight counter.
// Different deployments never block each other; only concurrent requests to the same
// deployment contend on the same lock.
type perDeploymentLock struct {
	mu       sync.Mutex
	inFlight int
}

// deploymentConcurrency manages per-deployment concurrency tracking.
// A global RWMutex protects the map of per-deployment locks; the read path
// (acquiring/releasing a slot for an existing deployment) only takes a read lock,
// while the write path (first access to a new deployment) takes a write lock.
var deploymentConcurrency struct {
	sync.RWMutex
	locks map[string]*perDeploymentLock
}

func init() {
	deploymentConcurrency.locks = make(map[string]*perDeploymentLock)
}

// getOrCreateLock returns the per-deployment lock, creating one on first access.
func getOrCreateLock(deploymentID string) *perDeploymentLock {
	deploymentConcurrency.RLock()
	lock, ok := deploymentConcurrency.locks[deploymentID]
	deploymentConcurrency.RUnlock()
	if ok {
		return lock
	}

	// First access — create under write lock
	deploymentConcurrency.Lock()
	defer deploymentConcurrency.Unlock()
	// Double-check after acquiring write lock
	lock, ok = deploymentConcurrency.locks[deploymentID]
	if ok {
		return lock
	}
	lock = &perDeploymentLock{}
	deploymentConcurrency.locks[deploymentID] = lock
	return lock
}

func TryAcquireDeploymentSlot(dep DeploymentConfig) (func(), bool, int) {
	limit := dep.MaxConcurrentRequests
	if limit <= 0 {
		return func() {}, true, 0
	}

	lock := getOrCreateLock(dep.ID)
	lock.mu.Lock()
	current := lock.inFlight
	if current >= limit {
		lock.mu.Unlock()
		return func() {}, false, current
	}
	lock.inFlight = current + 1
	lock.mu.Unlock()

	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			lock.mu.Lock()
			defer lock.mu.Unlock()
			if lock.inFlight <= 1 {
				lock.inFlight = 0
				return
			}
			lock.inFlight--
		})
	}

	return release, true, current + 1
}

func GetDeploymentInFlight(deploymentID string) int {
	deploymentConcurrency.RLock()
	lock, ok := deploymentConcurrency.locks[deploymentID]
	deploymentConcurrency.RUnlock()
	if !ok {
		return 0
	}
	lock.mu.Lock()
	defer lock.mu.Unlock()
	return lock.inFlight
}
