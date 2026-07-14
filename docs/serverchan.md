# Server酱 Turbo / Server酱³

> 渠道类型：`serverchan`

## 支持版本

| 版本 | SendKey 格式 | 接口地址 |
| --- | --- | --- |
| Server酱 Turbo | `sct` 开头 | `https://sctapi.ftqq.com/{sendkey}.send` |
| Server酱³ | `sctp{uid}t...` | `https://{uid}.push.ft07.com/send/{sendkey}.send` |

`msgch` 会根据 SendKey 自动选择接口。Server酱 Turbo 和 Server酱³ 使用不同的用户系统和 SendKey，两者不能混用。

## 获取 SendKey

### Server酱 Turbo

1. 访问 https://sct.ftqq.com/
2. 点击登录（GitHub 授权）
3. 进入"Key"页面
4. 复制 SendKey

### Server酱³

1. 访问 https://sc3.ft07.com/
2. 登录后打开 [SendKey 页面](https://sc3.ft07.com/sendkey)
3. 安装并配置 Server酱³ APP
4. 复制 `sctp{uid}t...` 格式的 SendKey

## 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `sendkey` | ✅ | Turbo 或 Server酱³ SendKey，自动识别版本 |
| `api_url` | ❌ | 自定义完整 API 地址（覆盖默认地址） |

## Server酱³ 示例

```go
cfg := msgch.ChannelConfig{
	"sendkey": "sctp12345txxxxxxxxx",
}
msg := &msgch.Message{
	Title:    "告警通知",
	Markdown: "**CPU 使用率** 90%\n\n请及时处理",
	Extra: map[string]any{
		"tags":  "告警|生产环境",
		"short": "CPU 使用率达到 90%",
	},
}
result, err := msgch.Push(ctx, "serverchan", cfg, msg)
```

## Server酱 Turbo 示例

```go
cfg := msgch.ChannelConfig{
	"sendkey": "sctxxxxxxxxx",
}
result, err := msgch.Push(ctx, "serverchan", cfg, &msgch.Message{
	Title: "部署完成",
	Text:  "服务已成功上线",
})
```

## 消息格式

- **Markdown**（优先）：msg.Title → 标题，msg.Markdown → 正文
- **Text**（兜底）：msg.Title → 标题，msg.Text → 正文

发送参数：

- `title`：消息标题（显示在微信通知中）
- `desp`：消息正文（支持 Markdown）
- `tags`：Server酱³ 标签，多个标签用 `|` 分隔，通过 `msg.Extra["tags"]` 传入
- `short`：Server酱³ 消息卡片摘要，通过 `msg.Extra["short"]` 传入

## 注意事项

- Server酱³ 专注于 APP 推送；Turbo 可推送到微信、企业微信、钉钉、飞书等通道
- `api_url` 优先级高于 `sendkey` 自动生成的地址
- SendKey 属于敏感凭证，请勿写入日志或提交到代码仓库
- Server酱³ API 说明：https://doc.sc3.ft07.com/zh/serverchan3/server/api
