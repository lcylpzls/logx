package main

import (
	testx "github.com/lcylpzls/testx"
	"io"
	"os"
	"testing"
	"time"

	"github.com/lcylpzls/logx"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 对比说明：三种库均输出到 io.Discard，均为纯文本格式，
// 消息相同并携带 3 个结构化字段（string/int/bool）。

// BenchmarkLogx 测 logx 结构化 API。
func BenchmarkLogx(b *testing.B) {
	logger, err := logx.NewBuilder().
		EnableWriter(io.Discard, logx.InfoLevel).
		Build()
	testx.RequireNoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", logx.Fields(
			logx.String("user", "admin"),
			logx.Int("attempt", 3),
			logx.Bool("ok", true),
		))
	}
}

// BenchmarkZap 测 Zap 控制台（纯文本）编码器 + 结构化字段。
func BenchmarkZap(b *testing.B) {
	encCfg := zap.NewProductionEncoderConfig()
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encCfg),
		zapcore.AddSync(io.Discard),
		zapcore.InfoLevel,
	)
	logger := zap.New(core)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message",
			zap.String("user", "admin"),
			zap.Int("attempt", 3),
			zap.Bool("ok", true),
		)
	}
}

// BenchmarkLogrus 测 Logrus TextFormatter + 结构化字段。
func BenchmarkLogrus(b *testing.B) {
	logrus.SetOutput(io.Discard)
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:    true,
		FullTimestamp:    false,
		DisableTimestamp: true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logrus.WithFields(logrus.Fields{
			"user":    "admin",
			"attempt": 3,
			"ok":      true,
		}).Info("benchmark message")
	}
}

// BenchmarkLogxAsyncFile 测 logx 文件异步写入路径（槽位复用，稳态应零分配）。
func BenchmarkLogxAsyncFile(b *testing.B) {
	dir, err := os.MkdirTemp("", "logx-bench-async-*")
	testx.RequireNoError(b, err)

	defer os.RemoveAll(dir)

	logger, err := logx.NewBuilder().
		EnableFileLog(
			logx.WithLogDir(dir),
			logx.WithFilename("bench.log"),
			logx.WithWriteMode(logx.AsyncWriteMode),
			logx.WithBufferSize(4096),
			logx.WithFlushInterval(50*time.Millisecond),
			logx.WithLevels(logx.InfoLevel),
		).
		Build()
	testx.RequireNoError(b, err)

	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", logx.Fields(logx.Int("i", i)))
	}
	b.StopTimer()
	if err := logger.Sync(); err != nil {
		b.Fatalf("Sync 失败：%v", err)
	}
}

// BenchmarkLogxSyncFile 测 logx 文件同步写入路径（应零分配）。
func BenchmarkLogxSyncFile(b *testing.B) {
	dir, err := os.MkdirTemp("", "logx-bench-sync-*")
	testx.RequireNoError(b, err)

	defer os.RemoveAll(dir)

	logger, err := logx.NewBuilder().
		EnableFileLog(
			logx.WithLogDir(dir),
			logx.WithFilename("bench.log"),
			logx.WithWriteMode(logx.SyncWriteMode),
			logx.WithLevels(logx.InfoLevel),
		).
		Build()
	testx.RequireNoError(b, err)

	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", logx.Fields(logx.Int("i", i)))
	}
}
