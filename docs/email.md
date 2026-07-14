# 邮件 SMTP

> 渠道类型：`email`

## 参数申请方法

使用任一邮箱的 SMTP 服务。常见邮箱的 SMTP 配置：

### QQ 邮箱

1. 登录 QQ 邮箱 → 设置 → 账户
2. 开启"POP3/SMTP 服务"
3. 按提示用手机发短信验证，获得**授权码**（非 QQ 密码）
4. SMTP 配置：
   - 服务器：`smtp.qq.com`
   - 端口：`465`（SSL）或 `587`（STARTTLS）
   - 账号：完整邮箱地址（如 `xxx@qq.com`）
   - 密码：授权码

### 163 邮箱

1. 登录 163 邮箱 → 设置 → POP3/SMTP/IMAP
2. 开启 SMTP 服务
3. 按提示验证，获得**授权码**
4. SMTP 配置：
   - 服务器：`smtp.163.com`
   - 端口：`465`（SSL）或 `25`（不加密）
   - 账号：完整邮箱地址
   - 密码：授权码

### Gmail

1. 登录 Google 账号 → 安全性 → 两步验证（需先开启）
2. 在"应用专用密码"中生成密码
3. SMTP 配置：
   - 服务器：`smtp.gmail.com`
   - 端口：`465`（SSL）或 `587`（STARTTLS）
   - 账号：Gmail 地址
   - 密码：应用专用密码

### 企业邮箱（腾讯企业邮/阿里企业邮等）

1. 登录企业邮箱管理后台
2. 开启 SMTP 服务
3. 获取 SMTP 服务器地址和端口
4. 通常使用邮箱密码或专用授权码

## 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `host` | ✅ | SMTP 服务器地址（如 `smtp.qq.com`） |
| `port` | ✅ | 端口：`25`（不加密）/ `465`（SSL）/ `587`（STARTTLS） |
| `account` | ✅ | 登录账号（通常为邮箱地址） |
| `passwd` | ✅ | 登录密码或授权码 |
| `from` | ❌ | 发件人地址（缺省时用 account） |
| `to` | ✅ | 收件人，多个用逗号分隔 |

## 代码示例

### 基本用法

```go
cfg := msgch.ChannelConfig{
    "host":    "smtp.qq.com",
    "port":    "465",
    "account": "sender@qq.com",
    "passwd":  "授权码",           // QQ 邮箱用授权码，非密码
    "from":    "sender@qq.com",   // 可选，默认用 account
    "to":      "receiver@163.com",
}
msg := &msgch.Message{
    Title: "告警通知",
    HTML:  "<h1>CPU 90%</h1><p>请及时处理</p>",
    Text:  "CPU 90%，请及时处理",
}
result, err := msgch.Push(ctx, "email", cfg, msg)
```

### 多收件人

```go
cfg := msgch.ChannelConfig{
    "host":    "smtp.qq.com",
    "port":    "465",
    "account": "sender@qq.com",
    "passwd":  "授权码",
    "to":      "a@b.com,c@d.com,e@f.com", // 逗号分隔
}
```

## 消息格式

- **HTML**（优先）：msg.HTML → 邮件正文（text/html）
- **Text**（兜底）：msg.Text → 邮件正文（text/plain）
- 两者都有时：发送 `multipart/alternative`（客户端自选显示）

msg.Title → 邮件主题 Subject（中文自动 RFC 2047 编码）

## 端口选择

| 端口 | 加密方式 | 说明 |
| --- | --- | --- |
| `25` | 不加密 | 传统 SMTP，部分云服务器封禁 |
| `465` | 隐式 SSL | 推荐，库自动处理 TLS 握手 |
| `587` | STARTTLS | Go 标准库 net/smtp 自动协商 |

## 注意事项

- QQ 邮箱/163 邮箱必须使用**授权码**，不能用登录密码
- Gmail 必须开启两步验证并使用应用专用密码
- 云服务器（如阿里云、腾讯云）可能封禁 25 端口出方向，建议用 465
- 中文主题自动使用 RFC 2047 B 编码（base64）
- 发件人地址 from 缺省时自动使用 account
- 多收件人用逗号或分号分隔均可
- 零第三方依赖，使用标准库 net/smtp
