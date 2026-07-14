package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rocuae/msgch"
)

// TestSend_MissingConfig 配置缺失应返回 FailResult。
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

// TestSend_Success 端到端模拟 Telegram 返回 ok=true。
func TestSend_Success(t *testing.T) {
	var gotBody []byte
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"bot_token": "test:token", "chat_id": "@test", "api_host": ts.URL}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Markdown: "**bold**"})
	if err != nil {
		t.Fatalf("发送 error: %v", err)
	}
	if !r.Success {
		t.Fatalf("期望成功，实际 %s", r.Response)
	}

	// 校验 URL 路径
	if !strings.Contains(gotURL, "/bottest:token/sendMessage") {
		t.Fatalf("URL 不正确: %q", gotURL)
	}

	// 校验请求体
	var req request
	if err := json.Unmarshal(gotBody, &req); err != nil {
		t.Fatalf("请求体非 JSON: %v", err)
	}
	if req.ChatID != "@test" {
		t.Fatalf("期望 chat_id=@test，实际 %q", req.ChatID)
	}
	if req.ParseMode != "Markdown" {
		t.Fatalf("期望 parse_mode=Markdown，实际 %q", req.ParseMode)
	}
	if req.Text != "**bold**" {
		t.Fatalf("期望 text=**bold**，实际 %q", req.Text)
	}
}

// TestSend_HTMLParseMode HTML 格式应设置 parse_mode=HTML。
func TestSend_HTMLParseMode(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"bot_token": "t:k", "chat_id": "1", "api_host": ts.URL}
	_, err := c.Send(context.Background(), cfg, &msgch.Message{HTML: "<b>hi</b>"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var req request
	json.Unmarshal(gotBody, &req)
	if req.ParseMode != "HTML" {
		t.Fatalf("期望 parse_mode=HTML，实际 %q", req.ParseMode)
	}
}

// TestSend_Fail 厂商返回 ok=false。
func TestSend_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"bot_token": "t:k", "chat_id": "bad", "api_host": ts.URL}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("厂商错误应 Success=false")
	}
	if !strings.Contains(r.Response, "chat not found") {
		t.Fatalf("应包含错误描述: %s", r.Response)
	}
}

// TestSplitText 长文本分段。
func TestSplitText(t *testing.T) {
	// 短文本不分段
	segs := splitText("hello", 10)
	if len(segs) != 1 || segs[0] != "hello" {
		t.Fatalf("短文本应不分段: %v", segs)
	}
	// 长文本分段
	long := strings.Repeat("a", 100)
	segs = splitText(long, 30)
	if len(segs) != 4 {
		t.Fatalf("期望 4 段，实际 %d 段", len(segs))
	}
	total := ""
	for _, s := range segs {
		total += s
	}
	if total != long {
		t.Fatal("分段后拼接应与原文一致")
	}
}
