# Discord Webhook

> 渠道类型：`discord`

## 参数申请方法

1. 打开 Discord，进入目标服务器（需有管理频道权限）
2. 右键点击目标频道 → 编辑频道 → 整合 → Webhook → 新建 Webhook
3. 设置 Webhook 名称和头像
4. 点击"复制 Webhook URL"，格式：
   `https://discord.com/api/webhooks/xxxx/yyyy`
5. 完整 URL 即为所需凭证

### 获取 Webhook URL 的步骤

1. Discord 桌面端/网页端，进入服务器
2. 右键频道 → 编辑频道
3. 左侧菜单 → 整合
4. Webhook → 新建 Webhook
5. 自定义名称 → 复制 Webhook URL

## 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `webhook_url` | ✅ | Discord Webhook 完整 URL |

## 代码示例

```go
cfg := msgch.ChannelConfig{
    "webhook_url": "https://discord.com/api/webhooks/xxxx/yyyy",
}
msg := &msgch.Message{Markdown: "**告警** 服务异常\n\n请及时处理"}
result, err := msgch.Push(ctx, "discord", cfg, msg)
```

## 消息格式

- **Markdown**（优先）：Discord Markdown（`**粗体**`、`*斜体*`、`` `代码` ``、`||隐藏||`）
- **Text**（兜底）：纯文本

## @提醒

```go
msg := &msgch.Message{
    Text: "请关注",
    At: &msgch.AtInfo{
        UserIDs: []string{"123456789"}, // Discord 用户 ID
        AtAll:   true,                   // @everyone
    },
}
```

Discord @ 语法自动转换：
- `AtAll: true` → `@everyone`
- `UserIDs` → `<@user_id>`

## 注意事项

- Webhook URL 包含敏感信息，不要泄露（他人可利用它发送消息）
- 成功发送返回 HTTP 204 No Content
- Discord 消息长度限制 2000 字符
- Webhook 无需 Bot 权限，但需要频道管理权限才能创建
- 可在 Webhook 设置中自定义名称和头像
- 同一频道可创建多个 Webhook
