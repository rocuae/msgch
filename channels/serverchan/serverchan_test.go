package serverchan

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rocuae/msgch"
)

func TestResolveAPIURL(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		want    string
		wantErr bool
	}{
		{
			name: "Turbo",
			key:  "sct123",
			want: "https://sctapi.ftqq.com/sct123.send",
		},
		{
			name: "Server酱³",
			key:  "sctp12345tabc",
			want: "https://12345.push.ft07.com/send/sctp12345tabc.send",
		},
		{
			name:    "无效的Server酱³ SendKey",
			key:     "sctp-invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAPIURL(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatal("期望返回错误")
				}
				return
			}
			if err != nil {
				t.Fatalf("解析地址失败: %v", err)
			}
			if got != tt.want {
				t.Fatalf("地址不正确: got=%q want=%q", got, tt.want)
			}
		})
	}
}

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

// TestSend_Success 端到端模拟 Server 酱返回 code=0。
func TestSend_Success(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"api_url": ts.URL + "/send"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Title: "标题", Text: "内容"})
	if err != nil {
		t.Fatalf("发送 error: %v", err)
	}
	if !r.Success {
		t.Fatalf("期望成功，实际 %s", r.Response)
	}

	// Server 酱用 form 编码
	vals, err := parseForm(gotBody)
	if err != nil {
		t.Fatalf("非 form 编码: %v", err)
	}
	if vals.Get("title") != "标题" {
		t.Fatalf("期望 title=标题，实际 %q", vals.Get("title"))
	}
	if vals.Get("desp") != "内容" {
		t.Fatalf("期望 desp=内容，实际 %q", vals.Get("desp"))
	}
}

func TestSend_ServerChan3ExtraParams(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	msg := &msgch.Message{
		Title: "标题",
		Text:  "内容",
		Extra: map[string]any{"tags": "告警|生产", "short": "CPU 90%"},
	}
	r, err := c.Send(context.Background(), msgch.ChannelConfig{"api_url": ts.URL}, msg)
	if err != nil || !r.Success {
		t.Fatalf("发送失败: result=%v err=%v", r, err)
	}

	vals, err := parseForm(gotBody)
	if err != nil {
		t.Fatalf("非 form 编码: %v", err)
	}
	if vals.Get("tags") != "告警|生产" {
		t.Fatalf("tags 不正确: %q", vals.Get("tags"))
	}
	if vals.Get("short") != "CPU 90%" {
		t.Fatalf("short 不正确: %q", vals.Get("short"))
	}
}

func TestSend_InvalidServerChan3Key(t *testing.T) {
	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	r, err := c.Send(context.Background(), msgch.ChannelConfig{"sendkey": "sctp-invalid"}, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("不应返回系统错误: %v", err)
	}
	if r.Success || !strings.Contains(r.Response, "invalid Server酱³ sendkey") {
		t.Fatalf("期望无效 SendKey，实际结果: %+v", r)
	}
}

// TestSend_Fail 厂商返回 code!=0。
func TestSend_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":40001,"message":"badkey"}`))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"api_url": ts.URL + "/send"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("厂商错误应 Success=false")
	}
}

// parseForm 解析 application/x-www-form-urlencoded body。
func parseForm(body []byte) (url.Values, error) {
	return url.ParseQuery(string(body))
}
