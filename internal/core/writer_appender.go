package core

import (
	"io"
	"sync"
)

// writerAppender 将日志写入任意 io.Writer，便于路由到网络、消息队列等自定义目标。
type writerAppender struct {
	w  io.Writer
	mu sync.Mutex
}

// newWriterAppender 创建一个 io.Writer 输出器。
func newWriterAppender(w io.Writer) Appender {
	return &writerAppender{w: w}
}

// Append 将日志数据写入目标 Writer。
func (a *writerAppender) Append(_ Level, p []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.w.Write(p)
}

// Sync 自定义 Writer 由调用方负责缓冲策略，此处为 no-op。
func (a *writerAppender) Sync() error {
	return nil
}

// Close 不关闭调用方持有的 Writer，此处为 no-op。
func (a *writerAppender) Close() error {
	return nil
}
