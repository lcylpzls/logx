package logx

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
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
	callerSkip int     // 0=禁用调用者追踪，3=启用（默认跳过值）
	enc        Encoder // 后续通道使用的编码器（nil=默认纯文本）
	sampler    *sampler
	redacted   map[string]struct{}
}

// coreConfig 描述一个输出通道的配置。
type coreConfig struct {
	appType appenderType // 通道类型
	minLvl  Level        // 该通道启用的最低日志级别
	color   bool         // 控制台是否启用颜色
	fileCfg *FileConfig  // 文件通道专属配置（nil 表示控制台通道）
	enc     Encoder      // 通道编码器（nil=默认纯文本）
	writer  io.Writer    // writer 通道的目标写入器
}

// appenderType 标识输出通道类型。
type appenderType string

const (
	consoleAppenderType appenderType = "console"
	fileAppenderType    appenderType = "file"
	writerAppenderType  appenderType = "writer"
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

// WithEncoder 设置后续添加通道使用的编码器，默认使用纯文本编码器。
// 例如 logx.NewBuilder().WithEncoder(logx.NewJSONEncoder()) 可以让后续通道输出 JSON。
func (b *Builder) WithEncoder(enc Encoder) *Builder {
	b.enc = enc
	return b
}

// WithSampling 启用按秒限流采样：同一秒内最多输出 maxPerSecond 条日志，超出部分丢弃。
// 用于故障风暴场景保护磁盘 IO。maxPerSecond <= 0 表示不采样。
func (b *Builder) WithSampling(maxPerSecond int) *Builder {
	if maxPerSecond > 0 {
		b.sampler = newSampler(maxPerSecond)
	}
	return b
}

// WithRedact 配置自动脱敏的字段 key。匹配的字段（含 Lazy 字段）在编码前
// 会被替换为 "***"，避免手机号、密码等敏感信息落入日志。
func (b *Builder) WithRedact(keys ...string) *Builder {
	for _, k := range keys {
		if k == "" {
			continue
		}
		if b.redacted == nil {
			b.redacted = make(map[string]struct{})
		}
		b.redacted[k] = struct{}{}
	}
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

// EnableWriter 开启自定义 io.Writer 输出通道，便于将日志路由到网络、消息队列等目标。
// 与 EnableConsole 一样，必须显式传入要启用的日志级别。
func (b *Builder) EnableWriter(w io.Writer, levels ...Level) *Builder {
	minLvl := OffLevel
	for _, lv := range levels {
		if minLvl == OffLevel || lv < minLvl {
			minLvl = lv
		}
	}

	cfg := coreConfig{
		appType: writerAppenderType,
		minLvl:  minLvl,
		enc:     b.enc,
		writer:  w,
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
	ErrorHandler  func(error)   // 内部错误统一回调（nil=输出到 stderr）
	OnDropped     func()        // 异步队列满、日志被丢弃时的回调
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

// WithErrorHandler 设置文件通道内部错误的统一处理回调。
// 未设置时，内部错误降级输出到标准错误流。
func WithErrorHandler(fn func(error)) FileOption {
	return func(c *FileConfig) {
		c.ErrorHandler = fn
	}
}

// WithOnDropped 设置异步队列满、日志被丢弃时的回调。
// 回调在调用方协程内同步执行，请保持轻量。
func WithOnDropped(fn func()) FileOption {
	return func(c *FileConfig) {
		c.OnDropped = fn
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
		enc:     b.enc,
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
		sampler:    b.sampler,
		redacted:   b.redacted,
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
		case writerAppenderType:
			if cfg.writer == nil {
				return nil, fmt.Errorf("logx：writer 通道未提供 io.Writer")
			}
			app = newWriterAppender(cfg.writer)
		default:
			return nil, fmt.Errorf("logx：未知的输出通道类型：%s", cfg.appType)
		}

		enc := cfg.enc
		if enc == nil {
			enc = newTextEncoder(cfg.color)
		}

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
	fields     FieldGroup
	ctx        context.Context
	hooks      *hookManager
	sampler    *sampler
	redacted   map[string]struct{}
	callerSkip int // 0=不追踪，>0=runtime.Caller 跳过值
}

// IsDebugEnabled 判断是否有任何 core 启用了 Debug 级别。
func (l *logger) IsDebugEnabled() bool {
	for _, c := range l.cores {
		if isLevelEnabled(c.minLevel(), DebugLevel) {
			return true
		}
	}
	return false
}

// --- 结构化 API 实现 ---

func (l *logger) Debug(msg string, fields FieldGroup) {
	l.log(DebugLevel, msg, fields)
}

func (l *logger) Info(msg string, fields FieldGroup) {
	l.log(InfoLevel, msg, fields)
}

func (l *logger) Warn(msg string, fields FieldGroup) {
	l.log(WarnLevel, msg, fields)
}

func (l *logger) Error(msg string, fields FieldGroup) {
	l.log(ErrorLevel, msg, fields)
}

func (l *logger) Panic(msg string, fields FieldGroup) {
	l.log(PanicLevel, msg, fields)
	l.Sync()
	panic(msg)
}

func (l *logger) Fatal(msg string, fields FieldGroup) {
	l.log(FatalLevel, msg, fields)
	l.Sync()
	osExit(1)
}

// --- 格式化 API 实现 ---

func (l *logger) Debugf(format string, args ...any) {
	l.log(DebugLevel, fmt.Sprintf(format, args...), FieldGroup{})
}

func (l *logger) Infof(format string, args ...any) {
	l.log(InfoLevel, fmt.Sprintf(format, args...), FieldGroup{})
}

func (l *logger) Warnf(format string, args ...any) {
	l.log(WarnLevel, fmt.Sprintf(format, args...), FieldGroup{})
}

func (l *logger) Errorf(format string, args ...any) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...), FieldGroup{})
}

func (l *logger) Panicf(format string, args ...any) {
	l.log(PanicLevel, fmt.Sprintf(format, args...), FieldGroup{})
	l.Sync()
	panic(fmt.Sprintf(format, args...))
}

func (l *logger) Fatalf(format string, args ...any) {
	l.log(FatalLevel, fmt.Sprintf(format, args...), FieldGroup{})
	l.Sync()
	osExit(1)
}

// --- 链路与上下文 ---

func (l *logger) WithContext(ctx context.Context) Logger {
	return &logger{
		cores:      l.cores,
		fields:     l.fields,
		ctx:        ctx,
		hooks:      l.hooks,
		sampler:    l.sampler,
		redacted:   l.redacted,
		callerSkip: l.callerSkip,
	}
}

func (l *logger) WithField(key string, val any) Logger {
	nl := &logger{
		cores:      l.cores,
		ctx:        l.ctx,
		hooks:      l.hooks,
		sampler:    l.sampler,
		redacted:   l.redacted,
		callerSkip: l.callerSkip,
	}
	nl.fields = l.fields
	nl.fields.appendField(Field{Key: key, Value: val})
	return nl
}

// SetLevel 动态调整日志级别。传入的所有 Level 中最低的一个将成为
// 所有通道的新启用的最低级别（不影响已关闭的静默通道）。
func (l *logger) SetLevel(levels ...Level) {
	if len(levels) == 0 {
		return
	}
	min := levels[0]
	for _, lv := range levels[1:] {
		if lv < min {
			min = lv
		}
	}
	for _, c := range l.cores {
		c.setMinLevel(min)
	}
}

// LevelUpdater 是提供运行时动态调整日志级别能力的可选接口。
// Logger 的默认实现支持该接口，通过类型断言使用：
//
//	if lu, ok := logger.(logx.LevelUpdater); ok {
//	    lu.SetLevel(logx.DebugLevel)
//	}
type LevelUpdater interface {
	SetLevel(levels ...Level)
}

// Metrics 汇总所有通道的运行指标快照。
func (l *logger) Metrics() Metrics {
	var m Metrics
	for _, c := range l.cores {
		mp, ok := c.app.(MetricProvider)
		if !ok {
			continue
		}
		cm := mp.Metrics()
		m.Writes += cm.Writes
		m.WriteBytes += cm.WriteBytes
		m.Drops += cm.Drops
		m.Rotations += cm.Rotations
		m.Compressions += cm.Compressions
		m.Cleanups += cm.Cleanups
	}
	return m
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
func (l *logger) log(level Level, msg string, fields FieldGroup) {
	// 采样：超过每秒上限的日志直接丢弃
	if l.sampler != nil && !l.sampler.allow() {
		return
	}

	// Entry 复用：未注册 Hook 时，Entry 在本次调用内同步使用完即可归还；
	// 注册 Hook 后 Hook 异步持有 Entry，必须每次独立分配。
	var e *Entry
	if l.hooks == nil {
		e = getEntry()
		defer putEntry(e)
	} else {
		e = &Entry{}
	}
	e.Level = level
	e.Time = time.Now()
	e.Message = msg
	e.Fields = l.redactFields(l.mergeFields(fields))
	e.ctx = l.ctx

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
				if isRuntimeFrameFn(frame.Function) {
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
func (l *logger) mergeFields(fields FieldGroup) FieldGroup {
	if l.fields.Len() == 0 {
		return fields
	}
	var g FieldGroup
	total := l.fields.Len() + fields.Len()
	if total <= maxInlineFields {
		for i := 0; i < l.fields.Len(); i++ {
			g.arr[g.n] = l.fields.At(i)
			g.n++
		}
		for i := 0; i < fields.Len(); i++ {
			g.arr[g.n] = fields.At(i)
			g.n++
		}
		return g
	}
	// 超出内联容量：按需分配（罕见）
	merged := make([]Field, total)
	for i := 0; i < l.fields.Len(); i++ {
		merged[i] = l.fields.At(i)
	}
	for i := 0; i < fields.Len(); i++ {
		merged[l.fields.Len()+i] = fields.At(i)
	}
	g.rest = merged
	return g
}

// redactFields 将命中脱敏配置的字段值替换为 "***"。
// 若未配置脱敏或没有字段，直接返回原值，避免额外分配。
func (l *logger) redactFields(fields FieldGroup) FieldGroup {
	if len(l.redacted) == 0 || fields.Len() == 0 {
		return fields
	}
	var g FieldGroup
	for i := 0; i < fields.Len(); i++ {
		f := fields.At(i)
		if _, ok := l.redacted[f.Key]; ok {
			f = Field{Key: f.Key, Value: "***"}
		}
		g.appendField(f)
	}
	return g
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

// isRuntimeFrameFn 运行时帧判断函数（测试注入用，生产行为不变）。
var isRuntimeFrameFn = isRuntimeFrame
