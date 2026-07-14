package msgch

import "time"

// SendOption 发送选项，用于 Push/Broadcast 调用时覆盖默认行为。
type SendOption func(*sendConfig)

type sendConfig struct {
	timeout   time.Duration
	retry     *RetryPolicy
	rateLimit *RateLimit
}

// WithTimeout 设置单次发送超时（覆盖渠道默认）。
func WithTimeout(d time.Duration) SendOption {
	return func(c *sendConfig) { c.timeout = d }
}

// WithRetry 设置重试策略。传空指针或 MaxAttempts<=1 表示关闭重试。
func WithRetry(p RetryPolicy) SendOption {
	return func(c *sendConfig) {
		cp := p
		c.retry = &cp
	}
}

// WithRateLimit 设置限流策略。QPS<=0 表示不限流。
func WithRateLimit(p RateLimit) SendOption {
	return func(c *sendConfig) {
		cp := p
		c.rateLimit = &cp
	}
}

// newSendConfig 构造发送配置并应用 opts。
func newSendConfig(opts []SendOption) *sendConfig {
	c := &sendConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// effectiveTimeout 返回生效超时，0 表示用渠道默认。
func (c *sendConfig) effectiveTimeout() time.Duration { return c.timeout }

// shouldRetry 判断某次发送结果是否应重试。
func (c *sendConfig) shouldRetry(r *Result) bool {
	if c.retry == nil || c.retry.MaxAttempts <= 1 {
		return false
	}
	if r == nil || r.Err != nil {
		// 系统错误不重试（网络层失败由调用方/上层决定）
		return false
	}
	if r.Success {
		return false
	}
	if c.retry.Retryable != nil {
		return c.retry.Retryable(r)
	}
	// 默认所有发送失败都重试
	return true
}
