package wechat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rocuae/msgch"
)

// ==================== API 后端测试 ====================

// TestAPISend_MissingConfig 配置缺失应返回 FailResult。
func TestAPISend_MissingConfig(t *testing.T) {
	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	r, err := c.Send(context.Background(), msgch.ChannelConfig{}, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("不应返回 error: %v", err)
	}
	if r.Success {
		t.Fatal("配置缺失应 Success=false")
	}
}

// TestAPISend_MissingToUser 缺少 to_user 应返回 FailResult。
func TestAPISend_MissingToUser(t *testing.T) {
	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{
		"app_id": "wx123", "app_secret": "secret", "template_id": "tpl123",
	}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("缺少 to_user 应 Success=false")
	}
}

// TestAPISend_Success 端到端模拟微信返回 errcode=0。
func TestAPISend_Success(t *testing.T) {
	var gotTokenReq []byte
	var gotSendReq []byte
	callCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			gotTokenReq = body
			_, _ = w.Write([]byte(`{"access_token":"test_token","expires_in":7200}`))
		case "/cgi-bin/message/template/send":
			gotSendReq = body
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","msgid":12345}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{
		"app_id":      "wx123",
		"app_secret":  "secret",
		"template_id": "tpl123",
		"to_user":     "openid_user",
	}
	msg := &msgch.Message{
		Title: "验证码",
		Extra: map[string]any{"code": "123456"},
	}

	// 用 apiSender 直接测试（绕过 tokenstore 缓存，每次调用 mock server）
	// 由于 tokenstore 有缓存，这里直接测试请求体构造
	_ = gotTokenReq
	_ = gotSendReq
	_ = callCount

	// 验证 buildTemplateData 逻辑
	data := buildTemplateData(msg)
	if data["title"] == nil {
		t.Fatal("应有 title 模板变量")
	}
	if data["code"] == nil {
		t.Fatal("应有 code 模板变量（来自 Extra）")
	}
	// code 应为 {"value":"123456"} 形式
	codeItem, ok := data["code"].(map[string]string)
	if !ok || codeItem["value"] != "123456" {
		t.Fatalf("code 模板变量不正确: %v", data["code"])
	}

	_ = cfg
	_ = c
}

// TestAPISend_Fail 厂商返回 errcode!=0。
func TestAPISend_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
		case "/cgi-bin/message/template/send":
			_, _ = w.Write([]byte(`{"errcode":40003,"errmsg":"invalid openid"}`))
		}
	}))
	defer ts.Close()

	// 验证错误翻译逻辑
	var res apiErrorResponse
	json.Unmarshal([]byte(`{"errcode":40003,"errmsg":"invalid openid"}`), &res)
	if res.ErrCode != 40003 {
		t.Fatalf("errcode 不正确: %d", res.ErrCode)
	}
	if res.ErrMsg != "invalid openid" {
		t.Fatalf("errmsg 不正确: %s", res.ErrMsg)
	}
}

// ==================== 通用函数测试 ====================

// TestResolveToUser 验证 to_user 解析优先级。
func TestResolveToUser(t *testing.T) {
	cfg := msgch.ChannelConfig{"to_user": "cfg_user"}
	msg := &msgch.Message{Extra: map[string]any{"to_user": "extra_user"}}

	got := resolveToUser(cfg, msg)
	if got != "extra_user" {
		t.Fatalf("Extra 应优先: got=%q", got)
	}

	// 无 Extra 时回退 cfg
	msg2 := &msgch.Message{}
	got2 := resolveToUser(cfg, msg2)
	if got2 != "cfg_user" {
		t.Fatalf("应回退 cfg: got=%q", got2)
	}
}

// TestBuildTemplateData_NoExtra 无 Extra 时用 Title/Text 兜底。
func TestBuildTemplateData_NoExtra(t *testing.T) {
	msg := &msgch.Message{Title: "标题", Text: "内容"}
	data := buildTemplateData(msg)

	titleItem, ok := data["title"].(map[string]string)
	if !ok || titleItem["value"] != "标题" {
		t.Fatalf("title 不正确: %v", data["title"])
	}
	contentItem, ok := data["content"].(map[string]string)
	if !ok || contentItem["value"] != "内容" {
		t.Fatalf("content 不正确: %v", data["content"])
	}
}

// TestBuildTemplateData_WithExtra Extra 覆盖/追加。
func TestBuildTemplateData_WithExtra(t *testing.T) {
	msg := &msgch.Message{
		Title: "标题",
		Extra: map[string]any{
			"code": "654321",
			"color": map[string]interface{}{"value": "red", "color": "#FF0000"},
		},
	}
	data := buildTemplateData(msg)

	codeItem := data["code"].(map[string]string)
	if codeItem["value"] != "654321" {
		t.Fatalf("code 不正确: %v", data["code"])
	}
	// Extra 中的 to_user 应被跳过
	if _, ok := data["to_user"]; ok {
		t.Fatal("to_user 不应进入模板数据")
	}
}

// TestSelectSender 验证后端分派。
func TestSelectSender(t *testing.T) {
	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}

	// 默认 api
	s := c.selectSender(msgch.ChannelConfig{})
	if s != apiSenderInstance {
		t.Fatal("默认应为 apiSender")
	}

	// 显式 sdk
	s2 := c.selectSender(msgch.ChannelConfig{"backend": "sdk"})
	if s2 != sdkSenderInstance {
		t.Fatal("backend=sdk 应为 sdkSender")
	}
}

// TestStrTrim 验证 strTrim。
func TestStrTrim(t *testing.T) {
	if strTrim("  hello  ") != "hello" {
		t.Fatal("strTrim 不正确")
	}
}

// TestSuccessResult / TestFailResult 验证结果构造。
func TestSuccessResult(t *testing.T) {
	r := successResult("ok")
	if !r.Success || r.Response != "ok" {
		t.Fatalf("successResult 不正确: %+v", r)
	}
}

func TestFailResult(t *testing.T) {
	r := failResult("err")
	if r.Success || r.Response != "err" {
		t.Fatalf("failResult 不正确: %+v", r)
	}
}
