package email

import (
	"context"
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

// TestSend_MissingTo 缺少收件人应返回 FailResult。
func TestSend_MissingTo(t *testing.T) {
	c := &Channel{BaseChannel: msgch.NewBase(ChannelType, nil)}
	cfg := msgch.ChannelConfig{
		"host": "smtp.test.com", "port": "25",
		"account": "user@test.com", "passwd": "pw",
	}
	r, err := c.Send(context.Background(), cfg, &msgch.Message{Text: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Success {
		t.Fatal("缺少 to 应 Success=false")
	}
}

// TestBuildMimeMessage_PlainText 纯文本邮件。
func TestBuildMimeMessage_PlainText(t *testing.T) {
	msg, err := buildMimeMessage("from@test.com", []string{"to@test.com"}, "主题", "", "纯文本内容")
	if err != nil {
		t.Fatalf("buildMimeMessage err: %v", err)
	}
	if !strings.Contains(msg, "From: from@test.com") {
		t.Fatal("缺少 From 头")
	}
	if !strings.Contains(msg, "To: to@test.com") {
		t.Fatal("缺少 To 头")
	}
	if !strings.Contains(msg, "Subject: ") {
		t.Fatal("缺少 Subject 头")
	}
	if !strings.Contains(msg, "Content-Type: text/plain") {
		t.Fatal("纯文本应有 text/plain Content-Type")
	}
	if strings.Contains(msg, "multipart") {
		t.Fatal("纯文本不应有 multipart")
	}
}

// TestBuildMimeMessage_HTML HTML 邮件应生成 multipart/alternative。
func TestBuildMimeMessage_HTML(t *testing.T) {
	msg, err := buildMimeMessage("from@test.com", []string{"to@test.com"}, "主题", "<h1>标题</h1>", "纯文本")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(msg, "multipart/alternative") {
		t.Fatal("HTML 邮件应有 multipart/alternative")
	}
	if !strings.Contains(msg, "text/html") {
		t.Fatal("应包含 text/html 部分")
	}
	if !strings.Contains(msg, "text/plain") {
		t.Fatal("应包含 text/plain 部分")
	}
}

// TestBuildMimeMessage_MultiRecipient 多收件人。
func TestBuildMimeMessage_MultiRecipient(t *testing.T) {
	msg, err := buildMimeMessage("from@test.com", []string{"a@test.com", "b@test.com"}, "主题", "", "内容")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(msg, "To: a@test.com, b@test.com") {
		t.Fatalf("多收件人格式不正确: %s", msg)
	}
}

// TestSplitAddrs 分割收件人地址。
func TestSplitAddrs(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"a@b.com,c@d.com", 2},
		{"a@b.com; c@d.com", 2},
		{" a@b.com , c@d.com ", 2},
		{"single@test.com", 1},
		{"", 0},
	}
	for _, tt := range tests {
		got := splitAddrs(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitAddrs(%q) = %d 个，期望 %d", tt.input, len(got), tt.want)
		}
	}
}

// TestMimeEncode 纯 ASCII 主题不编码，中文主题用 B 编码。
func TestMimeEncode(t *testing.T) {
	ascii := mimeEncode("Hello")
	if ascii != "Hello" {
		t.Fatalf("纯 ASCII 不应编码: %q", ascii)
	}
	cn := mimeEncode("中文主题")
	if !strings.HasPrefix(cn, "=?UTF-8?B?") {
		t.Fatalf("中文应用 B 编码: %q", cn)
	}
}
