// Package qy_wechat 企业微信群机器人与应用消息渠道。
//
// 本文件实现"应用消息"渠道（区别于 groupbot.go 的群机器人）。
//
// 应用消息渠道类型："qy_wechat_app"
// 凭证（ChannelConfig）：
//   - corp_id：企业ID
//   - corp_secret：应用 Secret
//   - agent_id：应用 AgentID
//   - to_user：接收消息的用户（userid，多个用 | 分隔；可被 msg.Extra["to_user"] 覆盖）
//
// Token 管理：access_token 经 tokenstore 缓存（同一 corp_id+corp_secret 共享）。
//
// 参考：https://developer.work.weixin.qq.com/document/path/90236
//       https://work.weixin.qq.com/api/doc/90000/90135/91039
package qy_wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
	"github.com/rocuae/msgch/tokenstore"
)

// 应用消息渠道类型标识。
const AppChannelType = "qy_wechat_app"

// 企业微信接口服务器地址与端点。
const (
	// 企业微信接口服务器地址
	qyApiServerUrl = "https://qyapi.weixin.qq.com"

	// 获取应用 access_token GET /cgi-bin/gettoken?corpid=&corpsecret=
	getTokenPath = "/cgi-bin/gettoken"

	// 发送应用消息 POST /cgi-bin/message/send
	messageSendPath = "/cgi-bin/message/send"
)

// tokenResponse 获取 access_token 响应。
type tokenResponse struct {
	// ErrCode 错误码
	ErrCode int `json:"errcode"`
	// ErrMsg 错误信息
	ErrMsg string `json:"errmsg"`
	// AccessToken 获取到的 access_token
	AccessToken string `json:"access_token"`
	// ExpiresIn 凭证有效时间，单位秒（通常 7200）
	ExpiresIn int `json:"expires_in"`
}

// messageRequest 应用消息发送请求体。
// 参考 https://developer.work.weixin.qq.com/document/path/90236
type messageRequest struct {
	// MsgType 消息类型：text / markdown / textcard
	MsgType string `json:"msgtype"`
	// ToUser 接收消息的用户 UserID，多个用 | 分隔；@all 发给所有人
	ToUser string `json:"touser"`
	// AgentID 应用 AgentID
	AgentID string `json:"agentid"`
	// Text 纯文本消息体（msgtype=text 时）
	Text textBody `json:"text"`
	// Markdown Markdown 消息体（msgtype=markdown 时）
	Markdown mdBody `json:"markdown"`
	// TextCard 文本卡片消息体（msgtype=textcard 时）
	TextCard textCardBody `json:"textcard"`
}

// textCardBody 文本卡片消息体。
type textCardBody struct {
	// Title 标题
	Title string `json:"title"`
	// Description 描述内容
	Description string `json:"description"`
	// URL 点击卡片跳转链接
	URL string `json:"url"`
}

// messageResponse 应用消息发送响应。
type messageResponse struct {
	// ErrCode 错误码，0 表示成功
	ErrCode int `json:"errcode"`
	// ErrMsg 错误信息
	ErrMsg string `json:"errmsg"`
}

// AppChannel 企业微信应用消息渠道。
// 需经 tokenstore 缓存 access_token，区别于群机器人渠道。
type AppChannel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(AppChannelType, func() msgch.Channel {
		return &AppChannel{BaseChannel: msgch.NewBase(AppChannelType, []msgch.Format{
			msgch.FormatMarkdown, msgch.FormatText,
		})}
	})
}

// Send 实现 msgch.Channel。
//
// params:
//   - ctx：超时/取消
//   - cfg：渠道认证配置（KV）
//   - msg：统一消息模型（Extra["to_user"] 可覆盖配置的 to_user）
//
// returns:
//   - *msgch.Result：Success=false 表示发送失败（厂商错误码），Err!=nil 表示系统错误
func (c *AppChannel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	corpID := cfg.GetString("corp_id")
	corpSecret := cfg.GetString("corp_secret")
	agentID := cfg.GetString("agent_id")
	if corpID == "" || corpSecret == "" || agentID == "" {
		return c.FailResult("missing corp_id/corp_secret/agent_id"), nil
	}
	toUser := cfg.GetString("to_user")
	if msg != nil && msg.Extra != nil {
		if v, ok := msg.Extra["to_user"].(string); ok && v != "" {
			toUser = v
		}
	}
	if toUser == "" {
		return c.FailResult("missing to_user"), nil
	}

	// 取 access_token（经 tokenstore 缓存）
	cacheKey := "qy_wechat_app:" + corpID + ":" + corpSecret
	token, err := tokenstore.Get(ctx, cacheKey, func() (string, time.Duration, error) {
		return fetchAccessToken(corpID, corpSecret)
	})
	if err != nil {
		return nil, err
	}

	// 组装请求体：有 Title+URL 走 textcard，否则 markdown 优先、text 兜底
	format, content := msg.FormatContent([]msgch.Format{msgch.FormatMarkdown, msgch.FormatText})
	req := messageRequest{
		ToUser:  toUser,
		AgentID: agentID,
	}
	switch format {
	case msgch.FormatMarkdown:
		req.MsgType = "markdown"
		req.Markdown = mdBody{Content: content}
	default:
		// 有标题且 URL → textcard；否则 text
		if msg != nil && msg.Title != "" && msg.URL != "" {
			req.MsgType = "textcard"
			req.TextCard = textCardBody{Title: msg.Title, Description: content, URL: msg.URL}
		} else {
			req.MsgType = "text"
			req.Text = textBody{Content: content}
		}
	}

	url := fmt.Sprintf("%s%s?access_token=%s", qyApiServerUrl, messageSendPath, token)
	respBytes, status, err := httpclient.PostJSON(ctx, url, req)
	if err != nil {
		return nil, err
	}

	// 翻译厂商响应
	var res messageResponse
	if jerr := json.Unmarshal(respBytes, &res); jerr != nil {
		if status >= 200 && status < 300 {
			return c.SuccessResult(string(respBytes)), nil
		}
		return c.FailResult(fmt.Sprintf("HTTP %d: %s", status, string(respBytes))), nil
	}
	if res.ErrCode != 0 {
		return c.FailResult(fmt.Sprintf("errcode=%d errmsg=%s", res.ErrCode, res.ErrMsg)), nil
	}
	return c.SuccessResult(string(respBytes)), nil
}

// fetchAccessToken 调用企业微信接口获取应用 access_token。
// GET https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=&corpsecret=
func fetchAccessToken(corpID, corpSecret string) (string, time.Duration, error) {
	url := fmt.Sprintf("%s%s?corpid=%s&corpsecret=%s", qyApiServerUrl, getTokenPath, corpID, corpSecret)
	respBytes, status, err := httpclient.Get(context.Background(), url)
	if err != nil {
		return "", 0, err
	}
	var res tokenResponse
	if jerr := json.Unmarshal(respBytes, &res); jerr != nil {
		return "", 0, fmt.Errorf("fetchAccessToken 解析响应失败: HTTP %d: %s", status, string(respBytes))
	}
	if res.ErrCode != 0 {
		return "", 0, fmt.Errorf("fetchAccessToken 企业微信接口错误: errcode=%d, errmsg=%s", res.ErrCode, res.ErrMsg)
	}
	return res.AccessToken, time.Duration(res.ExpiresIn) * time.Second, nil
}