package tokenstore

import (
	"context"
	"testing"
	"time"
)

// TestGet_CacheAndRefresh 验证首次刷新、缓存命中、过期再刷新。
func TestGet_CacheAndRefresh(t *testing.T) {
	calls := 0
	key := "test:cache_and_refresh"
	defer Invalidate(key)

	refresh := func() (string, time.Duration, error) {
		calls++
		return "tok-" + string(rune('0'+calls)), 10 * time.Second, nil
	}

	// 首次：调一次 refresh
	tok1, err := Get(context.Background(), key, refresh)
	if err != nil || tok1 != "tok-1" {
		t.Fatalf("首次 Get 期望 tok-1 无错，实际 %q %v", tok1, err)
	}
	// 二次：命中缓存，不调 refresh
	tok2, _ := Get(context.Background(), key, refresh)
	if tok2 != "tok-1" {
		t.Fatalf("命中缓存应返回 tok-1，实际 %q", tok2)
	}
	if calls != 1 {
		t.Fatalf("命中缓存应不调 refresh，实际调用 %d 次", calls)
	}
}

// TestGet_ExpiredRefresh 验证过期后重新刷新。
func TestGet_ExpiredRefresh(t *testing.T) {
	calls := 0
	key := "test:expired"
	defer Invalidate(key)

	// 第一次返回有效期 0 → 立即过期，第二次会刷新
	refresh := func() (string, time.Duration, error) {
		calls++
		if calls == 1 {
			return "short", 0, nil // 立即过期
		}
		return "long", 10 * time.Second, nil
	}
	_, _ = Get(context.Background(), key, refresh)
	_, _ = Get(context.Background(), key, refresh)
	if calls != 2 {
		t.Fatalf("过期后应再次刷新，实际调用 %d 次", calls)
	}
}

// TestGet_DifferentKeys 验证不同 key 各自独立缓存。
func TestGet_DifferentKeys(t *testing.T) {
	calls := 0
	refresh := func() (string, time.Duration, error) {
		calls++
		return "x", 10 * time.Second, nil
	}
	defer Invalidate("k1")
	defer Invalidate("k2")
	_, _ = Get(context.Background(), "k1", refresh)
	_, _ = Get(context.Background(), "k2", refresh)
	_, _ = Get(context.Background(), "k1", refresh) // 命中缓存
	if calls != 2 {
		t.Fatalf("不同 key 应各刷一次，k1 第二次命中，实际调用 %d 次", calls)
	}
}

// TestGet_NilRefresh 验证 refresh 为 nil 时报错。
func TestGet_NilRefresh(t *testing.T) {
	_, err := Get(context.Background(), "k", nil)
	if err == nil {
		t.Fatal("refresh 为 nil 应返回 error")
	}
}

// TestInvalidate 验证 Invalidate 使缓存失效。
func TestInvalidate(t *testing.T) {
	calls := 0
	key := "test:invalidate"
	refresh := func() (string, time.Duration, error) {
		calls++
		return "x", 10 * time.Second, nil
	}
	_, _ = Get(context.Background(), key, refresh)
	_, _ = Get(context.Background(), key, refresh) // 命中
	Invalidate(key)
	_, _ = Get(context.Background(), key, refresh) // 再刷
	if calls != 2 {
		t.Fatalf("Invalidate 后应再刷一次，实际调用 %d 次", calls)
	}
}