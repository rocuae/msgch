package qy_wechat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rocuae/msgch"
)

// ==================== 群机器人测试 ====================

// TestGroupBot_MissingConfig 配置缺失应返回 FailResult。
func TestGroupBot_MissingConfig(t *testing.T) {
	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	r, err := c.Send(context.Background(), msgch.ChannelConfig{}, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("不应返回 error: %v", err)
	}
	if r.Success {
		t.Fatal("配置缺失应 Success=false")
	}
}

// TestGroupBot_Success 端到端模拟企微群机器人返回 errcode=0。
func TestGroupBot_Success(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"webhook_url": ts.URL + "/hook"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Markdown: "**告警**"})
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
	if req.MsgType != "markdown" {
		t.Fatalf("期望 msgtype=markdown，实际 %s", req.MsgType)
	}
	if req.Markdown.Content != "**告警**" {
		t.Fatalf("期望 content=**告警**，实际 %q", req.Markdown.Content)
	}
}

// TestGroupBot_Text 纯文本消息。
func TestGroupBot_Text(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"webhook_url": ts.URL + "/hook"}
	_, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "纯文本"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var req request
	json.Unmarshal(gotBody, &req)
	if req.MsgType != "text" {
		t.Fatalf("期望 msgtype=text，实际 %s", req.MsgType)
	}
	if req.Text.Content != "纯文本" {
		t.Fatalf("期望 content=纯文本，实际 %q", req.Text.Content)
	}
}

// TestGroupBot_Fail 厂商返回 errcode!=0。
func TestGroupBot_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":93000,"errmsg":"invalid webhook url"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"webhook_url": ts.URL + "/hook"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("厂商错误应 Success=false")
	}
}

// ==================== 应用消息测试 ====================

// TestApp_MissingConfig 配置缺失应返回 FailResult。
func TestApp_MissingConfig(t *testing.T) {
	c := &AppChannel{BaseChannel: msgch.NewBase(AppChannelType, nil)}
	r, err := c.Send(context.Background(), msgch.ChannelConfig{}, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("不应返回 error: %v", err)
	}
	if r.Success {
		t.Fatal("配置缺失应 Success=false")
	}
}

// TestApp_MissingToUser 缺少 to_user 应返回 FailResult。
func TestApp_MissingToUser(t *testing.T) {
	c := &AppChannel{BaseChannel: msgch.NewBase(AppChannelType, nil)}
	cfg := msgch.ChannelConfig{
		"corp_id": "id", "corp_secret": "secret", "agent_id": "1000",
	}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("缺少 to_user 应 Success=false")
	}
}
