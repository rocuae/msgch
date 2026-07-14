// Package webhook 实现自定义 Webhook 渠道。
//
// 渠道类型："webhook"
// 凭证（ChannelConfig）：
//   - url：目标 URL（必须 HTTPS；HTTP 仅当 allow_http=true 时允许）
//   - method：可选，HTTP 方法，默认 POST
//   - headers：可选，JSON 格式 header map
//   - body_template：可选，请求体模板（支持 $title/$text/$markdown/$url 占位符替换）
//   - content_type：可选，默认 application/json
//
// 这是一个模板化 webhook 渠道，调用方可自定义 body 结构，适配任意第三方接收端。
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
)

const ChannelType = "webhook"

// Channel 自定义 webhook 渠道。
type Channel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(ChannelType, func() msgch.Channel {
		return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
			msgch.FormatText,
		})}
	})
}

func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	target := cfg.GetString("url")
	if target == "" {
		return c.FailResult("missing url"), nil
	}
	// 防止 SSRF：禁止非 HTTP(S)；HTTP 需显式允许
	parsed, err := url.Parse(target)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return c.FailResult("invalid url scheme"), nil
	}
	if parsed.Scheme == "http" && cfg.GetString("allow_http") != "true" {
		return c.FailResult("http url is not allowed unless allow_http=true"), nil
	}

	method := cfg.GetString("method")
	if method == "" {
		method = http.MethodPost
	}
	contentType := cfg.GetString("content_type")
	if contentType == "" {
		contentType = "application/json"
	}

	_, text := msg.FormatContent([]msgch.Format{msgch.FormatText, msgch.FormatMarkdown, msgch.FormatHTML})

	// 渲染 body 模板
	body := cfg.GetString("body_template")
	body = strings.ReplaceAll(body, "$title", msg.Title)
	body = strings.ReplaceAll(body, "$text", text)
	body = strings.ReplaceAll(body, "$markdown", msg.Markdown)
	body = strings.ReplaceAll(body, "$html", msg.HTML)
	body = strings.ReplaceAll(body, "$url", msg.URL)

	// 若无模板，默认构造一个 JSON body
	if body == "" {
		defaultBody := map[string]string{
			"title": msg.Title,
			"text":  text,
			"url":   msg.URL,
		}
		buf, err := json.Marshal(defaultBody)
		if err != nil {
			return nil, err
		}
		body = string(buf)
	}

	// 构造请求
	req, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	// 自定义 headers
	if h := cfg.GetString("headers"); h != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(h), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}

	respBytes, status, err := httpclient.Do(req)
	if err != nil {
		return nil, err
	}

	if status >= 200 && status < 300 {
		return c.SuccessResult(fmt.Sprintf("HTTP %d: %s", status, string(respBytes))), nil
	}
	return c.FailResult(fmt.Sprintf("HTTP %d: %s", status, string(respBytes))), nil
}