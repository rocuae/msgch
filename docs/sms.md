# 短信

> 渠道类型：`sms_aliyun`（阿里云短信）、`sms_tencent`（腾讯云短信）

## 短信渠道特殊说明

短信与 IM 机器人不同：**内容受模板约束**，发送需 templateCode + 模板变量，正文不直接发送（模板已在云端配置）。模板变量经 `msg.Extra` 传入。

---

## 一、阿里云短信（sms_aliyun）

### 参数申请方法

1. 登录 [阿里云控制台](https://dysmsnext.console.aliyun.com/)
2. 开通短信服务
3. 在"国内消息 → 签名管理"中申请短信签名（需审核，通常 2 小时内）
4. 在"国内消息 → 模板管理"中申请短信模板（需审核）
5. 在"AccessKey 管理"中创建 AccessKey，获得 **AccessKey ID** 和 **AccessKey Secret**
6. 记录以下信息：
   - **签名名称**（SignName）
   - **模板 Code**（TemplateCode，如 `SMS_123456`）
   - **AccessKey ID** 和 **AccessKey Secret**

### 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `access_key_id` | ✅ | 阿里云 AccessKey ID |
| `access_key_secret` | ✅ | 阿里云 AccessKey Secret |
| `sign_name` | ✅ | 短信签名（已审核通过的签名名称） |
| `template_code` | ✅ | 短信模板 Code（如 `SMS_123456`） |
| `phone_number` | ✅ | 接收手机号（如 `13800138000`） |
| `region_id` | ❌ | 区域，默认 `cn-hangzhou` |

### 代码示例

```go
cfg := msgch.ChannelConfig{
    "access_key_id":     "LTAIxxxx",
    "access_key_secret": "xxxx",
    "sign_name":         "goMessage",
    "template_code":     "SMS_123456",
    "phone_number":      "13800138000",
}
// 模板变量经 Extra 传入（对应模板中的 ${code} ${product}）
msg := &msgch.Message{
    Extra: map[string]any{
        "code":    "123456",
        "product": "goMessage",
    },
}
result, err := msgch.Push(ctx, "sms_aliyun", cfg, msg)
```

### 注意事项

- 签名和模板需审核通过后才能使用
- 每个签名对应一个业务场景（如"验证码"、"通知"等）
- 模板变量格式：模板中写 `${code}`，Extra 中传 `"code": "123456"`
- 国内短信和国际短信使用不同 API
- 短信有发送频率限制（默认每手机号每分钟 1 条，每小时 5 条，每天 10 条）
- 建议使用子账号 AccessKey 并限制短信服务权限

---

## 二、腾讯云短信（sms_tencent）

### 参数申请方法

1. 登录 [腾讯云控制台](https://console.cloud.tencent.com/smsv2)
2. 开通短信服务
3. 在"应用管理"中创建应用，获得 **SDK AppID**
4. 在"国内短信 → 签名管理"中申请签名（需审核）
5. 在"国内短信 → 正文模板管理"中申请模板（需审核）
6. 在"访问管理 → API 密钥管理"中创建密钥，获得 **SecretId** 和 **SecretKey**
7. 记录以下信息：
   - **SDK AppID**
   - **签名内容**（SignName）
   - **模板 ID**（TemplateId，纯数字）
   - **SecretId** 和 **SecretKey**

### 配置字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `secret_id` | ✅ | 腾讯云 SecretId |
| `secret_key` | ✅ | 腾讯云 SecretKey |
| `sign_name` | ✅ | 短信签名（已审核通过） |
| `template_id` | ✅ | 短信模板 ID（纯数字） |
| `phone_number` | ✅ | 手机号（需带国际区号，如 `+8613800138000`） |
| `sdk_app_id` | ✅ | 短信应用 SDK AppID |
| `region` | ❌ | 区域，默认 `ap-guangzhou` |

### 代码示例

```go
cfg := msgch.ChannelConfig{
    "secret_id":    "AKIDxxxx",
    "secret_key":   "xxxx",
    "sign_name":    "goMessage",
    "template_id":  "123456",
    "phone_number": "+8613800138000",
    "sdk_app_id":   "1400000000",
}
// 模板变量经 Extra 传入，按 key 字典序对应模板占位符 {1}{2}...
msg := &msgch.Message{
    Extra: map[string]any{
        "1": "123456",    // → {1}
        "2": "goMessage", // → {2}
    },
}
result, err := msgch.Push(ctx, "sms_tencent", cfg, msg)
```

### 模板变量说明

腾讯云短信模板参数为**有序列表**（`{1}` `{2}` ...），不是命名变量。msg.Extra 中的键按**字典序排序**后转为字符串数组：

- Extra `{"1":"123456","2":"goMessage"}` → `["123456","goMessage"]`
- 对应模板中的 `{1}=123456`、`{2}=goMessage`

### 注意事项

- 签名和模板需审核通过后才能使用
- 手机号必须带国际区号前缀（如 `+86`）
- SDK AppID 在"应用管理"中获取
- 短信有发送频率限制（默认每手机号每分钟 1 条）
- 建议使用子账号密钥并限制短信服务权限
- 签名 v3（TC3-HMAC-SHA256）由库内部自动处理，调用方无需关心
