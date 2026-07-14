# Telegram Bot

> 渠道类型：`telegram`

## 参数申请方法

1. 在 Telegram 中搜索 **@BotFather**，发送 `/newbot`
2. 按提示设置机器人名称和用户名（用户名必须以 `bot` 结尾）
3. BotFather 返回 **Bot Token**，格式：`123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11`
4. 获取 **chat_id**（接收消息的目标）：
   - 个人：给机器人发任意消息，然后访问 `https://api.telegram.org/bot<token>/getUpdates` 获取 chat_id
   - 群组：将机器人加入群组，发送消息后访问上述 URL 获取群组 chat_id（负数）
   - 频道：将机器人设为频道管理员，获取频道 chat_id

### 获取 chat_id 的步骤

1. 给你的 Bot 发一条消息
2. 浏览器访问：`https://api.telegram.org/bot你的token/getUpdates`
3. 在返回 JSON 中找到 `result[0].message.chat.id` 即为 chat_id

## 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `bot_token` | ✅ | BotFather 返回的 Bot Token |
| `chat_id` | ✅ | 接收消息的 chat_id（个人/群组/频道） |
| `api_host` | ❌ | 自定义 API 地址（代理场景），默认 `https://api.telegram.org` |

## 代码示例

```go
cfg := msgch.ChannelConfig{
    "bot_token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
    "chat_id":   "123456789",       // 个人 chat_id
    // "chat_id": "@channel_name",  // 或频道用户名
    // "chat_id": "-100123456789",  // 或群组 chat_id
}
msg := &msgch.Message{Markdown: "**告警** 服务异常"}
result, err := msgch.Push(ctx, "telegram", cfg, msg)
```

## 消息格式

- **Markdown**（优先）：Telegram Markdown 格式（`*粗体*`、`_斜体_`、`` `代码` ``、`[链接](url)`）
- **HTML**（次选）：Telegram HTML 格式（`<b>粗体</b>`、`<i>斜体</i>`、`<a href="url">链接</a>`）
- **Text**（兜底）：纯文本

## 代理配置

在受限网络环境下，可配置自定义 API 地址：

```go
cfg := msgch.ChannelConfig{
    "bot_token": "xxxx",
    "chat_id":   "xxxx",
    "api_host":  "https://your-proxy.example.com", // 代理地址
}
```

## 注意事项

- 超长消息（>4096 字符）会自动分段发送
- Bot 需要先与用户/群组有交互才能发送消息（用户需先给 Bot 发消息或将 Bot 加入群组）
- Telegram Bot API 有频率限制（同一 chat_id 每秒最多 1 条，群组每分钟约 20 条）
- 在中国大陆需配置代理或自建 API 反代
