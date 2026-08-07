package logx

import (
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
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	called := false
	fn := func() interface{} {
		called = true
		return "expensive result"
	}

	// Debug 未启用，Lazy 不应被执行
	logger.Debug("debug msg", Fields(Lazy("data", fn)))
	if called {
		t.Error("Debug 未启用时 Lazy 函数不应被调用")
	}

	// Info 已启用，Lazy 应被执行
	logger.Info("info msg", Fields(Lazy("data", fn)))
	if !called {
		t.Error("Info 启用时 Lazy 函数应被调用")
	}
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
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}
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
	if err != nil {
		t.Fatalf("读取文件失败：%v", err)
	}
	if !strings.Contains(string(data), "from std log") {
		t.Errorf("标准库劫持：日志内容不匹配：%s", string(data))
	}
}

// TestHook 测试 Hook 接口。
func TestHook(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

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
	if !ok {
		t.Fatal("logger 应实现 HookedLogger 接口")
	}
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
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	exited := false
	exitFunc := func() {
		exited = true
	}

	logger.SafeExit(exitFunc)

	if !exited {
		t.Error("SafeExit 应执行 exitFunc")
	}
}

// TestLazyFieldConstructor 测试 Lazy 字段构造。
func TestLazyFieldConstructor(t *testing.T) {
	f := Lazy("key", func() interface{} { return "val" })
	if f.Key != "key" {
		t.Errorf("Lazy field key: got %s, want key", f.Key)
	}
	lv, ok := f.Value.(*lazyValue)
	if !ok {
		t.Fatal("Lazy field value 应为 *lazyValue")
	}
	if lv.fn() != "val" {
		t.Error("Lazy fn 应返回 val")
	}
}

// TestHook_NoPanic 测试 Hook 内部 panic 不会影响主路径。
func TestHook_NoPanic(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

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
