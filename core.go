package logx

import (
	"fmt"
	"os"
	"sync"
)

// core 是日志处理的核心引擎，串联 Encoder 和 Appender，
// 负责级别过滤、编码（使用缓冲池）和写入。
type core struct {
	enc    Encoder
	app    Appender
	minLvl Level
	mu     sync.Mutex
}

// newCore 创建一个新的 core 实例。
func newCore(enc Encoder, app Appender, minLvl Level) *core {
	return &core{
		enc:    enc,
		app:    app,
		minLvl: minLvl,
	}
}

// write 对 Entry 进行级别过滤、编码和写入。
// 使用缓冲池实现零分配编码路径。
func (c *core) write(e *Entry) {
	if !isLevelEnabled(c.minLvl, e.Level) {
		return
	}

	// 从池中获取缓冲
	buf := getBuffer()
	defer putBuffer(buf)

	if err := c.enc.Encode(buf, e); err != nil {
		fmt.Fprintf(os.Stderr, "logx：编码失败：%v\n", err)
		return
	}

	c.mu.Lock()
	_, err := c.app.Append(e.Level, buf.B)
	c.mu.Unlock()

	if err != nil {
		fmt.Fprintf(os.Stderr, "logx：写入失败：%v\n", err)
	}
}

// sync 强制刷盘。
func (c *core) sync() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.app.Sync()
}

// close 关闭 core 并释放资源。
func (c *core) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.app.Close()
}
