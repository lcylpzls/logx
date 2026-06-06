// Package logx 是工业级零依赖高性能 Go 结构化日志库。
package logx

// Level 表示日志级别。数值越大，严重程度越高。
type Level uint32

// 7 级标准日志级别。OffLevel 用于关闭所有输出，默认静默。
const (
	OffLevel   Level = iota // 0 — 关闭所有日志输出
	DebugLevel              // 1 — 调试日志，开发环境专用
	InfoLevel               // 2 — 常规业务运行日志
	WarnLevel               // 3 — 警告日志，非错误但需关注
	ErrorLevel              // 4 — 业务错误日志
	PanicLevel              // 5 — 恐慌日志，输出后触发 panic
	FatalLevel              // 6 — 致命错误日志，输出后强制退出
)

// String 返回日志级别对应的英文短名称，固定 5 字符宽度。
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO "
	case WarnLevel:
		return "WARN "
	case ErrorLevel:
		return "ERROR"
	case PanicLevel:
		return "PANIC"
	case FatalLevel:
		return "FATAL"
	default:
		return "OFF  "
	}
}

// isLevelEnabled 判断 target 级别是否不低于启用的最低级别 minLvl。
// 当 minLvl 为 OffLevel 时，所有级别均不通过（默认静默）。
func isLevelEnabled(minLvl Level, target Level) bool {
	if minLvl == OffLevel {
		return false
	}
	return target >= minLvl
}
