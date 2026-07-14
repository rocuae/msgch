package msgch

import "context"

// Push 单次发送的统一入口（使用包级默认 Client）。
//
// 调用方提供渠道类型、凭证与消息，库内部完成"取渠道 → 应用选项 → 发送 → 可选重试"全流程。
// 调用方无需关心渠道实例化与注册细节，只需在程序启动时空白导入对应渠道子包。
//
// params:
//   - ctx：超时/取消上下文
//   - channelType：渠道类型标识（如 "dingtalk"、"feishu"、"qy_wechat_app"）
//   - cfg：渠道认证配置（KV），由调用方提供；nil 表示无配置
//   - msg：统一消息模型（Title 与 Text/HTML/Markdown 等载体）
//   - opts：发送选项（WithTimeout / WithRetry / WithRateLimit），可选
//
// returns:
//   - *Result：Success=true 表示厂商返回成功；Success=false 表示发送失败（厂商错误码）；Err!=nil 表示系统错误
//   - error：仅系统错误（网络中断、配置缺失、未知渠道等），发送失败不通过 error 返回
//
// 示例：
//
//	result, err := msgch.Push(ctx, "dingtalk",
//	    msgch.ChannelConfig{"access_token": "xxx", "secret": "SECxxx"},
//	    &msgch.Message{Title: "告警", Markdown: "**CPU 90%**"})
func Push(ctx context.Context, channelType string, cfg ChannelConfig, msg *Message, opts ...SendOption) (*Result, error) {
	return defaultClient.Push(ctx, channelType, cfg, msg, opts...)
}

// Broadcast 多渠道广播：同一条消息发给多个渠道，并发发送并聚合各渠道 Result。
// 使用包级默认 Client。某渠道失败不影响其他渠道发送。
//
// params:
//   - ctx：超时/取消上下文
//   - targets：广播目标列表，每个目标含渠道类型与配置
//   - msg：统一消息模型
//   - opts：发送选项，对所有目标生效
//
// returns:
//   - *BroadcastResult：含各渠道发送明细，顺序与 targets 一致；可通过 SuccessCount() 统计成功数
func Broadcast(ctx context.Context, targets []Target, msg *Message, opts ...SendOption) *BroadcastResult {
	return defaultClient.Broadcast(ctx, targets, msg, opts...)
}
