package fallback

import (
	"sync"
	"time"

	"github.com/songquanpeng/one-api/common/logger"
)

// freeSyncRunning / freeSyncStopCh 守卫 StartFreeSync,照抄 health.go 的
// globalHealth 模式,防重复启动 + 支持优雅停止。
var (
	freeSyncRunning bool
	freeSyncStopCh  chan struct{}
	freeSyncMu      sync.Mutex
)

// StartFreeSync 启动两个后台 goroutine:fetch 模型(6h)和同步配额(15m)。
// 启动后立即各 warm-up 一次。stopCh 非 nil 时监听退出,nil 则跟随进程生命周期。
// 由 main.go 在 StartHealthChecker 旁调一次。
func StartFreeSync(stopCh chan struct{}) {
	freeSyncMu.Lock()
	if freeSyncRunning {
		freeSyncMu.Unlock()
		return
	}
	freeSyncRunning = true
	freeSyncStopCh = stopCh
	freeSyncMu.Unlock()

	go runFreeSyncModels()
	go runFreeSyncCredits()
	logger.SysLog("[free-pool] free sync started: models 6h, credits 15m")
}

func runFreeSyncModels() {
	// warm-up 一次,再进 6h ticker
	refreshFreeProviderCatalogsWithCurrentConfig()
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			refreshFreeProviderCatalogsWithCurrentConfig()
		case <-freeSyncStopSignal():
			return
		}
	}
}

func runFreeSyncCredits() {
	// warm-up 一次,再进 15m ticker
	syncOpenRouterCredits(GetConfig())
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			syncOpenRouterCredits(GetConfig())
		case <-freeSyncStopSignal():
			return
		}
	}
}

// freeSyncStopSignal 返回一个停止信号 channel。stopCh 为 nil 时
// 返 nil(select 对 nil channel 永久阻塞,即不主动停,跟随进程)。
func freeSyncStopSignal() <-chan struct{} {
	freeSyncMu.Lock()
	defer freeSyncMu.Unlock()
	if freeSyncStopCh == nil {
		return nil
	}
	return freeSyncStopCh
}
