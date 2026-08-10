package core

// MetricSink 是外部指标接收器接口，metricsx 等家族底座天然满足。
// 未注入时仅保留内部快照统计，不产生外部开销。
type MetricSink interface {
	// IncCounter 增加一个计数指标。
	IncCounter(name string, labels ...string)
	// ObserveDuration 记录一次耗时观测（秒）。
	ObserveDuration(name string, seconds float64, labels ...string)
}

// CounterSink 是可选的增量计数扩展接口，支持字节量等非整数步长。
type CounterSink interface {
	// AddCounter 按增量累加一个计数指标。
	AddCounter(name string, delta float64, labels ...string)
}

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
