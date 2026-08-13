package core

import "context"

// nopLogger 是 Logger 的空实现：所有方法均为 no-op，
// 用于“不记录任何日志”的占位（如 jobx.WithLogger(nil) 归一）。
// 值类型空结构体，零分配、并发安全。
type nopLogger struct{}

// NewNopLogger 返回一个不产生任何输出与副作用的 Logger。
// Panic/Fatal 同样静默（不触发 panic、不退出进程）；
// SafeExit 保留退出回调语义（无可刷内容，直接执行 exitFunc）。
func NewNopLogger() Logger {
	return nopLogger{}
}

func (nopLogger) IsDebugEnabled() bool { return false }

func (nopLogger) Debug(msg string, fields FieldGroup) {}
func (nopLogger) Info(msg string, fields FieldGroup)  {}
func (nopLogger) Warn(msg string, fields FieldGroup)  {}
func (nopLogger) Error(msg string, fields FieldGroup) {}
func (nopLogger) Panic(msg string, fields FieldGroup) {}
func (nopLogger) Fatal(msg string, fields FieldGroup) {}

func (nopLogger) Debugf(format string, args ...any) {}
func (nopLogger) Infof(format string, args ...any)  {}
func (nopLogger) Warnf(format string, args ...any)  {}
func (nopLogger) Errorf(format string, args ...any) {}
func (nopLogger) Panicf(format string, args ...any) {}
func (nopLogger) Fatalf(format string, args ...any) {}

func (nopLogger) WithContext(ctx context.Context) Logger { return nopLogger{} }
func (nopLogger) WithField(key string, val any) Logger   { return nopLogger{} }

func (nopLogger) Sync() error  { return nil }
func (nopLogger) Close() error { return nil }

func (nopLogger) SafeExit(exitFunc func()) {
	if exitFunc != nil {
		exitFunc()
	}
}
