package feishu

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

// TestGroupBot_Success 端到端模拟飞书群机器人返回 code=0。
func TestGroupBot_Success(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"webhook_url": ts.URL + "/hook"}
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
	if req.MsgType != "text" {
		t.Fatalf("期望 msg_type=text，实际 %s", req.MsgType)
	}
	if req.Content.Text != "hello" {
		t.Fatalf("期望 text=hello，实际 %q", req.Content.Text)
	}
}

// TestGroupBot_Markdown Markdown 格式。
func TestGroupBot_Markdown(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"webhook_url": ts.URL + "/hook"}
	_, err := c.Send(context.Background(), cfg, &msgch.Message{Markdown: "**bold**"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var req request
	json.Unmarshal(gotBody, &req)
	if req.MsgType != "text" {
		t.Fatalf("期望 msg_type=text，实际 %s", req.MsgType)
	}
}

// TestGroupBot_Fail 厂商返回 code!=0。
func TestGroupBot_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":9499,"msg":"bad token"}`))
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

// TestGroupBot_Sign 验证签名函数不为空。
func TestGroupBot_Sign(t *testing.T) {
	sig, err := sign("SECtest", 1700000000)
	if err != nil {
		t.Fatalf("sign err: %v", err)
	}
	if sig == "" {
		t.Fatal("签名不应为空")
	}
}

// ==================== 自建应用测试 ====================

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

// TestApp_MissingReceiveID 缺少 receive_id 应返回 FailResult。
func TestApp_MissingReceiveID(t *testing.T) {
	c := &AppChannel{BaseChannel: msgch.NewBase(AppChannelType, nil)}
	cfg := msgch.ChannelConfig{"app_id": "id", "app_secret": "secret"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("缺少 receive_id 应 Success=false")
	}
}

// TestAtPrefix 验证 @ 前缀构造。
func TestAtPrefix(t *testing.T) {
	tests := []struct {
		at   *msgch.AtInfo
		want string
	}{
		{nil, ""},
		{&msgch.AtInfo{AtAll: true}, "<at user_id=\"all\">所有人</at>"},
		{&msgch.AtInfo{UserIDs: []string{"u1"}}, "<at user_id=\"u1\"> </at>"},
	}
	for _, tt := range tests {
		got := atPrefix(tt.at)
		if got != tt.want {
			t.Errorf("atPrefix(%+v) = %q，期望 %q", tt.at, got, tt.want)
		}
	}
}
