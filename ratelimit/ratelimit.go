// Package ratelimit 提供令牌桶限流器。
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter 令牌桶限流器，按 QPS 限流。
type Limiter struct {
	mu         sync.Mutex
	rate       float64       // 每秒令牌
	burst      int           // 桶容量
	tokens     float64       // 当前令牌
	last       time.Time     // 上次补充时间
	refillEvery time.Duration // 测试用：可覆盖时间推进
}

// New 创建限流器。qps<=0 表示不限流（Take 立即返回）。burst 为桶容量，<=0 时取 int(qps)。
func New(qps float64, burst int) *Limiter {
	if qps <= 0 {
		return nil
	}
	if burst <= 0 {
		burst = int(qps)
		if burst < 1 {
			burst = 1
		}
	}
	return &Limiter{
		rate:    qps,
		burst:   burst,
		tokens:  float64(burst),
		last:    time.Now(),
	}
}

// Take 阻塞等待获取一个令牌，直到成功或 ctx 取消。
// nil Limiter（未限流）立即返回 nil。
func (l *Limiter) Take(ctx context.Context) error {
	if l == nil {
		return nil
	}
	for {
		l.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(l.last).Seconds()
		l.tokens += elapsed * l.rate
		if l.tokens > float64(l.burst) {
			l.tokens = float64(l.burst)
		}
		l.last = now
		if l.tokens >= 1 {
			l.tokens -= 1
			l.mu.Unlock()
			return nil
		}
		// 计算等待时间：补到 1 个令牌所需
		need := 1 - l.tokens
		wait := time.Duration(need / l.rate * float64(time.Second))
		l.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}