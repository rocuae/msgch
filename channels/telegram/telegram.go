// Package telegram 实现 Telegram Bot 渠道。
//
// 渠道类型："telegram"
// 凭证（ChannelConfig）：
//   - bot_token：Bot Token
//   - chat_id：默认聊天 ID（可被 msg.To 覆盖）
//   - api_host：可选，自定义 API 地址（用于代理）
//
// 参考：https://core.telegram.org/bots/api#sendmessage
package telegram

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
)

const ChannelType = "telegram"

const defaultAPIHost = "https://api.telegram.org"

// Telegram 消息长度上限，超长需分段。
const maxMessageLength = 4096

// Channel Telegram Bot 渠道。
type Channel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(ChannelType, func() msgch.Channel {
		return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
			msgch.FormatMarkdown, msgch.FormatHTML, msgch.FormatText,
		})}
	})
}

// Telegram Bot sendMessage 请求体。
// 参考 https://core.telegram.org/bots/api#sendmessage
type request struct {
	// ChatID 接收消息的 chat_id（个人/群组/频道）
	ChatID string `json:"chat_id"`
	// Text 消息内容
	Text string `json:"text"`
	// ParseMode 解析模式：Markdown / HTML，空表示纯文本
	ParseMode string `json:"parse_mode,omitempty"`
}

// Telegram Bot API 响应。
type response struct {
	// Ok 是否成功
	Ok bool `json:"ok"`
	// Description 错误描述（ok=false 时）
	Description string `json:"description,omitempty"`
}

func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	token := cfg.GetString("bot_token")
	chatID := cfg.GetString("chat_id")
	if token == "" || chatID == "" {
		return c.FailResult("missing bot_token or chat_id"), nil
	}

	host := cfg.GetString("api_host")
	if host == "" {
		host = defaultAPIHost
	}

	format, text := msg.FormatContent([]msgch.Format{msgch.FormatMarkdown, msgch.FormatHTML, msgch.FormatText})

	req := request{ChatID: chatID, Text: text}
	switch format {
	case msgch.FormatMarkdown:
		req.ParseMode = "Markdown"
	case msgch.FormatHTML:
		req.ParseMode = "HTML"
	default:
		// 纯文本不设 parse_mode
	}

	// 分段发送
	segments := splitText(req.Text, maxMessageLength)
	for i, seg := range segments {
		req.Text = seg
		body, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		url := fmt.Sprintf("%s/bot%s/sendMessage", host, token)
		resp, status, err := httpclient.PostText(ctx, url, "application/json", body)
		if err != nil {
			return nil, err
		}
		// 仅末段返回结果
		if i < len(segments)-1 {
			continue
		}
		var res response
		if jerr := json.Unmarshal(resp, &res); jerr != nil {
			if status >= 200 && status < 300 {
				return c.SuccessResult(string(resp)), nil
			}
			return c.FailResult(fmt.Sprintf("HTTP %d: %s", status, string(resp))), nil
		}
		if !res.Ok {
			return c.FailResult(res.Description), nil
		}
	}
	return c.SuccessResult("ok"), nil
}

// splitText 按上限分段文本（按 UTF-8 边界）。
func splitText(s string, limit int) []string {
	if len(s) <= limit {
		return []string{s}
	}
	var segs []string
	for len(s) > limit {
		// 简单按字节切，回退到 UTF-8 边界
		end := limit
		for end > 0 && (s[end]&0xC0) == 0x80 {
			end--
		}
		segs = append(segs, s[:end])
		s = s[end:]
	}
	if len(s) > 0 {
		segs = append(segs, s)
	}
	return segs
}