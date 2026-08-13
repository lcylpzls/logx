package logx_test

import (
	"errors"
	"testing"
	"time"

	"github.com/lcylpzls/errx"
	"github.com/lcylpzls/logx"
)

// TestPublicAPI 黑盒冒烟测试：覆盖根包全部转发函数、类型别名与常量，
// 保证 internal/core 重构后公开 API 行为一致。
func TestPublicAPI(t *testing.T) {
	b := logx.NewBuilder()
	if b == nil {
		t.Fatal("NewBuilder 返回 nil")
	}

	var opts []logx.ConsoleOption
	opts = append(opts, logx.WithColor())
	if len(opts) == 0 {
		t.Fatal("WithColor 无效")
	}

	_ = logx.WithLogDir("dir")
	_ = logx.WithFilename("app.log")
	_ = logx.WithMaxSize(10)
	_ = logx.WithMaxAge(7)
	_ = logx.WithMaxBackups(3)
	_ = logx.WithCompressAfter(1)
	_ = logx.WithWriteMode(logx.AsyncWriteMode)
	_ = logx.WithBufferSize(128)
	_ = logx.WithFlushInterval(time.Second)
	_ = logx.WithLevels(logx.InfoLevel, logx.WarnLevel)
	_ = logx.WithErrorHandler(func(error) {})

	logx.ReplaceStdLogger(nil)
	logx.RestoreStdLogger()

	err := errx.New(errx.KindBusiness, errx.Code("smoke"), "冒烟")
	_ = logx.FieldsFromError(err)
	_ = logx.Fields(logx.String("k", "v"))
	_ = logx.String("s", "v")
	_ = logx.Int("i", 1)
	_ = logx.Int64("i64", 1)
	_ = logx.Bool("b", true)
	_ = logx.Any("a", 1)
	_ = logx.Err(errors.New("e"))
	_ = logx.Lazy("l", func() any { return 1 })
	if logx.NewJSONEncoder() == nil {
		t.Fatal("NewJSONEncoder 返回 nil")
	}
	if logx.NewNopLogger() == nil {
		t.Fatal("NewNopLogger 返回 nil")
	}

	_ = logx.DebugLevel
	_ = logx.InfoLevel
	_ = logx.WarnLevel
	_ = logx.ErrorLevel
	_ = logx.PanicLevel
	_ = logx.FatalLevel
	_ = logx.OffLevel
	_ = logx.CodeInvalidConfig
	_ = logx.CodeIOFailed
	_ = logx.CodeClosed

	var _ logx.Logger
	var _ logx.Level = logx.InfoLevel
	var _ logx.Entry
	var _ logx.Hook
	var _ logx.HookedLogger
	var _ logx.MetricSink
	var _ logx.CounterSink
	var _ logx.Metrics
	var _ logx.MetricProvider
	var _ logx.Encoder
	var _ logx.FieldGroup
	var _ logx.Field
	var _ logx.WriteMode = logx.AsyncWriteMode
	var _ logx.FileConfig
	var _ logx.FileOption
	var _ logx.LevelUpdater
	var _ logx.Appender
	var _ logx.Buffer
}
