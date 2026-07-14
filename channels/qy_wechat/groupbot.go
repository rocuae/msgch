// Package qy_wechat 实现企业微信群机器人渠道。
//
// 渠道类型："qy_wechat"
// 凭证（ChannelConfig）：
//   - key：群机器人 webhook 的 key
//     （或 webhook_url 传完整地址）
//
// 参考：https://developer.work.weixin.qq.com/document/path/91770
package qy_wechat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
)

// 渠道类型标识。
const ChannelType = "qy_wechat"

// Channel 企业微信群机器人渠道。
type Channel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(ChannelType, func() msgch.Channel {
		return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
			msgch.FormatMarkdown, msgch.FormatText,
		})}
	})
}

// 企微群机器人请求体。
// 参考 https://developer.work.weixin.qq.com/document/path/91770
type request struct {
	// MsgType 消息类型：text / markdown
	MsgType string `json:"msgtype"`
	// Text 纯文本消息体（msgtype=text 时）
	Text textBody `json:"text,omitempty"`
	// Markdown Markdown 消息体（msgtype=markdown 时）
	Markdown mdBody `json:"markdown,omitempty"`
	// MentionedList @指定用户的 userid 列表
	MentionedList []string `json:"mentioned_list,omitempty"`
	// MentionedMobiles @指定用户的手机号列表
	MentionedMobiles []string `json:"mentioned_mobile_list,omitempty"`
}

// textBody 纯文本消息体。
type textBody struct {
	// Content 消息内容
	Content string `json:"content"`
}

// mdBody Markdown 消息体。
type mdBody struct {
	// Content Markdown 内容
	Content string `json:"content"`
}

// 企微群机器人响应。
type response struct {
	// ErrCode 错误码，0 表示成功
	ErrCode int `json:"errcode"`
	// ErrMsg 错误信息
	ErrMsg string `json:"errmsg"`
}

// Send 实现 msgch.Channel。
func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	url := cfg.GetString("webhook_url")
	if url == "" {
		key := cfg.GetString("key")
		if key == "" {
			return c.FailResult("missing webhook_url or key"), nil
		}
		url = fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=%s", key)
	}

	format, content := msg.FormatContent([]msgch.Format{msgch.FormatMarkdown, msgch.FormatText})

	req := request{}
	switch format {
	case msgch.FormatMarkdown:
		req.MsgType = "markdown"
		req.Markdown = mdBody{Content: content}
	default:
		req.MsgType = "text"
		req.Text = textBody{Content: content}
	}

	// @提醒：企微 text 用 mentioned_list/mentioned_mobile_list；markdown 不支持
	if msg.At != nil && format == msgch.FormatText {
		if msg.At.AtAll {
			req.MentionedList = []string{"@all"}
		} else {
			req.MentionedList = msg.At.UserIDs
			req.MentionedMobiles = msg.At.Mobiles
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, status, err := httpclient.PostText(ctx, url, "application/json", body)
	if err != nil {
		return nil, err
	}

	var res response
	if jerr := json.Unmarshal(resp, &res); jerr != nil {
		if status >= 200 && status < 300 {
			return c.SuccessResult(string(resp)), nil
		}
		return c.FailResult(fmt.Sprintf("HTTP %d: %s", status, string(resp))), nil
	}
	if res.ErrCode != 0 {
		return c.FailResult(fmt.Sprintf("errcode=%d errmsg=%s", res.ErrCode, res.ErrMsg)), nil
	}
	return c.SuccessResult(string(resp)), nil
}