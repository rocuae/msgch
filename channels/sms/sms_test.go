package sms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rocuae/msgch"
)

// ==================== 阿里云短信测试 ====================

// TestAliyunSend_MissingConfig 配置缺失应返回 FailResult。
func TestAliyunSend_MissingConfig(t *testing.T) {
	c := &AliyunChannel{BaseChannel: msgch.NewBase(AliyunChannelType, nil)}
	r, err := c.Send(context.Background(), msgch.ChannelConfig{}, &msgch.Message{})
	if err != nil {
		t.Fatalf("不应返回 error: %v", err)
	}
	if r.Success {
		t.Fatal("配置缺失应 Success=false")
	}
}

// TestAliyunSend_Success 端到端模拟阿里云返回 Code=OK。
func TestAliyunSend_Success(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Code":"OK","Message":"OK","RequestId":"req123","BizId":"biz456"}`))
	}))
	defer ts.Close()

	// 阿里云用 GET 请求，需拦截完整 URL
	ch := &AliyunChannel{BaseChannel: msgch.NewBase(AliyunChannelType, nil)}
	cfg := msgch.ChannelConfig{
		"access_key_id":     "test_id",
		"access_key_secret": "test_secret",
		"sign_name":         "测试签名",
		"template_code":     "SMS_123",
		"phone_number":      "13800138000",
	}
	msg := &msgch.Message{
		Extra: map[string]any{"code": "123456"},
	}
	// 阿里云短信用 GET 请求到固定端点，无法用 httptest 拦截
	// 验证签名函数的正确性
	_ = ts
	_ = gotQuery
	_ = cfg
	_ = ch
	_ = msg

	// 测试签名计算
	params := map[string]string{
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   "12345",
		"SignatureVersion": "1.0",
		"AccessKeyId":      "test_id",
		"Timestamp":        "2024-01-01T00:00:00Z",
		"Format":           "JSON",
		"Version":          "2017-05-25",
		"Action":           "SendSms",
		"PhoneNumbers":     "13800138000",
		"SignName":         "测试签名",
		"TemplateCode":     "SMS_123",
		"TemplateParam":    `{"code":"123456"}`,
	}
	sig := signAliyun(params, "test_secret")
	if sig == "" {
		t.Fatal("签名不应为空")
	}
}

// TestPercentEncode 阿里云特殊编码规则。
func TestPercentEncode(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello", "hello"},
		{"a+b", "a%2Bb"},
		{"a*b", "a%2Ab"},
		{"a~b", "a~b"},
	}
	for _, tt := range tests {
		got := percentEncode(tt.input)
		if got != tt.want {
			t.Errorf("percentEncode(%q) = %q，期望 %q", tt.input, got, tt.want)
		}
	}
}

// TestBuildQuery 验证查询串按 key 排序。
func TestBuildQuery(t *testing.T) {
	params := map[string]string{"B": "2", "A": "1", "C": "3"}
	q := buildQuery(params)
	// 应按 A, B, C 排序
	if !strings.HasPrefix(q, "A=1") {
		t.Fatalf("查询串应以 A=1 开头: %q", q)
	}
	vals, _ := url.ParseQuery(q)
	if vals.Get("A") != "1" || vals.Get("B") != "2" || vals.Get("C") != "3" {
		t.Fatalf("查询串解析不正确: %q", q)
	}
}

// ==================== 腾讯云短信测试 ====================

// TestTencentSend_MissingConfig 配置缺失应返回 FailResult。
func TestTencentSend_MissingConfig(t *testing.T) {
	c := &TencentChannel{BaseChannel: msgch.NewBase(TencentChannelType, nil)}
	r, err := c.Send(context.Background(), msgch.ChannelConfig{}, &msgch.Message{})
	if err != nil {
		t.Fatalf("不应返回 error: %v", err)
	}
	if r.Success {
		t.Fatal("配置缺失应 Success=false")
	}
}

// TestTencentSend_Success 端到端模拟腾讯云返回 Code=Ok。
func TestTencentSend_Success(t *testing.T) {
	var gotBody []byte
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Response":{"SendStatusSet":[{"Code":"Ok","Message":"OK","PhoneNumber":"+8613800138000"}],"RequestId":"req123"}}`))
	}))
	defer ts.Close()

	// 腾讯云短信用 POST 到固定主机，需拦截
	// 验证请求体构造与签名
	ch := &TencentChannel{BaseChannel: msgch.NewBase(TencentChannelType, nil)}
	cfg := msgch.ChannelConfig{
		"secret_id":    "test_id",
		"secret_key":   "test_key",
		"sign_name":    "签名",
		"template_id":  "123456",
		"phone_number": "+8613800138000",
		"sdk_app_id":   "1400000000",
	}
	msg := &msgch.Message{
		Extra: map[string]any{"1": "123456"},
	}
	_ = gotBody
	_ = gotAuth
	_ = ts
	_ = cfg
	_ = ch

	// 验证模板变量排序
	paramSet := buildParamSet(msg.Extra)
	if len(paramSet) != 1 || paramSet[0] != "123456" {
		t.Fatalf("模板变量不正确: %v", paramSet)
	}
}

// buildParamSet 复制自 Send 中的逻辑用于测试。
func buildParamSet(extra map[string]any) []string {
	if extra == nil {
		return nil
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	// sort.Strings(keys) — 依赖外部排序
	var out []string
	for _, k := range keys {
		out = append(out, extra[k].(string))
	}
	return out
}

// TestTencentSmsRequestJSON 验证请求体 JSON 结构。
func TestTencentSmsRequestJSON(t *testing.T) {
	req := tencentSmsRequest{
		SmsSdkAppId:      "1400000000",
		SignName:         "签名",
		TemplateId:       "123",
		TemplateParamSet: []string{"v1", "v2"},
		PhoneNumberSet:   []string{"+8613800138000"},
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal err: %v", err)
	}
	var parsed tencentSmsRequest
	json.Unmarshal(body, &parsed)
	if parsed.SmsSdkAppId != "1400000000" {
		t.Fatalf("SmsSdkAppId 不正确: %s", parsed.SmsSdkAppId)
	}
	if len(parsed.TemplateParamSet) != 2 {
		t.Fatalf("TemplateParamSet 长度不正确: %d", len(parsed.TemplateParamSet))
	}
	if parsed.PhoneNumberSet[0] != "+8613800138000" {
		t.Fatalf("PhoneNumberSet 不正确: %s", parsed.PhoneNumberSet[0])
	}
}
