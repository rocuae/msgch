package msgch

import (
	"context"
	"sync"
)

// doBroadcast 并发向多个目标发送，聚合各渠道结果。
// 若 cli 非 nil，则用其默认配置合并；否则直接用 target.Config。
func doBroadcast(ctx context.Context, targets []Target, msg *Message, opts []SendOption, cli *Client) *BroadcastResult {
	results := make([]TargetResult, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(i int, t Target) {
			defer wg.Done()
			var cfg ChannelConfig
			if cli != nil {
				cfg = cli.mergeConfigForTarget(t)
			} else {
				cfg = t.Config
			}
			r, err := doSend(ctx, t.Type, cfg, msg, opts)
			results[i] = TargetResult{Type: t.Type, Result: r, Err: err}
		}(i, t)
	}
	wg.Wait()
	return &BroadcastResult{Results: results}
}
