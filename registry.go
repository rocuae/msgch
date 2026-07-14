package msgch

import (
	"errors"
	"sync"
)

// 渠道未注册错误。
var ErrChannelNotRegistered = errors.New("msgch: channel type not registered")

// registry 全局渠道工厂注册表。
type registry struct {
	mu        sync.RWMutex
	factories map[string]func() Channel
}

var globalRegistry = &registry{factories: make(map[string]func() Channel)}

// Register 注册渠道工厂。通常在各渠道包的 init() 中调用。
//
// 线程安全；重复注册同一类型会覆盖旧工厂（便于测试与重注册）。
func Register(typ string, factory func() Channel) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.factories[typ] = factory
}

// Get 取渠道实例。每次返回新实例（factory 调用一次），避免渠道内部状态串扰。
// 类型未注册时返回 ErrChannelNotRegistered。
func Get(typ string) (Channel, error) {
	globalRegistry.mu.RLock()
	factory, ok := globalRegistry.factories[typ]
	globalRegistry.mu.RUnlock()
	if !ok || factory == nil {
		return nil, ErrChannelNotRegistered
	}
	return factory(), nil
}

// RegisteredTypes 返回已注册的全部渠道类型标识（用于自省/调试）。
func RegisteredTypes() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	types := make([]string, 0, len(globalRegistry.factories))
	for t := range globalRegistry.factories {
		types = append(types, t)
	}
	return types
}
