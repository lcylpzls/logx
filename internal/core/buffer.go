package core

import "sync"

// ---------------------------------------------------------------------------
// Buffer 池 — 2MB 零分配缓冲
// ---------------------------------------------------------------------------

// Buffer 封装一个从 sync.Pool 中分配的字节切片，用于日志编码。
// 单条日志生命周期：GetBuffer → Encode → Appender.Write → PutBuffer
type Buffer struct {
	B []byte
}

// 全局缓冲池，默认 2MB 初始容量。
var bufPool = sync.Pool{
	New: func() interface{} {
		return &Buffer{B: make([]byte, 0, 2*1024*1024)} // 2MB
	},
}

// getBuffer 从池中获取一个已重置的 Buffer。
func getBuffer() *Buffer {
	buf := bufPool.Get().(*Buffer)
	buf.B = buf.B[:0] // 重置长度，保留容量
	return buf
}

// putBuffer 将 Buffer 归还池中。
func putBuffer(buf *Buffer) {
	if cap(buf.B) < 64*1024 {
		// 容量异常缩小的缓冲区不归还，防止池被污染
		return
	}
	bufPool.Put(buf)
}
