package logx

import (
	"io"
	"os"
	"sync"
)

// consoleAppender 控制台输出器，根据日志级别智能分流：
//   - Debug / Info / Warn → os.Stdout
//   - Error / Panic / Fatal  → os.Stderr
type consoleAppender struct {
	stdout io.Writer
	stderr io.Writer
	mu     sync.Mutex
}

// newConsoleAppender 创建一个控制台输出器实例。
func newConsoleAppender() Appender {
	return &consoleAppender{
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// Append 根据日志级别将数据写入 stdout 或 stderr。
func (a *consoleAppender) Append(level Level, p []byte) (n int, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if level >= ErrorLevel {
		return a.stderr.Write(p)
	}
	return a.stdout.Write(p)
}

// Sync 控制台输出器无需额外刷盘操作。
func (a *consoleAppender) Sync() error {
	// os.Stdout / os.Stderr 本身是无缓冲的，Sync 为 no-op。
	return nil
}

// Close 控制台输出器不关闭标准流。
func (a *consoleAppender) Close() error {
	return nil
}
