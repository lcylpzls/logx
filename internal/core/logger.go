package core

import "context"

// Logger 定义了对外暴露的顶层日志接口，隔离具体实现。
//
// Logger 同时支持两种调用风格：
//  1. 结构化字段模式：Info("msg", logx.String("key", "val"))
//  2. 格式化打印模式：Infof("count=%d", 42)
type Logger interface {
	// IsDebugEnabled 判断 Debug 级别是否在当前实例中启用。
	IsDebugEnabled() bool

	// --- 结构化 API ---

	Debug(msg string, fields FieldGroup)
	Info(msg string, fields FieldGroup)
	Warn(msg string, fields FieldGroup)
	Error(msg string, fields FieldGroup)
	Panic(msg string, fields FieldGroup) // 输出后触发 panic
	Fatal(msg string, fields FieldGroup) // 输出后退出进程

	// --- 格式化 API ---

	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
	Panicf(format string, args ...any) // 输出后触发 panic
	Fatalf(format string, args ...any) // 输出后退出进程

	// --- 链路与上下文 ---

	// WithContext 派生一个新的 Logger，携带指定的 context.Context。
	WithContext(ctx context.Context) Logger
	// WithField 派生一个新的 Logger，默认携带一个结构化字段。
	WithField(key string, val any) Logger

	// --- 生命周期 ---

	// Sync 强制刷盘所有通道的缓冲日志。
	Sync() error
	// Close 关闭 Logger 并释放所有资源。
	Close() error
	// SafeExit 优雅退出：先同步刷盘所有缓冲日志，再执行 exitFunc。
	// 用于替代直接调用 os.Exit，确保异步模式下临终日志不丢失。
	SafeExit(exitFunc func())
}
