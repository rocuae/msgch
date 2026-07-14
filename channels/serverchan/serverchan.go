// Package serverchan 实现 Server酱 Turbo 和 Server酱³ 推送渠道。
//
// 渠道类型："serverchan"
// 凭证（ChannelConfig）：
//   - sendkey：Server 酱 SendKey
//   - Turbo 版（sct 开头）走 https://sctapi.ftqq.com/{key}.send
//   - Server酱³（sctp{uid}t 开头）走 https://{uid}.push.ft07.com/send/{key}.send
//   - api_url：可选，自定义完整地址
//
// 参考：https://sct.ftqq.com/ 和 https://doc.sc3.ft07.com/zh/serverchan3
package serverchan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
)

const ChannelType = "serverchan"

var serverChan3SendKeyPattern = regexp.MustCompile(`^sctp(\d+)t`)

// Channel Server 酱渠道。
type Channel struct{ *msgch.BaseChannel }

// init 注册 Server酱渠道工厂。
func init() {
	msgch.Register(ChannelType, func() msgch.Channel {
		return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
			msgch.FormatMarkdown, msgch.FormatText,
		})}
	})
}

// Server 酱响应。
type response struct {
	// Code 错误码，0 表示成功
	Code int `json:"code"`
	// Msg 错误信息
	Msg string `json:"message"`
}

// Send 根据配置选择 Server酱 Turbo 或 Server酱³ 接口并发送消息。
// Server酱³ 的 tags 和 short 参数可通过 msg.Extra 传入。
func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	key := cfg.GetString("sendkey")
	apiURL := cfg.GetString("api_url")
	if key == "" && apiURL == "" {
		return c.FailResult("missing sendkey or api_url"), nil
	}
	if apiURL == "" {
		var err error
		apiURL, err = resolveAPIURL(key)
		if err != nil {
			return c.FailResult(err.Error()), nil
		}
	}

	_, text := msg.FormatContent([]msgch.Format{msgch.FormatMarkdown, msgch.FormatText})

	form := url.Values{}
	form.Set("title", msg.Title)
	form.Set("desp", text)
	if msg.Extra != nil {
		if tags, ok := msg.Extra["tags"].(string); ok && tags != "" {
			form.Set("tags", tags)
		}
		if short, ok := msg.Extra["short"].(string); ok && short != "" {
			form.Set("short", short)
		}
	}

	resp, status, err := httpclient.PostText(ctx, apiURL, "application/x-www-form-urlencoded", []byte(form.Encode()))
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
	if res.Code != 0 {
		return c.FailResult(fmt.Sprintf("code=%d message=%s", res.Code, res.Msg)), nil
	}
	return c.SuccessResult(string(resp)), nil
}

// resolveAPIURL 根据 SendKey 选择 Turbo 或 Server酱³ 接口地址。
func resolveAPIURL(key string) (string, error) {
	if strings.HasPrefix(key, "sctp") {
		match := serverChan3SendKeyPattern.FindStringSubmatch(key)
		if len(match) != 2 {
			return "", fmt.Errorf("invalid Server酱³ sendkey")
		}
		return fmt.Sprintf("https://%s.push.ft07.com/send/%s.send", match[1], key), nil
	}
	return fmt.Sprintf("https://sctapi.ftqq.com/%s.send", key), nil
}
