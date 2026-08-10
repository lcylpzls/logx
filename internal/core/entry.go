package core

import (
	"context"
	"sync"
	"time"
)

// Entry 表示一条待输出的日志条目，承载该条日志的完整上下文。
type Entry struct {
	// Level 日志级别
	Level Level
	// Time 日志产生时间
	Time time.Time
	// Message 日志正文
	Message string
	// Fields 结构化字段（内联容器，热路径零分配）
	Fields FieldGroup
	// CallerFile 调用者源文件名（WithCaller 启用后填充）
	CallerFile string
	// CallerLine 调用者行号（WithCaller 启用后填充）
	CallerLine int
	// ctx 链路上下文（未导出，通过 Context() 获取）
	ctx context.Context
}

// Context 返回日志携带的 context.Context，若为 nil 则返回 context.Background。
func (e *Entry) Context() context.Context {
	if e.ctx == nil {
		return context.Background()
	}
	return e.ctx
}

// entryPool 复用 Entry 对象，减少高频日志路径的内存分配。
// 注意：仅当 Logger 未注册 Hook 时启用复用（Hook 会异步持有 Entry）。
var entryPool = sync.Pool{
	New: func() any {
		return &Entry{}
	},
}

// getEntry 从池中获取一个 Entry。
func getEntry() *Entry {
	return entryPool.Get().(*Entry)
}

// putEntry 将 Entry 清零后归还池中，避免脏数据泄漏到下一次复用。
func putEntry(e *Entry) {
	*e = Entry{}
	entryPool.Put(e)
}
