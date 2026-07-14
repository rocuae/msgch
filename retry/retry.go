// Package retry 提供指数退避重试工具。
package retry

import (
	"context"
	"time"
)

// Policy 重试策略。
type Policy struct {
	// MaxAttempts 最大尝试次数（含首次）
	MaxAttempts int
	// BaseDelay 退避基准
	BaseDelay time.Duration
	// MaxDelay 退避上限
	MaxDelay time.Duration
}

// Do 执行 fn 直到返回 ok=true 或达到 MaxAttempts。
// 每次失败后按指数退避等待（base*2^(n-1)，上限 maxDelay）。ctx 取消会提前返回。
func Do(ctx context.Context, p Policy, fn func(attempt int) (ok bool, err error)) error {
	var lastErr error
	delay := p.BaseDelay
	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		ok, err := fn(attempt)
		if err != nil {
			lastErr = err
		}
		if ok {
			return nil
		}
		if attempt == p.MaxAttempts {
			break
		}
		if delay <= 0 {
			delay = time.Second
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
		if p.MaxDelay > 0 && delay > p.MaxDelay {
			delay = p.MaxDelay
		}
	}
	return lastErr
}