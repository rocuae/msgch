// Package discord 实现 Discord Webhook 渠道。
//
// 渠道类型："discord"
// 凭证（ChannelConfig）：
//   - webhook_url：Discord webhook 完整地址
//
// 参考：https://discord.com/developers/docs/reference#message-formatting
package discord

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
)

const ChannelType = "discord"

// Channel Discord webhook 渠道。
type Channel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(ChannelType, func() msgch.Channel {
		return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
			msgch.FormatMarkdown, msgch.FormatText,
		})}
	})
}

// Discord Webhook 请求体。
// 参考 https://discord.com/developers/docs/resources/webhook
type request struct {
	// Content 消息内容（支持 Discord Markdown 语法）
	Content string `json:"content"`
}

// Discord Webhook 响应。
type response struct {
	// Code 错误码
	Code int `json:"code"`
	// Message 错误描述
	Message string `json:"message"`
}

func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	url := cfg.GetString("webhook_url")
	if url == "" {
		return c.FailResult("missing webhook_url"), nil
	}

	_, content := msg.FormatContent([]msgch.Format{msgch.FormatMarkdown, msgch.FormatText})

	// Discord @ 语法：<@userid>
	prefix := ""
	if msg.At != nil {
		if msg.At.AtAll {
			prefix = "@everyone "
		}
		for _, id := range msg.At.UserIDs {
			prefix += fmt.Sprintf("<@%s> ", id)
		}
	}

	req := request{Content: prefix + content}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, status, err := httpclient.PostText(ctx, url, "application/json", body)
	if err != nil {
		return nil, err
	}
	// Discord 成功返回 204 No Content（无 body）
	if status == 204 {
		return c.SuccessResult("ok"), nil
	}

	var res response
	if jerr := json.Unmarshal(resp, &res); jerr != nil {
		if status >= 200 && status < 300 {
			return c.SuccessResult(string(resp)), nil
		}
		return c.FailResult(fmt.Sprintf("HTTP %d: %s", status, string(resp))), nil
	}
	if res.Code != 0 {
		return c.FailResult(fmt.Sprintf("code=%d message=%s", res.Code, res.Message)), nil
	}
	return c.SuccessResult(string(resp)), nil
}