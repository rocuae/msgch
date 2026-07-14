# Bark

> 渠道类型：`bark`

## 参数申请方法

1. 在 App Store 搜索并安装 **Bark**（iOS 推送工具）
2. 打开 Bark App，首页会显示你的推送地址，格式：
   `https://api.day.app/xxxx`
3. URL 末尾即为你的 **device_key**
4. 如有自建 Bark 服务端，使用自建地址作为 `server`

### 获取 device_key 的步骤

1. 打开 Bark App
2. 首页顶部显示推送 URL
3. 复制 URL，格式：`https://api.day.app/你的device_key`
4. 提取最后一段即为 device_key

## 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `server` | ✅ | Bark 服务端地址（官方 `https://api.day.app` 或自建地址） |
| `device_key` | ✅ | 设备 key，Bark App 首页推送 URL 末尾 |

## 代码示例

```go
cfg := msgch.ChannelConfig{
    "server":     "https://api.day.app",
    "device_key": "xxxx",
}
msg := &msgch.Message{
    Title: "告警通知",
    Text:  "CPU 使用率 90%",
    URL:   "https://alert.example.com/123", // 可选，点击通知跳转
}
result, err := msgch.Push(ctx, "bark", cfg, msg)
```

## 消息格式

- 仅支持 **Text**（纯文本）
- msg.Title → 通知标题
- msg.Text → 通知正文
- msg.URL → 点击跳转链接（可选）

## 注意事项

- Bark 仅支持 iOS 设备推送
- 每个 device_key 绑定一个 iOS 设备
- 推送频率无官方限制，但建议不要过于频繁
- 支持自建服务端：在 Bark App 设置中配置自建服务地址
- Bark 官方服务免费，自建需自行部署
