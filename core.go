package logx

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

// core 是日志处理的核心引擎，串联 Encoder 和 Appender，
// 负责级别过滤、编码（使用缓冲池）和写入。
type core struct {
	enc    Encoder
	app    Appender
	minLvl atomic.Uint32
	mu     sync.Mutex
}

// newCore 创建一个新的 core 实例。
func newCore(enc Encoder, app Appender, minLvl Level) *core {
	c := &core{
		enc: enc,
		app: app,
	}
	c.minLvl.Store(uint32(minLvl))
	return c
}

// write 对 Entry 进行级别过滤、编码和写入。
// 使用缓冲池实现零分配编码路径。
func (c *core) write(e *Entry) {
	if !isLevelEnabled(c.minLevel(), e.Level) {
		return
	}

	// 从池中获取缓冲
	buf := getBuffer()
	defer putBuffer(buf)

	if err := c.enc.Encode(buf, e); err != nil {
		c.reportError(fmt.Errorf("编码失败：%w", err))
		return
	}

	c.mu.Lock()
	_, err := c.app.Append(e.Level, buf.B)
	c.mu.Unlock()

	if err != nil {
		c.reportError(fmt.Errorf("写入失败：%w", err))
	}
}

// minLevel 返回当前启用的最低日志级别。
func (c *core) minLevel() Level {
	return Level(c.minLvl.Load())
}

// setMinLevel 动态更新最低日志级别。
func (c *core) setMinLevel(l Level) {
	c.minLvl.Store(uint32(l))
}

// reportError 将错误交给通道的错误处理器；未实现时降级输出到 stderr。
func (c *core) reportError(err error) {
	if r, ok := c.app.(interface{ reportError(error) }); ok {
		r.reportError(err)
		return
	}
	fmt.Fprintf(os.Stderr, "logx：%v\n", err)
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
