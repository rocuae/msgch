// Package wxpusher 实现 WxPusher 微信推送渠道。
//
// 渠道类型："wxpusher"
// 凭证（ChannelConfig）：
//   - app_token：应用 AppToken
//   - topic_ids：可选，主题 ID 列表（逗号分隔）
//   - uids：可选，用户 UID 列表（逗号分隔）
//   - content_type：可选，内容类型（1=文本 默认，2=html，3=markdown）
//
// 参考：https://wxpusher.zjiecode.com/docs/#/?id=%e5%8f%91%e9%80%81%e6%b6%88%e6%81%af
package wxpusher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
)

const ChannelType = "wxpusher"

const apiURL = "https://wxpusher.zjiecode.com/api/send/message"

// Channel WxPusher 渠道。
type Channel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(ChannelType, func() msgch.Channel {
		return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
			msgch.FormatMarkdown, msgch.FormatHTML, msgch.FormatText,
		})}
	})
}

// WxPusher 发送消息请求体。
// 参考 https://wxpusher.zjiecode.com/docs/#/?id=%e5%8f%91%e9%80%81%e6%b6%88%e6%81%af
type request struct {
	// AppToken 应用 AppToken
	AppToken string `json:"appToken"`
	// Content 消息内容
	Content string `json:"content"`
	// Summary 消息摘要（列表预览），可选
	Summary string `json:"summary,omitempty"`
	// ContentType 内容类型：1=文本，2=HTML，3=Markdown
	ContentType int `json:"contentType"`
	// TopicIDs 主题 ID 列表（推送给订阅该主题的所有用户）
	TopicIDs []int `json:"topicIds,omitempty"`
	// UIDs 接收人 UID 列表
	UIDs []string `json:"uids,omitempty"`
	// URL 消息链接（可选）
	URL string `json:"url,omitempty"`
}

// WxPusher 响应。
type response struct {
	// Code 响应码，1000 表示成功
	Code int `json:"code"`
	// Msg 响应描述
	Msg string `json:"msg"`
	// Success 是否成功
	Success bool `json:"success"`
}

func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	token := cfg.GetString("app_token")
	if token == "" {
		return c.FailResult("missing app_token"), nil
	}

	format, content := msg.FormatContent([]msgch.Format{msgch.FormatMarkdown, msgch.FormatHTML, msgch.FormatText})

	req := request{
		AppToken: token,
		Content:  content,
		Summary:  msg.Title,
	}
	switch format {
	case msgch.FormatMarkdown:
		req.ContentType = 3
	case msgch.FormatHTML:
		req.ContentType = 2
	default:
		req.ContentType = 1 // 文本
	}

	if s := cfg.GetString("topic_ids"); s != "" {
		req.TopicIDs = parseInts(s)
	}
	if s := cfg.GetString("uids"); s != "" {
		req.UIDs = strings.Split(s, ",")
	}
	if msg.URL != "" {
		req.URL = msg.URL
	}

	resp, status, err := httpclient.PostJSON(ctx, apiURL, req)
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
	if res.Code != 1000 || !res.Success {
		return c.FailResult(fmt.Sprintf("code=%d msg=%s", res.Code, res.Msg)), nil
	}
	return c.SuccessResult(string(resp)), nil
}

// parseInts 解析逗号分隔的整数列表。
func parseInts(s string) []int {
	parts := strings.Split(s, ",")
	var out []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err == nil {
			out = append(out, n)
		}
	}
	return out
}