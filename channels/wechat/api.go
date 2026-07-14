// Package wechat 的 api 后端：本库自实现的 HTTP + tokenstore 方式。
//
// 走微信稳定版接口（stable_token）取 access_token，再调模板消息发送接口。
// access_token 经 tokenstore 缓存（同一 app_id+app_secret 共享），零第三方依赖。
package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
	"github.com/rocuae/msgch/tokenstore"
)

// 微信接口服务器地址与端点。
const (
	// 微信接口服务器地址
	wechatApiServerUrl = "https://api.weixin.qq.com"

	// 获取稳定版接口调用凭据 POST /cgi-bin/stable_token
	// 推荐使用稳定版接口，普通 /cgi-bin/token 已不推荐
	stableAccessTokenPath = "/cgi-bin/stable_token"

	// 模板消息发送 POST /cgi-bin/message/template/send
	templateMessageSendPath = "/cgi-bin/message/template/send"
)

// stableAccessTokenRequest 获取稳定版接口调用凭据请求体。
type stableAccessTokenRequest struct {
	// 填写 client_credential
	GrantType string `json:"grant_type"`
	// 账号的唯一凭证，即 AppID
	AppID string `json:"appid"`
	// 唯一凭证密钥，即 AppSecret
	Secret string `json:"secret"`
	// 是否强制刷新 access_token，默认 false
	ForceRefresh bool `json:"force_refresh"`
}

// accessTokenResponse 获取接口调用凭据响应。
type accessTokenResponse struct {
	// 获取到的凭证，最长 512 字节
	AccessToken string `json:"access_token"`
	// 凭证有效时间，单位秒（通常 7200）
	ExpiresIn int `json:"expires_in"`
}

// apiErrorResponse 微信接口错误响应。
type apiErrorResponse struct {
	// 消息 ID（发送成功时返回）
	MsgID int `json:"msgid,omitempty"`
	// 错误码
	ErrCode int `json:"errcode"`
	// 错误信息
	ErrMsg string `json:"errmsg"`
}

// templateMessageRequest 模板消息发送请求体。
// 参考 https://developers.weixin.qq.com/doc/service/api/notify/template/api_sendtemplatemessage.html
type templateMessageRequest struct {
	// 接收消息的微信用户的 UserID
	ToUser string `json:"touser"`
	// 模板 ID
	TemplateID string `json:"template_id"`
	// 模板消息内容：每个键对应模板中的一个变量，值为 {"value":"...","color":"..."}
	Data map[string]interface{} `json:"data"`
}

// apiSender 本库自实现的 HTTP + tokenstore 后端。
type apiSender struct{}

// apiSenderInstance 单例，供 Channel.selectSender 返回。
var apiSenderInstance sender = apiSender{}

// Send 实现 sender 接口。
//
// params:
//   - ctx：超时/取消
//   - cfg：渠道认证配置（app_id/app_secret/template_id/to_user）
//   - msg：统一消息模型（Extra 可带模板变量与 to_user 覆盖）
//
// returns:
//   - *msgch.Result：Success=false 表示发送失败（厂商错误码），Err!=nil 表示系统错误
func (apiSender) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	appID := cfg.GetString("app_id")
	appSecret := cfg.GetString("app_secret")
	templateID := cfg.GetString("template_id")
	if appID == "" || appSecret == "" || templateID == "" {
		return failResult("missing app_id/app_secret/template_id"), nil
	}
	toUser := resolveToUser(cfg, msg)
	if toUser == "" {
		return failResult("missing to_user"), nil
	}

	// 取 access_token（经 tokenstore 缓存，同一凭证不重复取）
	cacheKey := "wechat:" + appID + ":" + appSecret
	token, err := tokenstore.Get(ctx, cacheKey, func() (string, time.Duration, error) {
		return fetchStableAccessToken(appID, appSecret)
	})
	if err != nil {
		return nil, err // 取 token 失败视为系统错误
	}

	// 组装模板消息请求体
	req := templateMessageRequest{
		ToUser:     toUser,
		TemplateID: templateID,
		Data:       buildTemplateData(msg),
	}

	// 发送 POST
	url := fmt.Sprintf("%s%s?access_token=%s", wechatApiServerUrl, templateMessageSendPath, token)
	respBytes, status, err := httpclient.PostJSON(ctx, url, req)
	if err != nil {
		return nil, err
	}

	// 翻译厂商响应
	var res apiErrorResponse
	if jerr := json.Unmarshal(respBytes, &res); jerr != nil {
		// 非 JSON 响应，按 HTTP 状态判定
		if status >= 200 && status < 300 {
			return successResult(string(respBytes)), nil
		}
		return failResult(fmt.Sprintf("HTTP %d: %s", status, string(respBytes))), nil
	}
	if res.ErrCode != 0 {
		return failResult(fmt.Sprintf("errcode=%d errmsg=%s", res.ErrCode, res.ErrMsg)), nil
	}
	return successResult(string(respBytes)), nil
}

// fetchStableAccessToken 调用微信稳定版接口获取 access_token。
// POST https://api.weixin.qq.com/cgi-bin/stable_token
func fetchStableAccessToken(appID, appSecret string) (string, time.Duration, error) {
	body := stableAccessTokenRequest{
		GrantType: "client_credential",
		AppID:     appID,
		Secret:    appSecret,
	}
	url := wechatApiServerUrl + stableAccessTokenPath
	respBytes, status, err := httpclient.PostJSON(context.Background(), url, body)
	if err != nil {
		return "", 0, err
	}
	var tokenResp accessTokenResponse
	var apiErr apiErrorResponse
	if jerr := json.Unmarshal(respBytes, &tokenResp); jerr != nil || tokenResp.AccessToken == "" {
		if jerr2 := json.Unmarshal(respBytes, &apiErr); jerr2 == nil && apiErr.ErrCode != 0 {
			return "", 0, fmt.Errorf("getStableAccessToken 微信接口错误: errcode=%d, errmsg=%s", apiErr.ErrCode, apiErr.ErrMsg)
		}
		if status >= 200 && status < 300 {
			return "", 0, fmt.Errorf("getStableAccessToken 解析响应失败: %s", string(respBytes))
		}
		return "", 0, fmt.Errorf("getStableAccessToken HTTP %d: %s", status, string(respBytes))
	}
	return tokenResp.AccessToken, time.Duration(tokenResp.ExpiresIn) * time.Second, nil
}

// strTrim 去除字符串首尾空白（供 buildTemplateData 用，避免 wechat.go 引 strings）。
func strTrim(s string) string { return strings.TrimSpace(s) }

// successResult / failResult 构造 *msgch.Result。
// 因 sender 实现不嵌入 BaseChannel，这里独立构造结果。
func successResult(response string) *msgch.Result {
	return &msgch.Result{Success: true, Response: response}
}

func failResult(response string) *msgch.Result {
	return &msgch.Result{Success: false, Response: response}
}