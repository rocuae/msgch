package msgch

import (
	"context"
	"errors"
	"testing"
)

// fakeChannel 测试用假渠道。
type fakeChannel struct {
	*BaseChannel
	sendFn func(ctx context.Context, cfg ChannelConfig, msg *Message) (*Result, error)
}

func (f *fakeChannel) Send(ctx context.Context, cfg ChannelConfig, msg *Message) (*Result, error) {
	return f.sendFn(ctx, cfg, msg)
}

// TestRegisterAndGet 验证注册表自注册与取实例。
func TestRegisterAndGet(t *testing.T) {
	Register("fake_test", func() Channel {
		return &fakeChannel{
			BaseChannel: NewBase("fake_test", []Format{FormatText}),
			sendFn: func(ctx context.Context, cfg ChannelConfig, msg *Message) (*Result, error) {
				return &Result{Success: true, Response: "ok"}, nil
			},
		}
	})

	ch, err := Get("fake_test")
	if err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if ch.Type() != "fake_test" {
		t.Fatalf("Type 不正确: %s", ch.Type())
	}
}

// TestGet_NotRegistered 未注册渠道应返回 ErrChannelNotRegistered。
func TestGet_NotRegistered(t *testing.T) {
	_, err := Get("not_exists_xxx")
	if !errors.Is(err, ErrChannelNotRegistered) {
		t.Fatalf("期望 ErrChannelNotRegistered，实际 %v", err)
	}
}

// TestPush 验证 Push 统一入口。
func TestPush(t *testing.T) {
	Register("fake_push", func() Channel {
		return &fakeChannel{
			BaseChannel: NewBase("fake_push", []Format{FormatText}),
			sendFn: func(ctx context.Context, cfg ChannelConfig, msg *Message) (*Result, error) {
				if cfg.GetString("token") != "abc" {
					return &Result{Success: false, Response: "bad token"}, nil
				}
				return &Result{Success: true, Response: "sent"}, nil
			},
		}
	})

	r, err := Push(context.Background(), "fake_push", ChannelConfig{"token": "abc"}, &Message{Text: "hi"})
	if err != nil {
		t.Fatalf("Push err: %v", err)
	}
	if !r.Success {
		t.Fatalf("期望成功，实际 %s", r.Response)
	}

	// 错误凭证应返回发送失败
	r, _ = Push(context.Background(), "fake_push", ChannelConfig{"token": "wrong"}, &Message{Text: "hi"})
	if r.Success {
		t.Fatal("错误凭证应 Success=false")
	}
}

// TestPush_SystemError 渠道返回系统错误应透传。
func TestPush_SystemError(t *testing.T) {
	Register("fake_syserr", func() Channel {
		return &fakeChannel{
			BaseChannel: NewBase("fake_syserr", []Format{FormatText}),
			sendFn: func(ctx context.Context, cfg ChannelConfig, msg *Message) (*Result, error) {
				return nil, errors.New("network down")
			},
		}
	})
	_, err := Push(context.Background(), "fake_syserr", nil, &Message{Text: "x"})
	if err == nil || err.Error() != "network down" {
		t.Fatalf("期望 network down error，实际 %v", err)
	}
}

// TestBroadcast 验证多渠道广播聚合。
func TestBroadcast(t *testing.T) {
	Register("fake_b1", func() Channel {
		return &fakeChannel{NewBase("fake_b1", []Format{FormatText}), func(ctx context.Context, cfg ChannelConfig, msg *Message) (*Result, error) {
			return &Result{Success: true, Response: "1"}, nil
		}}
	})
	Register("fake_b2", func() Channel {
		return &fakeChannel{NewBase("fake_b2", []Format{FormatText}), func(ctx context.Context, cfg ChannelConfig, msg *Message) (*Result, error) {
			return &Result{Success: false, Response: "2"}, nil
		}}
	})

	targets := []Target{
		{Type: "fake_b1"},
		{Type: "fake_b2"},
		{Type: "not_registered"},
	}
	br := Broadcast(context.Background(), targets, &Message{Text: "x"})
	if len(br.Results) != 3 {
		t.Fatalf("期望 3 个结果，实际 %d", len(br.Results))
	}
	if br.SuccessCount() != 1 {
		t.Fatalf("期望成功 1 个，实际 %d", br.SuccessCount())
	}
	if br.Results[2].Err == nil {
		t.Fatal("第 3 个未注册渠道应有 Err")
	}
}

// TestClient_DefaultConfig 验证 Client 默认配置合并与调用期覆盖。
func TestClient_DefaultConfig(t *testing.T) {
	Register("fake_cfg", func() Channel {
		return &fakeChannel{NewBase("fake_cfg", []Format{FormatText}), func(ctx context.Context, cfg ChannelConfig, msg *Message) (*Result, error) {
			v := cfg.GetString("k")
			if v == "" {
				v = "<empty>"
			}
			return &Result{Success: true, Response: v}, nil
		}}
	})

	cli := New()
	cli.SetDefaultConfig("fake_cfg", ChannelConfig{"k": "default"})

	// 调用期未传 cfg → 用默认
	r, _ := cli.Push(context.Background(), "fake_cfg", nil, &Message{Text: "x"})
	if r.Response != "default" {
		t.Fatalf("期望 default，实际 %s", r.Response)
	}
	// 调用期传 cfg 覆盖默认
	r, _ = cli.Push(context.Background(), "fake_cfg", ChannelConfig{"k": "override"}, &Message{Text: "x"})
	if r.Response != "override" {
		t.Fatalf("期望 override，实际 %s", r.Response)
	}
}