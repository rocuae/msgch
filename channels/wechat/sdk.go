// Package wechat 的 sdk 后端：基于 github.com/silenceper/wechat/v2 SDK 实现。
//
// 采用 silenceper/wechat v2 的 officialaccount 模块发送模板消息：
//   - SDK 自带 access_token 缓存（经 Cache 接口，默认内存）
//   - 调用方无需自行处理 token 刷新
//   - 选用此后端会引入 silenceper/wechat 第三方依赖
//
// 与 api 后端相比，sdk 后端功能更全（如需扩展菜单/用户/素材等能力可直接复用 SDK 实例）。
// 调用方按需选择：追求零依赖用 backend="api"（默认），追求功能完整用 backend="sdk"。
//
// SDK 参考：https://github.com/silenceper/wechat
package wechat

import (
	"context"
	"fmt"

	"github.com/rocuae/msgch"

	wechatSDK "github.com/silenceper/wechat/v2"
	"github.com/silenceper/wechat/v2/cache"
	offConfig "github.com/silenceper/wechat/v2/officialaccount/config"
	"github.com/silenceper/wechat/v2/officialaccount/message"
)

// sdkSender 基于 silenceper/wechat v2 SDK 的后端实现。
type sdkSender struct{}

// sdkSenderInstance 单例，供 Channel.selectSender 返回。
var sdkSenderInstance sender = sdkSender{}

// Send 实现 sender 接口。
//
// params:
//   - ctx：超时/取消（SDK 的 Send 当前未接受 ctx，超时需调用方自行控制）
//   - cfg：渠道认证配置（app_id/app_secret/template_id/to_user；可选 cache=memory`)
//   - msg：统一消息模型（Extra 可带模板变量与 to_user 覆盖）
//
// returns:
//   - *msgch.Result：Success=false 表示发送失败（厂商错误码），Err!=nil 表示系统错误
func (sdkSender) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	appID := cfg.GetString("app_id")
	appSecret := cfg.GetString("app_secret")
	templateID := cfg.GetString("template_id")
	if appID == "" || appSecret == "" || templateID == "" {
		return failResult("missing app_id/app_secret/template_id"), nil
	}
	toUser := resolveToUser(cfg, msg)
	if toUser == "" {
		return failResult("missing to_user"), nil
	}

	// 构造公众号配置：AppID/AppSecret/Cache（默认内存缓存）
	ocfg := &offConfig.Config{
		AppID:     appID,
		AppSecret: appSecret,
		// 默认使用稳定版接口取 token（与 api 后端保持一致行为）
		UseStableAK: true,
	}
	// 缓存：默认内存；调用方可经 cache="memory" 显式指定（后续可扩展 redis）
	// SDK 的 Cache 必须非 nil，否则取 token 时会 panic
	ocfg.Cache = cache.NewMemory()

	// 创建 SDK 公众号实例并取模板消息组件
	wc := wechatSDK.NewWechat()
	oa := wc.GetOfficialAccount(ocfg)
	tpl := oa.GetTemplate()

	// 组装 TemplateMessage：模板变量需转为 SDK 的 *TemplateDataItem 形式
	tplMsg := &message.TemplateMessage{
		ToUser:     toUser,
		TemplateID: templateID,
		URL:        msg.URL,
		Data:       buildSDKTemplateData(msg),
	}

	// 发送（SDK 内部完成取 token + POST + 错误解析）
	msgID, err := tpl.Send(tplMsg)
	if err != nil {
		// SDK 返回 error 含 errcode/errmsg，作为发送失败返回（非系统错误）
		return failResult(fmt.Sprintf("sdk send error: %v", err)), nil
	}
	_ = ctx // SDK Send 暂未接受 ctx，接口预留
	return successResult(fmt.Sprintf("msgid=%d", msgID)), nil
}

// buildSDKTemplateData 构造 SDK 模板变量（map[string]*TemplateDataItem，值为指针）。
// 复用 buildTemplateData 的逻辑，转为 SDK 要求的指针形式。
func buildSDKTemplateData(msg *msgch.Message) map[string]*message.TemplateDataItem {
	raw := buildTemplateData(msg)
	out := make(map[string]*message.TemplateDataItem, len(raw))
	for k, v := range raw {
		switch x := v.(type) {
		case map[string]string:
			item := &message.TemplateDataItem{Value: x["value"], Color: x["color"]}
			out[k] = item
		case map[string]interface{}:
			item := &message.TemplateDataItem{}
			if s, ok := x["value"].(string); ok {
				item.Value = s
			} else {
				item.Value = fmt.Sprintf("%v", x["value"])
			}
			if s, ok := x["color"].(string); ok {
				item.Color = s
			}
			out[k] = item
		default:
			out[k] = &message.TemplateDataItem{Value: fmt.Sprintf("%v", v)}
		}
	}
	return out
}