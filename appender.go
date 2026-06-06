package logx

// Appender 输出器接口，负责将字节流落地到目标介质。
// 内置实现：ConsoleAppender（控制台）、FileAppender（文件）。
type Appender interface {
	// Append 将日志数据写入目标。level 用于输出器内部路由（如 stdout/stderr 分流）。
	Append(level Level, p []byte) (n int, err error)
	// Sync 强制将缓冲数据刷入底层介质。
	Sync() error
	// Close 关闭输出器并释放资源。
	Close() error
}
