// Package bark 实现 Bark 推送渠道。
//
// 渠道类型："bark"
// 凭证（ChannelConfig）：
//   - server：Bark 服务端地址（如 https://api.day.app）
//   - device_key：设备 key
//
// 参考：https://github.com/Finb/barker
package bark

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
)

const ChannelType = "bark"

// Channel Bark 渠道。
type Channel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(ChannelType, func() msgch.Channel {
		return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
			msgch.FormatText,
		})}
	})
}

// Bark 推送请求体。
// 参考 https://github.com/Finb/Bark#%E8%AF%B7%E6%B1%82%E5%8F%82%E6%95%B0
type request struct {
	// Title 通知标题
	Title string `json:"title"`
	// Body 通知正文内容
	Body string `json:"body"`
	// URL 点击通知后跳转的链接（可选）
	URL string `json:"url,omitempty"`
}

// Bark 推送响应。
type response struct {
	// Code 响应码，200 表示成功
	Code int `json:"code"`
	// Message 响应描述信息
	Message string `json:"message"`
}

func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	server := cfg.GetString("server")
	deviceKey := cfg.GetString("device_key")
	if server == "" || deviceKey == "" {
		return c.FailResult("missing server or device_key"), nil
	}

	_, body := msg.FormatContent([]msgch.Format{msgch.FormatText, msgch.FormatMarkdown, msgch.FormatHTML})

	req := request{
		Title: msg.Title,
		Body:  body,
		URL:   msg.URL,
	}
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/%s", server, deviceKey)
	resp, status, err := httpclient.PostText(ctx, url, "application/json", reqBody)
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
	if res.Code != 200 {
		return c.FailResult(fmt.Sprintf("code=%d message=%s", res.Code, res.Message)), nil
	}
	return c.SuccessResult(string(resp)), nil
}