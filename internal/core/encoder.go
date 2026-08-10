package core

// Encoder 编码器接口，负责将 Entry 编码写入 Buffer。
// 内置实现：TextEncoder（纯文本）、JSONEncoder（结构化 JSON，后续版本）。
type Encoder interface {
	// Encode 将日志条目编码到 buf 中。buf 由调用方从缓冲池获取。
	Encode(buf *Buffer, e *Entry) error
}
