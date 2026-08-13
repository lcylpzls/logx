package logx

import (
	"time"

	"github.com/lcylpzls/logx/internal/core"
)

// 类型别名（实现见 internal/core）。
type (
	Logger         = core.Logger
	Level          = core.Level
	Entry          = core.Entry
	Hook           = core.Hook
	HookedLogger   = core.HookedLogger
	MetricSink     = core.MetricSink
	CounterSink    = core.CounterSink
	Metrics        = core.Metrics
	MetricProvider = core.MetricProvider
	Encoder        = core.Encoder
	FieldGroup     = core.FieldGroup
	Field          = core.Field
	Builder        = core.Builder
	ConsoleOption  = core.ConsoleOption
	WriteMode      = core.WriteMode
	FileConfig     = core.FileConfig
	FileOption     = core.FileOption
	LevelUpdater   = core.LevelUpdater
	Appender       = core.Appender
	Buffer         = core.Buffer
)

const (
	AsyncWriteMode = core.AsyncWriteMode
	SyncWriteMode  = core.SyncWriteMode
)

const (
	OffLevel   = core.OffLevel
	DebugLevel = core.DebugLevel
	InfoLevel  = core.InfoLevel
	WarnLevel  = core.WarnLevel
	ErrorLevel = core.ErrorLevel
	PanicLevel = core.PanicLevel
	FatalLevel = core.FatalLevel
)

const (
	CodeInvalidConfig = core.CodeInvalidConfig
	CodeIOFailed      = core.CodeIOFailed
	CodeClosed        = core.CodeClosed
)

func NewBuilder() *Builder { return core.NewBuilder() }

// NewNopLogger 返回一个不产生任何输出与副作用的 Logger。
// Panic/Fatal 静默；SafeExit 保留退出回调语义。
func NewNopLogger() Logger { return core.NewNopLogger() }

func WithColor() ConsoleOption { return core.WithColor() }

func WithLogDir(dir string) FileOption        { return core.WithLogDir(dir) }
func WithFilename(name string) FileOption     { return core.WithFilename(name) }
func WithMaxSize(mb int) FileOption           { return core.WithMaxSize(mb) }
func WithMaxAge(days int) FileOption          { return core.WithMaxAge(days) }
func WithMaxBackups(n int) FileOption         { return core.WithMaxBackups(n) }
func WithCompressAfter(days int) FileOption   { return core.WithCompressAfter(days) }
func WithWriteMode(mode WriteMode) FileOption { return core.WithWriteMode(mode) }
func WithBufferSize(size int) FileOption      { return core.WithBufferSize(size) }
func WithFlushInterval(d time.Duration) FileOption {
	return core.WithFlushInterval(d)
}
func WithLevels(levels ...Level) FileOption { return core.WithLevels(levels...) }
func WithErrorHandler(fn func(error)) FileOption {
	return core.WithErrorHandler(fn)
}

func ReplaceStdLogger(l Logger) { core.ReplaceStdLogger(l) }
func RestoreStdLogger()         { core.RestoreStdLogger() }
func FieldsFromError(err error) FieldGroup {
	return core.FieldsFromError(err)
}
func Fields(fs ...Field) FieldGroup       { return core.Fields(fs...) }
func String(key string, val string) Field { return core.String(key, val) }
func Int(key string, val int) Field       { return core.Int(key, val) }
func Int64(key string, val int64) Field   { return core.Int64(key, val) }
func Bool(key string, val bool) Field     { return core.Bool(key, val) }
func Any(key string, val any) Field       { return core.Any(key, val) }
func Err(err error) Field                 { return core.Err(err) }
func Lazy(key string, fn func() any) Field {
	return core.Lazy(key, fn)
}
func NewJSONEncoder() Encoder { return core.NewJSONEncoder() }
