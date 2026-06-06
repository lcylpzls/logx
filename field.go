package logx

// Field 表示一个结构化的键值对日志字段。
type Field struct {
	Key   string
	Value interface{}
}

// String 构造一个字符串字段。
func String(key string, val string) Field {
	return Field{Key: key, Value: val}
}

// Int 构造一个整数字段。
func Int(key string, val int) Field {
	return Field{Key: key, Value: val}
}

// Int64 构造一个 int64 字段。
func Int64(key string, val int64) Field {
	return Field{Key: key, Value: val}
}

// Bool 构造一个布尔字段。
func Bool(key string, val bool) Field {
	return Field{Key: key, Value: val}
}

// Any 构造一个任意类型字段。
func Any(key string, val any) Field {
	return Field{Key: key, Value: val}
}

// Err 构造一个错误字段，key 固定为 "error"。
func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

// --- 延迟求值 ---

// lazyValue 是延迟求值字段的内部标记类型。
type lazyValue struct {
	fn func() interface{}
}

// Lazy 构造一个延迟求值字段。fn 仅在日志级别通过过滤、实际编码时才被调用。
// 用于避免在高开销的 Debug 日志中执行不必要的计算。
//
//	logger.Debug("user", logx.Lazy("info", func() interface{} {
//	    return expensiveQuery()  // 仅 Debug 启用时才会执行
//	}))
func Lazy(key string, fn func() any) Field {
	return Field{Key: key, Value: &lazyValue{fn: fn}}
}
