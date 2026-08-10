package core

// maxInlineFields 是 FieldGroup 内联数组容量。
// 不超过该数量的字段在热路径上零堆分配。
const maxInlineFields = 8

// FieldGroup 是结构化字段的零分配容器：
// 前 8 个字段内联在值中（栈上或 Entry 内），超过 8 个时才按需分配。
type FieldGroup struct {
	arr  [maxInlineFields]Field
	n    int
	rest []Field
}

// Fields 构造一个 FieldGroup。常规数量（<=8）下零堆分配：
// 变参切片仅被读取（拷贝到内联数组），不会被保存，因此不逃逸。
func Fields(fs ...Field) FieldGroup {
	var g FieldGroup
	if len(fs) <= maxInlineFields {
		g.n = len(fs)
		copy(g.arr[:g.n], fs)
		return g
	}
	// 超过 8 个字段（罕见）：整体拷贝到新切片，避免保存调用方切片导致其逃逸
	g.rest = append([]Field(nil), fs...)
	return g
}

// Len 返回字段数量。
func (g FieldGroup) Len() int {
	return g.n + len(g.rest)
}

// At 返回第 i 个字段（i 必须小于 Len）。
func (g FieldGroup) At(i int) Field {
	if i < g.n {
		return g.arr[i]
	}
	return g.rest[i-g.n]
}

// slice 返回全部字段切片。不超过内联容量时零分配（直接指向内联数组），
// 超过时按需拼接（罕见路径）。
func (g *FieldGroup) slice() []Field {
	if g.rest == nil {
		return g.arr[:g.n]
	}
	all := make([]Field, 0, g.n+len(g.rest))
	all = append(all, g.arr[:g.n]...)
	all = append(all, g.rest...)
	return all
}

// appendField 追加一个字段；内联容量内零分配，超出时按需分配。
func (g *FieldGroup) appendField(f Field) {
	if g.rest == nil && g.n < maxInlineFields {
		g.arr[g.n] = f
		g.n++
		return
	}
	if g.rest == nil {
		g.rest = make([]Field, 0, 4)
	}
	g.rest = append(g.rest, f)
}

// fieldType 标识 Field 的类型化存储槽位。
type fieldType uint8

const (
	fieldAny fieldType = iota // 兜底：使用 Value 接口（Any/Err/Lazy/手动构造）
	fieldString
	fieldInt
	fieldInt64
	fieldBool
)

// Field 表示一个结构化的键值对日志字段。
// 常用类型直接存储在类型化槽位中，避免变量装箱分配；Value 仅作兜底。
type Field struct {
	Key   string
	str   string
	i64   int64
	Value any
	typ   fieldType
	b     bool
}

// String 构造一个字符串字段。
func String(key string, val string) Field {
	return Field{Key: key, typ: fieldString, str: val}
}

// Int 构造一个整数字段。
func Int(key string, val int) Field {
	return Field{Key: key, typ: fieldInt, i64: int64(val)}
}

// Int64 构造一个 int64 字段。
func Int64(key string, val int64) Field {
	return Field{Key: key, typ: fieldInt64, i64: val}
}

// Bool 构造一个布尔字段。
func Bool(key string, val bool) Field {
	return Field{Key: key, typ: fieldBool, b: val}
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
