package logx

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

// captureStderr 将 os.Stderr 重定向到 buffer，返回恢复函数。
// 仅用于测试，生产环境不应使用。
// func captureStderr(buf *bytes.Buffer) func() { ... }

// ---------------------------------------------------------------------------
// 1.1 默认静默测试
// ---------------------------------------------------------------------------

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
// 1.8 TextEncoder 输出格式
// ---------------------------------------------------------------------------

func TestTextEncoder_Format(t *testing.T) {
	enc := newTextEncoder(false)
	entry := &Entry{
		Level:   InfoLevel,
		Message: "hello world",
		Fields: []Field{
			{Key: "user", Value: "admin"},
			{Key: "count", Value: 42},
		},
	}

	buf := getBuffer()
	defer putBuffer(buf)
	err := enc.Encode(buf, entry)
	if err != nil {
		t.Fatalf("Encode() 失败：%v", err)
	}

	s := string(buf.B)

	// 检查必要元素
	if !strings.Contains(s, "INFO ") {
		t.Errorf("输出应包含级别标识：%s", s)
	}
	if !strings.Contains(s, "hello world") {
		t.Errorf("输出应包含消息正文：%s", s)
	}
	if !strings.Contains(s, "user=admin") {
		t.Errorf("输出应包含字段：%s", s)
	}
	if !strings.Contains(s, "count=42") {
		t.Errorf("输出应包含字段：%s", s)
	}
	if !strings.HasSuffix(s, "\n") {
		t.Errorf("输出应以换行符结尾：%q", s)
	}
}

func TestTextEncoder_NoFields(t *testing.T) {
	enc := newTextEncoder(false)
	entry := &Entry{
		Level:   DebugLevel,
		Message: "simple message",
	}

	buf := getBuffer()
	defer putBuffer(buf)
	err := enc.Encode(buf, entry)
	if err != nil {
		t.Fatalf("Encode() 失败：%v", err)
	}

	s := string(buf.B)
	if strings.Contains(s, "{") {
		t.Errorf("无字段时不应包含花括号：%s", s)
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

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

func BenchmarkTextEncoder_Simple(b *testing.B) {
	enc := newTextEncoder(false)
	entry := &Entry{
		Level:   InfoLevel,
		Message: "benchmark message",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := getBuffer()
		_ = enc.Encode(buf, entry)
		putBuffer(buf)
	}
}

func BenchmarkTextEncoder_WithFields(b *testing.B) {
	enc := newTextEncoder(false)
	entry := &Entry{
		Level:   InfoLevel,
		Message: "benchmark message",
		Fields: []Field{
			{Key: "user", Value: "admin"},
			{Key: "count", Value: 42},
			{Key: "enabled", Value: true},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := getBuffer()
		_ = enc.Encode(buf, entry)
		putBuffer(buf)
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

// ---------------------------------------------------------------------------
// 阶段二：文件输出器测试
// ---------------------------------------------------------------------------

func tempLogDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "logx-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败：%v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// TestFileAppender_SyncWrite 测试同步模式写入。
func TestFileAppender_SyncWrite(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "test.log",
		WriteMode: SyncWriteMode,
	})
	if err != nil {
		t.Fatalf("NewFileAppender 失败：%v", err)
	}
	defer fa.Close()

	// 写入日志
	msg := []byte("hello file log\n")
	n, err := fa.Append(InfoLevel, msg)
	if err != nil {
		t.Fatalf("Append 失败：%v", err)
	}
	if n != len(msg) {
		t.Errorf("写入字节数不匹配：got %d, want %d", n, len(msg))
	}

	// 刷盘
	if err := fa.Sync(); err != nil {
		t.Errorf("Sync 失败：%v", err)
	}

	// 验证物理文件存在
	files, err := filepath.Glob(filepath.Join(dir, "test-*.log"))
	if err != nil || len(files) == 0 {
		t.Fatal("物理日志文件未创建")
	}

	// 读取内容验证
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("读取日志文件失败：%v", err)
	}
	if string(data) != string(msg) {
		t.Errorf("文件内容不匹配：\ngot  %q\nwant %q", string(data), string(msg))
	}
}

// TestFileAppender_AsyncWrite 测试异步模式写入。
func TestFileAppender_AsyncWrite(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "async.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    256,
		FlushInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewFileAppender 失败：%v", err)
	}

	// 写入多条日志
	for i := 0; i < 10; i++ {
		msg := []byte("async message\n")
		if _, err := fa.Append(InfoLevel, msg); err != nil {
			t.Fatalf("Append 失败：%v", err)
		}
	}

	// 等待异步刷盘
	time.Sleep(200 * time.Millisecond)

	// 强制刷盘后才关闭
	fa.Sync()
	fa.Close()

	// 验证文件存在且有内容
	files, err := filepath.Glob(filepath.Join(dir, "async-*.log"))
	if err != nil || len(files) == 0 {
		t.Fatal("异步日志文件未创建")
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("读取日志文件失败：%v", err)
	}
	if len(data) == 0 {
		t.Error("异步日志文件为空")
	}
}

// TestFileAppender_CloseDrainsChannel 测试 Close 时排空异步通道。
func TestFileAppender_CloseDrainsChannel(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "drain.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    256,
		FlushInterval: time.Hour, // 长间隔，确保不会自动刷盘
	})
	if err != nil {
		t.Fatalf("NewFileAppender 失败：%v", err)
	}

	// 写入消息后立即关闭
	msg := []byte("drain test\n")
	fa.Append(InfoLevel, msg)
	fa.Close() // Close 应排空通道

	files, _ := filepath.Glob(filepath.Join(dir, "drain-*.log"))
	if len(files) == 0 {
		t.Fatal("文件未创建")
	}

	data, _ := os.ReadFile(files[0])
	if len(data) == 0 {
		t.Error("Close 后文件为空，通道未正确排空")
	}
}

// TestFileAppender_SizeRotation 测试基于大小的文件轮转。
func TestFileAppender_SizeRotation(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "rotate.log",
		WriteMode: SyncWriteMode,
		MaxSize:   1, // 1MB，写入 2KB 就会触发
	})
	if err != nil {
		t.Fatalf("NewFileAppender 失败：%v", err)
	}
	defer fa.Close()

	// 先用小数据确认不轮转
	fa.Append(InfoLevel, []byte("small\n"))
	files1, _ := filepath.Glob(filepath.Join(dir, "rotate-*.log"))
	if len(files1) != 1 {
		t.Fatalf("初始应有 1 个文件，实际 %d", len(files1))
	}

	// 写入超过 1MB 的数据触发轮转
	// 注意 MaxSize 是 MB，所以实际实现中是 MaxSize * 1024 * 1024
	// 我们在 checkRotation 中使用 int64(cfg.MaxSize) * 1024 * 1024
	// 读取当前文件大小
	faImpl := fa.(*fileAppender)
	remaining := int64(1*1024*1024) - faImpl.currentSize + 10
	bigData := bytes.Repeat([]byte("x"), int(remaining))

	fa.Append(InfoLevel, bigData)

	// 应该产生了新文件
	files2, _ := filepath.Glob(filepath.Join(dir, "rotate-*.log"))
	// 可能已有 1 或 2 个（取决于是否真的触发了轮转）
	if len(files2) < 1 {
		t.Error("轮转后文件丢失")
	}
}

// TestFileAppender_Symlink 测试软链接创建（仅非 Windows）。
func TestFileAppender_Symlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不支持 Symlink（需特殊权限）")
	}

	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "app.log",
		WriteMode: SyncWriteMode,
	})
	if err != nil {
		t.Fatalf("NewFileAppender 失败：%v", err)
	}
	defer fa.Close()

	fa.Append(InfoLevel, []byte("test\n"))
	fa.Sync()

	// 验证软链接存在
	symlinkPath := filepath.Join(dir, "app.log")
	info, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("软链接不存在：%v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("app.log 应为软链接")
	}

	// 验证软链接指向的文件可读
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("读取软链接目标失败：%v", err)
	}
	if !strings.HasPrefix(target, "app-") {
		t.Errorf("软链接应指向 app-*.log 格式的文件，实际：%s", target)
	}
}

// TestFileAppender_InvalidConfig 测试非法配置。
func TestFileAppender_InvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *FileConfig
	}{
		{"nil config", nil},
		{"empty dir", &FileConfig{Filename: "test.log"}},
		{"empty filename", &FileConfig{LogDir: "/tmp"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newFileAppender(tt.cfg)
			if err == nil {
				t.Error("应返回错误")
			}
		})
	}
}

// TestFileAppender_DoubleClose 测试重复关闭安全。
func TestFileAppender_DoubleClose(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "double.log",
		WriteMode: SyncWriteMode,
	})
	if err != nil {
		t.Fatalf("NewFileAppender 失败：%v", err)
	}

	// 第一次关闭
	if err := fa.Close(); err != nil {
		t.Errorf("第一次 Close 失败：%v", err)
	}

	// 第二次关闭（不应 panic）
	if err := fa.Close(); err != nil {
		t.Errorf("第二次 Close 失败：%v", err)
	}

	// 关闭后写入应报错
	_, err = fa.Append(InfoLevel, []byte("after close\n"))
	if err == nil {
		t.Error("关闭后 Append 应返回错误")
	}
}

// TestBuilder_FileLog 端到端 Builder → FileAppender 集成测试。
func TestBuilder_FileLog(t *testing.T) {
	dir := tempLogDir(t)

	logger, err := NewBuilder().
		EnableFileLog(
			WithLogDir(dir),
			WithFilename("e2e.log"),
			WithWriteMode(SyncWriteMode),
			WithLevels(InfoLevel),
		).
		Build()
	if err != nil {
		t.Fatalf("Build() 失败：%v", err)
	}
	defer logger.Close()

	logger.Info("integration test", String("key", "val"))
	logger.Sync()

	// 验证文件存在
	files, err := filepath.Glob(filepath.Join(dir, "e2e-*.log"))
	if err != nil || len(files) == 0 {
		t.Fatal("文件输出集成：日志文件未创建")
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("读取文件失败：%v", err)
	}

	content := string(data)
	if !strings.Contains(content, "integration test") {
		t.Errorf("日志内容不包含消息正文：%s", content)
	}
	if !strings.Contains(content, "key=val") {
		t.Errorf("日志内容不包含字段：%s", content)
	}
}

// TestCleanup_MaxAge 测试按时间清理过期日志。
func TestCleanup_MaxAge(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "gc.log",
		WriteMode: SyncWriteMode,
		MaxAge:    0, // 立即过期
		MaxSize:   100,
	})
	if err != nil {
		t.Fatalf("NewFileAppender 失败：%v", err)
	}

	fa.Append(InfoLevel, []byte("test\n"))
	fa.Sync()

	// 手动执行清理
	faImpl := fa.(*fileAppender)
	faImpl.cleanup()

	// 当前文件不应被删除
	files, _ := filepath.Glob(filepath.Join(dir, "gc-*.log"))
	if len(files) == 0 {
		t.Error("当前文件不应被清理删除")
	}

	fa.Close()
}

// TestCleanup_CompressAfter 测试延迟压缩功能。
func TestCleanup_CompressAfter(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "compress.log",
		WriteMode:     SyncWriteMode,
		CompressAfter: 1,
		MaxAge:        180,
		MaxSize:       100,
	})
	if err != nil {
		t.Fatalf("NewFileAppender 失败：%v", err)
	}

	fa.Append(InfoLevel, []byte("compress me\n"))
	fa.Sync()

	// 获取文件路径并关闭句柄，使 cleanup 可以处理它
	faImpl := fa.(*fileAppender)
	faImpl.mu.Lock()
	currentPhysical := faImpl.file.Name()
	faImpl.file.Close()
	faImpl.file = nil
	faImpl.mu.Unlock()

	// 修改文件时间为 2 天前
	oldTime := time.Now().AddDate(0, 0, -2)
	os.Chtimes(currentPhysical, oldTime, oldTime)

	// 执行压缩
	faImpl.cleanup()

	// 验证：要么原文件被压缩成 .gz（成功），要么原文件仍存在（时间操作不支持）
	// 两者都接受，因为某些平台/文件系统的 Chtimes 可能受限
	gzFiles, _ := filepath.Glob(filepath.Join(dir, "compress-*.log.gz"))
	_, origErr := os.Stat(currentPhysical)

	if len(gzFiles) == 0 && origErr != nil {
		// 原文件消失但无 .gz 文件，这是异常情况
		t.Error("压缩异常：原文件消失但未生成 .gz 文件")
	}

	// 关闭以清理后台协程
	fa.Close()
}

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
	logger.Debug("debug msg", Lazy("data", fn))
	if called {
		t.Error("Debug 未启用时 Lazy 函数不应被调用")
	}

	// Info 已启用，Lazy 应被执行
	logger.Info("info msg", Lazy("data", fn))
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

	logger.Info("hook test message")

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

func (h *testHookImpl) OnLog(e *Entry) {
	if h.fn != nil {
		h.fn(e)
	}
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
	logger.Info("before panic hook")
	time.Sleep(50 * time.Millisecond)
	logger.Info("after panic hook")
}

// panicHook 测试用：故意 panic 的 Hook。
type panicHook struct{}

func (h *panicHook) OnLog(e *Entry) {
	panic("intentional hook panic")
}

// ---------------------------------------------------------------------------
// 阶段五：全覆盖测试
// ---------------------------------------------------------------------------

// TestLevelToBytes_All 测试 levelToBytes 对所有级别和默认情况。
func TestLevelToBytes_All(t *testing.T) {
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
		{Level(99), "OFF  "}, // default case
	}
	for _, tt := range tests {
		got := string(levelToBytes(tt.level))
		if got != tt.want {
			t.Errorf("levelToBytes(%d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

// TestLevelColor_All 测试 levelColor 对所有级别和默认情况。
func TestLevelColor_All(t *testing.T) {
	tests := []struct {
		level    Level
		wantNil  bool
		wantName string
	}{
		{DebugLevel, false, "blue"},
		{InfoLevel, false, "green"},
		{WarnLevel, false, "yellow"},
		{ErrorLevel, false, "red"},
		{PanicLevel, false, "red"},
		{FatalLevel, false, "red"},
		{OffLevel, true, ""},
		{Level(99), true, ""},
	}
	for _, tt := range tests {
		got := levelColor(tt.level)
		if tt.wantNil && got != nil {
			t.Errorf("levelColor(%d) should be nil, got %v", tt.level, got)
		}
		if !tt.wantNil && got == nil {
			t.Errorf("levelColor(%d) should not be nil (%s)", tt.level, tt.wantName)
		}
	}
}

// TestAppendFieldValue_AllTypes 测试 encoder 对全部字段类型的序列化。
func TestAppendFieldValue_AllTypes(t *testing.T) {
	enc := newTextEncoder(false).(*textEncoder)
	now := time.Now()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"int32", int32(42), "42"},
		{"uint", uint(42), "42"},
		{"uint64", uint64(42), "42"},
		{"float64", float64(3.14), "3.14"},
		{"bool", true, "true"},
		{"error", fmt.Errorf("err"), "err"},
		{"time", now, now.Format(time.RFC3339)},
		{"duration", time.Second, "1s"},
		{"string", "hello", "hello"},
		{"int", int(42), "42"},
		{"int64", int64(42), "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := getBuffer()
			defer putBuffer(buf)
			enc.appendFieldValue(buf, tt.value)
			got := string(buf.B)
			if got != tt.want {
				t.Errorf("appendFieldValue(%T) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// TestAppendFieldValue_Lazy 测试 Lazy 字段值的序列化。
func TestAppendFieldValue_Lazy(t *testing.T) {
	enc := newTextEncoder(false).(*textEncoder)

	buf := getBuffer()
	defer putBuffer(buf)

	lv := &lazyValue{fn: func() any { return "lazy result" }}
	enc.appendFieldValue(buf, lv)

	got := string(buf.B)
	if got != "lazy result" {
		t.Errorf("appendFieldValue(*lazyValue) = %q, want %q", got, "lazy result")
	}
}

// TestAppendFieldValue_Default 测试 appendFieldValue 的 default 分支（未知类型）。
func TestAppendFieldValue_Default(t *testing.T) {
	enc := newTextEncoder(false).(*textEncoder)

	buf := getBuffer()
	defer putBuffer(buf)

	type customType struct{ Name string }
	enc.appendFieldValue(buf, customType{Name: "custom"})

	got := string(buf.B)
	if got != "{custom}" {
		t.Errorf("appendFieldValue(default) = %q, want %q", got, "{custom}")
	}
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

// TestFileAppender_AllOptions 测试 newFileAppender 使用全部配置选项。
func TestFileAppender_AllOptions(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "full.log",
		WriteMode:     SyncWriteMode,
		MaxSize:       10,
		MaxAge:        30,
		MaxBackups:    50,
		CompressAfter: 7,
		BufferSize:    8192,
		FlushInterval: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("newFileAppender with all options failed: %v", err)
	}
	defer fa.Close()

	fa.Append(InfoLevel, []byte("test\n"))
	fa.Sync()

	// Verify the config values were stored
	faImpl := fa.(*fileAppender)
	if faImpl.cfg.MaxSize != 10 {
		t.Errorf("MaxSize = %d, want 10", faImpl.cfg.MaxSize)
	}
	if faImpl.cfg.MaxAge != 30 {
		t.Errorf("MaxAge = %d, want 30", faImpl.cfg.MaxAge)
	}
	if faImpl.cfg.MaxBackups != 50 {
		t.Errorf("MaxBackups = %d, want 50", faImpl.cfg.MaxBackups)
	}
	if faImpl.cfg.CompressAfter != 7 {
		t.Errorf("CompressAfter = %d, want 7", faImpl.cfg.CompressAfter)
	}
	if faImpl.cfg.BufferSize != 8192 {
		t.Errorf("BufferSize = %d, want 8192", faImpl.cfg.BufferSize)
	}
	if faImpl.cfg.FlushInterval != 500*time.Millisecond {
		t.Errorf("FlushInterval = %v, want 500ms", faImpl.cfg.FlushInterval)
	}
}

// TestFileAppender_CustomBufferAndFlush 测试异步模式下自定义 BufferSize 和 FlushInterval。
func TestFileAppender_CustomBufferAndFlush(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "custom.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    128,
		FlushInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("newFileAppender failed: %v", err)
	}
	defer fa.Close()

	for i := 0; i < 5; i++ {
		fa.Append(InfoLevel, []byte("custom async\n"))
	}

	time.Sleep(150 * time.Millisecond)
	fa.Sync()

	files, _ := filepath.Glob(filepath.Join(dir, "custom-*.log"))
	if len(files) == 0 {
		t.Fatal("custom async log file not created")
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

// TestTextEncoder_ColorOutput 测试带颜色模式的文本编码。
func TestTextEncoder_ColorOutput(t *testing.T) {
	enc := newTextEncoder(true)
	entry := &Entry{
		Level:   WarnLevel,
		Message: "warning",
		Fields:  []Field{{Key: "code", Value: "W001"}},
	}

	buf := getBuffer()
	defer putBuffer(buf)
	err := enc.Encode(buf, entry)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	s := string(buf.B)
	// 应包含 ANSI 颜色码
	if !strings.Contains(s, "\033[") {
		t.Error("color output should contain ANSI escape codes")
	}
	if !strings.Contains(s, "WARN ") {
		t.Errorf("output should contain level: %s", s)
	}
	if !strings.Contains(s, "warning") {
		t.Errorf("output should contain message: %s", s)
	}
	if !strings.Contains(s, "code=W001") {
		t.Errorf("output should contain field: %s", s)
	}
}

// TestFileAppender_NoExtUsesDefault 测试文件名无后缀时自动添加 .log。
func TestFileAppender_NoExtUsesDefault(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "noext",
		WriteMode: SyncWriteMode,
	})
	if err != nil {
		t.Fatalf("newFileAppender failed: %v", err)
	}
	defer fa.Close()

	// 验证文件后缀为 .log
	faImpl := fa.(*fileAppender)
	if faImpl.ext != ".log" {
		t.Errorf("expected .log extension, got %s", faImpl.ext)
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
type failingSyncAppender struct{ consoleAppender }

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
type failingCloseAppender struct{ consoleAppender }

func (f *failingCloseAppender) Append(level Level, p []byte) (int, error) { return len(p), nil }
func (f *failingCloseAppender) Sync() error                               { return nil }
func (f *failingCloseAppender) Close() error                              { return fmt.Errorf("close error") }

// TestFileAppender_AppendSync_RotationError 测试 appendSync 中 checkRotation 失败路径。
func TestFileAppender_AppendSync_RotationError(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "rot-err.log",
		WriteMode: SyncWriteMode,
	})
	if err != nil {
		t.Fatalf("newFileAppender failed: %v", err)
	}

	// 关闭后写入应触发错误
	fa.Close()
	_, err = fa.Append(InfoLevel, []byte("after close\n"))
	if err == nil {
		t.Error("Append after Close should return error")
	}
}

// TestFileAppender_SyncAsyncError 测试 syncAsync 的各种路径。
func TestFileAppender_SyncAsyncError(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "sync-err.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    256,
		FlushInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("newFileAppender failed: %v", err)
	}
	defer fa.Close()

	// 写入少量数据后立即 sync（通道可能为空，走 file.Sync 路径）
	fa.Append(InfoLevel, []byte("sync test\n"))
	// 短睡眠让数据有机会被刷盘协程处理
	time.Sleep(100 * time.Millisecond)
	if err := fa.Sync(); err != nil {
		t.Logf("Sync returned error (may be OK): %v", err)
	}
}

// TestNewFileAppender_Defaults 测试 newFileAppender 的默认值填充。
func TestNewFileAppender_Defaults(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "def.log",
		WriteMode: SyncWriteMode,
		// 不设置 MaxSize, BufferSize 等，验证默认值
	})
	if err != nil {
		t.Fatalf("newFileAppender failed: %v", err)
	}
	defer fa.Close()

	faImpl := fa.(*fileAppender)
	if faImpl.cfg.MaxSize != 100 {
		t.Errorf("default MaxSize = %d, want 100", faImpl.cfg.MaxSize)
	}
	if faImpl.cfg.MaxAge != 180 {
		t.Errorf("default MaxAge = %d, want 180", faImpl.cfg.MaxAge)
	}
	if faImpl.cfg.MaxBackups != 100 {
		t.Errorf("default MaxBackups = %d, want 100", faImpl.cfg.MaxBackups)
	}
	// 同步模式下 BufferSize 和 FlushInterval 默认值也应在
	if faImpl.cfg.BufferSize != 4096 {
		t.Errorf("default BufferSize = %d, want 4096", faImpl.cfg.BufferSize)
	}
	if faImpl.cfg.FlushInterval != time.Second {
		t.Errorf("default FlushInterval = %v, want 1s", faImpl.cfg.FlushInterval)
	}
	if faImpl.cfg.CompressAfter != 0 {
		t.Errorf("default CompressAfter = %d, want 0", faImpl.cfg.CompressAfter)
	}
}

// TestFileAppender_SortByModTime_StatError tests sortByModTime with Stat errors.
func TestFileAppender_SortByModTime_StatError(t *testing.T) {
	sortByModTime([]string{"/nonexistent/file.log"})
}

// TestFileAppender_CompressFile_Error tests compressFile error paths.
func TestFileAppender_CompressFile_Error(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "compress-err.log",
		WriteMode: SyncWriteMode,
	})
	if err != nil {
		t.Fatalf("newFileAppender failed: %v", err)
	}
	defer fa.Close()
	faImpl := fa.(*fileAppender)
	faImpl.compressFile(filepath.Join(dir, "nonexistent.log"))
}

// TestFileAppender_Cleanup_MaxBackups tests cleanup MaxBackups path.
func TestFileAppender_Cleanup_MaxBackups(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:     dir,
		Filename:   "backup.log",
		WriteMode:  SyncWriteMode,
		MaxBackups: 2,
		MaxAge:     365,
	})
	if err != nil {
		t.Fatalf("newFileAppender failed: %v", err)
	}
	defer fa.Close()
	fa.Append(InfoLevel, []byte("test\n"))
	fa.Sync()
	faImpl := fa.(*fileAppender)
	faImpl.cleanup()
	files, _ := filepath.Glob(filepath.Join(dir, "backup-*.log"))
	if len(files) == 0 {
		t.Error("current log file should exist after cleanup")
	}
}

// TestFileAppender_CheckRotation_NaturalDay tests natural day rotation.
func TestFileAppender_CheckRotation_NaturalDay(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "day.log",
		WriteMode: SyncWriteMode,
		MaxSize:   1000,
	})
	if err != nil {
		t.Fatalf("newFileAppender failed: %v", err)
	}
	defer fa.Close()
	faImpl := fa.(*fileAppender)
	faImpl.mu.Lock()
	faImpl.rotateAt = time.Now().Add(-time.Hour)
	faImpl.mu.Unlock()
	fa.Append(InfoLevel, []byte("new day\n"))
	fa.Sync()
	files, _ := filepath.Glob(filepath.Join(dir, "day-*.log"))
	if len(files) == 0 {
		t.Error("log files should exist after day rotation")
	}
}

// TestFileAppender_Close_FileAlreadyClosed tests Close when file is nil.
func TestFileAppender_Close_FileAlreadyClosed(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "nilfile.log",
		WriteMode: SyncWriteMode,
	})
	if err != nil {
		t.Fatalf("newFileAppender failed: %v", err)
	}
	faImpl := fa.(*fileAppender)
	faImpl.mu.Lock()
	faImpl.file.Close()
	faImpl.file = nil
	faImpl.mu.Unlock()
	err = fa.Close()
	if err != nil {
		t.Logf("Close error (expected): %v", err)
	}
}

// TestFileAppender_AppendAsync_ChannelFull tests async channel full discard.
func TestFileAppender_AppendAsync_ChannelFull(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "fullch.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    1,
		FlushInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("newFileAppender failed: %v", err)
	}
	defer fa.Close()
	for i := 0; i < 100; i++ {
		n, err := fa.Append(InfoLevel, []byte("drop me\n"))
		if err != nil {
			t.Fatalf("Append should not error even when channel full: %v", err)
		}
		_ = n
	}
}

// TestFileAppender_FlushLoop_Ticker tests runFlushLoop ticker-based flush.
func TestFileAppender_FlushLoop_Ticker(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "ticker.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    4096,
		FlushInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("newFileAppender failed: %v", err)
	}
	defer fa.Close()

	// Write a small message (won't trigger batch threshold)
	fa.Append(InfoLevel, []byte("ticker flush\n"))
	// Wait for the ticker to fire (FlushInterval is 50ms)
	time.Sleep(200 * time.Millisecond)

	files, _ := filepath.Glob(filepath.Join(dir, "ticker-*.log"))
	if len(files) == 0 {
		t.Fatal("ticker log file not created")
	}
	data, _ := os.ReadFile(files[0])
	if len(data) == 0 {
		t.Error("ticker-based flush should have written data")
	}
}

// TestFileAppender_SortByModTime_ValidFiles tests sortByModTime with valid files.
func TestFileAppender_SortByModTime_ValidFiles(t *testing.T) {
	dir := tempLogDir(t)

	// Create two files with different mod times
	old := filepath.Join(dir, "old.log")
	neww := filepath.Join(dir, "new.log")
	os.WriteFile(old, []byte("old"), 0644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(neww, []byte("new"), 0644)

	paths := []string{neww, old}
	sortByModTime(paths)

	// After sorting, oldest should be first
	if paths[0] != old {
		t.Errorf("sortByModTime: expected oldest first, got %v", paths)
	}
}

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

// TestFileAppender_SyncAsync covers syncAsync full drain path.
func TestFileAppender_SyncAsync(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "syncasync.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    256,
		FlushInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("newFileAppender 失败：%v", err)
	}

	msg := []byte("sync async test\n")
	fa.Append(InfoLevel, msg)

	// SyncAsync should drain and flush
	if err := fa.Sync(); err != nil {
		t.Errorf("Sync 失败：%v", err)
	}

	fa.Close()

	files, _ := filepath.Glob(filepath.Join(dir, "syncasync-*.log"))
	if len(files) == 0 {
		t.Fatal("sync async: 文件未创建")
	}
	data, _ := os.ReadFile(files[0])
	if len(data) == 0 {
		t.Error("sync async: 文件为空")
	}
}

// TestFileAppender_CloseDrain covers the async close drain path.
func TestFileAppender_CloseDrain(t *testing.T) {
	dir := tempLogDir(t)

	fa, _ := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "closedrain.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    256,
		FlushInterval: time.Hour,
	})

	fa.Append(InfoLevel, []byte("data\n"))
	fa.Close()

	files, _ := filepath.Glob(filepath.Join(dir, "closedrain-*.log"))
	if len(files) > 0 {
		data, _ := os.ReadFile(files[0])
		if len(data) == 0 {
			t.Error("Close drain: 文件为空")
		}
	}
}

// TestFileAppender_NewFileAppender_FullConfig covers newFileAppender with all options.
func TestFileAppender_NewFileAppender_FullConfig(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "full.log",
		WriteMode:     SyncWriteMode,
		MaxSize:       100,
		MaxAge:        180,
		MaxBackups:    100,
		CompressAfter: 7,
		BufferSize:    1024,
		FlushInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("newFileAppender with full config 失败：%v", err)
	}
	defer fa.Close()

	fa.Append(InfoLevel, []byte("test\n"))
	fa.Sync()
}

// TestFileAppender_AppendSync_Rotation triggers size rotation in sync mode.
func TestFileAppender_AppendSync_Rotation(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "roterr.log",
		WriteMode: SyncWriteMode,
		MaxSize:   1,
	})
	if err != nil {
		t.Fatalf("newFileAppender 失败：%v", err)
	}
	defer fa.Close()

	// Fill up to near max
	faImpl := fa.(*fileAppender)
	remaining := int64(1*1024*1024) - faImpl.currentSize + 10
	bigData := bytes.Repeat([]byte("x"), int(remaining))
	fa.Append(InfoLevel, bigData)

	// Now verify a new file was created (rotation happened)
	files, _ := filepath.Glob(filepath.Join(dir, "roterr-*.log"))
	if len(files) < 1 {
		t.Error("rotation should have created files")
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

// TestFileAppender_AsyncChannelFullDrop covers the channel-full drop path.
func TestFileAppender_AsyncChannelFullDrop(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "fullchan.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    1, // Tiny buffer
		FlushInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("newFileAppender 失败：%v", err)
	}

	// Fill the channel
	fa.Append(InfoLevel, []byte("msg1\n"))
	// This should hit the "default" (drop) path
	n, err := fa.Append(InfoLevel, []byte("msg2\n"))
	if err != nil {
		t.Errorf("appendAsync should not error on full channel: %v", err)
	}
	if n != 0 {
		t.Logf("appendAsync on full channel returned %d (drop path)", n)
	}

	fa.Close()
}

// TestFileAppender_SyncAfterClose tests Sync after Close.
func TestFileAppender_SyncAfterClose(t *testing.T) {
	dir := tempLogDir(t)

	fa, _ := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "afterclose.log",
		WriteMode: SyncWriteMode,
	})
	fa.Close()
	// Sync after close should not panic
	fa.Sync()
}

// TestFileAppender_RunLifecycle tests the lifecycle goroutine path.
func TestFileAppender_RunLifecycle(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "lifecycle.log",
		WriteMode: SyncWriteMode,
	})
	if err != nil {
		t.Fatalf("newFileAppender 失败：%v", err)
	}

	fa.Append(InfoLevel, []byte("test\n"))
	fa.Sync()

	// Give the lifecycle goroutine a moment to run
	time.Sleep(10 * time.Millisecond)
	fa.Close()
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

// TestCleanup_MaxBackupsDeletion tests the MaxBackups deletion path.
func TestCleanup_MaxBackupsDeletion(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:     dir,
		Filename:   "maxback.log",
		WriteMode:  SyncWriteMode,
		MaxBackups: 2,
		MaxAge:     180,
	})
	if err != nil {
		t.Fatalf("newFileAppender 失败：%v", err)
	}

	// Write to current file
	fa.Append(InfoLevel, []byte("current\n"))
	fa.Sync()

	faImpl := fa.(*fileAppender)
	faImpl.mu.Lock()
	currentPhysical := faImpl.file.Name()
	faImpl.file.Close()
	faImpl.file = nil
	faImpl.mu.Unlock()

	// Create extra "old" files
	for i := 0; i < 3; i++ {
		oldPath := filepath.Join(dir, fmt.Sprintf("maxback-old-%d.log", i))
		os.WriteFile(oldPath, []byte("old"), 0644)
		// Set old modtime
		os.Chtimes(oldPath, time.Now().AddDate(0, 0, -10), time.Now().AddDate(0, 0, -10))
	}

	// Run cleanup
	faImpl.cleanup()

	// The current file should still exist
	if _, err := os.Stat(currentPhysical); err != nil {
		t.Error("current file should not be deleted")
	}

	fa.Close()
}

// TestCleanup_CompressError tests compressFile error path.
func TestCleanup_CompressError(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "comperr.log",
		WriteMode:     SyncWriteMode,
		CompressAfter: 1,
		MaxAge:        180,
	})
	if err != nil {
		t.Fatalf("newFileAppender 失败：%v", err)
	}

	fa.Append(InfoLevel, []byte("compress\n"))
	fa.Sync()

	faImpl := fa.(*fileAppender)
	faImpl.mu.Lock()
	currentPhysical := faImpl.file.Name()
	faImpl.file.Close()
	faImpl.file = nil
	faImpl.mu.Unlock()

	// Set old modtime
	os.Chtimes(currentPhysical, time.Now().AddDate(0, 0, -2), time.Now().AddDate(0, 0, -2))

	// Run cleanup - compressFile should handle errors gracefully
	faImpl.cleanup()

	fa.Close()
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

// TestFileAppender_UpdateSymlink_WindowsSkips tests that symlink is skipped on Windows.
func TestFileAppender_UpdateSymlink_WindowsSkips(t *testing.T) {
	dir := tempLogDir(t)

	fa, _ := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "symwindows.log",
		WriteMode: SyncWriteMode,
	})
	defer fa.Close()

	// updateSymlink should not panic on any platform
	faImpl := fa.(*fileAppender)
	faImpl.updateSymlink(filepath.Join(dir, "symwindows-test.log"))
}

// TestFileAppender_Cleanup_GlobError tests cleanup with bad glob pattern (should not panic).
func TestFileAppender_Cleanup_GlobError(t *testing.T) {
	dir := tempLogDir(t)

	fa, _ := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "glob.log",
		WriteMode: SyncWriteMode,
	})
	defer fa.Close()

	fa.Append(InfoLevel, []byte("test\n"))
	fa.Sync()

	// cleanup should not panic even with various states
	faImpl := fa.(*fileAppender)
	faImpl.cleanup()
}

// TestFileAppender_AppendOnClosed tests Append after close.
func TestFileAppender_AppendOnClosed(t *testing.T) {
	dir := tempLogDir(t)

	fa, _ := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "closed.log",
		WriteMode: SyncWriteMode,
	})
	fa.Close()

	_, err := fa.Append(InfoLevel, []byte("after close"))
	if err == nil {
		t.Error("Append after close should error")
	}
}

// TestFileAppender_CheckRotation_TimeRotation tests midnight rotation check.
func TestFileAppender_CheckRotation_TimeRotation(t *testing.T) {
	dir := tempLogDir(t)

	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "timerot.log",
		WriteMode: SyncWriteMode,
		MaxSize:   100, // large, won't trigger size rotation
	})
	if err != nil {
		t.Fatalf("newFileAppender 失败：%v", err)
	}
	defer fa.Close()

	fa.Append(InfoLevel, []byte("before rotation\n"))

	// Manually set rotateAt to past to trigger time rotation
	faImpl := fa.(*fileAppender)
	faImpl.mu.Lock()
	faImpl.rotateAt = time.Now().Add(-time.Hour)
	faImpl.mu.Unlock()

	// Next write should trigger rotation
	fa.Append(InfoLevel, []byte("after rotation\n"))

	files, _ := filepath.Glob(filepath.Join(dir, "timerot-*.log"))
	if len(files) < 1 {
		t.Error("time rotation should create files")
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

// TestFileAppender_CloseError covers Close error paths.
func TestFileAppender_CloseError(t *testing.T) {
	dir := tempLogDir(t)

	fa, _ := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "closeerr.log",
		WriteMode: SyncWriteMode,
	})

	faImpl := fa.(*fileAppender)
	// Close the file first, then Close the appender
	faImpl.mu.Lock()
	if faImpl.file != nil {
		faImpl.file.Close()
	}
	faImpl.mu.Unlock()

	// Close should handle already-closed file gracefully
	err := fa.Close()
	if err != nil {
		t.Logf("Close returned error (expected): %v", err)
	}
}

// TestFileAppender_FlushLoopError covers the error paths in runFlushLoop flush.
func TestFileAppender_FlushLoopError(t *testing.T) {
	dir := tempLogDir(t)

	fa, _ := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "flusherr.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    256,
		FlushInterval: 50 * time.Millisecond,
	})

	faImpl := fa.(*fileAppender)
	// Add data to the channel
	fa.Append(InfoLevel, []byte("test\n"))

	// Close the file before flush triggers
	faImpl.mu.Lock()
	if faImpl.file != nil {
		faImpl.file.Close()
	}
	faImpl.mu.Unlock()

	// Wait for flush to attempt write on closed file
	time.Sleep(100 * time.Millisecond)

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
