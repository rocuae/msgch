// Package wechat 实现个人微信（微信公众号/测试号模板消息）渠道。
//
// 渠道类型："wechat"
//
// 支持两种后端实现，通过 ChannelConfig["backend"] 选择：
//   - "api"（默认）：本库自实现的 HTTP + tokenstore，零第三方依赖，
//     access_token 经 tokenstore 缓存（同一 app_id+app_secret 共享）。
//   - "sdk"：基于 github.com/silenceper/wechat/v2 SDK 实现，自带 token 缓存
//     与成熟的公众号能力。选用此后端会引入 silenceper/wechat 第三方依赖。
//
// 凭证（ChannelConfig）：
//   - backend：后端实现，"api"(默认) 或 "sdk"
//   - app_id：公众号/测试号的 AppID
//   - app_secret：公众号/测试号的 AppSecret
//   - template_id：模板消息的模板 ID
//   - to_user：接收消息的微信用户 UserID（可被 msg.Extra["to_user"] 覆盖）
//   - (sdk 后端可选) cache：缓存类型，"memory"(默认)/"redis"，仅 sdk 后端认；
//       redis 时还需 redis_host/redis_port 等（见 sdk.go）
//
// 消息内容：
//   - 模板变量通过 msg.Extra 传入，格式 map[string]any，每项可为 string 或 {"value":...,"color":...}
//   - 若未显式传模板变量，则自动用 Title 填 title、Text/Markdown 填 content 作为兜底
//
// 参考：
//   微信测试号: https://mp.weixin.qq.com/debug/cgi-bin/sandbox?t=sandbox/login
//   稳定版接口凭据: https://developers.weixin.qq.com/doc/service/api/base/api_getstableaccesstoken.html
//   模板消息发送: https://developers.weixin.qq.com/doc/service/api/notify/template/api_sendtemplatemessage.html
//   SDK: https://github.com/silenceper/wechat
package wechat

import (
	"context"
	"fmt"

	"github.com/rocuae/msgch"
)

// 渠道类型标识。
const ChannelType = "wechat"

// 后端实现标识。
const (
	// BackendAPI 本库自实现（HTTP + tokenstore，零依赖）
	BackendAPI = "api"
	// BackendSDK 基于 silenceper/wechat v2 SDK
	BackendSDK = "sdk"
)

// Channel 个人微信（公众号模板消息）渠道。
//
// 通过 ChannelConfig["backend"] 在 api 与 sdk 两种实现间分派。
type Channel struct{ *msgch.BaseChannel }

func init() {
	msgch.Register(ChannelType, func() msgch.Channel {
		return &Channel{BaseChannel: msgch.NewBase(ChannelType, []msgch.Format{
			msgch.FormatText,
		})}
	})
}

// sender 抽象发送实现，apiSender 与 sdkSender 各自实现。
//
// 设计目的：让 Channel.Send 依据 cfg["backend"] 分派，调用方可按需选择
// 零依赖的 api 实现或功能更全的 sdk 实现，而 Channel 接口对外不变。
type sender interface {
	// Send 执行发送，返回结果与系统错误（发送失败经 Result.Success=false 表达）。
	Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error)
}

// Send 实现 msgch.Channel。
//
// params:
//   - ctx：超时/取消
//   - cfg：渠道认证配置（KV）；backend 字段决定使用 api 或 sdk 实现
//   - msg：统一消息模型（Extra 可带模板变量与 to_user 覆盖）
//
// returns:
//   - *msgch.Result：Success=false 表示发送失败（厂商错误码），Err!=nil 表示系统错误
//   - error：系统错误透传
func (c *Channel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	return c.selectSender(cfg).Send(ctx, cfg, msg)
}

// selectSender 依据配置选择后端实现，默认 api。
func (c *Channel) selectSender(cfg msgch.ChannelConfig) sender {
	switch cfg.GetString("backend") {
	case BackendSDK:
		return sdkSenderInstance
	default:
		return apiSenderInstance
	}
}

// resolveToUser 解析收件人 OpenID：优先 msg.Extra["to_user"]，回退 cfg["to_user"]。
func resolveToUser(cfg msgch.ChannelConfig, msg *msgch.Message) string {
	if msg != nil && msg.Extra != nil {
		if v, ok := msg.Extra["to_user"].(string); ok && v != "" {
			return v
		}
	}
	return cfg.GetString("to_user")
}

// buildTemplateData 构造模板消息 data 体：
//   - 若 msg.Extra 已含非 to_user 的键，则用 Extra 作为模板变量（兼容 string 与 {"value":...} 两种形式）
//   - 否则用 Title 填 title、内容(Text/Markdown)填 content 作为兜底
//
// 各后端实现共用此逻辑，仅最终序列化形式略有差异。
func buildTemplateData(msg *msgch.Message) map[string]interface{} {
	data := make(map[string]interface{})
	// 兜底：title / content
	if msg.Title != "" {
		data["title"] = map[string]string{"value": msg.Title}
	}
	_, content := msg.FormatContent([]msgch.Format{msgch.FormatText, msgch.FormatMarkdown, msgch.FormatHTML})
	if content != "" {
		data["content"] = map[string]string{"value": content}
	}
	// Extra 覆盖/追加
	if msg != nil && msg.Extra != nil {
		for k, v := range msg.Extra {
			if k == "to_user" {
				continue
			}
			switch x := v.(type) {
			case string:
				data[k] = map[string]string{"value": x}
			case map[string]interface{}:
				data[k] = x
			default:
				// 其他类型转字符串
				data[k] = map[string]string{"value": strTrim(fmt.Sprintf("%v", v))}
			}
		}
	}
	return data
}