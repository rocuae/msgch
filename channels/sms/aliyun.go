// Package sms 实现短信渠道（阿里云、腾讯云）。
//
// 短信渠道与 IM 机器人不同：内容受模板约束，发送需 templateCode + 模板变量，
// 正文不直接发送（模板已在云端配置）。模板变量经 msg.Extra 传入。
//
// 子包文件：
//   - aliyun.go：阿里云短信（dysmsapi SendSms）
//   - tencent.go：腾讯云短信（SmsSendSms，HMAC-SHA256 签名）
package sms

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
)

// 阿里云短信渠道类型标识。
const AliyunChannelType = "sms_aliyun"

// 阿里云短信接口地址。
const (
	// 阿里云短信服务 RPC 端点
	aliyunSmsApiUrl = "https://dysmsapi.aliyuncs.com"
)

// AliyunChannel 阿里云短信渠道。
type AliyunChannel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(AliyunChannelType, func() msgch.Channel {
		return &AliyunChannel{BaseChannel: msgch.NewBase(AliyunChannelType, []msgch.Format{
			msgch.FormatText,
		})}
	})
}

// aliyunSmsResponse 阿里云短信发送响应。
type aliyunSmsResponse struct {
	// 业务码：OK 表示成功
	Code string `json:"Code"`
	// 业务信息
	Message string `json:"Message"`
	// 请求 ID
	RequestId string `json:"RequestId"`
	// 发送回执 ID
	BizId string `json:"BizId"`
}

// Send 实现 msgch.Channel。
//
// 凭证（ChannelConfig）：access_key_id / access_key_secret / sign_name /
// template_code / phone_number / region_id(可选，默认 cn-hangzhou)
//
// 模板变量：从 msg.Extra 取（key→value），序列化为 JSON 作为 TemplateParam。
// 例如 msg.Extra = {"code":"123456"} 对应模板 "您的验证码${code}"。
func (c *AliyunChannel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	accessKeyID := cfg.GetString("access_key_id")
	accessKeySecret := cfg.GetString("access_key_secret")
	signName := cfg.GetString("sign_name")
	templateCode := cfg.GetString("template_code")
	phoneNumber := cfg.GetString("phone_number")
	if accessKeyID == "" || accessKeySecret == "" || signName == "" {
		return c.FailResult("missing access_key_id/access_key_secret/sign_name"), nil
	}
	if phoneNumber == "" || templateCode == "" {
		return c.FailResult("missing phone_number/template_code"), nil
	}
	regionID := cfg.GetString("region_id")
	if regionID == "" {
		regionID = "cn-hangzhou"
	}

	// 组装公共参数 + 业务参数
	params := map[string]string{
		"SignatureMethod":  "HMAC-SHA1",                         // 签名方式
		"SignatureNonce":   fmt.Sprintf("%d", time.Now().UnixNano()), // 唯一随机数
		"SignatureVersion": "1.0",                               // 签名算法版本
		"AccessKeyId":      accessKeyID,
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"), // ISO8601 格式
		"Format":           "JSON", // 返回格式
		"Version":          "2017-05-25",
		"RegionId":         regionID,
		"Action":           "SendSms",
		"PhoneNumbers":     phoneNumber,
		"SignName":         signName,
		"TemplateCode":     templateCode,
	}
	// 模板变量：模板变量经 msg.Extra 传入，序列化为 JSON
	if msg != nil && msg.Extra != nil {
		tplParam, err := json.Marshal(msg.Extra)
		if err != nil {
			return nil, err
		}
		params["TemplateParam"] = string(tplParam)
	}

	// 计算签名（阿里云 RPC 签名 v1.0）
	signature := signAliyun(params, accessKeySecret)
	params["Signature"] = signature

	// 组装查询串并 GET 请求
	query := buildQuery(params)
	reqURL := aliyunSmsApiUrl + "/?" + query
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	respBytes, _, err := httpclient.Do(req)
	if err != nil {
		return nil, err
	}

	// 翻译响应
	var res aliyunSmsResponse
	if jerr := json.Unmarshal(respBytes, &res); jerr != nil {
		return c.FailResult(fmt.Sprintf("parse response failed: %s", string(respBytes))), nil
	}
	if res.Code != "OK" {
		return c.FailResult(fmt.Sprintf("Code=%s Message=%s RequestId=%s", res.Code, res.Message, res.RequestId)), nil
	}
	return c.SuccessResult(string(respBytes)), nil
}

// signAliyun 计算阿里云 RPC 签名。
// 签名步骤：参数按字典序排序 → 拼接 "key1=value1&key2=value2..." →
// 构造待签串 "GET&%2F&<urlencode(querystring)>" → HMAC-SHA1(key+"&", 待签串) → base64。
func signAliyun(params map[string]string, accessKeySecret string) string {
	// 参数按 key 字典序排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接规范化请求串（key 与 value 均需 URL 编码）
	sb := strings.Builder{}
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(percentEncode(k))
		sb.WriteByte('=')
		sb.WriteString(percentEncode(params[k]))
	}
	canonical := sb.String()

	// 构造待签串：GET&%2F&<urlencode(canonical)>
	stringToSign := "GET&" + percentEncode("/") + "&" + percentEncode(canonical)

	// HMAC-SHA1，key 为 AccessKeySecret + "&"
	h := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// buildQuery 用排序后的参数构造查询串。
// 注意：编码必须与签名时一致（均用 percentEncode），否则签名校验失败。
func buildQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, percentEncode(k)+"="+percentEncode(params[k]))
	}
	return strings.Join(parts, "&")
}

// percentEncode 阿里云要求的 URL 编码：+ → %20、* → %2A、%7E → ~ 等。
func percentEncode(s string) string {
	s = url.QueryEscape(s)
	s = strings.ReplaceAll(s, "+", "%20")
	s = strings.ReplaceAll(s, "*", "%2A")
	s = strings.ReplaceAll(s, "%7E", "~")
	return s
}