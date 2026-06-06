package logx

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Builder — 链式配置构造器
// ---------------------------------------------------------------------------

// Builder 是 logx 的链式配置构造器，支持多通道精细化级别控制。
//
// 典型用法：
//
//	logger, err := logx.NewBuilder().
//	    EnableConsole(logx.InfoLevel, logx.ErrorLevel, logx.WithColor()).
//	    EnableFileLog(
//	        logx.WithLogDir("/var/log/myapp"),
//	        logx.WithFilename("app.log"),
//	        logx.WithLevels(logx.InfoLevel),
//	    ).
//	    Build()
type Builder struct {
	cores      []coreConfig
	callerSkip int // 0=禁用调用者追踪，3=启用（默认跳过值）
}

// coreConfig 描述一个输出通道的配置。
type coreConfig struct {
	appType appenderType // 通道类型
	minLvl  Level        // 该通道启用的最低日志级别
	color   bool         // 控制台是否启用颜色
	fileCfg *FileConfig  // 文件通道专属配置（nil 表示控制台通道）
}

// appenderType 标识输出通道类型。
type appenderType string

const (
	consoleAppenderType appenderType = "console"
	fileAppenderType    appenderType = "file"
)

// osExit 是 os.Exit 的包装，允许测试中替换。
var osExit = os.Exit

// NewBuilder 创建一个新的 Builder 实例，所有通道默认静默。
func NewBuilder() *Builder {
	return &Builder{}
}

// WithCaller 启用调用者追踪，日志输出中将包含源文件名和行号。
// 格式：file.go:42
func (b *Builder) WithCaller() *Builder {
	b.callerSkip = 1 // 从 runtime.Caller 开始，由 isLogxFrame 过滤到业务代码
	return b
}

// ---------------------------------------------------------------------------
// ConsoleOption — 控制台通道配置选项
// ---------------------------------------------------------------------------

// ConsoleOption 是 EnableConsole 接受的配置选项。
// Level 和 WithColor() 均实现此接口。
type ConsoleOption interface {
	isConsoleOption()
}

// isConsoleOption 使 Level 可用作 ConsoleOption。
func (l Level) isConsoleOption() { _ = l }

// colorOption 是 WithColor() 返回的 ConsoleOption 实现。
type colorOption struct{}

func (c colorOption) isConsoleOption() { _ = c }

// WithColor 返回一个 ConsoleOption，为控制台通道启用 ANSI 色彩高亮。
func WithColor() ConsoleOption {
	return colorOption{}
}

// ---------------------------------------------------------------------------
// 控制台通道
// ---------------------------------------------------------------------------

// EnableConsole 开启控制台输出通道。
// 接受零个或多个 ConsoleOption 参数：
//   - Level 值（InfoLevel, ErrorLevel 等）指定启用的日志级别
//   - WithColor() 启用 ANSI 色彩
//
// 若未传入任何 Level，该通道保持静默。
func (b *Builder) EnableConsole(opts ...ConsoleOption) *Builder {
	cfg := coreConfig{
		appType: consoleAppenderType,
		minLvl:  OffLevel,
	}

	for _, opt := range opts {
		switch v := opt.(type) {
		case Level:
			if cfg.minLvl == OffLevel || v < cfg.minLvl {
				cfg.minLvl = v
			}
		case colorOption:
			cfg.color = true
		}
	}

	b.cores = append(b.cores, cfg)
	return b
}

// ---------------------------------------------------------------------------
// 文件通道配置
// ---------------------------------------------------------------------------

// WriteMode 文件写入模式。
type WriteMode int

const (
	// AsyncWriteMode 异步批量模式（默认推荐）。业务协程无阻塞，后台批量刷盘。
	AsyncWriteMode WriteMode = iota
	// SyncWriteMode 绝对同步模式。每次写入立即 fsync，强可靠性。
	SyncWriteMode
)

// FileConfig 文件通道专属配置项。
type FileConfig struct {
	LogDir        string        // 日志目录
	Filename      string        // 基础文件名
	MaxSize       int           // 单文件最大容量（MB），默认 100
	MaxAge        int           // 最长保留天数，默认 180
	MaxBackups    int           // 最多保留历史文件数，默认 100
	CompressAfter int           // 超过 N 天的历史日志自动压缩为 gz，默认 0（不压缩）
	WriteMode     WriteMode     // 写入模式：异步（默认）或同步
	BufferSize    int           // 异步通道缓冲大小，默认 4096
	FlushInterval time.Duration // 异步批量刷盘间隔，默认 1 秒
	Levels        []Level       // 启用的日志级别列表
}

// FileOption 文件通道配置函数类型。
type FileOption func(*FileConfig)

// WithLogDir 设置日志目录。
func WithLogDir(dir string) FileOption {
	return func(c *FileConfig) {
		c.LogDir = dir
	}
}

// WithFilename 设置基础文件名。
func WithFilename(name string) FileOption {
	return func(c *FileConfig) {
		c.Filename = name
	}
}

// WithMaxSize 设置单文件最大容量（MB）。
func WithMaxSize(mb int) FileOption {
	return func(c *FileConfig) {
		c.MaxSize = mb
	}
}

// WithMaxAge 设置最长保留天数。
func WithMaxAge(days int) FileOption {
	return func(c *FileConfig) {
		c.MaxAge = days
	}
}

// WithMaxBackups 设置最多保留历史文件数。
func WithMaxBackups(n int) FileOption {
	return func(c *FileConfig) {
		c.MaxBackups = n
	}
}

// WithCompressAfter 设置压缩延迟天数。
func WithCompressAfter(days int) FileOption {
	return func(c *FileConfig) {
		c.CompressAfter = days
	}
}

// WithWriteMode 设置文件写入模式（异步或同步）。
func WithWriteMode(mode WriteMode) FileOption {
	return func(c *FileConfig) {
		c.WriteMode = mode
	}
}

// WithBufferSize 设置异步通道缓冲大小。
func WithBufferSize(size int) FileOption {
	return func(c *FileConfig) {
		c.BufferSize = size
	}
}

// WithFlushInterval 设置异步批量刷盘间隔。
func WithFlushInterval(d time.Duration) FileOption {
	return func(c *FileConfig) {
		c.FlushInterval = d
	}
}

// WithLevels 为文件通道指定启用的日志级别。
func WithLevels(levels ...Level) FileOption {
	return func(c *FileConfig) {
		c.Levels = levels
	}
}

// EnableFileLog 开启文件输出通道，接收文件配置项。
//
// 必须通过 WithLevels() 指定启用的日志级别，否则该通道保持静默。
func (b *Builder) EnableFileLog(opts ...FileOption) *Builder {
	fc := &FileConfig{
		MaxSize:       100,
		MaxAge:        180,
		MaxBackups:    100,
		WriteMode:     AsyncWriteMode,
		BufferSize:    4096,
		FlushInterval: time.Second,
	}
	for _, opt := range opts {
		opt(fc)
	}

	// 从 Levels 计算最低启用级别
	minLvl := OffLevel
	for _, lv := range fc.Levels {
		if minLvl == OffLevel || lv < minLvl {
			minLvl = lv
		}
	}

	cfg := coreConfig{
		appType: fileAppenderType,
		minLvl:  minLvl,
		fileCfg: fc,
	}
	b.cores = append(b.cores, cfg)
	return b
}

// ---------------------------------------------------------------------------
// 构建与实现
// ---------------------------------------------------------------------------

// Build 根据当前配置构建 Logger 实例。
// 若未配置任何输出通道或所有通道均为静默，返回的 Logger 不产生任何输出。
func (b *Builder) Build() (Logger, error) {
	l := &logger{
		callerSkip: b.callerSkip,
	}

	for i := range b.cores {
		cfg := &b.cores[i]

		if cfg.minLvl == OffLevel {
			continue // 静默通道，跳过
		}

		var app Appender
		switch cfg.appType {
		case consoleAppenderType:
			app = newConsoleAppender()
		case fileAppenderType:
			fa, err := newFileAppender(cfg.fileCfg)
			if err != nil {
				return nil, fmt.Errorf("logx：创建文件输出器失败：%w", err)
			}
			app = fa
		default:
			return nil, fmt.Errorf("logx：未知的输出通道类型：%s", cfg.appType)
		}

		enc := newTextEncoder(cfg.color)

		c := newCore(enc, app, cfg.minLvl)
		l.cores = append(l.cores, c)
	}

	return l, nil
}

// ---------------------------------------------------------------------------
// logger — Logger 接口的内部实现
// ---------------------------------------------------------------------------

// logger 是 Logger 接口的默认实现，内部挂载多个 core。
type logger struct {
	cores      []*core
	fields     []Field
	ctx        context.Context
	hooks      *hookManager
	callerSkip int // 0=不追踪，>0=runtime.Caller 跳过值
	mu         sync.Mutex
}

// IsDebugEnabled 判断是否有任何 core 启用了 Debug 级别。
func (l *logger) IsDebugEnabled() bool {
	for _, c := range l.cores {
		if isLevelEnabled(c.minLvl, DebugLevel) {
			return true
		}
	}
	return false
}

// --- 结构化 API 实现 ---

func (l *logger) Debug(msg string, fields ...Field) {
	l.log(DebugLevel, msg, fields)
}

func (l *logger) Info(msg string, fields ...Field) {
	l.log(InfoLevel, msg, fields)
}

func (l *logger) Warn(msg string, fields ...Field) {
	l.log(WarnLevel, msg, fields)
}

func (l *logger) Error(msg string, fields ...Field) {
	l.log(ErrorLevel, msg, fields)
}

func (l *logger) Panic(msg string, fields ...Field) {
	l.log(PanicLevel, msg, fields)
	l.Sync()
	panic(msg)
}

func (l *logger) Fatal(msg string, fields ...Field) {
	l.log(FatalLevel, msg, fields)
	l.Sync()
	osExit(1)
}

// --- 格式化 API 实现 ---

func (l *logger) Debugf(format string, args ...any) {
	l.log(DebugLevel, fmt.Sprintf(format, args...), nil)
}

func (l *logger) Infof(format string, args ...any) {
	l.log(InfoLevel, fmt.Sprintf(format, args...), nil)
}

func (l *logger) Warnf(format string, args ...any) {
	l.log(WarnLevel, fmt.Sprintf(format, args...), nil)
}

func (l *logger) Errorf(format string, args ...any) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...), nil)
}

func (l *logger) Panicf(format string, args ...any) {
	l.log(PanicLevel, fmt.Sprintf(format, args...), nil)
	l.Sync()
	panic(fmt.Sprintf(format, args...))
}

func (l *logger) Fatalf(format string, args ...any) {
	l.log(FatalLevel, fmt.Sprintf(format, args...), nil)
	l.Sync()
	osExit(1)
}

// --- 链路与上下文 ---

func (l *logger) WithContext(ctx context.Context) Logger {
	return &logger{
		cores:      l.cores,
		fields:     l.copyFields(),
		ctx:        ctx,
		hooks:      l.hooks,
		callerSkip: l.callerSkip,
	}
}

func (l *logger) WithField(key string, val any) Logger {
	newFields := make([]Field, len(l.fields)+1)
	copy(newFields, l.fields)
	newFields[len(l.fields)] = Field{Key: key, Value: val}

	return &logger{
		cores:      l.cores,
		fields:     newFields,
		ctx:        l.ctx,
		hooks:      l.hooks,
		callerSkip: l.callerSkip,
	}
}

// --- 生命周期 ---

func (l *logger) Sync() error {
	for _, c := range l.cores {
		if err := c.sync(); err != nil {
			return fmt.Errorf("logx：同步刷盘失败：%w", err)
		}
	}
	return nil
}

func (l *logger) Close() error {
	for _, c := range l.cores {
		if err := c.close(); err != nil {
			return fmt.Errorf("logx：关闭失败：%w", err)
		}
	}
	return nil
}

func (l *logger) SafeExit(exitFunc func()) {
	l.Sync()
	l.Close()
	if exitFunc != nil {
		exitFunc()
	}
}

// --- 内部方法 ---

// log 是统一的日志写入入口。
func (l *logger) log(level Level, msg string, fields []Field) {
	e := &Entry{
		Level:   level,
		Time:    time.Now(),
		Message: msg,
		Fields:  l.mergeFields(fields),
		ctx:     l.ctx,
	}

	// 调用者追踪：跳过 logx 自身和 Go runtime 栈帧，定位到业务调用代码
	if l.callerSkip > 0 {
		var pcs [15]uintptr
		n := runtime.Callers(l.callerSkip, pcs[:])
		if n > 0 {
			frames := runtime.CallersFrames(pcs[:n])
			for {
				frame, more := frames.Next()
				if !more {
					break
				}
				// 跳过 logx 内部方法（logger.*、core.* 等）
				if isLogxInternal(frame.Function) {
					continue
				}
				// 跳过 Go runtime 栈帧
				if isRuntimeFrame(frame.Function) {
					continue
				}
				e.CallerFile = frame.File
				e.CallerLine = frame.Line
				break
			}
		}
	}

	for _, c := range l.cores {
		c.write(e)
	}

	// 触发 Hook（异步，不阻塞日志主路径）
	if l.hooks != nil {
		l.hooks.fire(e)
	}
}

// mergeFields 合并 logger 级别的 fields 和方法调用传入的 fields。
func (l *logger) mergeFields(fields []Field) []Field {
	if len(l.fields) == 0 {
		return fields
	}
	merged := make([]Field, len(l.fields)+len(fields))
	copy(merged, l.fields)
	copy(merged[len(l.fields):], fields)
	return merged
}

// copyFields 深拷贝 fields 切片。
func (l *logger) copyFields() []Field {
	if len(l.fields) == 0 {
		return nil
	}
	f := make([]Field, len(l.fields))
	copy(f, l.fields)
	return f
}

// isLogxInternal 判断函数是否属于 logx 库内部（logger 方法、core.write 等）。
// 通过函数名而非文件路径判断，不受目录名和 inlining 影响。
func isLogxInternal(fn string) bool {
	// 匹配 github.com/lcylpzls/logx 包内的非测试函数
	// 测试函数（如 TestXxx）虽然也在 logx 包，但属于用户代码
	return strings.Contains(fn, "github.com/lcylpzls/logx.") &&
		!strings.Contains(fn, "github.com/lcylpzls/logx.Test") &&
		!strings.Contains(fn, "github.com/lcylpzls/logx.Benchmark") &&
		!strings.Contains(fn, "github.com/lcylpzls/logx.Example")
}

// isRuntimeFrame 判断是否属于 Go runtime 栈帧。
func isRuntimeFrame(fn string) bool {
	return strings.HasPrefix(fn, "runtime.") ||
		strings.HasPrefix(fn, "testing.")
}
