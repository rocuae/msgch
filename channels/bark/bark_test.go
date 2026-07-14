package bark

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rocuae/msgch"
)

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

// TestSend_Success 端到端模拟 Bark 返回 code=200，验证请求体与结果。
func TestSend_Success(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"success"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"server": ts.URL, "device_key": "dev123"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Title: "标题", Text: "内容"})
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
	if req.Title != "标题" {
		t.Fatalf("期望 title=标题，实际 %q", req.Title)
	}
	if req.Body != "内容" {
		t.Fatalf("期望 body=内容，实际 %q", req.Body)
	}
}

// TestSend_WithURL 验证 URL 字段透传。
func TestSend_WithURL(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"server": ts.URL, "device_key": "dev123"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "内容", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !r.Success {
		t.Fatalf("期望成功: %s", r.Response)
	}

	var req request
	json.Unmarshal(gotBody, &req)
	if req.URL != "https://example.com" {
		t.Fatalf("期望 url=https://example.com，实际 %q", req.URL)
	}
}

// TestSend_Fail 厂商返回 code!=200，应返回 FailResult。
func TestSend_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":400,"message":"invalid device"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"server": ts.URL, "device_key": "bad"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("厂商错误应 Success=false")
	}
}
