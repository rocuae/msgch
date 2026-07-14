# WxPusher

> 渠道类型：`wxpusher`

## 参数申请方法

1. 访问 [WxPusher](https://wxpusher.zjiecode.com/)，注册账号
2. 创建应用，获得 **App Token**
3. 用户扫码关注应用（微信扫码后关注 WxPusher 公众号，自动绑定）
4. 获取接收人 UID：
   - 应用管理 → 关注者管理 → 查看用户 UID
5. 可选：创建 Topic（主题），批量推送给多个用户

### 获取凭证的步骤

1. 访问 https://wxpusher.zjiecode.com/
2. 注册/登录
3. 应用管理 → 创建应用 → 获得 AppToken
4. 用户扫码关注后，在"关注者管理"获取 UID
5. 如需批量推送，创建 Topic 并让用户订阅

## 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `app_token` | ✅ | 应用 AppToken |
| `uids` | ❌ | 接收人 UID 列表（逗号分隔），与 `topic_ids` 至少填一个 |
| `topic_ids` | ❌ | 主题 ID 列表（逗号分隔），推送给订阅该主题的所有用户 |
| `content_type` | ❌ | 内容类型：`1`=文本（默认）、`2`=HTML、`3`=Markdown |

## 代码示例

```go
cfg := msgch.ChannelConfig{
    "app_token": "AT_xxxx",
    "uids":      "UID_xxxx",                  // 单个用户
    // "uids":   "UID_aaa,UID_bbb,UID_ccc",  // 多个用户
    // "topic_ids": "1234",                   // 或推送给主题
}
msg := &msgch.Message{
    Title:    "告警通知",
    Markdown: "**CPU 使用率** 90%",
}
result, err := msgch.Push(ctx, "wxpusher", cfg, msg)
```

## 消息格式

- **Markdown**（优先）：content_type=3
- **HTML**（次选）：content_type=2
- **Text**（兜底）：content_type=1

msg.Title 作为消息摘要（列表预览），msg.Text/HTML/Markdown 作为消息正文。

## 注意事项

- 用户需主动扫码关注才能收到消息
- 每个应用最多 200 个关注者（免费版）
- 每天发送次数有限制（具体看套餐）
- Topic 适合批量推送场景（如系统公告）
- AppToken 不会过期
