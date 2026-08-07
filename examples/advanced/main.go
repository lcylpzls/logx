// 完整示例：控制台 + 文件双通道、色彩、Debug、Hook。
package main

import (
	"fmt"
	"os"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/logx/examples/advanced/a"
)

// alertHook 示例 Hook：Error 级别以上打印告警。
type alertHook struct{}

func (h *alertHook) OnLog(e *logx.Entry) {
	if e.Level >= logx.ErrorLevel {
		fmt.Printf("[HOOK] 告警触发：%s - %s\n", e.Level, e.Message)
	}
}

func main() {
	logDir := "./logx-example-advanced"
	os.RemoveAll(logDir)

	logger, err := logx.NewBuilder().
		// 控制台通道：Debug + 色彩
		EnableConsole(logx.DebugLevel, logx.WithColor()).
		// 文件通道：Info + 异步批量
		EnableFileLog(
			logx.WithLogDir(logDir),
			logx.WithFilename("advanced.log"),
			logx.WithMaxSize(10),
			logx.WithWriteMode(logx.AsyncWriteMode),
			logx.WithLevels(logx.InfoLevel),
		).
		WithCaller().
		Build()
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// 注册 Hook
	logger.(logx.HookedLogger).AddHook(&alertHook{})

	// 派生带上下文的 logger
	userLogger := logger.WithField("user_id", "10086")

	logger.Debug("调试信息", logx.Fields(logx.Int("goroutine_count", 42)))
	userLogger.Info("用户操作", logx.FieldGroup{})
	logger.Warn("磁盘使用率偏高", logx.Fields(logx.Int("percent", 85)))
	logger.Error("连接超时", logx.Fields(logx.Err(fmt.Errorf("connection timeout"))))

	a.TestLogCaller(logger)

	// 优雅退出
	logger.SafeExit(func() {
		fmt.Println("所有日志已落盘，程序退出")
		os.Exit(0)
	})
}
