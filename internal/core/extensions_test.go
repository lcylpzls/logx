package core

import (
	testx "github.com/lcylpzls/testx"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// 阶段四：进阶特性测试
// ---------------------------------------------------------------------------
// TestLazy_DeferredEvaluation 测试 Lazy 延迟求值——仅在级别通过时才执行。
func TestLazy_DeferredEvaluation(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	testx.RequireNoError(t, err)

	called := false
	fn := func() interface{} {
		called = true
		return "expensive result"
	}

	// Debug 未启用，Lazy 不应被执行
	logger.Debug("debug msg", Fields(Lazy("data", fn)))
	testx.False(t, called)

	// Info 已启用，Lazy 应被执行
	logger.Info("info msg", Fields(Lazy("data", fn)))
	testx.True(t, called)

}

// TestReplaceStdLogger 测试标准库 log 劫持。
func TestReplaceStdLogger(t *testing.T) {
	dir := tempLogDir(t)

	logger, err := NewBuilder().
		EnableFileLog(
			WithLogDir(dir),
			WithFilename("std.log"),
			WithWriteMode(SyncWriteMode),
			WithLevels(InfoLevel),
		).
		Build()
	testx.RequireNoError(t, err)

	defer logger.Close()

	ReplaceStdLogger(logger)
	defer RestoreStdLogger()

	// 使用标准库 log 打印
	log.Println("from std log")
	logger.Sync()

	// 验证文件输出
	files, _ := filepath.Glob(filepath.Join(dir, "std-*.log"))
	if len(files) == 0 {
		t.Fatal("标准库劫持：日志文件未创建")
	}

	data, err := os.ReadFile(files[0])
	testx.RequireNoError(t, err)

	if !strings.Contains(string(data), "from std log") {
		t.Errorf("标准库劫持：日志内容不匹配：%s", string(data))
	}
}

// TestHook 测试 Hook 接口。
func TestHook(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	testx.RequireNoError(t, err)

	var mu sync.Mutex
	var received []string

	testHook := &testHookImpl{
		fn: func(e *Entry) {
			mu.Lock()
			received = append(received, e.Message)
			mu.Unlock()
		},
	}

	hl, ok := logger.(HookedLogger)
	testx.RequireTrue(t, ok)

	hl.AddHook(testHook)

	logger.Info("hook test message", FieldGroup{})

	// 等待 Hook 异步执行
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Error("Hook 未收到日志")
	}
	if len(received) > 0 && received[0] != "hook test message" {
		t.Errorf("Hook 收到错误消息：%v", received)
	}
}

// testHookImpl 测试用 Hook 实现。
type testHookImpl struct {
	fn func(e *Entry)
}

// TestSafeExit 测试 SafeExit 闭包拦截。
func TestSafeExit_NoPanic(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	testx.RequireNoError(t, err)

	exited := false
	exitFunc := func() {
		exited = true
	}

	logger.SafeExit(exitFunc)

	testx.True(t, exited)

}

// TestLazyFieldConstructor 测试 Lazy 字段构造。
func TestLazyFieldConstructor(t *testing.T) {
	f := Lazy("key", func() interface{} { return "val" })
	testx.Equal(t, f.Key, "key")

	lv, ok := f.Value.(*lazyValue)
	testx.RequireTrue(t, ok)

	if lv.fn() != "val" {
		t.Error("Lazy fn 应返回 val")
	}
}

// TestHook_NoPanic 测试 Hook 内部 panic 不会影响主路径。
func TestHook_NoPanic(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	testx.RequireNoError(t, err)

	// 注册一个会 panic 的 Hook
	hl := logger.(HookedLogger)
	hl.AddHook(&panicHook{})

	// 应该正常输出，不会 panic
	logger.Info("before panic hook", FieldGroup{})
	time.Sleep(50 * time.Millisecond)
	logger.Info("after panic hook", FieldGroup{})
}

// panicHook 测试用：故意 panic 的 Hook。
type panicHook struct{}
