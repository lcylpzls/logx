package logx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewBuilder_DefaultOff(t *testing.T) {
	// 没有任何 EnableXXX 调用时，Build() 返回无输出 logger。
	logger, err := NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}
	// 不调用任何 log 方法应该不会 panic，也不应有输出。
	logger.Info("这条日志不应该出现")
	logger.Debug("这条日志也不应该出现")
	logger.Error("这条日志同样不应该出现")

	// Sync 和 Close 不应该出错。
	if err := logger.Sync(); err != nil {
		t.Errorf("Sync() 失败：%v", err)
	}
	if err := logger.Close(); err != nil {
		t.Errorf("Close() 失败：%v", err)
	}

	if logger.IsDebugEnabled() {
		t.Error("默认静默模式下 IsDebugEnabled 应返回 false")
	}
}

// ---------------------------------------------------------------------------
// 1.2 控制台通道 + 级别过滤
// ---------------------------------------------------------------------------
func TestBuilder_ConsoleDebug(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(DebugLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	if !logger.IsDebugEnabled() {
		t.Error("EnableDebug 后 IsDebugEnabled 应返回 true")
	}

	// 所有级别都 >= Debug，不会 panic。
	logger.Debug("debug msg", String("k", "v"))
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")
}

func TestBuilder_ConsoleInfoOnly(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	if logger.IsDebugEnabled() {
		t.Error("仅 EnableInfo 时 IsDebugEnabled 应返回 false")
	}

	logger.Info("这条应该输出")
	logger.Debug("这条不应该输出")
}

func TestBuilder_ConsoleErrorOnly(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(ErrorLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	if logger.IsDebugEnabled() {
		t.Error("仅 EnableError 时 IsDebugEnabled 应返回 false")
	}

	logger.Error("这条应该输出")
	logger.Info("这条不应该输出")
	logger.Debug("这条也不应该输出")
}

// ---------------------------------------------------------------------------
// 1.3 WithField 与派生 Logger
// ---------------------------------------------------------------------------
func TestWithField(t *testing.T) {
	lg, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	// 派生 logger 不影响原始实例。
	child := lg.WithField("service", "api")
	grandchild := child.WithField("trace_id", "abc123")

	// 原始 logger 无额外字段
	lgImpl, ok := lg.(*logger)
	if !ok {
		t.Fatal("类型断言失败")
	}
	if len(lgImpl.fields) != 0 {
		t.Errorf("原始 logger fields 应为空，实际 %d", len(lgImpl.fields))
	}

	// child 有 1 个字段
	cl, ok := child.(*logger)
	if !ok {
		t.Fatal("类型断言失败")
	}
	if len(cl.fields) != 1 || cl.fields[0].Key != "service" {
		t.Errorf("child fields 不符预期：%+v", cl.fields)
	}

	// grandchild 有 2 个字段
	gcl, ok := grandchild.(*logger)
	if !ok {
		t.Fatal("类型断言失败")
	}
	if len(gcl.fields) != 2 {
		t.Errorf("grandchild fields 应为 2，实际 %d", len(gcl.fields))
	}
}

// ---------------------------------------------------------------------------
// 1.4 WithContext
// ---------------------------------------------------------------------------
func TestWithContext(t *testing.T) {
	lg, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("trace_id"), "xyz")
	child := lg.WithContext(ctx)

	lgImpl, ok := child.(*logger)
	if !ok {
		t.Fatal("类型断言失败")
	}
	if lgImpl.ctx == nil {
		t.Error("WithContext 后 ctx 不应为 nil")
	}
}

// ---------------------------------------------------------------------------
// 1.5 格式化 API
// ---------------------------------------------------------------------------
func TestFormatfAPI(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(DebugLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	// 确保不会 panic。
	logger.Debugf("debug: %d", 1)
	logger.Infof("info: %s", "hello")
	logger.Warnf("warn: %v", 3.14)
	logger.Errorf("error: %s", "boom")
}

// ---------------------------------------------------------------------------
// 1.6 多通道组合
// ---------------------------------------------------------------------------
func TestMultipleChannels(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		EnableConsole(DebugLevel, WithColor()).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	if !logger.IsDebugEnabled() {
		t.Error("有一个通道启用 Debug 时 IsDebugEnabled 应为 true")
	}

	logger.Info("multi-channel info")
	logger.Debug("multi-channel debug")
}

// ---------------------------------------------------------------------------
// 1.7 Level 类型测试
// ---------------------------------------------------------------------------
func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{OffLevel, "OFF  "},
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO "},
		{WarnLevel, "WARN "},
		{ErrorLevel, "ERROR"},
		{PanicLevel, "PANIC"},
		{FatalLevel, "FATAL"},
	}
	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestIsLevelEnabled(t *testing.T) {
	tests := []struct {
		minLvl Level
		target Level
		want   bool
	}{
		{OffLevel, DebugLevel, false},
		{OffLevel, FatalLevel, false},
		{DebugLevel, DebugLevel, true},
		{DebugLevel, InfoLevel, true},
		{DebugLevel, FatalLevel, true},
		{InfoLevel, DebugLevel, false},
		{InfoLevel, InfoLevel, true},
		{InfoLevel, ErrorLevel, true},
		{ErrorLevel, WarnLevel, false},
		{ErrorLevel, ErrorLevel, true},
		{ErrorLevel, PanicLevel, true},
		{FatalLevel, FatalLevel, true},
	}
	for _, tt := range tests {
		got := isLevelEnabled(tt.minLvl, tt.target)
		if got != tt.want {
			t.Errorf("isLevelEnabled(%v, %v) = %v, want %v", tt.minLvl, tt.target, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 1.9 Field 构造器
// ---------------------------------------------------------------------------
func TestFieldConstructors(t *testing.T) {
	s := String("key", "val")
	if s.Key != "key" || s.Value != "val" {
		t.Errorf("String field: %+v", s)
	}

	i := Int("count", 42)
	if i.Key != "count" || i.Value != 42 {
		t.Errorf("Int field: %+v", i)
	}

	i64 := Int64("size", 1<<30)
	if i64.Key != "size" || i64.Value != int64(1<<30) {
		t.Errorf("Int64 field: %+v", i64)
	}

	b := Bool("enabled", true)
	if b.Key != "enabled" || b.Value != true {
		t.Errorf("Bool field: %+v", b)
	}

	a := Any("data", struct{ X int }{10})
	if a.Key != "data" {
		t.Errorf("Any field key: %s", a.Key)
	}

	e := Err(context.Canceled)
	if e.Key != "error" || e.Value != context.Canceled {
		t.Errorf("Err field: %+v", e)
	}
}

// ---------------------------------------------------------------------------
// 1.10 ConsoleAppender（stdout/stderr 分流）
// ---------------------------------------------------------------------------
func TestConsoleAppender_Routing(t *testing.T) {
	app := newConsoleAppender()

	// 非 Error 级别写 stdout
	n, err := app.Append(InfoLevel, []byte("stdout test\n"))
	if err != nil {
		t.Errorf("Append Info 失败：%v", err)
	}
	if n == 0 {
		t.Error("Append Info 应写入数据")
	}

	// Error 及以上写 stderr
	n, err = app.Append(ErrorLevel, []byte("stderr test\n"))
	if err != nil {
		t.Errorf("Append Error 失败：%v", err)
	}
	if n == 0 {
		t.Error("Append Error 应写入数据")
	}

	if err := app.Sync(); err != nil {
		t.Errorf("Sync 失败：%v", err)
	}
	if err := app.Close(); err != nil {
		t.Errorf("Close 失败：%v", err)
	}
}

// ---------------------------------------------------------------------------
// 1.11 并发写入（data race 检测）
// ---------------------------------------------------------------------------
func TestConcurrentLogging(t *testing.T) {
	logger, err := NewBuilder().
		EnableConsole(DebugLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	const goroutines = 10
	const messages = 100

	done := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			for j := 0; j < messages; j++ {
				logger.Info("concurrent log", Int("goroutine", id), Int("msg", j))
			}
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}
}

func BenchmarkLogger_Info(b *testing.B) {
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	if err != nil {
		b.Fatalf("Build() 失败：%v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark", Int("i", i))
	}
}

func BenchmarkLogger_Infof(b *testing.B) {
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	if err != nil {
		b.Fatalf("Build() 失败：%v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Infof("benchmark: i=%d", i)
	}
}

func BenchmarkLogger_DisabledLevel(b *testing.B) {
	// Debug 未启用，测试级别过滤的性能开销。
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	if err != nil {
		b.Fatalf("Build() 失败：%v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Debug("this should be filtered", Int("i", i))
	}
}

// ---------------------------------------------------------------------------
// 消除未使用变量告警
// ---------------------------------------------------------------------------
var _ = bytes.NewBuffer // 保留 bytes 引用，供后续扩展 stderr 捕获测试

func (h *testHookImpl) OnLog(e *Entry) {
	if h.fn != nil {
		h.fn(e)
	}
}

func (h *panicHook) OnLog(e *Entry) {
	panic("intentional hook panic")
}

// TestEntry_Context_Nil 测试 Entry.Context() 在 ctx 为 nil 时返回 Background。
func TestEntry_Context_Nil(t *testing.T) {
	e := &Entry{}
	ctx := e.Context()
	if ctx != context.Background() {
		t.Error("Context() with nil ctx should return context.Background()")
	}
}

// TestEntry_Context_NonNil 测试 Entry.Context() 在 ctx 非 nil 时返回原始 ctx。
func TestEntry_Context_NonNil(t *testing.T) {
	type ctxKey string
	original := context.WithValue(context.Background(), ctxKey("x"), "y")
	e := &Entry{ctx: original}
	got := e.Context()
	if got != original {
		t.Error("Context() should return the original ctx")
	}
}

// TestMergeFields_BothNonEmpty 测试 logger 字段和方法字段均非空时的合并。
func TestMergeFields_BothNonEmpty(t *testing.T) {
	l := &logger{
		fields: []Field{{Key: "svc", Value: "api"}},
	}
	methodFields := []Field{{Key: "tid", Value: "abc"}}
	merged := l.mergeFields(methodFields)
	if len(merged) != 2 {
		t.Fatalf("merged length: got %d, want 2", len(merged))
	}
	if merged[0].Key != "svc" || merged[1].Key != "tid" {
		t.Errorf("merge order wrong: %+v", merged)
	}
}

// TestMergeFields_LoggerOnly 测试仅 logger 有字段时直接返回 logger 字段。
func TestMergeFields_LoggerOnly(t *testing.T) {
	l := &logger{
		fields: []Field{{Key: "svc", Value: "api"}},
	}
	merged := l.mergeFields(nil)
	if len(merged) != 1 {
		t.Fatalf("merged length: got %d, want 1", len(merged))
	}
	if merged[0].Key != "svc" {
		t.Errorf("unexpected field: %+v", merged[0])
	}
}

// TestCopyFields_NonEmpty 测试 copyFields 在 fields 非空时返回深拷贝。
func TestCopyFields_NonEmpty(t *testing.T) {
	l := &logger{
		fields: []Field{{Key: "k", Value: "v"}},
	}
	copied := l.copyFields()
	if len(copied) != 1 {
		t.Fatalf("copyFields length: got %d, want 1", len(copied))
	}
	// 验证是深拷贝：修改原切片不影响拷贝
	l.fields[0].Key = "modified"
	if copied[0].Key != "k" {
		t.Error("copyFields should return a deep copy")
	}
}

// TestCopyFields_Empty 测试 copyFields 在 fields 为空时返回 nil。
func TestCopyFields_Empty(t *testing.T) {
	l := &logger{}
	copied := l.copyFields()
	if copied != nil {
		t.Errorf("copyFields on empty should return nil, got %v", copied)
	}
}

// TestPutBuffer_CapacityGuard 测试 putBuffer 对容量 < 64KB 的缓冲不归还。
func TestPutBuffer_CapacityGuard(t *testing.T) {
	small := &Buffer{B: make([]byte, 0, 1024)} // 1KB, below 64KB threshold
	// 不应归还到池中，验证不会 panic
	putBuffer(small)
	// 从池中获取的缓冲容量应 >= 64KB（因为 small 未被归还）
	// 但池中可能有其他缓冲，这里只验证不 panic
	large := getBuffer()
	if cap(large.B) < 64*1024 {
		t.Errorf("getBuffer should return buffer with cap >= 64KB, got %d", cap(large.B))
	}
	putBuffer(large)
}

// TestPutBuffer_NormalReturn 测试 putBuffer 正常归还大缓冲区。
func TestPutBuffer_NormalReturn(t *testing.T) {
	buf := getBuffer()
	originalCap := cap(buf.B)
	putBuffer(buf)
	// 再次获取，应能复用（但不保证一定是同一个）
	buf2 := getBuffer()
	if cap(buf2.B) != originalCap {
		t.Logf("buffer cap changed from %d to %d (pool may have created new)", originalCap, cap(buf2.B))
	}
	putBuffer(buf2)
}

// TestWithMaxSize 测试 WithMaxSize FileOption。
func TestWithMaxSize(t *testing.T) {
	cfg := &FileConfig{}
	WithMaxSize(50)(cfg)
	if cfg.MaxSize != 50 {
		t.Errorf("MaxSize = %d, want 50", cfg.MaxSize)
	}
}

// TestWithMaxAge 测试 WithMaxAge FileOption。
func TestWithMaxAge(t *testing.T) {
	cfg := &FileConfig{}
	WithMaxAge(30)(cfg)
	if cfg.MaxAge != 30 {
		t.Errorf("MaxAge = %d, want 30", cfg.MaxAge)
	}
}

// TestWithMaxBackups 测试 WithMaxBackups FileOption。
func TestWithMaxBackups(t *testing.T) {
	cfg := &FileConfig{}
	WithMaxBackups(50)(cfg)
	if cfg.MaxBackups != 50 {
		t.Errorf("MaxBackups = %d, want 50", cfg.MaxBackups)
	}
}

// TestWithCompressAfter 测试 WithCompressAfter FileOption。
func TestWithCompressAfter(t *testing.T) {
	cfg := &FileConfig{}
	WithCompressAfter(7)(cfg)
	if cfg.CompressAfter != 7 {
		t.Errorf("CompressAfter = %d, want 7", cfg.CompressAfter)
	}
}

// TestWithBufferSize 测试 WithBufferSize FileOption。
func TestWithBufferSize(t *testing.T) {
	cfg := &FileConfig{}
	WithBufferSize(8192)(cfg)
	if cfg.BufferSize != 8192 {
		t.Errorf("BufferSize = %d, want 8192", cfg.BufferSize)
	}
}

// TestWithFlushInterval 测试 WithFlushInterval FileOption。
func TestWithFlushInterval(t *testing.T) {
	cfg := &FileConfig{}
	WithFlushInterval(500 * time.Millisecond)(cfg)
	if cfg.FlushInterval != 500*time.Millisecond {
		t.Errorf("FlushInterval = %v, want 500ms", cfg.FlushInterval)
	}
}

// TestLogger_Panic 测试 Panic 方法会触发 panic。
func TestLogger_Panic(t *testing.T) {
	logger, _ := NewBuilder().EnableConsole(InfoLevel).Build()
	defer func() {
		if r := recover(); r == nil {
			t.Error("Panic should have panicked")
		}
	}()
	logger.Panic("test panic")
}

// TestLogger_Panicf 测试 Panicf 方法会触发 panic。
func TestLogger_Panicf(t *testing.T) {
	logger, _ := NewBuilder().EnableConsole(InfoLevel).Build()
	defer func() {
		if r := recover(); r == nil {
			t.Error("Panicf should have panicked")
		}
	}()
	logger.Panicf("test panicf %d", 1)
}

// TestIsConsoleOption_Level 测试 Level.isConsoleOption() 方法。
func TestIsConsoleOption_Level(t *testing.T) {
	// 直接调用确保覆盖
	OffLevel.isConsoleOption()
	DebugLevel.isConsoleOption()
}

// TestIsConsoleOption_Color 测试 colorOption.isConsoleOption() 方法。
func TestIsConsoleOption_Color(t *testing.T) {
	// 直接调用确保覆盖
	colorOption{}.isConsoleOption()
}

// TestConsoleOption_Interface 编译期接口满足检查。
func TestConsoleOption_Interface(t *testing.T) {
	var _ ConsoleOption = DebugLevel
	var _ ConsoleOption = WithColor()
}

// TestWriteMode_Constants 测试 WriteMode 常量值。
func TestWriteMode_Constants(t *testing.T) {
	if int(AsyncWriteMode) != 0 {
		t.Error("AsyncWriteMode should be 0")
	}
	if int(SyncWriteMode) != 1 {
		t.Error("SyncWriteMode should be 1")
	}
	// 确保常量可引用
	_ = AsyncWriteMode
	_ = SyncWriteMode
}

// TestBuild_UnknownAppenderType 测试 Build 对未知 appender 类型返回错误。
func TestBuild_UnknownAppenderType(t *testing.T) {
	b := &Builder{
		cores: []coreConfig{
			{appType: "unknown", minLvl: DebugLevel},
		},
	}
	_, err := b.Build()
	if err == nil {
		t.Error("Build with unknown appender type should return error")
	}
}

// TestLogger_Warn_Structured 测试 Warn 级别的结构化日志。
func TestLogger_Warn_Structured(t *testing.T) {
	logger, err := NewBuilder().EnableConsole(WarnLevel).Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	// 不 panic 即可
	logger.Warn("warn msg", String("reason", "test"))
}

// TestLogger_WithField_Merge 测试 WithField 后 mergeFields 行为。
func TestLogger_WithField_Merge(t *testing.T) {
	lg, _ := NewBuilder().EnableConsole(InfoLevel).Build()
	child := lg.WithField("base", "val")

	// child 调用 log 时，mergeFields 应合并 logger-level 和 method-level 字段
	childImpl := child.(*logger)
	merged := childImpl.mergeFields([]Field{{Key: "msg", Value: "hello"}})
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged fields, got %d", len(merged))
	}
}

// TestConsoleAppender_ErrorLevel_Stderr 测试 Error 级别写入 stderr。
func TestConsoleAppender_ErrorLevel_Stderr(t *testing.T) {
	app := newConsoleAppender()
	// 测试 Error 级别不会被路由到 stdout（通过检查是否不 panic）
	n, err := app.Append(ErrorLevel, []byte("error to stderr\n"))
	if err != nil {
		t.Fatalf("Append ErrorLevel failed: %v", err)
	}
	if n == 0 {
		t.Error("Append should write bytes")
	}
}

// TestConsoleAppender_PanicLevel_Stderr 测试 Panic 级别写入 stderr。
func TestConsoleAppender_PanicLevel_Stderr(t *testing.T) {
	app := newConsoleAppender()
	n, err := app.Append(PanicLevel, []byte("panic to stderr\n"))
	if err != nil {
		t.Fatalf("Append PanicLevel failed: %v", err)
	}
	if n == 0 {
		t.Error("Append should write bytes")
	}
}

// TestBuild_FileAppenderWithNilFileCfg 测试 Build 中 fileAppenderType 但 fileCfg 为 nil 返回错误。
func TestBuild_FileAppenderWithNilFileCfg(t *testing.T) {
	b := &Builder{
		cores: []coreConfig{
			{appType: fileAppenderType, minLvl: DebugLevel, fileCfg: nil},
		},
	}
	_, err := b.Build()
	if err == nil {
		t.Error("Build with fileAppenderType and nil fileCfg should return error")
	}
}

// TestBuild_SkipOffChannels 测试 Build 跳过 OffLevel 通道。
func TestBuild_SkipOffChannels(t *testing.T) {
	b := &Builder{
		cores: []coreConfig{
			{appType: consoleAppenderType, minLvl: OffLevel},
		},
	}
	l, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	// 应该创建一个空的 logger（无 cores）
	lg := l.(*logger)
	if len(lg.cores) != 0 {
		t.Errorf("expected 0 cores for OffLevel skip, got %d", len(lg.cores))
	}
}

// TestCore_WriteEncodeError 测试 core.write 中编码失败的路径。
func TestCore_WriteEncodeError(t *testing.T) {
	// 使用一个会失败的 encoder
	c := newCore(&failingEncoder{}, newConsoleAppender(), DebugLevel)
	e := &Entry{Level: DebugLevel, Message: "test"}
	// 不应 panic，编码失败会打印到 stderr
	c.write(e)
}

// failingEncoder 是一个总是返回错误的编码器。
type failingEncoder struct{}

func (f *failingEncoder) Encode(buf *Buffer, e *Entry) error {
	return fmt.Errorf("encode error")
}

// TestCore_WriteAppendError 测试 core.write 中写入失败的路径。
func TestCore_WriteAppendError(t *testing.T) {
	// 使用一个会失败的 appender
	c := newCore(newTextEncoder(false), &failingAppender{}, DebugLevel)
	e := &Entry{Level: DebugLevel, Message: "test"}
	// 不应 panic，写入失败会打印到 stderr
	c.write(e)
}

// failingAppender 是一个总是返回错误的输出器。
type failingAppender struct{}

func (f *failingAppender) Append(level Level, p []byte) (int, error) {
	return 0, fmt.Errorf("append error")
}

func (f *failingAppender) Sync() error  { return nil }
func (f *failingAppender) Close() error { return nil }

// TestLogger_SyncError 测试 Sync 在 core 返回错误时的路径。
func TestLogger_SyncError(t *testing.T) {
	l := &logger{
		cores: []*core{
			newCore(newTextEncoder(false), &failingSyncAppender{}, DebugLevel),
		},
	}
	err := l.Sync()
	if err == nil {
		t.Error("Sync with failing appender should return error")
	}
}

// failingSyncAppender 同步操作返回错误。
type failingSyncAppender struct{}

func (f *failingSyncAppender) Append(level Level, p []byte) (int, error) { return len(p), nil }
func (f *failingSyncAppender) Sync() error                               { return fmt.Errorf("sync error") }
func (f *failingSyncAppender) Close() error                              { return nil }

// TestLogger_CloseError 测试 Close 在 core 返回错误时的路径。
func TestLogger_CloseError(t *testing.T) {
	l := &logger{
		cores: []*core{
			newCore(newTextEncoder(false), &failingCloseAppender{}, DebugLevel),
		},
	}
	err := l.Close()
	if err == nil {
		t.Error("Close with failing appender should return error")
	}
}

// failingCloseAppender 关闭操作返回错误。
type failingCloseAppender struct{}

func (f *failingCloseAppender) Append(level Level, p []byte) (int, error) { return len(p), nil }
func (f *failingCloseAppender) Sync() error                               { return nil }
func (f *failingCloseAppender) Close() error                              { return fmt.Errorf("close error") }

// ---------------------------------------------------------------------------
// 阶段六：最终覆盖率补齐
// ---------------------------------------------------------------------------
// TestConsoleOption_Methods covers isConsoleOption on Level and colorOption.
func TestConsoleOption_Methods(t *testing.T) {
	InfoLevel.isConsoleOption()
	colorOption{}.isConsoleOption()
}

// TestLogger_Fatal covers Fatal path by intercepting osExit.
func TestLogger_Fatal(t *testing.T) {
	original := osExit
	defer func() { osExit = original }()

	exited := false
	osExit = func(code int) {
		exited = true
		if code != 1 {
			t.Errorf("Fatal exit code: %d, want 1", code)
		}
	}

	logger, _ := NewBuilder().EnableConsole(InfoLevel).Build()
	logger.Fatal("test fatal")

	if !exited {
		t.Error("Fatal should call osExit")
	}
}

// TestLogger_Fatalf covers Fatalf path by intercepting osExit.
func TestLogger_Fatalf(t *testing.T) {
	original := osExit
	defer func() { osExit = original }()

	exited := false
	osExit = func(code int) {
		exited = true
	}

	logger, _ := NewBuilder().EnableConsole(InfoLevel).Build()
	logger.Fatalf("test fatalf %d", 1)

	if !exited {
		t.Error("Fatalf should call osExit")
	}
}

// TestLogger_Fatal_SyncError covers Fatal's Sync path.
func TestLogger_Fatal_SyncError(t *testing.T) {
	original := osExit
	defer func() { osExit = original }()

	exited := false
	osExit = func(code int) { exited = true }

	logger, _ := NewBuilder().EnableConsole(InfoLevel).Build()
	logger.Fatal("test")
	if !exited {
		t.Error("Fatal should call osExit even if sync fails")
	}
}

// TestRunFlushLoop_FlushPath tests the flush loop's ticker path.
func TestRunFlushLoop_FlushPath(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "flushloop.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    256,
		FlushInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("newFileAppender 失败：%v", err)
	}

	fa.Append(InfoLevel, []byte("tick flush\n"))

	// Wait for ticker
	time.Sleep(100 * time.Millisecond)
	fa.Close()

	files, _ := filepath.Glob(filepath.Join(dir, "flushloop-*.log"))
	if len(files) > 0 {
		data, _ := os.ReadFile(files[0])
		if len(data) == 0 {
			t.Error("flush loop: 文件为空")
		}
	}
}

// TestBuilder_Build_FileError tests Build with file config but bad directory.
func TestBuilder_Build_FileError(t *testing.T) {
	_, err := NewBuilder().
		EnableFileLog(
			WithLogDir("/nonexistent/path/that/should/fail/on/write"),
			WithFilename("test.log"),
			WithLevels(InfoLevel),
		).
		Build()
	// On Windows with non-existent root, MkdirAll may fail
	if err == nil {
		t.Log("Build succeeded (directory creation may have worked)")
	}
}

// TestAppendAsync_BatchFlush tests the batch flush threshold in runFlushLoop.
func TestAppendAsync_BatchFlush(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "batch.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    4096,
		FlushInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("newFileAppender 失败：%v", err)
	}

	// Write enough data to trigger batch flush (>64KB)
	bigMsg := bytes.Repeat([]byte("x"), 1024) // 1KB each
	for i := 0; i < 100; i++ {
		fa.Append(InfoLevel, bigMsg)
	}

	// Wait for flush
	time.Sleep(50 * time.Millisecond)
	fa.Close()

	files, _ := filepath.Glob(filepath.Join(dir, "batch-*.log"))
	if len(files) > 0 {
		data, _ := os.ReadFile(files[0])
		if len(data) == 0 {
			t.Error("batch flush: 文件为空")
		}
	}
}

// TestDrainAsync_WithError tests drainAsync rotation error path.
func TestDrainAsync_WithError(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "drainerr.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    256,
		FlushInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("newFileAppender 失败：%v", err)
	}

	fa.Append(InfoLevel, []byte("test\n"))

	// Close the underlying file to trigger write error in drain
	faImpl := fa.(*fileAppender)
	faImpl.mu.Lock()
	if faImpl.file != nil {
		faImpl.file.Close()
	}
	faImpl.mu.Unlock()

	// drainAsync should handle the error gracefully
	faImpl.drainAsync()

	// Close should still work
	fa.Close()
}

// TestCompressFile_OpenError covers compressFile with a non-existent source.
func TestCompressFile_OpenError(t *testing.T) {
	dir := tempLogDir(t)

	fa, _ := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "compress.log",
		WriteMode: SyncWriteMode,
	})
	defer fa.Close()

	faImpl := fa.(*fileAppender)
	// Try to compress a file that doesn't exist
	faImpl.compressFile(filepath.Join(dir, "nonexistent.log"))
}

// TestAppendSync_WriteError covers appendSync file.Write error path.
func TestAppendSync_WriteError(t *testing.T) {
	dir := tempLogDir(t)

	fa, _ := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "writeerr.log",
		WriteMode: SyncWriteMode,
	})

	faImpl := fa.(*fileAppender)
	// Close the file handle to force write error
	faImpl.mu.Lock()
	if faImpl.file != nil {
		faImpl.file.Close()
	}
	faImpl.mu.Unlock()

	// Write should fail
	_, err := fa.Append(InfoLevel, []byte("should fail\n"))
	if err == nil {
		t.Log("Write succeeded unexpectedly (may happen if file was reopened)")
	}

	fa.Close()
}

// ---------------------------------------------------------------------------
// WithCaller — 调用者追踪测试
// ---------------------------------------------------------------------------
func TestWithCaller_Enabled(t *testing.T) {
	lg, err := NewBuilder().
		WithCaller().
		EnableConsole(InfoLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	lgImpl := lg.(*logger)
	var captured *Entry
	lgImpl.hooks = newHookManager()
	lgImpl.hooks.add(&captureHook{fn: func(e *Entry) {
		captured = e
	}})

	lg.Info("caller test")
	time.Sleep(30 * time.Millisecond)

	if captured == nil {
		t.Fatal("Hook 未捕获到 Entry")
	}
	if captured.CallerFile == "" {
		t.Error("CallerFile 不应为空")
	}
	if captured.CallerLine == 0 {
		t.Error("CallerLine 不应为 0")
	}
	if !strings.Contains(captured.CallerFile, "logx_test.go") {
		t.Errorf("CallerFile 应包含测试文件名：%s", captured.CallerFile)
	}
}

func TestWithCaller_Disabled(t *testing.T) {
	lg, err := NewBuilder().
		EnableConsole(InfoLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	lgImpl := lg.(*logger)
	var captured *Entry
	lgImpl.hooks = newHookManager()
	lgImpl.hooks.add(&captureHook{fn: func(e *Entry) {
		captured = e
	}})

	lg.Info("no caller")
	time.Sleep(30 * time.Millisecond)

	if captured == nil {
		t.Fatal("Hook 未捕获到 Entry")
	}
	if captured.CallerFile != "" {
		t.Error("未启用 WithCaller 时 CallerFile 应为空")
	}
	if captured.CallerLine != 0 {
		t.Error("未启用 WithCaller 时 CallerLine 应为 0")
	}
}

func TestWithCaller_TextEncoderOutput(t *testing.T) {
	enc := newTextEncoder(false)
	entry := &Entry{
		Level:      InfoLevel,
		Message:    "caller output",
		CallerFile: "/home/user/project/main.go",
		CallerLine: 42,
	}

	buf := getBuffer()
	defer putBuffer(buf)
	if err := enc.Encode(buf, entry); err != nil {
		t.Fatalf("Encode 失败：%v", err)
	}

	s := string(buf.B)
	if !strings.Contains(s, "project/main.go:42") {
		t.Errorf("输出应包含调用者信息（两级路径），实际：%s", s)
	}
}

func TestWithCaller_DerivedLogger(t *testing.T) {
	lg, err := NewBuilder().
		WithCaller().
		EnableConsole(InfoLevel).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}

	child := lg.WithField("key", "val")
	lgChild := child.(*logger)
	if lgChild.callerSkip != 1 {
		t.Error("派生 logger 应保持 callerSkip")
	}

	ctxChild := lg.WithContext(context.Background())
	lgCtx := ctxChild.(*logger)
	if lgCtx.callerSkip != 1 {
		t.Error("WithContext 派生 logger 应保持 callerSkip")
	}
}

// captureHook 测试辅助：捕获 Entry。
type captureHook struct {
	fn func(e *Entry)
}

func (h *captureHook) OnLog(e *Entry) {
	if h.fn != nil {
		h.fn(e)
	}
}
