<!-- markdownlint-disable MD010 -->
# msgch

> 一个 Go 实现的统一消息推送纯库。一处调用，多渠道可插拔，零第三方依赖。

`msgch` 提供统一的消息入口（`Push` / `Broadcast`），支持钉钉、飞书、企业微信、个人微信、Bark、Telegram、Discord、Server酱、WxPusher、自定义 Webhook、邮件、短信（阿里云/腾讯云）等渠道的可插拔扩展。


## 快速开始

### 1. 发送一条消息（钉钉群机器人）

```go
package main

import (
	"context"
	"fmt"
	"log"

	"msgch"
	_ "msgch/channels" // 空白导入注册全部内置渠道
)

func main() {
	ctx := context.Background()
	msg := &msgch.Message{
		Title:    "部署完成",
		Markdown: "**服务**已上线 ✅",
	}
	cfg := msgch.ChannelConfig{
		"access_token": "xxxx",
		"secret":       "SECxxxx", // 可选，加签
	}
	result, err := msgch.Push(ctx, "dingtalk", cfg, msg)
	if err != nil {
		log.Fatal("系统错误:", err) // 网络/配置/未知渠道
	}
	if !result.Success {
		log.Println("发送失败:", result.Response) // 厂商错误码
	}
	fmt.Println("发送成功:", result.Response)
}
```

### 2. 复用凭证：Client + 默认配置

适合长期复用同一套凭证的场景（如 HTTP 服务）：

```go
client := msgch.New(
	msgch.WithRetry(msgch.RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    10 * time.Second,
		Jitter:      true,
	}),
)
client.SetDefaultConfig("dingtalk", msgch.ChannelConfig{
	"access_token": "xxxx", "secret": "SECxxxx",
})
client.SetDefaultConfig("email", msgch.ChannelConfig{
	"host": "smtp.qq.com", "port": "465",
	"account": "x@qq.com", "passwd": "xxxx",
	"from": "x@qq.com", "to": "a@b.com",
})

// 后续发送无需重复传凭证；cfg=nil 用默认配置
client.Push(ctx, "dingtalk", nil, &msgch.Message{Text: "hello"})
```

### 3. 多渠道广播

同一条消息并发发给多个渠道，聚合各渠道结果：

```go
targets := []msgch.Target{
	{Type: "dingtalk", Config: dingCfg},
	{Type: "feishu", Config: feishuCfg},
	{Type: "email", Config: emailCfg},
}
br := msgch.Broadcast(ctx, targets, &msgch.Message{
	Title: "告警", Markdown: "**CPU 90%**",
})
log.Printf("成功 %d/%d", br.SuccessCount(), len(targets))
for _, r := range br.Results {
	if r.Err != nil || (r.Result != nil && !r.Result.Success) {
		log.Printf("渠道 %s 失败: %v %s", r.Type, r.Err, r.Result.Response)
	}
}
```

### 4. 需 token 渠道：企业微信应用消息 / 个人微信 / 飞书应用

这些渠道自动经 tokenstore 缓存 access_token，无需调用方处理 token：

```go
// 企业微信应用消息
msgch.Push(ctx, "qy_wechat_app", msgch.ChannelConfig{
	"corp_id": "xxx", "corp_secret": "xxx",
	"agent_id": "1000002", "to_user": "UserID1|UserID2",
}, &msgch.Message{Markdown: "**告警** 内容"})

// 个人微信公众号模板消息（变量经 Extra 传入）
// 默认使用 API 后端（零第三方依赖，tokenstore 缓存 token）
msgch.Push(ctx, "wechat", msgch.ChannelConfig{
	"app_id": "xxx", "app_secret": "xxx",
	"template_id": "xxx", "to_user": "openid_xxx",
}, &msgch.Message{
	Title: "验证码",
	Extra: map[string]any{"code": "123456"}, // 模板变量 ${code}
})

// 个人微信公众号 - SDK 后端（基于 github.com/silenceper/wechat/v2，功能更全）
msgch.Push(ctx, "wechat", msgch.ChannelConfig{
	"backend": "sdk", // 切换为 SDK 后端，不传或 "api" 用默认 API 后端
	"app_id": "xxx", "app_secret": "xxx",
	"template_id": "xxx", "to_user": "openid_xxx",
}, &msgch.Message{
	Title: "验证码",
	Extra: map[string]any{"code": "123456"},
})

// 飞书自建应用
msgch.Push(ctx, "feishu_app", msgch.ChannelConfig{
	"app_id": "cli_xxx", "app_secret": "xxx",
	"receive_id": "ou_xxx", "receive_id_type": "open_id",
}, &msgch.Message{Text: "hello"})
```

### 5. 短信（模板变量经 Extra）

短信内容受模板约束，正文不直接发送，模板变量通过 `msg.Extra` 传入：

```go
// 阿里云短信
msgch.Push(ctx, "sms_aliyun", msgch.ChannelConfig{
	"access_key_id": "LTAIxxx", "access_key_secret": "xxx",
	"sign_name": "签名", "template_code": "SMS_xxx",
	"phone_number": "13800138000",
}, &msgch.Message{
	Extra: map[string]any{"code": "123456", "product": "msgch"},
})

// 腾讯云短信
msgch.Push(ctx, "sms_tencent", msgch.ChannelConfig{
	"secret_id": "xxx", "secret_key": "xxx",
	"sign_name": "签名", "template_id": "123456",
	"phone_number": "+8613800138000", "sdk_app_id": "1400000000",
}, &msgch.Message{
	Extra: map[string]any{"1": "123456"}, // 腾讯云模板按序号 {1}{2}...
})
```

### 6. 邮件（HTML 优先，Text 兜底）

```go
msgch.Push(ctx, "email", msgch.ChannelConfig{
	"host": "smtp.qq.com", "port": "465",
	"account": "x@qq.com", "passwd": "授权码",
	"from": "x@qq.com", "to": "a@b.com,c@d.com",
}, &msgch.Message{
	Title: "告警通知",
	HTML:  "<h1>CPU 90%</h1><p>请处理</p>",
	Text:  "CPU 90% 请处理", // 客户端不支持 HTML 时兜底
})
```

### 7. 异步发送（队列）

```go
import "msgch/queue"

q := queue.New(queue.Config{BufferSize: 1024, Workers: 4})
q.Start()
defer q.Stop()

// 投递后立即返回，后台 worker 发送
q.Push(ctx, "dingtalk", cfg, &msgch.Message{Text: "async hello"})
```

### 8. 发送选项

```go
msgch.Push(ctx, "dingtalk", cfg, msg,
	msgch.WithTimeout(10*time.Second),          // 单次发送超时
	msgch.WithRetry(msgch.RetryPolicy{           // 失败重试
		MaxAttempts: 3, BaseDelay: 1*time.Second,
	}),
	msgch.WithRateLimit(msgch.RateLimit{         // 按渠道限流
		QPS: 20, Burst: 5,
	}),
)
```

## 渠道清单与配置

| 渠道类型 | 凭证字段（ChannelConfig） | 支持格式 | 说明 |
| --- | --- | --- | --- |
| `dingtalk` | `access_token`, `secret`(签名) | Markdown, Text | 钉钉群机器人 |
| `feishu` | `token` 或 `webhook_url`, `secret` | Markdown, Text | 飞书群机器人 |
| `feishu_app` | `app_id`, `app_secret`, `receive_id`, `receive_id_type` | Markdown, Text | 飞书自建应用 |
| `qy_wechat` | `key` 或 `webhook_url` | Markdown, Text | 企业微信群机器人 |
| `qy_wechat_app` | `corp_id`, `corp_secret`, `agent_id`, `to_user` | Markdown, Text | 企业微信应用消息 |
| `wechat` | `app_id`, `app_secret`, `template_id`, `to_user`, `backend`(api/sdk) | Text | 个人微信模板消息（API 零依赖 / SDK silenceper/wechat） |
| `bark` | `server`, `device_key` | Text | Bark 推送 |
| `telegram` | `bot_token`, `chat_id`, `api_host`(可选) | Markdown, HTML, Text | Telegram Bot |
| `discord` | `webhook_url` | Markdown, Text | Discord Webhook |
| `serverchan` | `sendkey` 或 `api_url` | Markdown, Text | Server酱 Turbo / Server酱³ |
| `wxpusher` | `app_token`, `topic_ids`/`uids` | Markdown, HTML, Text | WxPusher |
| `webhook` | `url`, `method`, `headers`, `body_template`, `content_type`, `allow_http` | Text | 自定义 Webhook |
| `email` | `host`, `port`, `account`, `passwd`, `from`, `to` | HTML, Text | SMTP 邮件 |
| `sms_aliyun` | `access_key_id`, `access_key_secret`, `sign_name`, `template_code`, `phone_number`, `region_id` | Text | 阿里云短信 |
| `sms_tencent` | `secret_id`, `secret_key`, `sign_name`, `template_id`, `phone_number`, `sdk_app_id`, `region` | Text | 腾讯云短信 |

## 消息模型

`Message` 四载体并存，渠道按 `Formats()` 自选最优、兜底 `Text`：

```go
type Message struct {
    Title    string              // 标题（邮件 subject、模板消息首行等）
    Text     string              // 纯文本载体
    HTML     string              // HTML 载体（邮件优先）
    Markdown string              // Markdown 载体（IM 机器人优先）
    URL      string              // 详情跳转链接
    ImageURL string              // 图片链接
    At       *AtInfo             // @提醒（Mobiles/UserIDs/AtAll）
    Extra    map[string]any      // 透传：短信模板变量、渠道专属参数等
}
```

调用方一次性填入多种载体，渠道自动挑选：邮件优先 HTML、IM 机器人优先 Markdown、短信走 Extra 模板变量、所有渠道兜底 Text。

## 结果与错误处理

库区分两类失败：

| 情况 | 判断 | 处理建议 |
| --- | --- | --- |
| 系统错误 | `err != nil` | 网络/配置/未知渠道；记录告警，不重试 |
| 发送失败 | `err == nil && !result.Success` | 厂商错误码；可重试 |
| 成功 | `err == nil && result.Success` | — |

```go
result, err := msgch.Push(ctx, "dingtalk", cfg, msg)
switch {
case err != nil:
    // 系统错误
case !result.Success:
    // 发送失败（厂商错误码），看 result.Response
default:
    // 成功
}
```

## 新增一个渠道

以新增 Slack 为例，**全程不改动任何核心文件**：

```go
// channels/slack/slack.go
package slack

import (
    "context"
    "msgch"
    "msgch/httpclient"
)

const ChannelType = "slack"

type Channel struct{ *msgch.BaseChannel }

func init() {
    msgch.Register(ChannelType, func() msgch.Channel {
        return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
            msgch.FormatMarkdown, msgch.FormatText,
        })}
    })
}

func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
    url := cfg.GetString("webhook_url")
    if url == "" {
        return c.FailResult("missing webhook_url"), nil
    }
    _, content := msg.FormatContent(c.Formats())
    resp, status, err := httpclient.PostJSON(ctx, url, map[string]any{"text": content})
    if err != nil {
        return nil, err // 系统错误
    }
    if status >= 200 && status < 300 {
        return c.SuccessResult("ok"), nil
    }
    return c.FailResult(string(resp)), nil
}
```

在 `channels/import.go` 加一行空白导入即生效。

## 目录结构

```text
msgch/
├── msgch.go              // Push/Broadcast 入口
├── channel.go            // Channel interface + BaseChannel
├── types.go              // Message/Result/ChannelConfig/Target
├── client.go             // Client（默认配置合并）
├── registry.go           // 渠道工厂注册表
├── options.go            // SendOption/RetryPolicy/RateLimit
├── content.go            // FormatContent + Markdown 转换
├── broadcast.go          // 多渠道广播聚合
├── httpclient/           // 统一 HTTP 出口（默认 net/http）
├── tokenstore/           // access_token 缓存刷新
├── retry/                // 重试退避
├── ratelimit/            // 令牌桶限流
├── queue/                // 异步队列
└── channels/             // 各渠道实现（init 自注册）
    ├── import.go         // 集中空白导入
    ├── dingtalk/ feishu/ qy_wechat/ wechat/ bark/
    ├── telegram/ discord/ serverchan/ wxpusher/ webhook/
    ├── email/            // SMTP 邮件
    └── sms/              // aliyun + tencent
```