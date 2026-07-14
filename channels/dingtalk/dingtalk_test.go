package dingtalk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rocuae/msgch"
)

// TestSign 校验钉钉加签格式：HMAC-SHA256(secret, "{ts}\n{secret}") → base64 → URL encode。
func TestSign(t *testing.T) {
	got, err := sign("SECtest", 1700000000000)
	if err != nil {
		t.Fatalf("sign err: %v", err)
	}
	if got == "" {
		t.Fatal("sign 返回空")
	}
	// 应是 URL-encoded 的 base64 字符串
	dec, err := url.QueryUnescape(got)
	if err != nil || dec == "" {
		t.Fatalf("sign 结果非合法 URL 编码: %q", got)
	}
}

// TestBuildRequest_Text 校验纯文本消息请求体。
func TestBuildRequest_Text(t *testing.T) {
	msg := &msgch.Message{Text: "hello"}
	req := buildRequest(msg)
	if req.MsgType != "text" {
		t.Fatalf("期望 msgtype=text，实际 %s", req.MsgType)
	}
	if req.Text.Content != "hello" {
		t.Fatalf("期望 content=hello，实际 %q", req.Text.Content)
	}
}

// TestBuildRequest_Markdown 校验 markdown 消息请求体与 At。
func TestBuildRequest_Markdown(t *testing.T) {
	msg := &msgch.Message{
		Title:    "标题",
		Markdown: "**内容**",
		At:       &msgch.AtInfo{UserIDs: []string{"user1", "user2"}},
	}
	req := buildRequest(msg)
	if req.MsgType != "markdown" {
		t.Fatalf("期望 msgtype=markdown，实际 %s", req.MsgType)
	}
	if req.Markdown.Title != "标题" {
		t.Fatalf("期望 title=标题，实际 %q", req.Markdown.Title)
	}
	if req.Markdown.Text != "**内容**" {
		t.Fatalf("期望 text=**内容**，实际 %q", req.Markdown.Text)
	}
	if len(req.At.AtUserIds) != 2 || req.At.AtUserIds[0] != "user1" {
		t.Fatalf("AtUserIds 不正确: %v", req.At.AtUserIds)
	}
	if req.At.IsAtAll {
		t.Fatal("不应 AtAll")
	}
}

// TestBuildRequest_AtAll 校验 @所有人。
func TestBuildRequest_AtAll(t *testing.T) {
	msg := &msgch.Message{Text: "x", At: &msgch.AtInfo{AtAll: true}}
	req := buildRequest(msg)
	if !req.At.IsAtAll {
		t.Fatal("期望 IsAtAll=true")
	}
}

// TestSend_MissingConfig 配置缺失应返回 FailResult（非 error）。
func TestSend_MissingConfig(t *testing.T) {
	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	r, err := c.Send(context.Background(), msgch.ChannelConfig{}, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("不应返回 error: %v", err)
	}
	if r.Success {
		t.Fatal("配置缺失应 Success=false")
	}
}

// TestSend_Success 端到端模拟钉钉厂商 API 返回 errcode=0，验证签名、请求体、结果翻译。
func TestSend_Success(t *testing.T) {
	var gotQuery url.Values
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{
		"webhook_url": ts.URL + "/robot/send?access_token=test",
		"secret":      "SECtest",
	}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{
		Title: "标题", Markdown: "**内容**",
		At: &msgch.AtInfo{UserIDs: []string{"u1"}},
	})
	if err != nil {
		t.Fatalf("发送 error: %v", err)
	}
	if !r.Success {
		t.Fatalf("期望成功，实际 %s", r.Response)
	}

	// 校验签名参数存在
	if gotQuery.Get("timestamp") == "" || gotQuery.Get("sign") == "" {
		t.Fatalf("缺少 timestamp/sign 查询参数: %v", gotQuery)
	}

	// 校验请求体
	var reqBody request
	if err := json.Unmarshal(gotBody, &reqBody); err != nil {
		t.Fatalf("请求体非 JSON: %v", err)
	}
	if reqBody.MsgType != "markdown" {
		t.Fatalf("期望 msgtype=markdown，实际 %s", reqBody.MsgType)
	}
	if reqBody.Markdown.Title != "标题" {
		t.Fatalf("期望 title=标题，实际 %q", reqBody.Markdown.Title)
	}
	if len(reqBody.At.AtUserIds) != 1 || reqBody.At.AtUserIds[0] != "u1" {
		t.Fatalf("AtUserIds 不正确: %v", reqBody.At.AtUserIds)
	}
}

// TestSend_Fail 厂商返回 errcode!=0，应返回 FailResult。
func TestSend_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":300001,"errmsg":"token 无效"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"webhook_url": ts.URL + "/robot/send"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("厂商错误应 Success=false")
	}
	if r.Response == "" {
		t.Fatal("应有错误响应")
	}
}