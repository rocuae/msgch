// Package channels 集中空白导入所有内置渠道，触发其 init() 自注册。
//
// 调用方只需 import _ "github.com/rocuae/msgch/channels" 即可注册全部内置渠道；
// 如需减小二进制体积，也可按需只导入特定渠道子包。
package channels

import (
	// 首批：Webhook 机器人类
	_ "github.com/rocuae/msgch/channels/bark"
	_ "github.com/rocuae/msgch/channels/dingtalk"
	_ "github.com/rocuae/msgch/channels/discord"
	_ "github.com/rocuae/msgch/channels/feishu"
	_ "github.com/rocuae/msgch/channels/qy_wechat"
	_ "github.com/rocuae/msgch/channels/serverchan"
	_ "github.com/rocuae/msgch/channels/telegram"
	_ "github.com/rocuae/msgch/channels/webhook"
	_ "github.com/rocuae/msgch/channels/wxpusher"

	// 次批：需 token 渠道 + 短信 + 邮件
	_ "github.com/rocuae/msgch/channels/email"
	_ "github.com/rocuae/msgch/channels/sms"
	_ "github.com/rocuae/msgch/channels/wechat"
)