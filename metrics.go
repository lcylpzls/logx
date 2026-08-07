package logx

// Metrics 是日志库运行指标的汇总快照。
// 所有计数均为原子累加，可安全地在任意时刻读取。
type Metrics struct {
	// Writes 成功写入的日志条数。
	Writes uint64
	// WriteBytes 成功写入的字节数。
	WriteBytes uint64
	// Rotations 文件轮转次数。
	Rotations uint64
	// Compressions gzip 压缩成功的次数。
	Compressions uint64
	// Cleanups 生命周期清理执行次数。
	Cleanups uint64
}

// MetricProvider 是提供运行指标的可选接口。
// 通过类型断言使用：
//
//	if mp, ok := logger.(logx.MetricProvider); ok {
//	    m := mp.Metrics()
//	}
type MetricProvider interface {
	Metrics() Metrics
}
