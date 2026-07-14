package discord

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

// TestSend_Success 端到端模拟 Discord 返回 204 No Content。
func TestSend_Success(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(204) // Discord 成功返回 204
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"webhook_url": ts.URL + "/webhooks/test"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "hello"})
	if err != nil {
		t.Fatalf("发送 error: %v", err)
	}
	if !r.Success {
		t.Fatalf("期望成功，实际 %s", r.Response)
	}

	var req request
	if err := json.Unmarshal(gotBody, &req); err != nil {
		t.Fatalf("请求体非 JSON: %v", err)
	}
	if req.Content != "hello" {
		t.Fatalf("期望 content=hello，实际 %q", req.Content)
	}
}

// TestSend_WithAtAll 验证 @everyone 语法。
func TestSend_WithAtAll(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"webhook_url": ts.URL + "/webhooks/test"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{
		Text: "alert",
		At:   &msgch.AtInfo{AtAll: true, UserIDs: []string{"u1"}},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !r.Success {
		t.Fatalf("期望成功: %s", r.Response)
	}

	var req request
	json.Unmarshal(gotBody, &req)
	if req.Content != "@everyone <@u1> alert" {
		t.Fatalf("期望 @everyone <@u1> alert，实际 %q", req.Content)
	}
}

// TestSend_Fail 厂商返回错误 JSON。
func TestSend_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"code":50035,"message":"Invalid Webhook Token"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"webhook_url": ts.URL + "/webhooks/bad"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("厂商错误应 Success=false")
	}
}
