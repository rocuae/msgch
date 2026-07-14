// Package tokenstore 提供 access_token 的缓存与刷新抽象。
//
// 多个需 access_token 的渠道（企业微信应用消息、微信公众号模板消息、飞书自建应用等）
// 通过统一 TokenStore 命中缓存或自动刷新，避免每次发送都重新取 token。同一凭证被多个渠道
// 引用时共享同一份缓存（IsShared 语义）。
//
// 设计参考：message-pusher 的 TokenStoreItem（Key/Token/Refresh/IsFilled/IsShared）。
//
// 用法（渠道实现侧）：
//
//	token, err := tokenstore.Get(ctx, key, func() (string, time.Duration, error) {
//	    return fetchTokenFromVendor(cfg) // 首次/过期时调厂商接口，返回 token 与有效期
//	})
package tokenstore

import (
	"context"
	"errors"
	"sync"
	"time"
)

// refreshErr 刷新失败时记录的哨兵错误，便于上层判断。
var refreshErr = errors.New("tokenstore: refresh token failed")

// entry 单个 token 缓存条目。
type entry struct {
	mu         sync.Mutex
	token      string       // 当前 token
	expiresAt  time.Time    // 过期时刻
	refreshing bool         // 是否正在刷新中（防击穿）
}

// store 全局 token 缓存表。
type store struct {
	mu      sync.RWMutex
	entries map[string]*entry
}

var globalStore = &store{entries: make(map[string]*entry)}

// Get 取 token：命中缓存且未过期则直接返回；否则触发刷新。
//
//   - key：缓存键（建议为 "渠道类型:凭证指纹"，如 "qy_wechat:corpid+corpsecret"）
//   - refresh：刷新函数，返回新 token 与有效期（duration<=0 表示不缓存，每次都刷新）
//
// 刷新由单协程执行（互斥锁防击穿），其他调用方阻塞等待其结果。
func Get(ctx context.Context, key string, refresh func() (string, time.Duration, error)) (string, error) {
	if refresh == nil {
		return "", errors.New("tokenstore: refresh function is nil")
	}

	e := globalStore.getOrCreate(key)

	e.mu.Lock()
	// 命中且未过期 → 直接返回
	if e.token != "" && time.Now().Before(e.expiresAt) {
		tok := e.token
		e.mu.Unlock()
		return tok, nil
	}
	// 未命中或已过期 → 刷新
	tok, dur, err := refresh()
	if err != nil {
		e.mu.Unlock()
		return "", err
	}
	e.token = tok
	if dur > 0 {
		// 提前过期以避免临界点失效：长 TTL 提前 5 分钟，短 TTL 取 90%（取较小提前量）
		advance := 5 * time.Minute
		if advance > dur/10 {
			advance = dur / 10 // 短 TTL 时仅提前 10%，避免负数过期
		}
		e.expiresAt = time.Now().Add(dur - advance)
		if e.expiresAt.Before(time.Now()) {
			// 极端短 TTL 兜底：至少保留 1 秒
			e.expiresAt = time.Now().Add(time.Second)
		}
	} else {
		e.expiresAt = time.Now() // 立即过期，下次再刷
	}
	e.mu.Unlock()
	return tok, nil
}

// Invalidate 使某个 key 的缓存失效，下次 Get 会强制刷新。
func Invalidate(key string) {
	globalStore.mu.Lock()
	defer globalStore.mu.Unlock()
	if e, ok := globalStore.entries[key]; ok {
		e.mu.Lock()
		e.token = ""
		e.expiresAt = time.Time{}
		e.mu.Unlock()
	}
}

// getOrCreate 取或创建缓存条目（线程安全）。
func (s *store) getOrCreate(key string) *entry {
	s.mu.RLock()
	if e, ok := s.entries[key]; ok {
		s.mu.RUnlock()
		return e
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	// 双检
	if e, ok := s.entries[key]; ok {
		return e
	}
	e := &entry{}
	s.entries[key] = e
	return e
}

// _ 防止 refreshErr 未用告警（保留以便未来扩展错误判断）。
var _ = refreshErr