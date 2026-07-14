package msgch

import (
	"context"
	"time"
)

// Client 预设默认配置的客户端，适合长期复用同一套凭证。
//
// 默认配置由调用方注入（库不读取配置文件）；发送时调用期 cfg 与默认配置 merge，
// 调用期覆盖默认。
type Client struct {
	defaults map[string]ChannelConfig
	opts     []SendOption
}

// New 构造 Client，opts 作为每次发送的默认选项（可被调用期 opts 覆盖）。
//
// params:
//   - opts：默认发送选项，如 WithRetry / WithTimeout；后续 Push/Broadcast 的调用期 opts 追加在后
//
// returns:
//   - *Client：可复用客户端，建议长驻复用同一套凭证
func New(opts ...SendOption) *Client {
	return &Client{
		defaults: make(map[string]ChannelConfig),
		opts:     opts,
	}
}

// SetDefaultConfig 设置某渠道的默认配置，返回 Client 便于链式调用。
// 发送时若调用期 cfg 为 nil 或某键为空，则回退到默认配置；调用期同键覆盖默认。
//
// params:
//   - channelType：渠道类型标识
//   - cfg：默认配置（KV）
//
// returns:
//   - *Client：自身，便于链式 SetDefaultConfig
func (c *Client) SetDefaultConfig(channelType string, cfg ChannelConfig) *Client {
	c.defaults[channelType] = cfg
	return c
}

// Push 发送消息。cfg 可为 nil（用默认配置）或覆盖默认配置（调用期优先）。
// opts 会与构造 Client 时的默认 opts 合并（调用期 opts 追加在后，覆盖靠后者生效）。
//
// params:
//   - ctx：超时/取消上下文
//   - channelType：渠道类型标识
//   - cfg：调用期配置（KV），nil 表示用默认配置；非 nil 时与默认配置 merge，同键调用期优先
//   - msg：统一消息模型
//   - opts：调用期发送选项，追加到默认 opts 之后
//
// returns:
//   - *Result：Success=false 表示发送失败（厂商错误码）；Err!=nil 表示系统错误
//   - error：系统错误透传
func (c *Client) Push(ctx context.Context, channelType string, cfg ChannelConfig, msg *Message, opts ...SendOption) (*Result, error) {
	merged := c.mergeConfig(channelType, cfg)
	allOpts := append(append([]SendOption{}, c.opts...), opts...)
	return doSend(ctx, channelType, merged, msg, allOpts)
}

// Broadcast 多渠道广播。targets 中每个目标的 Config 与默认配置 merge（调用期覆盖默认）。
// 并发发送，某渠道失败不影响其他渠道。
//
// params:
//   - ctx：超时/取消上下文
//   - targets：广播目标列表
//   - msg：统一消息模型
//   - opts：发送选项
//
// returns:
//   - *BroadcastResult：聚合各渠道发送明细
func (c *Client) Broadcast(ctx context.Context, targets []Target, msg *Message, opts ...SendOption) *BroadcastResult {
	allOpts := append(append([]SendOption{}, c.opts...), opts...)
	return doBroadcast(ctx, targets, msg, allOpts, c)
}

// mergeConfig 合并默认配置与调用期配置，调用期覆盖默认。
func (c *Client) mergeConfig(channelType string, cfg ChannelConfig) ChannelConfig {
	def, ok := c.defaults[channelType]
	if !ok {
		return cfg
	}
	merged := make(ChannelConfig, len(def)+len(cfg))
	for k, v := range def {
		merged[k] = v
	}
	for k, v := range cfg {
		merged[k] = v
	}
	return merged
}

// mergeConfigForTarget 供 doBroadcast 使用：按 target.Type 合并默认配置。
func (c *Client) mergeConfigForTarget(t Target) ChannelConfig {
	return c.mergeConfig(t.Type, t.Config)
}

// defaultClient 是包级别的默认客户端，供 Push/Broadcast 顶层函数复用。
var defaultClient = New()

// doSend 执行单次发送的内部流程：取渠道 → 应用超时 → 调 Send → 可选重试。
func doSend(ctx context.Context, channelType string, cfg ChannelConfig, msg *Message, opts []SendOption) (*Result, error) {
	ch, err := Get(channelType)
	if err != nil {
		return nil, err // 未知渠道 → 系统错误
	}
	sc := newSendConfig(opts)

	// 应用超时：用 ctx 派生带超时的子 context
	sendCtx := ctx
	if t := sc.effectiveTimeout(); t > 0 {
		var cancel context.CancelFunc
		sendCtx, cancel = context.WithTimeout(ctx, t)
		defer cancel()
	}

	// 首次发送
	result, err := ch.Send(sendCtx, cfg, msg)
	if err != nil {
		return nil, err // 系统错误（网络层等）
	}

	// 重试逻辑
	if !sc.shouldRetry(result) {
		return result, nil
	}
	policy := sc.retry
	delay := policy.BaseDelay
	if delay <= 0 {
		delay = time.Second
	}
	for attempt := 2; attempt <= policy.MaxAttempts; attempt++ {
		select {
		case <-sendCtx.Done():
			return result, sendCtx.Err()
		case <-time.After(delay):
		}
		result, err = ch.Send(sendCtx, cfg, msg)
		if err != nil {
			return nil, err
		}
		if !sc.shouldRetry(result) {
			return result, nil
		}
		// 指数退避
		delay *= 2
		if policy.MaxDelay > 0 && delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
	}
	return result, nil
}
