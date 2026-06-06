package logx

import (
	"log"
	"strings"
)

// ---------------------------------------------------------------------------
// ReplaceStdLogger — 劫持标准库 log，统一路由到 logx
// ---------------------------------------------------------------------------

// ReplaceStdLogger 将 Go 标准库 log 包的默认输出重定向到 logx Logger。
// 这允许已有代码中任何 log.Println / log.Printf 调用自动流经 logx 引擎。
//
// 用法：
//
//	logx.ReplaceStdLogger(myLogger)
//	defer logx.RestoreStdLogger() // 可选：恢复标准库原始输出
func ReplaceStdLogger(l Logger) {
	log.SetFlags(0) // 去掉标准库自带的时间前缀，由 logx 统一格式化
	log.SetOutput(&stdLogWriter{logger: l})
}

// RestoreStdLogger 恢复标准库 log 包的默认输出到 os.Stderr。
func RestoreStdLogger() {
	log.SetFlags(log.LstdFlags)
	log.SetOutput(nil) // nil 会恢复为 os.Stderr
}

// stdLogWriter 将标准库 log 的输出适配到 logx Logger。
type stdLogWriter struct {
	logger Logger
}

// Write 实现 io.Writer。标准库 log 会在末尾追加换行符。
func (w *stdLogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n\r")
	w.logger.Info(msg)
	return len(p), nil
}
