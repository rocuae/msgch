# 个人微信公众号模板消息

> 渠道类型：`wechat`
> 支持两种后端：API（默认，零依赖）/ SDK（silenceper/wechat v2）

## 参数申请方法

### 1. 注册公众号

1. 访问 [微信公众平台](https://mp.weixin.qq.com/)，注册公众号（个人订阅号即可）
2. 注册完成后在"开发 → 基本配置"获取 **AppID** 和 **AppSecret**

### 2. 申请测试号（推荐开发测试用）

1. 访问 [微信测试号平台](https://mp.weixin.qq.com/debug/cgi-bin/sandbox?t=sandbox/login)
2. 扫码登录后获得 **appID** 和 **appsecret**
3. 测试号无权限限制，适合开发调试

### 3. 获取模板 ID

1. 在公众号后台"功能 → 模板消息"中申请模板
2. 或在测试号管理页面"模板消息"中添加模板
3. 记录 **模板 ID**（template_id）

### 4. 获取用户 OpenID

1. 用户关注公众号后，通过 OAuth 或事件推送获取 OpenID
2. 测试号可在管理页面手动添加测试用户

### 5. 配置服务器地址（如需接收消息）

在"开发 → 基本配置 → 服务器配置"中填写服务器 URL（仅接收消息需要，发送不需要）

## 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `app_id` | ✅ | 公众号/测试号 AppID |
| `app_secret` | ✅ | 公众号/测试号 AppSecret |
| `template_id` | ✅ | 模板消息的模板 ID |
| `to_user` | ✅ | 接收人 OpenID（可被 `msg.Extra["to_user"]` 覆盖） |
| `backend` | ❌ | 后端选择：`api`（默认）或 `sdk` |

## 代码示例

### API 后端（默认，零第三方依赖）

```go
cfg := msgch.ChannelConfig{
    "app_id":      "wx123456",
    "app_secret":  "xxxx",
    "template_id": "TEMPLATE_ID",
    "to_user":     "openid_xxx",
}
msg := &msgch.Message{
    Title: "验证码通知",
    Extra: map[string]any{
        "code":    "123456",
        "product": "goMessage",
    },
}
result, err := msgch.Push(ctx, "wechat", cfg, msg)
```

### SDK 后端（基于 silenceper/wechat v2）

```go
cfg := msgch.ChannelConfig{
    "backend":     "sdk",
    "app_id":      "wx123456",
    "app_secret":  "xxxx",
    "template_id": "TEMPLATE_ID",
    "to_user":     "openid_xxx",
}
msg := &msgch.Message{
    Title: "验证码通知",
    Extra: map[string]any{"code": "123456"},
}
result, err := msgch.Push(ctx, "wechat", cfg, msg)
```

### 覆盖接收人

```go
cfg := msgch.ChannelConfig{
    "app_id":      "wx123456",
    "app_secret":  "xxxx",
    "template_id": "TEMPLATE_ID",
    "to_user":     "default_openid",
}
msg := &msgch.Message{
    Title: "通知",
    Extra: map[string]any{
        "to_user": "other_openid", // 覆盖 cfg 中的 to_user
        "code":    "654321",
    },
}
msgch.Push(ctx, "wechat", cfg, msg)
```

## 消息格式

仅支持 **Text**（模板消息）。模板变量通过 `msg.Extra` 传入：

- `msg.Title` → 模板中的 `title` 变量（自动填入）
- `msg.Text`/`msg.Markdown` → 模板中的 `content` 变量（自动填入）
- `msg.Extra` 中的其他键 → 对应模板变量（如 `code`→`${code}`）

## 模板变量示例

假设模板内容为：

```
{{title.DATA}}
验证码：{{code.DATA}}
产品：{{product.DATA}}
```

发送方式：

```go
msg := &msgch.Message{
    Title: "验证码通知",                    // → title
    Extra: map[string]any{
        "code":    "123456",              // → code
        "product": "goMessage",           // → product
    },
}
```

## 两种后端对比

| 维度 | API 后端（默认） | SDK 后端 |
| --- | --- | --- |
| 依赖 | 零第三方依赖 | 引入 silenceper/wechat v2 |
| Token 管理 | tokenstore 缓存 | SDK 自带缓存 |
| 功能 | 仅模板消息 | SDK 提供公众号全部能力 |
| 推荐场景 | 只发模板消息、追求零依赖 | 需要更多公众号能力（菜单/用户/素材等） |

## Token 管理

API 后端：`access_token` 经 tokenstore 自动缓存与刷新（同一 `app_id+app_secret` 共享），使用稳定版接口 `/cgi-bin/stable_token`。

SDK 后端：SDK 自带 token 缓存机制。

## 注意事项

- 模板消息需先在公众号后台申请模板，获得 template_id
- 每个模板有字段数量限制（通常 5~7 个变量）
- 用户需关注公众号才能收到模板消息
- 测试号无关注人数限制，正式号需认证后才能使用模板消息
- access_token 有效期 2 小时，tokenstore 自动管理
- 每个用户每天最多接收同一条模板消息的次数有限（取决于模板类型）
