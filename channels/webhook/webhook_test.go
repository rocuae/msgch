package webhook

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

// TestSend_HTTPNotAllowed HTTP URL 未显式允许时应拒绝。
func TestSend_HTTPNotAllowed(t *testing.T) {
	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"url": "http://internal:8080/hook"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("HTTP 未允许应 Success=false")
	}
}

// TestSend_HTTPAllowed HTTP URL 显式允许时应成功。
func TestSend_HTTPAllowed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"url": ts.URL + "/hook", "allow_http": "true"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "hello"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !r.Success {
		t.Fatalf("期望成功: %s", r.Response)
	}
}

// TestSend_DefaultJSONBody 无模板时默认构造 JSON body。
func TestSend_DefaultJSONBody(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"url": ts.URL + "/hook", "allow_http": "true"}
	_, err := c.Send(context.Background(), cfg, &msgch.Message{Title: "T", Text: "body", URL: "https://x.com"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var parsed map[string]string
	json.Unmarshal(gotBody, &parsed)
	if parsed["title"] != "T" || parsed["text"] != "body" || parsed["url"] != "https://x.com" {
		t.Fatalf("默认 JSON body 不正确: %v", parsed)
	}
}

// TestSend_BodyTemplate 模板占位符替换。
func TestSend_BodyTemplate(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{
		"url":           ts.URL + "/hook",
		"allow_http":    "true",
		"body_template": `{"text":"$title: $text"}`,
	}
	_, err := c.Send(context.Background(), cfg, &msgch.Message{Title: "告警", Text: "CPU 90%"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	expected := `{"text":"告警: CPU 90%"}`
	if string(gotBody) != expected {
		t.Fatalf("模板替换不正确: got=%q want=%q", string(gotBody), expected)
	}
}

// TestSend_CustomHeaders 自定义 headers。
func TestSend_CustomHeaders(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{
		"url":        ts.URL + "/hook",
		"allow_http": "true",
		"headers":    `{"Authorization":"Bearer mytoken"}`,
	}
	_, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if gotAuth != "Bearer mytoken" {
		t.Fatalf("期望 Authorization=Bearer mytoken，实际 %q", gotAuth)
	}
}

// TestSend_Fail 非 2xx 应返回 FailResult。
func TestSend_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{"url": ts.URL + "/hook", "allow_http": "true"}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("500 应 Success=false")
	}
}
