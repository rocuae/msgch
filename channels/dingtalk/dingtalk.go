// Package dingtalk 实现钉钉群机器人渠道。
//
// 渠道类型："dingtalk"
// 凭证（ChannelConfig）：
//   - access_token：机器人 webhook 的 access_token
//   - secret：可选，加签密钥（SEC 开头）
//
// 参考：https://open.dingtalk.com/document/robots/custom-robot-access
package dingtalk

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
)

// 渠道类型标识。
const ChannelType = "dingtalk"

// Channel 钉钉群机器人渠道。
type Channel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(ChannelType, func() msgch.Channel {
		return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
			msgch.FormatMarkdown, msgch.FormatText,
		})}
	})
}

// 钉钉机器人请求体。
// 钉钉群机器人请求体。
// 参考 https://open.dingtalk.com/document/robots/custom-robot-access
type request struct {
	// MsgType 消息类型：text / markdown
	MsgType string `json:"msgtype"`
	// Text 纯文本消息体（msgtype=text 时）
	Text textBody `json:"text"`
	// Markdown Markdown 消息体（msgtype=markdown 时）
	Markdown mdBody `json:"markdown"`
	// At @提醒配置
	At atBody `json:"at"`
}

// textBody 纯文本消息体。
type textBody struct {
	// Content 消息内容
	Content string `json:"content"`
}

// mdBody Markdown 消息体。
type mdBody struct {
	// Title 标题（第一行显示）
	Title string `json:"title"`
	// Text Markdown 正文内容
	Text string `json:"text"`
}

// atBody @提醒配置。
type atBody struct {
	// AtUserIds @指定用户的 userId 列表
	AtUserIds []string `json:"atUserIds,omitempty"`
	// IsAtAll 是否 @所有人
	IsAtAll bool `json:"isAtAll,omitempty"`
}

// 钉钉群机器人响应。
type response struct {
	// ErrCode 错误码，0 表示成功
	ErrCode int `json:"errcode"`
	// ErrMsg 错误信息
	ErrMsg string `json:"errmsg"`
}

// Send 实现 msgch.Channel。
func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	token := cfg.GetString("access_token")
	webhookURL := cfg.GetString("webhook_url")
	if token == "" && webhookURL == "" {
		return c.FailResult("missing access_token or webhook_url"), nil // 配置缺失 → 发送失败
	}

	// 组装请求体
	reqBody := buildRequest(msg)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// 组装 URL：优先 webhook_url（便于测试/自建网关），否则官方 oapi + access_token
	var u string
	if webhookURL != "" {
		u = webhookURL
	} else {
		u = fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s", token)
	}
	if secret := cfg.GetString("secret"); secret != "" {
		ts := time.Now().UnixMilli()
		sign, err := sign(secret, ts)
		if err != nil {
			return nil, err
		}
		sep := "&"
		if !strings.Contains(u, "?") {
			sep = "?"
		}
		u = fmt.Sprintf("%s%stimestamp=%d&sign=%s", u, sep, ts, sign)
	}

	resp, status, err := httpclient.PostText(ctx, u, "application/json", body)
	if err != nil {
		return nil, err // 网络层失败 → 系统错误
	}

	// 翻译厂商响应
	var res response
	if jerr := json.Unmarshal(resp, &res); jerr != nil {
		// 非 JSON 响应，按 HTTP 状态判定
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

// buildRequest 按消息内容格式组装钉钉请求体。
func buildRequest(msg *msgch.Message) request {
	format, content := msg.FormatContent([]msgch.Format{msgch.FormatMarkdown, msgch.FormatText})

	req := request{}
	switch format {
	case msgch.FormatMarkdown:
		req.MsgType = "markdown"
		req.Markdown = mdBody{Title: msg.Title, Text: content}
	default: // text
		req.MsgType = "text"
		req.Text = textBody{Content: content}
	}

	// @提醒
	if msg.At != nil {
		if msg.At.AtAll {
			req.At.IsAtAll = true
		} else if len(msg.At.UserIDs) > 0 {
			req.At.AtUserIds = msg.At.UserIDs
		} else if len(msg.At.Mobiles) > 0 {
			// 钉钉群机器人不支持手机号 @，仅支持 userId；手机号忽略
		}
	}
	return req
}

// sign 钉钉加签：HMAC-SHA256(secret, "{timestamp_ms}\n{secret}") → base64 → URL encode。
func sign(secret string, timestampMs int64) (string, error) {
	stringToSign := fmt.Sprintf("%d\n%s", timestampMs, secret)
	h := hmac.New(sha256.New, []byte(secret))
	if _, err := h.Write([]byte(stringToSign)); err != nil {
		return "", err
	}
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return url.QueryEscape(signature), nil
}