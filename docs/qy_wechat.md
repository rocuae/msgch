# 企业微信

> 渠道类型：`qy_wechat`（群机器人）、`qy_wechat_app`（应用消息）

## 一、群机器人（qy_wechat）

### 参数申请方法

1. 打开企业微信桌面端/移动端，进入目标群 → 群机器人 → 添加
2. 设置机器人名称，点击"添加"
3. 获得 Webhook 地址，格式：
   `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxx`
4. 从 URL 中提取 `key` 参数即为所需凭证

### 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `key` | ✅ | Webhook URL 中的 key 参数（或用 `webhook_url` 传完整地址） |

### 代码示例

```go
cfg := msgch.ChannelConfig{"key": "xxxx"}
msg := &msgch.Message{Markdown: "**部署通知** 服务已上线"}
result, err := msgch.Push(ctx, "qy_wechat", cfg, msg)
```

### 消息格式

- **Markdown**（优先）：msg.Markdown 直接发送
- **Text**（兜底）：纯文本

### @提醒

```go
msg := &msgch.Message{
    Text: "请关注",
    At: &msgch.AtInfo{
        Mobiles: []string{"13800138000"}, // @手机号
        AtAll:   true,                    // @所有人
    },
}
```

### 注意事项

- 群机器人每分钟最多发送 20 条消息
- Markdown 格式支持有限（不支持图片、表格等复杂语法）
- key 仅用于标识机器人，不会过期

---

## 二、应用消息（qy_wechat_app）

### 参数申请方法

1. 登录 [企业微信管理后台](https://work.weixin.qq.com/wework_admin/frame)
2. 应用管理 → 自建 → 创建应用
3. 记录以下信息：
   - **AgentId**：应用详情页可见
   - **Secret**：应用详情页可见（点击"查看"）
4. 在"我的企业"页获取 **企业 ID**（CorpID）
5. 在"通讯录"中找到接收用户的 **UserID**
6. 如需接收消息的用户不在应用可见范围内，需在应用设置中添加可见范围

### 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `corp_id` | ✅ | 企业 ID（CorpID），管理后台"我的企业"页获取 |
| `corp_secret` | ✅ | 应用 Secret，应用详情页获取 |
| `agent_id` | ✅ | 应用 AgentId，应用详情页获取 |
| `to_user` | ✅ | 接收人 UserID，多个用 `\|` 分隔；`@all` 发给所有人 |

> `to_user` 可被 `msg.Extra["to_user"]` 覆盖。

### 代码示例

```go
cfg := msgch.ChannelConfig{
    "corp_id":     "wwxxxx",
    "corp_secret": "xxxx",
    "agent_id":    "1000002",
    "to_user":     "UserID1|UserID2", // 多人用 | 分隔
}
msg := &msgch.Message{Markdown: "**告警** 服务异常"}
result, err := msgch.Push(ctx, "qy_wechat_app", cfg, msg)
```

### 消息格式

- **Markdown**（优先）：msgtype=markdown
- **Text**（兜底）：msgtype=text
- **TextCard**（有 Title + URL 时）：msgtype=textcard，带标题、描述和跳转链接

```go
msg := &msgch.Message{
    Title: "查看详情",
    Text:  "告警详情描述",
    URL:   "https://alert.example.com/123",
}
// 自动选择 textcard 格式
```

### Token 管理

`access_token` 经 tokenstore 自动缓存与刷新（同一 `corp_id+corp_secret` 共享），调用方无需处理。

### 注意事项

- 接收人必须在应用的可见范围内（管理后台配置）
- 每个应用有消息发送频率限制（默认每分钟 200 条）
- Secret 建议定期轮换（管理后台可重置）
- 发送给非应用可见用户会返回 errcode=30202 错误
