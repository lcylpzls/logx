package logx

// ---------------------------------------------------------------------------
// ANSI 色彩码 — 预编码为 []byte 常量，零分配颜色拼接
// ---------------------------------------------------------------------------

var (
	colorReset  = []byte("\033[0m")
	colorRed    = []byte("\033[31m")
	colorGreen  = []byte("\033[32m")
	colorYellow = []byte("\033[33m")
	colorBlue   = []byte("\033[34m")
	colorCyan   = []byte("\033[36m")
	colorGray   = []byte("\033[90m")
)

// levelColor 返回指定日志级别对应的 ANSI 颜色码字节序列。
func levelColor(l Level) []byte {
	switch l {
	case DebugLevel:
		return colorBlue
	case InfoLevel:
		return colorGreen
	case WarnLevel:
		return colorYellow
	case ErrorLevel:
		return colorRed
	case PanicLevel:
		return colorRed
	case FatalLevel:
		return colorRed
	default:
		return nil
	}
}
