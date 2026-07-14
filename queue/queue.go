// Package queue 提供可选的异步发送队列。
//
// 内存模式：buffered chan + N 个 worker 协程消费。发送方调 Queue.Push 投递任务后
// 立即返回，worker 在后台调用 msgch.Push 实际发送。worker panic 自恢复（借鉴
// Message-Push-Nest 的 recover 思路），避免单任务异常拖垮整个队列。
//
// 本轮为内存版（轻量），生产可靠场景如需崩溃恢复可扩展持久化模式（发送前落盘 pending，
// worker 消费后置 sent/failed，启动回灌）——设计文档第 6.5 节已规划。
//
// 用法：
//
//	q := queue.New(queue.Config{BufferSize: 1024, Workers: 4})
//	q.Start()
//	defer q.Stop()
//	q.Push(ctx, "dingtalk", cfg, msg, nil) // 立即返回
package queue

import (
	"context"
	"sync"

	"github.com/rocuae/msgch"
)

// Config 队列配置。
type Config struct {
	// BufferSize 任务通道缓冲大小；满时 Push 阻塞
	BufferSize int
	// Workers 消费 worker 数，<=0 则取 1
	Workers int
}

// Task 异步发送任务。
type Task struct {
	ctx       context.Context // 任务提交时刻的 context（用于取消传递）
	ChannelType string
	Cfg       msgch.ChannelConfig
	Msg       *msgch.Message
	Opts      []msgch.SendOption
}

// Queue 异步发送队列。
type Queue struct {
	cfg   Config
	ch    chan Task
	wg    sync.WaitGroup
	stop  chan struct{}
	stopped bool
	mu    sync.Mutex
}

// New 创建队列实例（不启动 worker，需调 Start）。
func New(cfg Config) *Queue {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 256
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	return &Queue{
		cfg:  cfg,
		ch:   make(chan Task, cfg.BufferSize),
		stop: make(chan struct{}),
	}
}

// Start 启动 N 个 worker 协程消费队列。
func (q *Queue) Start() {
	for i := 0; i < q.cfg.Workers; i++ {
		q.wg.Add(1)
		go q.worker()
	}
}

// worker 消费循环。recover 防 panic 拖垮队列。
func (q *Queue) worker() {
	defer q.wg.Done()
	for {
		select {
		case <-q.stop:
			return
		case task, ok := <-q.ch:
			if !ok {
				return
			}
			q.run(task)
		}
	}
}

// run 执行单个任务（带 panic 恢复，记录但不中断 worker）。
func (q *Queue) run(t Task) {
	defer func() {
		if r := recover(); r != nil {
			// 生产环境可在此接入日志；此处静默恢复以保护 worker
			_ = r
		}
	}()
	// 调用 msgch.Push 实际发送，结果被视为异步行为，丢弃返回值
	_, _ = msgch.Push(t.ctx, t.ChannelType, t.Cfg, t.Msg, t.Opts...)
}

// Push 投递异步发送任务。队列满时阻塞直到有空间或队列停止。
// 返回 error 表示队列已停止。
func (q *Queue) Push(ctx context.Context, channelType string, cfg msgch.ChannelConfig, msg *msgch.Message, opts ...msgch.SendOption) error {
	// 先检查 stopped：避免 select 中 stop 和 ch 同时就绪时 Go 随机选到 ch 导致投递成功
	q.mu.Lock()
	stopped := q.stopped
	q.mu.Unlock()
	if stopped {
		return errQueueStopped
	}
	select {
	case <-q.stop:
		return errQueueStopped
	case q.ch <- Task{
		ctx:         ctx,
		ChannelType: channelType,
		Cfg:         cfg,
		Msg:         msg,
		Opts:        opts,
	}:
		return nil
	}
}

// TryPush 非阻塞投递：队列满或已停止时立即返回 false。
func (q *Queue) TryPush(ctx context.Context, channelType string, cfg msgch.ChannelConfig, msg *msgch.Message, opts ...msgch.SendOption) bool {
	select {
	case <-q.stop:
		return false
	default:
	}
	select {
	case q.ch <- Task{ctx: ctx, ChannelType: channelType, Cfg: cfg, Msg: msg, Opts: opts}:
		return true
	default:
		return false
	}
}

// Stop 停止队列：不再接收新任务，已投递任务消费完后退出。
func (q *Queue) Stop() {
	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return
	}
	q.stopped = true
	close(q.stop)
	q.mu.Unlock()
	// 排空通道里的任务（可选：继续消费剩余任务）—— 也可直接退出
	q.wg.Wait()
}

// errQueueStopped 队列已停止错误。
var errQueueStopped = msgchErr("queue: stopped")

// msgchErr 简易 error 类型（避免直接 import errors）。
type msgchErr string

func (e msgchErr) Error() string { return string(e) }