package core

import (
	"context"
	"testing"
)

// TestNewNopLogger 覆盖 no-op logger 的全部接口行为。
func TestNewNopLogger(t *testing.T) {
	logger := NewNopLogger()
	if logger == nil {
		t.Fatal("NewNopLogger 返回 nil")
	}
	if logger.IsDebugEnabled() {
		t.Fatal("no-op logger 不应启用 Debug")
	}
	// 全部结构化方法：静默且不 panic/退出。
	logger.Debug("d", FieldGroup{})
	logger.Info("i", FieldGroup{})
	logger.Warn("w", FieldGroup{})
	logger.Error("e", FieldGroup{})
	logger.Panic("p", FieldGroup{})
	logger.Fatal("f", FieldGroup{})
	// 全部格式化方法：静默且不 panic/退出。
	logger.Debugf("d=%d", 1)
	logger.Infof("i=%s", "x")
	logger.Warnf("w=%v", 1)
	logger.Errorf("e=%v", 1)
	logger.Panicf("p=%v", 1)
	logger.Fatalf("f=%v", 1)
	// 派生与生命周期。
	if ctxLogger := logger.WithContext(context.Background()); ctxLogger == nil {
		t.Fatal("WithContext 返回 nil")
	}
	if fieldLogger := logger.WithField("k", "v"); fieldLogger == nil {
		t.Fatal("WithField 返回 nil")
	}
	if err := logger.Sync(); err != nil {
		t.Fatalf("Sync 应返回 nil：%v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close 应返回 nil：%v", err)
	}
	called := false
	logger.SafeExit(func() { called = true })
	if !called {
		t.Fatal("SafeExit 应执行退出回调")
	}
	logger.SafeExit(nil)
}
