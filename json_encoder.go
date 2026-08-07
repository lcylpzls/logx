package logx

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// JSONEncoder — 单行 JSON 编码器
// ---------------------------------------------------------------------------

// jsonEncoder 将日志条目编码为单行 JSON，便于 ELK/Loki 等日志采集系统解析。
//
// 输出格式：
//
//	{"time":"2006-01-02 15:04:05.000","level":"info","caller":"main.go:15","message":"...","key":value}
type jsonEncoder struct{}

// NewJSONEncoder 创建一个 JSON 编码器实例，可配合 Builder.WithEncoder 使用。
func NewJSONEncoder() Encoder {
	return &jsonEncoder{}
}

var (
	jsonTimeKey    = []byte(`{"time":`)
	jsonLevelKey   = []byte(`,"level":`)
	jsonCallerKey  = []byte(`,"caller":`)
	jsonMessageKey = []byte(`,"message":`)
	jsonLineEnd    = []byte("}\n")
)

var (
	jsonLevelDebug = []byte("debug")
	jsonLevelInfo  = []byte("info")
	jsonLevelWarn  = []byte("warn")
	jsonLevelError = []byte("error")
	jsonLevelPanic = []byte("panic")
	jsonLevelFatal = []byte("fatal")
	jsonLevelOff   = []byte("off")
)

// Encode 将 Entry 编码为 JSON 字节流写入 Buffer。
func (e *jsonEncoder) Encode(buf *Buffer, entry *Entry) error {
	buf.B = append(buf.B, jsonTimeKey...)
	buf.B = append(buf.B, '"')
	buf.B = append(buf.B, getCachedTime()...)
	buf.B = append(buf.B, '"')

	buf.B = append(buf.B, jsonLevelKey...)
	buf.B = append(buf.B, '"')
	buf.B = append(buf.B, jsonLevelName(entry.Level)...)
	buf.B = append(buf.B, '"')

	buf.B = append(buf.B, jsonCallerKey...)
	caller := ""
	if entry.CallerFile != "" {
		caller = trimCallerPath(entry.CallerFile) + ":" + strconv.Itoa(entry.CallerLine)
	}
	appendJSONString(buf, caller)

	buf.B = append(buf.B, jsonMessageKey...)
	appendJSONString(buf, entry.Message)

	for i := 0; i < entry.Fields.Len(); i++ {
		f := entry.Fields.At(i)
		buf.B = append(buf.B, ',')
		appendJSONString(buf, f.Key)
		buf.B = append(buf.B, ':')
		e.appendFieldValue(buf, f.Value)
	}

	buf.B = append(buf.B, jsonLineEnd...)
	return nil
}

// jsonLevelName 返回日志级别的小写名称字节常量。
func jsonLevelName(l Level) []byte {
	switch l {
	case DebugLevel:
		return jsonLevelDebug
	case InfoLevel:
		return jsonLevelInfo
	case WarnLevel:
		return jsonLevelWarn
	case ErrorLevel:
		return jsonLevelError
	case PanicLevel:
		return jsonLevelPanic
	case FatalLevel:
		return jsonLevelFatal
	default:
		return jsonLevelOff
	}
}

// appendFieldValue 将字段值追加到 JSON 缓冲区，常见类型零分配。
func (e *jsonEncoder) appendFieldValue(buf *Buffer, v any) {
	switch val := v.(type) {
	case *lazyValue:
		e.appendFieldValue(buf, val.fn())
		return
	case nil:
		buf.B = append(buf.B, "null"...)
	case string:
		appendJSONString(buf, val)
	case error:
		appendJSONString(buf, val.Error())
	case int:
		buf.B = strconv.AppendInt(buf.B, int64(val), 10)
	case int8:
		buf.B = strconv.AppendInt(buf.B, int64(val), 10)
	case int16:
		buf.B = strconv.AppendInt(buf.B, int64(val), 10)
	case int32:
		buf.B = strconv.AppendInt(buf.B, int64(val), 10)
	case int64:
		buf.B = strconv.AppendInt(buf.B, val, 10)
	case uint:
		buf.B = strconv.AppendUint(buf.B, uint64(val), 10)
	case uint8:
		buf.B = strconv.AppendUint(buf.B, uint64(val), 10)
	case uint16:
		buf.B = strconv.AppendUint(buf.B, uint64(val), 10)
	case uint32:
		buf.B = strconv.AppendUint(buf.B, uint64(val), 10)
	case uint64:
		buf.B = strconv.AppendUint(buf.B, val, 10)
	case bool:
		buf.B = strconv.AppendBool(buf.B, val)
	case float32:
		buf.B = strconv.AppendFloat(buf.B, float64(val), 'f', -1, 32)
	case float64:
		buf.B = strconv.AppendFloat(buf.B, val, 'f', -1, 64)
	case time.Time:
		appendJSONString(buf, val.Format(time.RFC3339Nano))
	case time.Duration:
		appendJSONString(buf, val.String())
	default:
		// 通用类型的后备方案（仅非常见类型触发）
		appendJSONString(buf, fmt.Sprint(val))
	}
}

// appendJSONString 将字符串安全编码为 JSON 字符串字面量。
func appendJSONString(buf *Buffer, s string) {
	buf.B = append(buf.B, '"')
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '\\':
			buf.B = append(buf.B, s[start:i]...)
			buf.B = append(buf.B, '\\', c)
			start = i + 1
		case '\n':
			buf.B = append(buf.B, s[start:i]...)
			buf.B = append(buf.B, `\n`...)
			start = i + 1
		case '\r':
			buf.B = append(buf.B, s[start:i]...)
			buf.B = append(buf.B, `\r`...)
			start = i + 1
		case '\t':
			buf.B = append(buf.B, s[start:i]...)
			buf.B = append(buf.B, `\t`...)
			start = i + 1
		default:
			if c < 0x20 {
				buf.B = append(buf.B, s[start:i]...)
				buf.B = append(buf.B, `\u00`...)
				const hex = "0123456789abcdef"
				buf.B = append(buf.B, hex[c>>4], hex[c&0x0f])
				start = i + 1
			}
		}
	}
	buf.B = append(buf.B, s[start:]...)
	buf.B = append(buf.B, '"')
}

// trimCallerPath 提取调用者路径的末尾两级（Zap TrimmedPath 同款算法）。
func trimCallerPath(file string) string {
	idx := strings.LastIndexByte(file, '/')
	if idx >= 0 {
		idx = strings.LastIndexByte(file[:idx], '/')
	}
	if idx >= 0 {
		return file[idx+1:]
	}
	return file
}
