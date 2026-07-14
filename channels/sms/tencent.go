// Package sms 实现短信渠道（阿里云、腾讯云）。
//
// 本文件实现腾讯云短信渠道。
//
// 腾讯云短信渠道类型："sms_tencent"
// 凭证（ChannelConfig）：
//   - secret_id：腾讯云 SecretId
//   - secret_key：腾讯云 SecretKey
//   - sign_name：短信签名
//   - template_id：短信模板 ID
//   - phone_number：手机号（需带国际区号前缀，如 +8613800138000）
//   - region：可选，SDK 区域，默认 ap-guangzhou
//   - sdk_app_id：短信应用 SDKAppID
//
// 模板变量：从 msg.Extra 取，按模板占位符顺序转为字符串数组（腾讯云短信模板参数为有序列表）。
// 例如 msg.Extra = {"code":"123456"} → ["123456"]。
//
// 参考：https://cloud.tencent.com/document/product/382/55981
//       签名 v3（TC3-HMAC-SHA256）：https://cloud.tencent.com/document/api/382/52076
package sms

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/rocuae/msgch"
	"github.com/rocuae/msgch/httpclient"
)

// 腾讯云短信渠道类型标识。
const TencentChannelType = "sms_tencent"

// 腾讯云短信接口端点与常量。
const (
	// 腾讯云短信 API 主机
	tencentSmsHost = "sms.tencentcloudapi.com"
	// 发送短信服务名
	tencentService = "sms"
	// 发送短信接口动作
	tencentAction = "SendSms"
	// 接口版本
	tencentApiVersion = "2021-01-11"
)

// TencentChannel 腾讯云短信渠道。
type TencentChannel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(TencentChannelType, func() msgch.Channel {
		return &TencentChannel{BaseChannel: msgch.NewBase(TencentChannelType, []msgch.Format{
			msgch.FormatText,
		})}
	})
}

// tencentSmsResponse 腾讯云短信发送响应。
type tencentSmsResponse struct {
	// 响应体（包含 SendStatusSet）
	Response struct {
		// 发送结果集
		SendStatusSet []struct {
			// 发送状态码：Ok 表示成功
			Code string `json:"Code"`
			// 错误信息
			Message string `json:"Message"`
			// 手机号
			PhoneNumber string `json:"PhoneNumber"`
		} `json:"SendStatusSet"`
		// 请求 ID
		RequestId string `json:"RequestId"`
	} `json:"Response"`
}

// tencentSmsRequest 腾讯云短信发送请求体。
type tencentSmsRequest struct {
	// 短信应用 SDKAppID
	SmsSdkAppId string `json:"SmsSdkAppId"`
	// 短信签名
	SignName string `json:"SignName"`
	// 模板 ID
	TemplateId string `json:"TemplateId"`
	// 模板参数（有序字符串列表，对应模板占位符 {1}{2}...）
	TemplateParamSet []string `json:"TemplateParamSet"`
	// 手机号列表（带国际区号）
	PhoneNumberSet []string `json:"PhoneNumberSet"`
}

// Send 实现 msgch.Channel。
//
// params:
//   - ctx：超时/取消
//   - cfg：渠道认证配置（KV）
//   - msg：统一消息模型（Extra 传模板变量，按 key 排序后转字符串列表）
//
// returns:
//   - *msgch.Result：Success=false 表示发送失败，Err!=nil 表示系统错误
func (c *TencentChannel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	secretID := cfg.GetString("secret_id")
	secretKey := cfg.GetString("secret_key")
	signName := cfg.GetString("sign_name")
	templateID := cfg.GetString("template_id")
	phoneNumber := cfg.GetString("phone_number")
	sdkAppID := cfg.GetString("sdk_app_id")
	if secretID == "" || secretKey == "" || signName == "" {
		return c.FailResult("missing secret_id/secret_key/sign_name"), nil
	}
	if phoneNumber == "" || templateID == "" || sdkAppID == "" {
		return c.FailResult("missing phone_number/template_id/sdk_app_id"), nil
	}
	region := cfg.GetString("region")
	if region == "" {
		region = "ap-guangzhou"
	}

	// 模板变量：msg.Extra 按 key 排序后取值转字符串列表。
	// 腾讯云短信模板参数 {1}{2}... 为有序列表，故按 key 字典序对应模板序号。
	var paramSet []string
	if msg != nil && msg.Extra != nil {
		keys := make([]string, 0, len(msg.Extra))
		for k := range msg.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			paramSet = append(paramSet, fmt.Sprintf("%v", msg.Extra[k]))
		}
	}
	if paramSet == nil {
		paramSet = []string{}
	}

	// 组装请求体
	reqBody := tencentSmsRequest{
		SmsSdkAppId:    sdkAppID,
		SignName:       signName,
		TemplateId:     templateID,
		TemplateParamSet: paramSet,
		PhoneNumberSet: []string{phoneNumber},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// 构造 HTTP 请求
	url := "https://" + tencentSmsHost
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	// 计算腾讯云签名 v3 并设置请求头
	signTencentV3(httpReq, payload, secretID, secretKey, region)

	respBytes, _, err := httpclient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	// 翻译响应
	var res tencentSmsResponse
	if jerr := json.Unmarshal(respBytes, &res); jerr != nil {
		return c.FailResult(fmt.Sprintf("parse response failed: %s", string(respBytes))), nil
	}
	if len(res.Response.SendStatusSet) == 0 {
		return c.FailResult("no SendStatusSet in response"), nil
	}
	st := res.Response.SendStatusSet[0]
	if st.Code != "Ok" {
		return c.FailResult(fmt.Sprintf("Code=%s Message=%s RequestId=%s", st.Code, st.Message, res.Response.RequestId)), nil
	}
	return c.SuccessResult(string(respBytes)), nil
}

// signTencentV3 计算腾讯云签名 v3（TC3-HMAC-SHA256）并设置请求头。
//
// 参考签名流程：
//  1. 拼接规范请求串 CanonicalRequest
//  2. 拼接待签名字符串 StringToSign
//  3. 计算签名 Signature = HMAC-SHA256层层派生
//  4. 设置 Authorization 头
func signTencentV3(req *http.Request, payload []byte, secretID, secretKey, region string) {
	algorithm := "TC3-HMAC-SHA256"
	service := tencentService
	timestamp := time.Now().Unix()
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")

	// 1. 规范请求串
	canonicalURI := "/"
	canonicalQueryString := "" // POST 体在 payload，查询串为空
	signedHeaders := "content-type;host;x-tc-action"
	canonicalHeaders := "content-type:application/json; charset=utf-8\nhost:" + tencentSmsHost + "\nx-tc-action:" + tencentAction + "\n"
	hashedPayload := sha256Hex(payload)
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	}, "\n")

	// 2. 待签名字符串
	credentialScope := date + "/" + service + "/tc3_request"
	hashedCanonical := sha256Hex([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		algorithm,
		fmt.Sprintf("%d", timestamp),
		credentialScope,
		hashedCanonical,
	}, "\n")

	// 3. 派生签名密钥并计算签名
	secretDate := hmacSHA256([]byte("TC3"+secretKey), []byte(date))
	secretService := hmacSHA256(secretDate, []byte(service))
	secretSigning := hmacSHA256(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(secretSigning, []byte(stringToSign)))

	// 4. 设置 Authorization 头
	authorization := algorithm + " Credential=" + secretID + "/" + credentialScope +
		", SignedHeaders=" + signedHeaders + ", Signature=" + signature
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", tencentSmsHost)
	req.Header.Set("X-TC-Action", tencentAction)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-TC-Version", tencentApiVersion)
	req.Header.Set("X-TC-Region", region)
}

// sha256Hex 计算 SHA256 并返回十六进制字符串。
func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// hmacSHA256 计算 HMAC-SHA256。
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}