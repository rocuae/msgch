// Package feishu 实现飞书群机器人渠道。
//
// 渠道类型："feishu"
// 凭证（ChannelConfig）：
//   - token：机器人 webhook 的 token（或直接用 webhook_url 传完整地址）
//   - secret：可选，加签密钥
//
// 参考：https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN
package feishu

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
)

// 渠道类型标识。
const ChannelType = "feishu"

// Channel 飞书群机器人渠道。
type Channel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(ChannelType, func() msgch.Channel {
		return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
			msgch.FormatMarkdown, msgch.FormatText,
		})}
	})
}

// 飞书群机器人请求体。
// 参考 https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN
type request struct {
	// MsgType 消息类型：text
	MsgType string `json:"msg_type"`
	// Timestamp 时间戳（加签时必填）
	Timestamp string `json:"timestamp,omitempty"`
	// Sign 签名（加签时必填）
	Sign string `json:"sign,omitempty"`
	// Content 消息内容
	Content textContent `json:"content"`
}

// textContent 飞书 text 消息内容：{"text": "..."}。
type textContent struct {
	// Text 文本内容
	Text string `json:"text,omitempty"`
}

// 飞书群机器人响应。
type response struct {
	// Code 错误码，0 表示成功
	Code int `json:"code"`
	// Msg 错误信息
	Msg string `json:"msg"`
	// Message 错误描述（部分接口返回此字段）
	Message string `json:"message,omitempty"`
}

// Send 实现 msgch.Channel。
func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	webhook := cfg.GetString("webhook_url")
	if webhook == "" {
		token := cfg.GetString("token")
		if token == "" {
			return c.FailResult("missing webhook_url or token"), nil
		}
		webhook = fmt.Sprintf("https://open.feishu.cn/open-apis/bot/v2/hook/%s", token)
	}

	_, text := msg.FormatContent([]msgch.Format{msgch.FormatMarkdown, msgch.FormatText})

	// 群机器人最通用的是 text 类型；markdown 文本也用 text 承载（飞书 text 支持普通文本）。
	// 若需交互卡片，后续可扩展为 interactive 类型。
	reqBody := request{MsgType: "text", Content: textContent{Text: text}}

	// @提醒：飞书用 <at user_id="..."> 语法嵌入文本
	if msg.At != nil {
		reqBody.Content.Text = atPrefix(msg.At) + reqBody.Content.Text
	}

	// 可选签名
	if secret := cfg.GetString("secret"); secret != "" {
		ts := time.Now().Unix()
		sign, err := sign(secret, ts)
		if err != nil {
			return nil, err
		}
		reqBody.Timestamp = strconv.FormatInt(ts, 10)
		reqBody.Sign = sign
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	resp, status, err := httpclient.PostText(ctx, webhook, "application/json", body)
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
		msg := res.Msg
		if msg == "" {
			msg = res.Message
		}
		return c.FailResult(fmt.Sprintf("code=%d msg=%s", res.Code, msg)), nil
	}
	return c.SuccessResult(string(resp)), nil
}

// sign 飞书加签：HMAC-SHA256("{timestamp}\n{secret}", "") → base64。
// 注意：飞书签名与钉钉不同——key 是 "{timestamp}\n{secret}"，对空数据做 HMAC。
func sign(secret string, timestampSec int64) (string, error) {
	stringToSign := fmt.Sprintf("%d\n%s", timestampSec, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// atPrefix 构造飞书 @前缀。
func atPrefix(at *msgch.AtInfo) string {
	if at == nil {
		return ""
	}
	if at.AtAll {
		return `<at user_id="all">所有人</at>`
	}
	prefix := ""
	for _, id := range at.UserIDs {
		prefix += fmt.Sprintf(`<at user_id="%s"> </at>`, id)
	}
	return prefix
}