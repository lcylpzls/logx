package logx

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// 1.8 TextEncoder 输出格式
// ---------------------------------------------------------------------------
func TestTextEncoder_Format(t *testing.T) {
	enc := newTextEncoder(false)
	entry := &Entry{
		Level:   InfoLevel,
		Message: "hello world",
		Fields: Fields(
			Field{Key: "user", Value: "admin"},
			Field{Key: "count", Value: 42},
		),
	}

	buf := getBuffer()
	defer putBuffer(buf)
	err := enc.Encode(buf, entry)
	if err != nil {
		t.Fatalf("Encode() 失败：%v", err)
	}

	s := string(buf.B)

	// 检查必要元素
	if !strings.Contains(s, "INFO ") {
		t.Errorf("输出应包含级别标识：%s", s)
	}
	if !strings.Contains(s, "hello world") {
		t.Errorf("输出应包含消息正文：%s", s)
	}
	if !strings.Contains(s, "user=admin") {
		t.Errorf("输出应包含字段：%s", s)
	}
	if !strings.Contains(s, "count=42") {
		t.Errorf("输出应包含字段：%s", s)
	}
	if !strings.HasSuffix(s, "\n") {
		t.Errorf("输出应以换行符结尾：%q", s)
	}
}

func TestTextEncoder_NoFields(t *testing.T) {
	enc := newTextEncoder(false)
	entry := &Entry{
		Level:   DebugLevel,
		Message: "simple message",
	}

	buf := getBuffer()
	defer putBuffer(buf)
	err := enc.Encode(buf, entry)
	if err != nil {
		t.Fatalf("Encode() 失败：%v", err)
	}

	s := string(buf.B)
	if strings.Contains(s, "{") {
		t.Errorf("无字段时不应包含花括号：%s", s)
	}
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------
func BenchmarkTextEncoder_Simple(b *testing.B) {
	enc := newTextEncoder(false)
	entry := &Entry{
		Level:   InfoLevel,
		Message: "benchmark message",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := getBuffer()
		_ = enc.Encode(buf, entry)
		putBuffer(buf)
	}
}

func BenchmarkTextEncoder_WithFields(b *testing.B) {
	enc := newTextEncoder(false)
	entry := &Entry{
		Level:   InfoLevel,
		Message: "benchmark message",
		Fields: Fields(
			Field{Key: "user", Value: "admin"},
			Field{Key: "count", Value: 42},
			Field{Key: "enabled", Value: true},
		),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := getBuffer()
		_ = enc.Encode(buf, entry)
		putBuffer(buf)
	}
}

// ---------------------------------------------------------------------------
// 阶段五：全覆盖测试
// ---------------------------------------------------------------------------
// TestLevelToBytes_All 测试 levelToBytes 对所有级别和默认情况。
func TestLevelToBytes_All(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{OffLevel, "OFF  "},
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO "},
		{WarnLevel, "WARN "},
		{ErrorLevel, "ERROR"},
		{PanicLevel, "PANIC"},
		{FatalLevel, "FATAL"},
		{Level(99), "OFF  "}, // default case
	}
	for _, tt := range tests {
		got := string(levelToBytes(tt.level))
		if got != tt.want {
			t.Errorf("levelToBytes(%d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

// TestLevelColor_All 测试 levelColor 对所有级别和默认情况。
func TestLevelColor_All(t *testing.T) {
	tests := []struct {
		level    Level
		wantNil  bool
		wantName string
	}{
		{DebugLevel, false, "blue"},
		{InfoLevel, false, "green"},
		{WarnLevel, false, "yellow"},
		{ErrorLevel, false, "red"},
		{PanicLevel, false, "red"},
		{FatalLevel, false, "red"},
		{OffLevel, true, ""},
		{Level(99), true, ""},
	}
	for _, tt := range tests {
		got := levelColor(tt.level)
		if tt.wantNil && got != nil {
			t.Errorf("levelColor(%d) should be nil, got %v", tt.level, got)
		}
		if !tt.wantNil && got == nil {
			t.Errorf("levelColor(%d) should not be nil (%s)", tt.level, tt.wantName)
		}
	}
}

// TestAppendFieldValue_AllTypes 测试 encoder 对全部字段类型的序列化。
func TestAppendFieldValue_AllTypes(t *testing.T) {
	enc := newTextEncoder(false).(*textEncoder)
	now := time.Now()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"int32", int32(42), "42"},
		{"uint", uint(42), "42"},
		{"uint64", uint64(42), "42"},
		{"float64", float64(3.14), "3.14"},
		{"bool", true, "true"},
		{"error", fmt.Errorf("err"), "err"},
		{"time", now, now.Format(time.RFC3339)},
		{"duration", time.Second, "1s"},
		{"string", "hello", "hello"},
		{"int", int(42), "42"},
		{"int64", int64(42), "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := getBuffer()
			defer putBuffer(buf)
			enc.appendFieldValue(buf, tt.value)
			got := string(buf.B)
			if got != tt.want {
				t.Errorf("appendFieldValue(%T) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// TestAppendFieldValue_Lazy 测试 Lazy 字段值的序列化。
func TestAppendFieldValue_Lazy(t *testing.T) {
	enc := newTextEncoder(false).(*textEncoder)

	buf := getBuffer()
	defer putBuffer(buf)

	lv := &lazyValue{fn: func() any { return "lazy result" }}
	enc.appendFieldValue(buf, lv)

	got := string(buf.B)
	if got != "lazy result" {
		t.Errorf("appendFieldValue(*lazyValue) = %q, want %q", got, "lazy result")
	}
}

// TestAppendFieldValue_Default 测试 appendFieldValue 的 default 分支（未知类型）。
func TestAppendFieldValue_Default(t *testing.T) {
	enc := newTextEncoder(false).(*textEncoder)

	buf := getBuffer()
	defer putBuffer(buf)

	type customType struct{ Name string }
	enc.appendFieldValue(buf, customType{Name: "custom"})

	got := string(buf.B)
	if got != "{custom}" {
		t.Errorf("appendFieldValue(default) = %q, want %q", got, "{custom}")
	}
}

// TestTextEncoder_ColorOutput 测试带颜色模式的文本编码。
func TestTextEncoder_ColorOutput(t *testing.T) {
	enc := newTextEncoder(true)
	entry := &Entry{
		Level:   WarnLevel,
		Message: "warning",
		Fields:  Fields(Field{Key: "code", Value: "W001"}),
	}

	buf := getBuffer()
	defer putBuffer(buf)
	err := enc.Encode(buf, entry)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	s := string(buf.B)
	// 应包含 ANSI 颜色码
	if !strings.Contains(s, "\033[") {
		t.Error("color output should contain ANSI escape codes")
	}
	if !strings.Contains(s, "WARN ") {
		t.Errorf("output should contain level: %s", s)
	}
	if !strings.Contains(s, "warning") {
		t.Errorf("output should contain message: %s", s)
	}
	if !strings.Contains(s, "code=W001") {
		t.Errorf("output should contain field: %s", s)
	}
}
