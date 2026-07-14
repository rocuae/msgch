# 钉钉群机器人

> 渠道类型：`dingtalk`

## 参数申请方法

1. 打开钉钉桌面端/移动端，进入目标群 → 群设置 → 智能群助手 → 添加机器人
2. 选择"自定义（通过 Webhook 接入的自定义服务）"
3. 设置机器人名称，安全设置可选：
   - **自定义关键词**：消息必须包含指定关键词才能发送
   - **加签**：生成 `SEC` 开头的密钥（推荐，防止他人冒用）
   - **IP 地址（段）**：限制来源 IP
4. 创建后获得 Webhook 地址，格式：
   `https://oapi.dingtalk.com/robot/send?access_token=xxxx`
5. 从 URL 中提取 `access_token` 参数即为所需凭证

## 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `access_token` | ✅ | Webhook URL 中的 access_token 参数 |
| `secret` | ❌ | 加签密钥（SEC 开头），创建机器人时选择"加签"安全设置后获得 |

## 代码示例

```go
cfg := msgch.ChannelConfig{
    "access_token": "xxxx",
    "secret":       "SECxxxx", // 可选，选择加签安全设置时需要
}
msg := &msgch.Message{
    Title:    "告警通知",
    Markdown: "**CPU 使用率** 90%\n\n请及时处理",
}
result, err := msgch.Push(ctx, "dingtalk", cfg, msg)
```

## 消息格式

- **Markdown**（优先）：支持标题、粗体、链接、图片等，msg.Title 作为标题行
- **Text**（兜底）：纯文本

## @提醒

```go
msg := &msgch.Message{
    Text: "请关注",
    At: &msgch.AtInfo{
        UserIDs: []string{"user1", "user2"}, // @指定用户（需填 userId）
        AtAll:   true,                        // @所有人
    },
}
```

## 注意事项

- 钉钉机器人每分钟最多发送 20 条消息（超过会被限流）
- 安全设置选择"自定义关键词"时，消息内容必须包含该关键词，否则发送失败
- access_token 仅用于标识机器人，不会过期；secret 用于签名验证，也不会过期
- 群机器人无法主动发消息给个人，只能发到群里
