package core

import (
	"sync"
)

// ---------------------------------------------------------------------------
// Hook — 日志拦截与扩展接口
// ---------------------------------------------------------------------------

// Hook 定义日志拦截钩子。当一条日志通过级别过滤并被成功写入后，
// OnLog 会被异步调用（不阻塞日志主路径）。
//
// 典型用途：错误报警（飞书/钉钉/Sentry）、监控打点、审计日志。
type Hook interface {
	// OnLog 在日志被写入后调用。entry 是已编码的日志条目。
	// 实现必须保证 OnLog 不会 panic，且应尽快返回。
	OnLog(e *Entry)
}

// ---------------------------------------------------------------------------
// hookManager — 内部 Hook 管理
// ---------------------------------------------------------------------------

// hookManager 管理一组 Hook，线程安全。
type hookManager struct {
	mu    sync.RWMutex
	hooks []Hook
}

// newHookManager 创建一个空的 Hook 管理器。
func newHookManager() *hookManager {
	return &hookManager{}
}

// add 添加一个 Hook。
func (hm *hookManager) add(h Hook) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.hooks = append(hm.hooks, h)
}

// fire 异步触发所有 Hook。不阻塞调用方。
func (hm *hookManager) fire(e *Entry) {
	hm.mu.RLock()
	hooks := make([]Hook, len(hm.hooks))
	copy(hooks, hm.hooks)
	hm.mu.RUnlock()

	for _, h := range hooks {
		go func(h Hook) {
			defer func() {
				// 防止 Hook 内部 panic 影响日志主路径
				recover()
			}()
			h.OnLog(e)
		}(h)
	}
}

// ---------------------------------------------------------------------------
// AddHook — 为 Logger 注册 Hook
// ---------------------------------------------------------------------------

// HookedLogger 扩展 Logger 接口，支持添加 Hook。
type HookedLogger interface {
	Logger
	// AddHook 注册一个日志 Hook。所有通过该 Logger 的日志都会被 Hook 拦截。
	AddHook(h Hook)
}

// AddHook 为 logger 注册一个 Hook。
func (l *logger) AddHook(h Hook) {
	if l.hooks == nil {
		l.hooks = newHookManager()
	}
	l.hooks.add(h)
}

// hooks 字段需要添加到 logger 结构体。
// 已在 builder.go 中 logger 结构体添加 hooks 字段。
