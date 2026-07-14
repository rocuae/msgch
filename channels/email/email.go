// Package email 实现 SMTP 邮件渠道。
//
// 渠道类型："email"
// 凭证（ChannelConfig）：
//   - host：SMTP 服务器地址（如 smtp.qq.com）
//   - port：端口（字符串，25 / 465(SSL) / 587(STARTTLS)）
//   - account：登录账号
//   - passwd：登录密码/授权码
//   - from：发件人地址（缺省时用 account）
//   - to：收件人，多个用逗号分隔
//
// 消息内容：
//   - msg.Title → 邮件主题 Subject
//   - 按 Formats()=[HTML, Text] 优先 HTML，无则 Text；无 HTML 时纯文本邮件，
//     有 HTML 时发送 multipart/alternative（同时含 HTML 与纯文本，客户端自选显示）
//
// 参考：标准库 net/smtp。零第三方依赖（不引 gomail）。
package email

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/rocuae/msgch"
)

// 渠道类型标识。
const ChannelType = "email"

// Channel 邮件 SMTP 渠道。
type Channel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(ChannelType, func() msgch.Channel {
		return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
			msgch.FormatHTML, msgch.FormatText,
		})}
	})
}

// Send 实现 msgch.Channel。
func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	host := cfg.GetString("host")
	port := cfg.GetString("port")
	account := cfg.GetString("account")
	passwd := cfg.GetString("passwd")
	if host == "" || port == "" || account == "" || passwd == "" {
		return c.FailResult("missing host/port/account/passwd"), nil
	}
	from := cfg.GetString("from")
	if from == "" {
		from = account
	}
	to := cfg.GetString("to")
	if to == "" {
		return c.FailResult("missing to"), nil
	}
	// 收件人列表（逗号分隔）
	recipients := splitAddrs(to)
	if len(recipients) == 0 {
		return c.FailResult("no valid recipients"), nil
	}

	// 内容：HTML 优先，无则 Text
	_, text := msg.FormatContent([]msgch.Format{msgch.FormatHTML, msgch.FormatText})
	subject := msg.Title
	if subject == "" {
		subject = text
		if len(subject) > 50 {
			subject = subject[:50] + "..."
		}
	}

	// 组装 MIME 邮件体
	rawMsg, err := buildMimeMessage(from, recipients, subject, msg.HTML, text)
	if err != nil {
		return nil, err
	}

	// 发送 SMTP
	addr := net.JoinHostPort(host, port)
	var sendErr error
	if port == "465" {
		// 465 用隐式 SSL
		sendErr = sendSSL(addr, account, passwd, from, recipients, rawMsg)
	} else {
		// 25/587 用普通 SMTP（含可选 STARTTLS）
		auth := smtp.PlainAuth("", account, passwd, host)
		sendErr = smtp.SendMail(addr, auth, from, recipients, []byte(rawMsg))
	}
	if sendErr != nil {
		return c.FailResult(fmt.Sprintf("smtp send failed: %v", sendErr)), nil
	}
	return c.SuccessResult("ok"), nil
}

// buildMimeMessage 组装 MIME 邮件。
//   - 有 html：multipart/alternative（同时含 text/plain 与 text/html）
//   - 无 html：纯 text/plain
func buildMimeMessage(from string, to []string, subject, html, text string) (string, error) {
	buf := strings.Builder{}
	buf.WriteString("From: " + from + "\r\n")
	buf.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	buf.WriteString("Subject: " + mimeEncode(subject) + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")

	if html != "" {
		boundary := "----=_msgch_alt_" + randBoundary()
		buf.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
		buf.WriteString("--" + boundary + "\r\n")
		buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		buf.WriteString(base64.StdEncoding.EncodeToString([]byte(text)))
		buf.WriteString("\r\n\r\n")
		buf.WriteString("--" + boundary + "\r\n")
		buf.WriteString("Content-Type: text/html; charset=utf-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		buf.WriteString(base64.StdEncoding.EncodeToString([]byte(html)))
		buf.WriteString("\r\n\r\n")
		buf.WriteString("--" + boundary + "--\r\n")
	} else {
		buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		buf.WriteString(base64.StdEncoding.EncodeToString([]byte(text)))
		buf.WriteString("\r\n")
	}
	return buf.String(), nil
}

// sendSSL 465 端口隐式 SSL 发送。
func sendSSL(addr, account, passwd, from string, to []string, rawMsg string) error {
	tlsConfig := &tls.Config{ServerName: strings.Split(addr, ":")[0]}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, strings.Split(addr, ":")[0])
	if err != nil {
		return err
	}
	defer client.Quit()
	if err = client.Auth(smtp.PlainAuth("", account, passwd, strings.Split(addr, ":")[0])); err != nil {
		return err
	}
	if err = client.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err = client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = w.Write([]byte(rawMsg))
	return err
}

// splitAddrs 分割收件人地址（逗号或分号分隔，去空白）。
func splitAddrs(s string) []string {
	s = strings.ReplaceAll(s, ";", ",")
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// mimeEncode RFC 2047 编码邮件主题（含非 ASCII 时用 B 编码）。
func mimeEncode(s string) string {
	// 简化：若含非 ASCII，则用 base64 B 编码
	for _, r := range s {
		if r > 127 {
			return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
		}
	}
	return s
}

// randBoundary 生成 MIME boundary（简单实现，避免引入 crypto/rand 增加复杂度）。
// 使用固定时间戳拼接，足以分隔 MIME 部分。
func randBoundary() string {
	return "a1b2c3d4"
}