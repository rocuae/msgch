package msgch

import "strings"

// MarkdownToText 将 Markdown 粗略降级为纯文本（去除常见标记符号）。
// 用于不支持 Markdown 的渠道兜底。非完整 Markdown 解析，仅清理常见语法。
func MarkdownToText(md string) string {
	if md == "" {
		return ""
	}
	s := md
	// 去除标题井号
	s = stripLeadingPrefix(s, "#")
	s = stripLeadingPrefix(s, "##")
	s = stripLeadingPrefix(s, "###")
	// 去除粗体/斜体标记
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "_", "")
	// 去除行内代码反引号
	s = strings.ReplaceAll(s, "`", "")
	// 去除删除线
	s = strings.ReplaceAll(s, "~~", "")
	// 去除图片/链接，保留 alt/文本
	s = replaceImages(s)
	s = replaceLinks(s)
	// 去除引用块前缀
	s = strings.ReplaceAll(s, "\n> ", "\n")
	if strings.HasPrefix(s, "> ") {
		s = strings.TrimPrefix(s, "> ")
	}
	return strings.TrimSpace(s)
}

// MarkdownToHTML 将 Markdown 粗略转为 HTML（用于邮件渠道）。
// 仅处理常见语法（标题/粗体/斜体/代码块/链接/列表/换行），非完整实现。
// 生产场景如需完整渲染，调用方可在 Send 前自行用更完善的库转换后填入 msg.HTML。
func MarkdownToHTML(md string) string {
	if md == "" {
		return ""
	}
	lines := strings.Split(md, "\n")
	var b strings.Builder
	inCodeBlock := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			if inCodeBlock {
				b.WriteString("</code></pre>\n")
				inCodeBlock = false
			} else {
				b.WriteString("<pre><code>")
				inCodeBlock = true
			}
			continue
		}
		if inCodeBlock {
			b.WriteString(escapeHTML(line))
			b.WriteString("\n")
			continue
		}
		switch {
		case strings.HasPrefix(t, "### "):
			b.WriteString("<h3>" + inlineMD(strings.TrimPrefix(t, "### ")) + "</h3>\n")
		case strings.HasPrefix(t, "## "):
			b.WriteString("<h2>" + inlineMD(strings.TrimPrefix(t, "## ")) + "</h2>\n")
		case strings.HasPrefix(t, "# "):
			b.WriteString("<h1>" + inlineMD(strings.TrimPrefix(t, "# ")) + "</h1>\n")
		case strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* "):
			b.WriteString("<li>" + inlineMD(strings.TrimPrefix(strings.TrimPrefix(t, "- "), "* ")) + "</li>\n")
		case t == "":
			b.WriteString("<br>\n")
		default:
			b.WriteString("<p>" + inlineMD(line) + "</p>\n")
		}
	}
	if inCodeBlock {
		b.WriteString("</code></pre>\n")
	}
	return b.String()
}

// inlineMD 处理行内 Markdown：粗体/斜体/行内代码/链接。
func inlineMD(s string) string {
	s = escapeHTML(s)
	// 粗体
	s = replacePair(s, "**", "<strong>", "</strong>")
	s = replacePair(s, "__", "<strong>", "</strong>")
	// 行内代码
	s = replacePair(s, "`", "<code>", "</code>")
	// 链接 [text](url)
	s = replaceLinksHTML(s)
	return s
}

func replacePair(s, delim, open, close string) string {
	out := strings.Builder{}
	out.Grow(len(s))
	opened := false
	i := 0
	for i < len(s) {
		if strings.HasPrefix(s[i:], delim) {
			if opened {
				out.WriteString(close)
			} else {
				out.WriteString(open)
			}
			opened = !opened
			i += len(delim)
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	if opened {
		out.WriteString(close)
	}
	return out.String()
}

func replaceImages(s string) string {
	// ![alt](url) -> alt
	for {
		idx := strings.Index(s, "![")
		if idx < 0 {
			break
		}
		closeAlt := strings.Index(s[idx:], "](")
		if closeAlt < 0 {
			break
		}
		closeAlt += idx
		alt := s[idx+2 : closeAlt]
		closeURL := strings.Index(s[closeAlt+2:], ")")
		if closeURL < 0 {
			break
		}
		s = s[:idx] + alt + s[closeAlt+2+closeURL+1:]
	}
	return s
}

func replaceLinks(s string) string {
	// [text](url) -> text
	for {
		idx := strings.Index(s, "[")
		if idx < 0 {
			break
		}
		closeText := strings.Index(s[idx:], "](")
		if closeText < 0 {
			break
		}
		closeText += idx
		text := s[idx+1 : closeText]
		closeURL := strings.Index(s[closeText+2:], ")")
		if closeURL < 0 {
			break
		}
		s = s[:idx] + text + s[closeText+2+closeURL+1:]
	}
	return s
}

func replaceLinksHTML(s string) string {
	// [text](url) -> <a href="url">text</a>
	for {
		idx := strings.Index(s, "[")
		if idx < 0 {
			break
		}
		closeText := strings.Index(s[idx:], "](")
		if closeText < 0 {
			break
		}
		closeText += idx
		text := s[idx+1 : closeText]
		closeURL := strings.Index(s[closeText+2:], ")")
		if closeURL < 0 {
			break
		}
		url := s[closeText+2 : closeText+2+closeURL]
		s = s[:idx] + `<a href="` + url + `">` + text + `</a>` + s[closeText+2+closeURL+1:]
	}
	return s
}

func stripLeadingPrefix(s, prefix string) string {
	if strings.HasPrefix(s, prefix+" ") {
		return strings.TrimPrefix(s, prefix+" ")
	}
	return s
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
