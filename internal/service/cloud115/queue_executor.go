package cloud115

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// QueueExecutor 全局 115 请求调度执行器，通过三级令牌桶（QPS / QPM / QPH）平滑所有外发请求。
type QueueExecutor struct {
	sync.RWMutex
	qpsLimiter      *rate.Limiter
	qpmLimiter      *rate.Limiter
	qphLimiter      *rate.Limiter
	throttleManager *ThrottleManager

	qpsConfig int
	qpmConfig int
	qphConfig int
}

var (
	globalExecutor *QueueExecutor
	executorOnce   sync.Once
)

// GetGlobalExecutor 获取全局队列执行器单例（默认 QPS=8, QPM=480, QPH=20000，保障 115 API 调用安全不超频）。
// QPS 从 2 提到 8：下载/列表等场景下过去 QPS=2 将所有直链换取串行为每秒 2 个，是下载吞吐的最大瓶颈。
// 8 是经过折中的安全值——远低于 115 WAF 风控触发阈值（QPS≈20 起才有明显风险），
// 又能让多 worker 并发换取直链，显著提升元数据下载速度。
func GetGlobalExecutor() *QueueExecutor {
	executorOnce.Do(func() {
		globalExecutor = NewQueueExecutor(8, 480, 20000)
	})
	return globalExecutor
}

// NewQueueExecutor 创建新的队列调度执行器。
func NewQueueExecutor(qps, qpm, qph int) *QueueExecutor {
	if qps <= 0 {
		qps = 3
	}
	if qpm <= 0 {
		qpm = 200
	}
	if qph <= 0 {
		qph = 12000
	}

	return &QueueExecutor{
		qpsLimiter:      rate.NewLimiter(rate.Limit(qps), qps),
		qpmLimiter:      rate.NewLimiter(rate.Every(time.Minute/time.Duration(qpm)), qpm),
		qphLimiter:      rate.NewLimiter(rate.Every(time.Hour/time.Duration(qph)), qph),
		throttleManager: GetGlobalThrottleManager(),
		qpsConfig:       qps,
		qpmConfig:       qpm,
		qphConfig:       qph,
	}
}

// Acquire 统一在发送请求前获取令牌，并等待熔断恢复。
func (qe *QueueExecutor) Acquire(ctx context.Context) error {
	// 1. 如果处于限流状态，先阻塞等待 60s 静默冷却结束
	if err := qe.throttleManager.WaitThrottleRecovery(ctx); err != nil {
		return err
	}

	// 2. 依次获取小时级、分钟级、秒级令牌
	if err := qe.qphLimiter.Wait(ctx); err != nil {
		return err
	}
	if err := qe.qpmLimiter.Wait(ctx); err != nil {
		return err
	}
	if err := qe.qpsLimiter.Wait(ctx); err != nil {
		return err
	}

	return nil
}

// SetRateLimitConfig 动态调整速率限制。
func (qe *QueueExecutor) SetRateLimitConfig(qps, qpm, qph int) {
	qe.Lock()
	defer qe.Unlock()

	if qps > 0 {
		qe.qpsConfig = qps
		qe.qpsLimiter.SetLimit(rate.Limit(qps))
		qe.qpsLimiter.SetBurst(qps)
	}
	if qpm > 0 {
		qe.qpmConfig = qpm
		qe.qpmLimiter.SetLimit(rate.Every(time.Minute / time.Duration(qpm)))
		qe.qpmLimiter.SetBurst(qpm)
	}
	if qph > 0 {
		qe.qphConfig = qph
		qe.qphLimiter.SetLimit(rate.Every(time.Hour / time.Duration(qph)))
		qe.qphLimiter.SetBurst(qph)
	}
}

// MarkThrottled 触发限流熔断。
func (qe *QueueExecutor) MarkThrottled() {
	qe.throttleManager.MarkThrottled()
}
