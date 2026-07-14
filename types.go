package msgch

import "time"

// ChannelConfig 渠道认证配置，KV 形式。
//
// 各渠道自定义字段名（如钉钉的 "access_token"、"secret"）通过 GetString/Get
// 安全取值。不为每个渠道定义专属 struct，换取接口极简与扩展零成本。
type ChannelConfig map[string]string

// GetString 取字符串字段，空安全（键不存在或空串都返回 ""）。
func (c ChannelConfig) GetString(key string) string {
	if c == nil {
		return ""
	}
	return c[key]
}

// GetOrDefault 取字符串字段，为空时返回默认值。
func (c ChannelConfig) GetOrDefault(key, def string) string {
	if v := c.GetString(key); v != "" {
		return v
	}
	return def
}

// AtInfo @提醒信息（钉钉/企微/飞书等用）。
type AtInfo struct {
	// Mobiles @手机号列表
	Mobiles []string
	// UserIDs @用户ID列表（钉钉 atUserIds、飞书 user_id 等）
	UserIDs []string
	// AtAll @所有人
	AtAll bool
}

// Message 统一消息模型。
//
// 设计要点：四载体（Text/HTML/Markdown）并存，渠道按 Formats() 自选最优、
// 兜底 Text；Extra 透传短信模板变量、渠道专属参数等异构字段。
type Message struct {
	// Title 标题（邮件 subject、Bark title、模板消息首行等）
	Title string
	// Text 纯文本载体
	Text string
	// HTML HTML 载体（邮件、部分平台）
	HTML string
	// Markdown Markdown 载体（钉钉/飞书/Telegram/Discord 等）
	Markdown string
	// URL 详情跳转链接（模板消息、外链富文本）
	URL string
	// ImageURL 图片链接（部分渠道）
	ImageURL string

	// At @提醒信息
	At *AtInfo
	// Extra 透传字段：短信模板变量、渠道专属参数等
	Extra map[string]any
}

// FormatContent 按渠道声明的格式优先级，从 msg 中选第一个非空载体，兜底 Text。
// 返回选中的格式与对应内容。
//
// 例如渠道 Formats()=[Markdown, Text]：
//   - 若 msg.Markdown 非空 → 返回 (FormatMarkdown, msg.Markdown)
//   - 否则若 msg.Text 非空 → 返回 (FormatText, msg.Text)
//   - 否若 msg.HTML 非空（渠道未声明 HTML 则跳过）→ 兜底 Text
//   - 全空 → 返回 (FormatText, "")
func (m *Message) FormatContent(formats []Format) (Format, string) {
	for _, f := range formats {
		switch f {
		case FormatMarkdown:
			if m.Markdown != "" {
				return FormatMarkdown, m.Markdown
			}
		case FormatHTML:
			if m.HTML != "" {
				return FormatHTML, m.HTML
			}
		case FormatText:
			if m.Text != "" {
				return FormatText, m.Text
			}
		}
	}
	// 全部声明格式都未命中非空，兜底：按 Text/Markdown/HTML 顺序取第一个非空
	if m.Text != "" {
		return FormatText, m.Text
	}
	if m.Markdown != "" {
		return FormatMarkdown, m.Markdown
	}
	if m.HTML != "" {
		return FormatHTML, m.HTML
	}
	return FormatText, ""
}

// Result 发送结果。
//
// 判断准则：
//   - Err != nil              → 系统错误（网络中断、配置缺失、未知渠道等），调用方应记录/告警
//   - Err == nil && !Success   → 发送失败（厂商错误码），可重试
//   - Err == nil && Success    → 成功
type Result struct {
	// Success 厂商是否返回成功
	Success bool
	// Response 厂商原始响应（便于排查）
	Response string
	// Err 仅系统级错误（网络中断、配置缺失、未知渠道等）；发送失败不写入 Err
	Err error
}

// Target 广播目标。
type Target struct {
	// Type 渠道类型标识（如 "dingtalk"）
	Type string
	// Config 渠道认证配置
	Config ChannelConfig
}

// BroadcastResult 多渠道广播聚合结果。
type BroadcastResult struct {
	// Results 各渠道发送明细，顺序与传入 targets 一致
	Results []TargetResult
}

// TargetResult 单个广播目标的发送结果。
type TargetResult struct {
	// Type 渠道类型标识
	Type string
	// Result 发送结果（可能为 nil，表示该渠道未注册等系统错误）
	Result *Result
	// Err 系统错误（渠道未注册、参数校验失败等）
	Err error
}

// SuccessCount 统计成功的渠道数。
func (b *BroadcastResult) SuccessCount() int {
	n := 0
	for _, r := range b.Results {
		if r.Err == nil && r.Result != nil && r.Result.Success {
			n++
		}
	}
	return n
}

// RetryPolicy 重试策略。
type RetryPolicy struct {
	// MaxAttempts 最大尝试次数（含首次），<=1 表示不重试
	MaxAttempts int
	// BaseDelay 退避基准
	BaseDelay time.Duration
	// MaxDelay 退避上限
	MaxDelay time.Duration
	// Jitter 是否加抖动（避免惊群）
	Jitter bool
	// Retryable 判断哪些发送失败可重试；nil 表示所有 Success=false 都重试
	Retryable func(*Result) bool
}

// RateLimit 限流策略。
type RateLimit struct {
	// QPS 每秒最大请求数，<=0 表示不限流
	QPS float64
	// Burst 突发上限
	Burst int
}
