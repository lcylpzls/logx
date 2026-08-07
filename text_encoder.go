package logx

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// TextEncoder — 零分配纯文本编码器
// ---------------------------------------------------------------------------

// textEncoder 纯文本编码器，输出人类可读的单行日志。
// 使用缓冲池 + 预编码常量 + 时间缓存实现零内存分配。
//
// 格式：2006-01-02 15:04:05.000 LEVEL  message  {key=val, ...}
type textEncoder struct {
	color bool
}

// newTextEncoder 创建一个纯文本编码器实例。
func newTextEncoder(color bool) Encoder {
	return &textEncoder{color: color}
}

// Encode 将 Entry 编码为纯文本字节流写入 Buffer。
func (e *textEncoder) Encode(buf *Buffer, entry *Entry) error {
	// 时间 — 从时间缓存获取（~100ms 精度，零分配）
	buf.B = append(buf.B, getCachedTime()...)
	buf.B = append(buf.B, ' ')

	// 级别 — 可选颜色 + 预编码字节常量
	if e.color {
		buf.B = append(buf.B, levelColor(entry.Level)...)
	}
	buf.B = append(buf.B, levelToBytes(entry.Level)...)
	if e.color {
		buf.B = append(buf.B, colorReset...)
	}

	buf.B = append(buf.B, ' ', ' ')

	// 调用者信息（包目录/文件名:行号，Zap TrimmedPath 同款算法）
	if entry.CallerFile != "" {
		file := entry.CallerFile
		// Go runtime.Caller 在所有平台（含 Windows）均返回 '/' 分隔路径
		idx := strings.LastIndexByte(file, '/') // 最后一个 /
		if idx >= 0 {
			idx = strings.LastIndexByte(file[:idx], '/') // 倒数第二个 /
		}
		if idx >= 0 {
			buf.B = append(buf.B, file[idx+1:]...)
		} else {
			buf.B = append(buf.B, file...)
		}
		buf.B = append(buf.B, ':')
		buf.B = strconv.AppendInt(buf.B, int64(entry.CallerLine), 10)
		buf.B = append(buf.B, ' ', ' ')
	}

	// 日志正文
	buf.B = append(buf.B, entry.Message...)

	// 结构化字段
	if entry.Fields.Len() > 0 {
		buf.B = append(buf.B, ' ', ' ', '{')
		for i := 0; i < entry.Fields.Len(); i++ {
			f := entry.Fields.At(i)
			if i > 0 {
				buf.B = append(buf.B, ',', ' ')
			}
			buf.B = append(buf.B, f.Key...)
			buf.B = append(buf.B, '=')
			e.appendField(buf, f)
		}
		buf.B = append(buf.B, '}')
	}

	buf.B = append(buf.B, '\n')
	return nil
}

// appendField 将字段值追加到缓冲区。常用类型走类型化槽位（零装箱分配），
// 其余类型回退到 appendFieldValue。
func (e *textEncoder) appendField(buf *Buffer, f Field) {
	switch f.typ {
	case fieldString:
		buf.B = append(buf.B, f.str...)
	case fieldInt, fieldInt64:
		buf.B = strconv.AppendInt(buf.B, f.i64, 10)
	case fieldBool:
		buf.B = strconv.AppendBool(buf.B, f.b)
	default:
		e.appendFieldValue(buf, f.Value)
	}
}

// appendFieldValue 将字段值直接追加到缓冲区，针对常见类型零分配。
func (e *textEncoder) appendFieldValue(buf *Buffer, v any) {
	switch val := v.(type) {
	case *lazyValue:
		e.appendFieldValue(buf, val.fn())
		return
	case string:
		buf.B = append(buf.B, val...)
	case int:
		buf.B = strconv.AppendInt(buf.B, int64(val), 10)
	case int64:
		buf.B = strconv.AppendInt(buf.B, val, 10)
	case int32:
		buf.B = strconv.AppendInt(buf.B, int64(val), 10)
	case uint:
		buf.B = strconv.AppendUint(buf.B, uint64(val), 10)
	case uint64:
		buf.B = strconv.AppendUint(buf.B, val, 10)
	case bool:
		buf.B = strconv.AppendBool(buf.B, val)
	case float64:
		buf.B = strconv.AppendFloat(buf.B, val, 'f', -1, 64)
	case error:
		buf.B = append(buf.B, val.Error()...)
	case time.Time:
		buf.B = append(buf.B, val.Format(time.RFC3339)...)
	case time.Duration:
		buf.B = append(buf.B, val.String()...)
	default:
		// 通用类型的后备方案
		buf.B = fmt.Appendf(buf.B, "%v", val)
	}
}

// ---------------------------------------------------------------------------
// 预编码级别字节常量
// ---------------------------------------------------------------------------

var (
	levelDebugB = []byte("DEBUG")
	levelInfoB  = []byte("INFO ")
	levelWarnB  = []byte("WARN ")
	levelErrorB = []byte("ERROR")
	levelPanicB = []byte("PANIC")
	levelFatalB = []byte("FATAL")
	levelOffB   = []byte("OFF  ")
)

// levelToBytes 返回日志级别的预编码字节常量。
func levelToBytes(l Level) []byte {
	switch l {
	case DebugLevel:
		return levelDebugB
	case InfoLevel:
		return levelInfoB
	case WarnLevel:
		return levelWarnB
	case ErrorLevel:
		return levelErrorB
	case PanicLevel:
		return levelPanicB
	case FatalLevel:
		return levelFatalB
	default:
		return levelOffB
	}
}
