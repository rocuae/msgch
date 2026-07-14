package msgch

import "testing"

func TestFormatContent_MarkdownPreferred(t *testing.T) {
	msg := &Message{Text: "t", Markdown: "# md"}
	f, c := msg.FormatContent([]Format{FormatMarkdown, FormatText})
	if f != FormatMarkdown || c != "# md" {
		t.Fatalf("期望 markdown/# md，实际 %s/%q", f, c)
	}
}

func TestFormatContent_TextFallback(t *testing.T) {
	msg := &Message{Text: "only text"}
	f, c := msg.FormatContent([]Format{FormatMarkdown, FormatText})
	if f != FormatText || c != "only text" {
		t.Fatalf("期望 text/only text，实际 %s/%q", f, c)
	}
}

func TestFormatContent_GlobalFallback(t *testing.T) {
	// 渠道只声明 HTML，但消息只有 Text → 兜底返回 Text
	msg := &Message{Text: "hi"}
	f, c := msg.FormatContent([]Format{FormatHTML})
	if f != FormatText || c != "hi" {
		t.Fatalf("期望 text/hi，实际 %s/%q", f, c)
	}
}

func TestFormatContent_Empty(t *testing.T) {
	msg := &Message{}
	f, c := msg.FormatContent([]Format{FormatMarkdown, FormatText})
	if f != FormatText || c != "" {
		t.Fatalf("期望 text/空，实际 %s/%q", f, c)
	}
}

func TestMarkdownToText_Bold(t *testing.T) {
	if got := MarkdownToText("**hi**"); got != "hi" {
		t.Fatalf("期望 hi，实际 %q", got)
	}
}

func TestMarkdownToHTML_Heading(t *testing.T) {
	got := MarkdownToHTML("# 标题\n正文")
	if got == "" {
		t.Fatal("html 为空")
	}
	if !contains(got, "<h1>") {
		t.Fatalf("期望含 <h1>，实际 %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}