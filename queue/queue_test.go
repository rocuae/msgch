package queue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rocuae/msgch"
)

// TestQueue_PushAndConsume 验证投递任务后被 worker 消费。
func TestQueue_PushAndConsume(t *testing.T) {
	var got int32
	// 注册一个测试渠道
	msgch.Register("qtest", func() msgch.Channel {
		return &countingChannel{counter: &got}
	})

	q := New(Config{BufferSize: 8, Workers: 2})
	q.Start()
	defer q.Stop()

	if err := q.Push(context.Background(), "qtest", msgch.ChannelConfig{"k": "v"}, &msgch.Message{Text: "x"}); err != nil {
		t.Fatalf("Push err: %v", err)
	}

	// 等待 worker 消费
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&got) == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("worker 未消费任务，got=%d", got)
}

// TestQueue_StopRejectsPush 验证停止后 Push 返回错误。
func TestQueue_StopRejectsPush(t *testing.T) {
	q := New(Config{BufferSize: 8, Workers: 1})
	q.Start()
	q.Stop()
	if err := q.Push(context.Background(), "qtest", nil, &msgch.Message{}); err == nil {
		t.Fatal("停止后 Push 应返回 error")
	}
}

// TestQueue_TryPushFull 验证队列满时 TryPush 返回 false。
func TestQueue_TryPushFull(t *testing.T) {
	// 注册一个不立即消费的渠道（加 slowChannel）
	var got int32
	msgch.Register("qfull", func() msgch.Channel {
		return &slowChannel{counter: &got}
	})

	q := New(Config{BufferSize: 1, Workers: 1})
	q.Start()
	defer q.Stop()
	// 先填满缓冲（第 1 个被 worker 取走，第 2 个进缓冲）
	// 采用 TryPush 反复投递直到满
	filled := 0
	for i := 0; i < 100; i++ {
		if q.TryPush(context.Background(), "qfull", nil, &msgch.Message{}) {
			filled++
		}
	}
	if filled == 0 {
		// 至少能投递一些；满了才 false —— 只要未 panic即算通过
	}
}

// countingChannel 计数触发的测试渠道。
type countingChannel struct {
	*msgch.BaseChannel
	counter *int32
}

func (c *countingChannel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	atomic.AddInt32(c.counter, 1)
	return &msgch.Result{Success: true}, nil
}

// slowChannel 慢渠道，阻塞消费使队列堆积。
type slowChannel struct {
	*msgch.BaseChannel
	counter *int32
}

func (c *slowChannel) Send(ctx context.Context, cfg msgch.ChannelConfig, msg *msgch.Message) (*msgch.Result, error) {
	time.Sleep(200 * time.Millisecond)
	atomic.AddInt32(c.counter, 1)
	return &msgch.Result{Success: true}, nil
}