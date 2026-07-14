// Package feishu 飞书群机器人与自建应用渠道。
//
// 本文件实现"自建应用消息"渠道（区别于 feishu.go 的群机器人）。
//
// 自建应用渠道类型："feishu_app"
// 凭证（ChannelConfig）：
//   - app_id：飞书应用 App ID
//   - app_secret：飞书应用 App Secret
//   - receive_id：消息接收者 ID（与 receive_id_type 配合，如 "ou_xxx" / "oc_xxx"）
//   - receive_id_type：接收者类型（open_id / user_id / union_id / chat_id 等，默认 open_id）
//     以上均可被 msg.Extra 同名字段覆盖
//
// Token 管理：tenant_access_token 经 tokenstore 缓存（同一 app_id+app_secret 共享）。
//
// 参考：https://open.feishu.cn/document/ukTMukTMukTM/ukDNz4SO0MjL5QzM/auth-v3/auth/tenant_access_token_internal
//       https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/create
package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
	"github.com/rocuae/msgch/tokenstore"
)

// 自建应用渠道类型标识。
const AppChannelType = "feishu_app"

// 飞书接口服务器地址与端点。
const (
	// 飞书开放平台服务器地址
	feishuApiServerUrl = "https://open.feishu.cn"

	// 获取 tenant_access_token（内部应用）POST /open-apis/auth/v3/tenant_access_token/internal
	tenantAccessTokenPath = "/open-apis/auth/v3/tenant_access_token/internal"

	// 发送消息 POST /open-apis/im/v1/messages?receive_id_type=
	messageCreatePath = "/open-apis/im/v1/messages"
)

// tokenRequest 获取 tenant_access_token 请求体。
type tokenRequest struct {
	// AppID 飞书应用 App ID
	AppID string `json:"app_id"`
	// AppSecret 飞书应用 App Secret
	AppSecret string `json:"app_secret"`
}

// tokenResponse 获取 tenant_access_token 响应。
type tokenResponse struct {
	// Code 错误码
	Code int `json:"code"`
	// Msg 错误信息
	Msg string `json:"msg"`
	// TenantAccessToken 租户 access_token
	TenantAccessToken string `json:"tenant_access_token"`
	// Expire token 有效期，单位秒
	Expire int `json:"expire"`
}

// messageRequest 飞书发送消息请求体（im/v1/messages/create）。
type messageRequest struct {
	// ReceiveID 接收者 ID
	ReceiveID string `json:"receive_id"`
	// MsgType 消息类型：text / interactive / post 等
	MsgType string `json:"msg_type"`
	// Content 消息内容（JSON 字符串）
	Content string `json:"content"`
}

// messageResponse 飞书发送消息响应。
type messageResponse struct {
	// Code 错误码
	Code int `json:"code"`
	// Msg 错误信息
	Msg string `json:"msg"`
}

// AppChannel 飞书自建应用消息渠道。
// 需经 tokenstore 缓存 tenant_access_token，区别于群机器人渠道。
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
//   - msg：统一消息模型（Extra 可覆盖 receive_id / receive_id_type）
//
// returns:
//   - *msgch.Result：Success=false 表示发送失败（厂商错误码），Err!=nil 表示系统错误
func (c *AppChannel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	appID := cfg.GetString("app_id")
	appSecret := cfg.GetString("app_secret")
	if appID == "" || appSecret == "" {
		return c.FailResult("missing app_id/app_secret"), nil
	}

	receiveID := cfg.GetString("receive_id")
	receiveIDType := cfg.GetString("receive_id_type")
	if receiveIDType == "" {
		receiveIDType = "open_id"
	}
	// msg.Extra 覆盖
	if msg != nil && msg.Extra != nil {
		if v, ok := msg.Extra["receive_id"].(string); ok && v != "" {
			receiveID = v
		}
		if v, ok := msg.Extra["receive_id_type"].(string); ok && v != "" {
			receiveIDType = v
		}
	}
	if receiveID == "" {
		return c.FailResult("missing receive_id"), nil
	}

	// 取 tenant_access_token（经 tokenstore 缓存）
	cacheKey := "feishu_app:" + appID + ":" + appSecret
	token, err := tokenstore.Get(ctx, cacheKey, func() (string, time.Duration, error) {
		return fetchTenantAccessToken(appID, appSecret)
	})
	if err != nil {
		return nil, err
	}

	// 组装消息内容：text 类型承载内容；@前缀嵌入
	_, text := msg.FormatContent([]msgch.Format{msgch.FormatMarkdown, msgch.FormatText})
	if msg.At != nil {
		text = atPrefix(msg.At) + text
	}
	content, err := json.Marshal(textContent{Text: text})
	if err != nil {
		return nil, err
	}
	req := messageRequest{
		ReceiveID: receiveID,
		MsgType:   "text",
		Content:   string(content),
	}

	// 发送：需 Authorization 头，用 http.NewRequest 自定义后经 httpclient.Do 执行
	url := fmt.Sprintf("%s%s?receive_id_type=%s", feishuApiServerUrl, messageCreatePath, receiveIDType)
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	respBytes, status, err := httpclient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	var res messageResponse
	if jerr := json.Unmarshal(respBytes, &res); jerr != nil {
		if status >= 200 && status < 300 {
			return c.SuccessResult(string(respBytes)), nil
		}
		return c.FailResult(fmt.Sprintf("HTTP %d: %s", status, string(respBytes))), nil
	}
	if res.Code != 0 {
		return c.FailResult(fmt.Sprintf("code=%d msg=%s", res.Code, res.Msg)), nil
	}
	return c.SuccessResult(string(respBytes)), nil
}

// fetchTenantAccessToken 调飞书接口获取 tenant_access_token。
// POST https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal
func fetchTenantAccessToken(appID, appSecret string) (string, time.Duration, error) {
	url := feishuApiServerUrl + tenantAccessTokenPath
	body := tokenRequest{AppID: appID, AppSecret: appSecret}
	respBytes, status, err := httpclient.PostJSON(context.Background(), url, body)
	if err != nil {
		return "", 0, err
	}
	var res tokenResponse
	if jerr := json.Unmarshal(respBytes, &res); jerr != nil {
		return "", 0, fmt.Errorf("fetchTenantAccessToken 解析响应失败: HTTP %d: %s", status, string(respBytes))
	}
	if res.Code != 0 {
		return "", 0, fmt.Errorf("fetchTenantAccessToken 飞书接口错误: code=%d, msg=%s", res.Code, res.Msg)
	}
	return res.TenantAccessToken, time.Duration(res.Expire) * time.Second, nil
}