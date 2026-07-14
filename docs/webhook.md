# 自定义 Webhook

> 渠道类型：`webhook`

## 概述

自定义 Webhook 渠道允许调用方将消息发送到任意 HTTP 接口。支持自定义请求方法、请求头、请求体模板，适配任意第三方接收端。

## 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `url` | ✅ | 目标 URL（必须 HTTPS；HTTP 仅当 `allow_http=true` 时允许） |
| `method` | ❌ | HTTP 方法，默认 `POST` |
| `headers` | ❌ | 自定义请求头，JSON 格式（如 `{"Authorization":"Bearer token"}`） |
| `body_template` | ❌ | 请求体模板，支持占位符替换（见下方） |
| `content_type` | ❌ | Content-Type，默认 `application/json` |
| `allow_http` | ❌ | 设为 `true` 允许 HTTP URL（默认仅允许 HTTPS，防 SSRF） |

## 占位符

`body_template` 支持以下占位符，发送时自动替换为消息内容：

| 占位符 | 替换为 |
| --- | --- |
| `$title` | msg.Title |
| `$text` | msg.Text |
| `$markdown` | msg.Markdown |
| `$html` | msg.HTML |
| `$url` | msg.URL |

## 代码示例

### 示例 1：使用默认 JSON body（不配置 body_template）

```go
cfg := msgch.ChannelConfig{
    "url":         "https://my-server.com/webhook",
    "allow_http":  "true", // 如果是 HTTP 需开启
}
msg := &msgch.Message{Title: "告警", Text: "CPU 90%", URL: "https://alert.com/1"}
result, err := msgch.Push(ctx, "webhook", cfg, msg)
```

默认发送 JSON：

```json
{"title":"告警","text":"CPU 90%","url":"https://alert.com/1"}
```

### 示例 2：自定义 body 模板

```go
cfg := msgch.ChannelConfig{
    "url":           "https://hooks.slack.com/services/xxx",
    "body_template": `{"text":"*$title*\n$text\n$url"}`,
}
msg := &msgch.Message{Title: "告警", Text: "CPU 90%", URL: "https://alert.com/1"}
msgch.Push(ctx, "webhook", cfg, msg)
```

### 示例 3：自定义请求头

```go
cfg := msgch.ChannelConfig{
    "url":     "https://my-server.com/webhook",
    "headers": `{"Authorization":"Bearer mytoken","X-Custom":"value"}`,
}
msgch.Push(ctx, "webhook", cfg, &msgch.Message{Text: "hello"})
```

## 安全机制

- 默认仅允许 **HTTPS** URL，防止 SSRF 攻击
- HTTP URL 需显式设置 `allow_http=true`
- 仅支持 `http` 和 `https` 协议

## 注意事项

- 无 body_template 时，默认发送 `{"title":...,"text":...,"url":...}` JSON
- 2xx 状态码视为成功，其他视为发送失败
- headers 值必须是合法 JSON 字符串
- 长消息不受本渠道限制（取决于目标服务端）
