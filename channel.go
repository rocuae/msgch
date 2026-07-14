// Package msgch 是一个 Go 实现的统一消息推送纯库。
//
// msgch 提供统一的消息入口（Push/Broadcast），支持多种消息渠道（钉钉、飞书、
// 企业微信、Bark、Telegram、Discord 等）的可插拔扩展。库本身不实现 CLI、HTTP
// 服务与配置读取，这些职责由调用方实现。
//
// 快速开始：
//
//	import (
//	    "github.com/<owner>/msgch"
//	    _ "github.com/<owner>/msgch/channels" // 空白导入注册全部内置渠道
//	)
//
//	result, err := msgch.Push(ctx, "dingtalk", msgch.ChannelConfig{
//	    "access_token": "xxxx",
//	    "secret":       "SECxxxx",
//	}, &msgch.Message{Title: "标题", Markdown: "**内容**"})
package msgch

import "context"

// Format 内容载体类型，按优先级排列。
type Format string

const (
	// FormatMarkdown Markdown 载体（钉钉/飞书/Telegram/Discord 等支持）
	FormatMarkdown Format = "markdown"
	// FormatHTML HTML 载体（邮件、部分平台支持）
	FormatHTML Format = "html"
	// FormatText 纯文本载体（所有渠道兜底）
	FormatText Format = "text"
)

// Channel 渠道接口 —— 新增渠道只需实现此接口。
//
// 实现要点：
//   - Type 返回全局唯一的渠道类型标识（如 "dingtalk"、"email"）。
//   - Formats 返回支持的内容格式，按优先级降序；FormatContent 据此挑选。
//   - Send 执行发送；系统错误返回 Go error，发送失败返回 *Result{Success:false}。
type Channel interface {
	// Type 渠道类型标识，全局唯一。
	Type() string
	// Formats 返回支持的内容格式，按优先级降序。
	Formats() []Format
	// Send 发送消息。
	//   ctx：超时/取消；cfg：渠道认证配置（KV）；msg：统一消息模型。
	//   返回 *Result：Success=false 表示发送失败（厂商错误码），Err!=nil 表示系统错误。
	Send(ctx context.Context, cfg ChannelConfig, msg *Message) (*Result, error)
}

// BaseChannel 渠道嵌入基类，提供 Type/Formats 的默认实现与内容格式选择、
// 结果构造的辅助方法。新增渠道嵌入 *BaseChannel 后通常只需实现 Send。
//
// 用法：
//
//	type Channel struct{ *msgch.BaseChannel }
//	func init() {
//	    msgch.Register("dingtalk", func() msgch.Channel {
//	        return &Channel{BaseChannel: msgch.NewBase("dingtalk", []msgch.Format{
//	            msgch.FormatMarkdown, msgch.FormatText,
//	        })}
//	    })
//	}
type BaseChannel struct {
	typ     string
	formats []Format
}

// NewBase 构造 BaseChannel。
func NewBase(typ string, formats []Format) *BaseChannel {
	return &BaseChannel{typ: typ, formats: formats}
}

// Type 返回渠道类型标识。
func (b *BaseChannel) Type() string { return b.typ }

// Formats 返回支持的内容格式（按优先级降序）。
func (b *BaseChannel) Formats() []Format { return b.formats }

// FormatContent 按渠道声明的格式优先级，从 msg 中选第一个非空载体，兜底 Text。
// 返回选中的格式与对应内容。
func (b *BaseChannel) FormatContent(msg *Message) (Format, string) {
	return msg.FormatContent(b.formats)
}

// SuccessResult 构造成功结果。
func (b *BaseChannel) SuccessResult(response string) *Result {
	return &Result{Success: true, Response: response}
}

// FailResult 构造发送失败结果（厂商错误码等，非系统错误）。
func (b *BaseChannel) FailResult(response string) *Result {
	return &Result{Success: false, Response: response}
}
