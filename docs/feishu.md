# 飞书

> 渠道类型：`feishu`（群机器人）、`feishu_app`（自建应用）

## 一、群机器人（feishu）

### 参数申请方法

1. 打开飞书桌面端，进入目标群 → 群设置 → 群机器人 → 添加机器人
2. 选择"自定义机器人"
3. 设置机器人名称和描述，点击"添加"
4. 获得 Webhook 地址，格式：
   `https://open.feishu.cn/open-apis/bot/v2/hook/xxxx`
5. 从 URL 末尾提取 token 即为所需凭证
6. 可选：设置"签名校验"，获得签名密钥

### 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `token` | ✅ | Webhook URL 末尾的 token（或直接用 `webhook_url` 传完整地址） |
| `secret` | ❌ | 签名密钥，开启"签名校验"后获得 |

### 代码示例

```go
// 方式一：用 token
cfg := msgch.ChannelConfig{
    "token":  "xxxx",
    "secret": "SECxxxx", // 可选
}

// 方式二：用完整 webhook_url
cfg := msgch.ChannelConfig{
    "webhook_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx",
}

msg := &msgch.Message{Text: "飞书消息内容"}
result, err := msgch.Push(ctx, "feishu", cfg, msg)
```

### 注意事项

- 群机器人每分钟最多发送 100 条消息，每秒最多 5 条
- 签名校验为可选安全设置，建议生产环境开启
- 消息格式为纯文本（飞书群机器人 text 类型）

---

## 二、自建应用（feishu_app）

### 参数申请方法

1. 访问 [飞书开放平台](https://open.feishu.cn/app)，注册/登录开发者账号
2. 创建企业自建应用，获得 App ID 和 App Secret
3. 在应用的"权限管理"中申请以下权限：
   - `im:message:send_as_bot`（以机器人身份发送消息）
4. 在"应用发布"中发布应用（需管理员审批）
5. 获取接收者 ID：
   - 个人消息：用户的 `open_id`（格式 `ou_xxx`），可通过飞书管理后台或 API 获取
   - 群消息：群的 `chat_id`（格式 `oc_xxx`），通过 API 获取

### 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `app_id` | ✅ | 飞书应用 App ID |
| `app_secret` | ✅ | 飞书应用 App Secret |
| `receive_id` | ✅ | 接收者 ID（open_id/user_id/chat_id 等） |
| `receive_id_type` | ❌ | 接收者类型，默认 `open_id`，可选 `user_id`/`union_id`/`chat_id` |

> 以上字段均可被 `msg.Extra` 同名字段覆盖。

### 代码示例

```go
cfg := msgch.ChannelConfig{
    "app_id":          "cli_xxxx",
    "app_secret":      "xxxx",
    "receive_id":      "ou_xxxx",
    "receive_id_type": "open_id",
}
msg := &msgch.Message{Markdown: "**告警** 服务异常"}
result, err := msgch.Push(ctx, "feishu_app", cfg, msg)
```

### Token 管理

`tenant_access_token` 经 tokenstore 自动缓存与刷新（同一 `app_id+app_secret` 共享），调用方无需处理。

### 注意事项

- 自建应用需管理员审批后才能正式使用
- 发消息需要先获取接收者的 open_id/chat_id，可通过飞书管理后台或通讯录 API 获取
- token 有效期通常 2 小时，tokenstore 会在过期前自动刷新
