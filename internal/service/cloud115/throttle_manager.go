package cloud115

import (
	"context"
	"sync"
	"time"
)

// ThrottleManager 全局限流/熔断管理器，用于控制 115 API 访问频率与被风控时的全局静默冷却。
type ThrottleManager struct {
	sync.RWMutex
	isThrottled       bool
	throttleStartTime time.Time
	throttleNotify    chan struct{}
	throttleDuration  time.Duration
}

var (
	globalThrottleManager *ThrottleManager
	throttleOnce          sync.Once
)

// GetGlobalThrottleManager 获取全局限流管理器单例。
func GetGlobalThrottleManager() *ThrottleManager {
	throttleOnce.Do(func() {
		globalThrottleManager = NewThrottleManager(60 * time.Second)
	})
	return globalThrottleManager
}

// NewThrottleManager 创建限流管理器。
func NewThrottleManager(duration time.Duration) *ThrottleManager {
	if duration <= 0 {
		duration = 60 * time.Second
	}
	return &ThrottleManager{
		isThrottled:      false,
		throttleNotify:   make(chan struct{}),
		throttleDuration: duration,
	}
}

// IsThrottled 检查是否处于限流冷却状态。
func (tm *ThrottleManager) IsThrottled() bool {
	tm.RLock()
	defer tm.RUnlock()

	if !tm.isThrottled {
		return false
	}
	if time.Since(tm.throttleStartTime) >= tm.throttleDuration {
		return false
	}
	return true
}

// MarkThrottled 标记为限流状态，并启动恢复计时器。
func (tm *ThrottleManager) MarkThrottled() {
	tm.Lock()
	defer tm.Unlock()

	if tm.isThrottled && time.Since(tm.throttleStartTime) < tm.throttleDuration {
		return
	}

	tm.isThrottled = true
	tm.throttleStartTime = time.Now()

	go tm.startRecoveryTimer()
}

func (tm *ThrottleManager) startRecoveryTimer() {
	time.Sleep(tm.throttleDuration)

	tm.Lock()
	defer tm.Unlock()

	tm.isThrottled = false

	select {
	case tm.throttleNotify <- struct{}{}:
	default:
	}
}

// WaitThrottleRecovery 如果处于限流状态，挂起等待直到冷却恢复。
func (tm *ThrottleManager) WaitThrottleRecovery(ctx context.Context) error {
	for {
		if !tm.IsThrottled() {
			return nil
		}

		ticker := time.NewTicker(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			ticker.Stop()
			return ctx.Err()
		case <-ticker.C:
			ticker.Stop()
			if !tm.IsThrottled() {
				return nil
			}
		}
	}
}

// Reset 重置限流状态（主要用于单元测试）。
func (tm *ThrottleManager) Reset() {
	tm.Lock()
	defer tm.Unlock()
	tm.isThrottled = false
}

// SetDuration 调整限流冷却时间（用于测试或动态配置）。
func (tm *ThrottleManager) SetDuration(d time.Duration) {
	tm.Lock()
	defer tm.Unlock()
	tm.throttleDuration = d
}
