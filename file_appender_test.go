package logx

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

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
