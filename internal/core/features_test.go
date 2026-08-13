package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	testx "github.com/lcylpzls/testx"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// JSON 编码器
// ---------------------------------------------------------------------------

func TestJSONEncoder_Basic(t *testing.T) {
	buf := getBuffer()
	defer putBuffer(buf)

	err := NewJSONEncoder().Encode(buf, &Entry{
		Level:   InfoLevel,
		Time:    time.Now(),
		Message: "hello json",
		Fields: Fields(
			String("name", "logx"),
			Int("count", 3),
			Int64("big", 1234567890123),
			Bool("ok", true),
			Any("ratio", 0.5),
			Any("err", fmt.Errorf("boom")),
			Any("nil", nil),
		),
	})
	testx.RequireNoError(t, err)

	var out map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.B), &out); err != nil {
		t.Fatalf("输出不是合法 JSON：%v\n%s", err, buf.B)
	}
	if out["level"] != "info" {
		t.Errorf("level 不符：%v", out["level"])
	}
	if out["message"] != "hello json" {
		t.Errorf("message 不符：%v", out["message"])
	}
	if out["count"] != float64(3) {
		t.Errorf("count 不符：%v", out["count"])
	}
	if out["ok"] != true {
		t.Errorf("ok 不符：%v", out["ok"])
	}
	if out["err"] != "boom" {
		t.Errorf("err 不符：%v", out["err"])
	}
	if out["nil"] != nil {
		t.Errorf("nil 不符：%v", out["nil"])
	}
}

func TestJSONEncoder_Escaping(t *testing.T) {
	buf := getBuffer()
	defer putBuffer(buf)

	msg := "a\"b\\c\nd\re\tf\x01g"
	err := NewJSONEncoder().Encode(buf, &Entry{
		Level:   ErrorLevel,
		Time:    time.Now(),
		Message: msg,
		Fields:  Fields(String("k", msg)),
	})
	testx.RequireNoError(t, err)

	var out map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.B), &out); err != nil {
		t.Fatalf("转义后不是合法 JSON：%v\n%s", err, buf.B)
	}
	if out["message"] != msg || out["k"] != msg {
		t.Errorf("转义往返不一致：%q %q", out["message"], out["k"])
	}
}

func TestJSONEncoder_Lazy(t *testing.T) {
	buf := getBuffer()
	defer putBuffer(buf)

	called := false
	err := NewJSONEncoder().Encode(buf, &Entry{
		Level:   InfoLevel,
		Time:    time.Now(),
		Message: "lazy",
		Fields:  Fields(Lazy("info", func() any { called = true; return "computed" })),
	})
	testx.RequireNoError(t, err)

	testx.True(t, called)

	if !bytes.Contains(buf.B, []byte(`"computed"`)) {
		t.Errorf("Lazy 结果未输出：%s", buf.B)
	}
}

func TestJSONEncoder_Caller(t *testing.T) {
	buf := getBuffer()
	defer putBuffer(buf)

	err := NewJSONEncoder().Encode(buf, &Entry{
		Level:      InfoLevel,
		Time:       time.Now(),
		Message:    "with caller",
		CallerFile: "github.com/lcylpzls/logx/file.go",
		CallerLine: 42,
	})
	testx.RequireNoError(t, err)

	var out map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.B), &out); err != nil {
		t.Fatalf("输出不是合法 JSON：%v\n%s", err, buf.B)
	}
	if out["caller"] != "logx/file.go:42" {
		t.Errorf("caller 不符：%v", out["caller"])
	}
}

func TestJSONEncoder_FieldTypes(t *testing.T) {
	buf := getBuffer()
	defer putBuffer(buf)

	err := NewJSONEncoder().Encode(buf, &Entry{
		Level:   InfoLevel,
		Time:    time.Now(),
		Message: "types",
		Fields: Fields(
			Any("i8", int8(-8)),
			Any("i16", int16(-16)),
			Any("i32", int32(-32)),
			Any("plain_int", int(7)),
			Any("plain_i64", int64(77)),
			Any("u", uint(7)),
			Any("u8", uint8(8)),
			Any("u16", uint16(16)),
			Any("u32", uint32(32)),
			Any("u64", uint64(64)),
			Any("f32", float32(1.5)),
			Any("f64", float64(2.5)),
			Any("plain_bool", true),
			Any("t", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)),
			Any("d", 3*time.Second),
			Any("struct", struct{ A int }{1}),
		),
	})
	testx.RequireNoError(t, err)

	var out map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.B), &out); err != nil {
		t.Fatalf("输出不是合法 JSON：%v\n%s", err, buf.B)
	}
	if out["i8"] != float64(-8) || out["u64"] != float64(64) || out["f32"] != 1.5 {
		t.Errorf("数值字段不符：%v", out)
	}
	if out["t"] != "2026-01-02T03:04:05Z" || out["d"] != "3s" || out["struct"] != "{1}" {
		t.Errorf("特殊字段不符：%v", out)
	}
}

func TestTextEncoder_TypedFields(t *testing.T) {
	buf := getBuffer()
	defer putBuffer(buf)

	err := newTextEncoder(false).Encode(buf, &Entry{
		Level:   InfoLevel,
		Time:    time.Now(),
		Message: "typed",
		Fields: Fields(
			String("s", "v"),
			Int("i", 42),
			Int64("i64", 43),
			Bool("b", true),
		),
	})
	testx.RequireNoError(t, err)

	out := string(buf.B)
	for _, want := range []string{"s=v", "i=42", "i64=43", "b=true"} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %s：%s", want, out)
		}
	}
}

func TestTextEncoder_CallerSingleSegment(t *testing.T) {
	buf := getBuffer()
	defer putBuffer(buf)

	err := newTextEncoder(false).Encode(buf, &Entry{
		Level:      InfoLevel,
		Time:       time.Now(),
		Message:    "single",
		CallerFile: "main.go",
		CallerLine: 7,
	})
	testx.RequireNoError(t, err)

	if !bytes.Contains(buf.B, []byte("main.go:7")) {
		t.Errorf("单段调用者路径输出不符：%q", buf.B)
	}
}

func TestJSONEncoder_AllLevels(t *testing.T) {
	cases := []struct {
		level Level
		name  string
	}{
		{OffLevel, "off"},
		{DebugLevel, "debug"},
		{InfoLevel, "info"},
		{WarnLevel, "warn"},
		{ErrorLevel, "error"},
		{PanicLevel, "panic"},
		{FatalLevel, "fatal"},
	}
	for _, tc := range cases {
		buf := getBuffer()
		err := NewJSONEncoder().Encode(buf, &Entry{
			Level:   tc.level,
			Time:    time.Now(),
			Message: "level",
		})
		if err != nil {
			putBuffer(buf)
			t.Fatalf("Encode 失败：%v", err)
		}
		var out map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(buf.B), &out); err != nil {
			putBuffer(buf)
			t.Fatalf("输出不是合法 JSON：%v", err)
		}
		if out["level"] != tc.name {
			t.Errorf("级别 %s 输出不符：%v", tc.level, out["level"])
		}
		putBuffer(buf)
	}
}

func TestJSONEncoder_CallerSingleSegment(t *testing.T) {
	buf := getBuffer()
	defer putBuffer(buf)

	err := NewJSONEncoder().Encode(buf, &Entry{
		Level:      InfoLevel,
		Time:       time.Now(),
		Message:    "single",
		CallerFile: "main.go",
		CallerLine: 3,
	})
	testx.RequireNoError(t, err)

	var out map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.B), &out); err != nil {
		t.Fatalf("输出不是合法 JSON：%v", err)
	}
	if out["caller"] != "main.go:3" {
		t.Errorf("单段 caller 不符：%v", out["caller"])
	}
}

func TestCaller_SkipsRuntimeFrames(t *testing.T) {
	var buf bytes.Buffer
	l := &logger{
		callerSkip: 1,
		cores: []*core{
			newCore(newTextEncoder(false), newWriterAppender(&buf), DebugLevel),
		},
	}
	old := isRuntimeFrameFn
	isRuntimeFrameFn = func(string) bool { return true } // 覆盖 continue 跳过分支
	t.Cleanup(func() { isRuntimeFrameFn = old })

	// 辅助函数（内部帧）→ 测试函数 → testing.tRunner → runtime.goexit，
	// 配合始终返回 true 的帧判断，覆盖 continue 跳过与 !more 退出分支。
	logViaInternalHelper(l)
	if buf.Len() == 0 {
		t.Fatal("日志未输出")
	}
}

func TestUpdateSymlink_NonWindowsPath(t *testing.T) {
	dir := tempLogDir(t)
	oldPlatform := platformIsWindows
	oldRemove := removePathFn
	oldSymlink := createSymlinkFn
	platformIsWindows = false
	removePathFn = func(string) error { return nil }
	createSymlinkFn = func(_, _ string) error { return nil }
	t.Cleanup(func() {
		platformIsWindows = oldPlatform
		removePathFn = oldRemove
		createSymlinkFn = oldSymlink
	})

	fa := &fileAppender{
		symlinkPath: filepath.Join(dir, "app.log"),
	}
	// 不应 panic，且应走 createSymlink 分支
	fa.updateSymlink("app-2026-01-01.log")
}

//go:noinline
func logViaInternalHelper(l *logger) {
	l.log(InfoLevel, "internal helper", FieldGroup{})
}

func TestBuilder_WithJSONEncoderAndWriter(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewBuilder().
		WithEncoder(NewJSONEncoder()).
		EnableWriter(&buf, InfoLevel).
		Build()
	testx.RequireNoError(t, err)

	logger.Info("json via writer", Fields(String("k", "v")))

	var out map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &out); err != nil {
		t.Fatalf("输出不是合法 JSON：%v\n%s", err, buf.String())
	}
	if out["message"] != "json via writer" {
		t.Errorf("message 不符：%v", out["message"])
	}
}

// ---------------------------------------------------------------------------
// io.Writer 通道
// ---------------------------------------------------------------------------

func TestBuilder_EnableWriter(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewBuilder().
		EnableWriter(&buf, InfoLevel).
		Build()
	testx.RequireNoError(t, err)

	logger.Info("writer msg", FieldGroup{})
	logger.Debug("should be filtered", FieldGroup{})
	got := buf.String()
	if !strings.Contains(got, "writer msg") {
		t.Errorf("writer 未收到日志：%q", got)
	}
	if strings.Contains(got, "should be filtered") {
		t.Errorf("级别过滤失效：%q", got)
	}
}

func TestBuilder_EnableWriter_SilentWithoutLevels(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewBuilder().EnableWriter(&buf).Build()
	testx.RequireNoError(t, err)

	logger.Info("no output", FieldGroup{})
	if buf.Len() != 0 {
		t.Errorf("未指定级别时不应输出：%q", buf.String())
	}
}

func TestWriterAppender_NoopLifecycle(t *testing.T) {
	var buf bytes.Buffer
	a := newWriterAppender(&buf)
	if _, err := a.Append(InfoLevel, []byte("abc")); err != nil {
		t.Fatalf("Append 失败：%v", err)
	}
	if buf.String() != "abc" {
		t.Errorf("写入内容不符：%q", buf.String())
	}
	if err := a.Sync(); err != nil {
		t.Errorf("Sync 失败：%v", err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("Close 失败：%v", err)
	}
}

func TestFileOptions_ErrorHandler(t *testing.T) {
	fc := &FileConfig{}
	WithErrorHandler(func(error) {})(fc)
	if fc.ErrorHandler == nil {
		t.Error("WithErrorHandler 未生效")
	}
}

// ---------------------------------------------------------------------------
// 采样
// ---------------------------------------------------------------------------

func TestSampler_Allow(t *testing.T) {
	base := time.Now()
	s := newSampler(2)
	s.now = func() time.Time { return base }

	if !s.allow() {
		t.Error("第一条应放行")
	}
	if !s.allow() {
		t.Error("第二条应放行")
	}
	if s.allow() {
		t.Error("第三条应被拒绝")
	}

	s.now = func() time.Time { return base.Add(2 * time.Second) }
	if !s.allow() {
		t.Error("跨秒后应重新计数并放行")
	}
}

func TestBuilder_WithSampling(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewBuilder().
		WithSampling(1).
		EnableWriter(&buf, InfoLevel).
		Build()
	testx.RequireNoError(t, err)

	// 冒烟测试：仅验证采样配置不 panic、不阻塞正常使用。
	logger.Info("sampled", FieldGroup{})
	logger.Info("maybe dropped", FieldGroup{})
}

// ---------------------------------------------------------------------------
// 脱敏
// ---------------------------------------------------------------------------

func TestWithRedact(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewBuilder().
		WithRedact("", "password").
		WithRedact("token").
		EnableWriter(&buf, InfoLevel).
		Build()
	testx.RequireNoError(t, err)

	lazyCalled := false
	logger.Info("login", Fields(
		String("password", "s3cret"),
		String("user", "alice"),
		Lazy("token", func() any { lazyCalled = true; return "raw-token" }),
	))

	got := buf.String()
	if strings.Contains(got, "s3cret") || strings.Contains(got, "raw-token") {
		t.Errorf("敏感信息未脱敏：%q", got)
	}
	if !strings.Contains(got, "password=***") || !strings.Contains(got, "token=***") {
		t.Errorf("脱敏占位缺失：%q", got)
	}
	if !strings.Contains(got, "user=alice") {
		t.Errorf("非敏感字段被误伤：%q", got)
	}
	testx.False(t, lazyCalled)

}

func TestWithFieldAndRedact_MergeInLog(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewBuilder().
		WithRedact("secret").
		EnableWriter(&buf, InfoLevel).
		Build()
	testx.RequireNoError(t, err)

	child := logger.WithField("base", "v")
	child.Info("merged", Fields(String("secret", "hidden"), Int("n", 1)))

	got := buf.String()
	if !strings.Contains(got, "base=v") || !strings.Contains(got, "n=1") {
		t.Errorf("合并字段缺失：%q", got)
	}
	if strings.Contains(got, "hidden") || !strings.Contains(got, "secret=***") {
		t.Errorf("脱敏未生效：%q", got)
	}
}

func TestMergeFields_EmptyLogger(t *testing.T) {
	// logger 无级联字段时，mergeFields 应原样返回调用字段（早退分支）
	l := &logger{}
	fields := Fields(String("k", "v"))
	got := l.mergeFields(fields)
	if got.Len() != 1 || got.At(0).Key != "k" {
		t.Errorf("空 logger 合并结果不符：%+v", got)
	}
}

func TestRedactFields_NoConfig(t *testing.T) {
	// 未配置脱敏时，redactFields 应原样返回（早退分支）
	l := &logger{}
	fields := Fields(String("k", "v"))
	got := l.redactFields(fields)
	if got.Len() != 1 || got.At(0).Key != "k" || got.At(0).str != "v" {
		t.Errorf("未配置脱敏时不应修改字段：%+v", got)
	}
}

// ---------------------------------------------------------------------------
// 动态级别
// ---------------------------------------------------------------------------

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewBuilder().
		EnableWriter(&buf, InfoLevel).
		Build()
	testx.RequireNoError(t, err)

	if logger.IsDebugEnabled() {
		t.Fatal("初始不应启用 Debug")
	}

	lu, ok := logger.(LevelUpdater)
	testx.RequireTrue(t, ok)

	lu.SetLevel(DebugLevel)
	if !logger.IsDebugEnabled() {
		t.Fatal("SetLevel 后应启用 Debug")
	}

	logger.Debug("debug now visible", FieldGroup{})
	if !strings.Contains(buf.String(), "debug now visible") {
		t.Errorf("Debug 日志未输出：%q", buf.String())
	}
}

func TestSetLevel_Concurrent(t *testing.T) {
	logger, err := NewBuilder().
		EnableWriter(io.Discard, InfoLevel).
		Build()
	testx.RequireNoError(t, err)

	lu := logger.(LevelUpdater)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			lu.SetLevel(DebugLevel, InfoLevel, WarnLevel)
			logger.IsDebugEnabled()
		}
	}()
	for i := 0; i < 500; i++ {
		logger.Info("concurrent", FieldGroup{})
	}
	<-done
}

func TestSetLevel_EmptyAndMin(t *testing.T) {
	lg := &logger{
		cores: []*core{
			newCore(newTextEncoder(false), newConsoleAppender(), InfoLevel),
		},
	}
	var lu LevelUpdater = lg

	lu.SetLevel()
	if lg.IsDebugEnabled() {
		t.Error("空参数不应改变级别")
	}

	lu.SetLevel(ErrorLevel, DebugLevel)
	if !lg.IsDebugEnabled() {
		t.Error("多级别参数应取最低级别")
	}
}

// ---------------------------------------------------------------------------
// 错误回调与核心错误路由
// ---------------------------------------------------------------------------

func TestFileAppender_ErrorHandler(t *testing.T) {
	dir := tempLogDir(t)
	var gotErr error

	app, err := newFileAppender(&FileConfig{
		LogDir:       dir,
		Filename:     "err.log",
		WriteMode:    SyncWriteMode,
		ErrorHandler: func(err error) { gotErr = err },
	})
	testx.RequireNoError(t, err)

	defer app.Close()
	fa := app.(*fileAppender)

	// 关闭底层文件，强制写入失败
	fa.mu.Lock()
	fa.file.Close()
	fa.mu.Unlock()

	c := newCore(newTextEncoder(false), app, DebugLevel)
	c.write(&Entry{Level: InfoLevel, Message: "x"})
	testx.NotNil(t, gotErr)

}

type probeFailingEncoder struct{}

func (probeFailingEncoder) Encode(*Buffer, *Entry) error {
	return fmt.Errorf("encode boom")
}

type errorProbeAppender struct {
	handler func(error)
}

func (a *errorProbeAppender) Append(Level, []byte) (int, error) { return 0, nil }
func (a *errorProbeAppender) Sync() error                       { return nil }
func (a *errorProbeAppender) Close() error                      { return nil }
func (a *errorProbeAppender) reportError(err error)             { a.handler(err) }

func TestCore_ReportEncodeError(t *testing.T) {
	var gotErr error
	c := newCore(probeFailingEncoder{}, &errorProbeAppender{handler: func(err error) { gotErr = err }}, DebugLevel)
	c.write(&Entry{Level: InfoLevel, Message: "x"})
	testx.NotNil(t, gotErr)

}

type failingWriteAppender struct {
	handler func(error)
}

func (a *failingWriteAppender) Append(Level, []byte) (int, error) { return 0, fmt.Errorf("write boom") }
func (a *failingWriteAppender) Sync() error                       { return nil }
func (a *failingWriteAppender) Close() error                      { return nil }
func (a *failingWriteAppender) reportError(err error)             { a.handler(err) }

func TestCore_ReportWriteError(t *testing.T) {
	var gotErr error
	c := newCore(newTextEncoder(false), &failingWriteAppender{handler: func(err error) { gotErr = err }}, DebugLevel)
	c.write(&Entry{Level: InfoLevel, Message: "x"})
	testx.NotNil(t, gotErr)

}

// ---------------------------------------------------------------------------
// 有界背压（异步槽位复用）
// ---------------------------------------------------------------------------

func TestAppendAsync_Backpressure(t *testing.T) {
	fa := &fileAppender{
		writeCh: make(chan []byte, 1),
		freeCh:  make(chan []byte, 1),
		cfg:     FileConfig{},
	}

	done := make(chan struct{})
	go func() {
		fa.appendAsync([]byte("x"))
		close(done)
	}()

	// 无空闲槽时应阻塞（背压），而非丢弃或分配新槽
	select {
	case <-done:
		t.Fatal("无空闲槽时不应立即返回")
	case <-time.After(50 * time.Millisecond):
	}

	fa.freeCh <- make([]byte, 0, 64)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("槽位可用后 appendAsync 应完成")
	}

	select {
	case data := <-fa.writeCh:
		if string(data) != "x" {
			t.Errorf("写入内容不符：%q", data)
		}
	default:
		t.Fatal("日志未进入写通道")
	}
}

func TestAppendAsync_OversizeSlot(t *testing.T) {
	fa := &fileAppender{
		writeCh: make(chan []byte, 1),
		freeCh:  make(chan []byte, 1),
		cfg:     FileConfig{},
	}
	fa.freeCh <- make([]byte, 0, 64)

	payload := bytes.Repeat([]byte("x"), defaultSlotSize+128)
	if _, err := fa.appendAsync(payload); err != nil {
		t.Fatalf("appendAsync 失败：%v", err)
	}
	select {
	case data := <-fa.writeCh:
		if string(data) != string(payload) {
			t.Error("超长日志内容不符")
		}
	default:
		t.Fatal("超长日志未进入写通道")
	}
}

func TestFieldGroup_Overflow(t *testing.T) {
	fs := make([]Field, 0, maxInlineFields+2)
	for i := 0; i < maxInlineFields+1; i++ {
		fs = append(fs, Field{Key: fmt.Sprintf("k%d", i), Value: i})
	}
	g := Fields(fs...)
	if g.Len() != maxInlineFields+1 {
		t.Fatalf("Len 不符：got %d, want %d", g.Len(), maxInlineFields+1)
	}
	if g.At(maxInlineFields).Key != fmt.Sprintf("k%d", maxInlineFields) {
		t.Errorf("rest 字段访问不符：%v", g.At(maxInlineFields))
	}

	// appendField 超出内联容量后的按需分配分支
	g.appendField(Field{Key: "extra", Value: true})
	if g.Len() != maxInlineFields+2 {
		t.Errorf("appendField 后 Len 不符：%d", g.Len())
	}
	testx.Equal(t, g.At(maxInlineFields+1).Key, "extra")

}

func TestFieldGroup_AppendAllocatesRest(t *testing.T) {
	// 逐条 appendField 填满内联容量，第 9 条触发 rest 分配分支
	var g FieldGroup
	for i := 0; i < maxInlineFields; i++ {
		g.appendField(Field{Key: fmt.Sprintf("k%d", i), Value: i})
	}
	g.appendField(Field{Key: "overflow", Value: true})
	if g.Len() != maxInlineFields+1 {
		t.Fatalf("Len 不符：%d", g.Len())
	}
	testx.Equal(t, g.At(maxInlineFields).Key, "overflow")

}

func TestRecycleSlot_Full(t *testing.T) {
	fa := &fileAppender{freeCh: make(chan []byte, 1)}
	fa.freeCh <- make([]byte, 0, 64)

	// 池已满：归还应静默丢弃，绝不阻塞
	fa.recycleSlot(make([]byte, 0, 64))
	if len(fa.freeCh) != 1 {
		t.Errorf("池满时槽位不应入池：%d", len(fa.freeCh))
	}
}

func TestReportError_Nil(t *testing.T) {
	called := false
	fa := &fileAppender{errorHandler: func(error) { called = true }}
	fa.reportError(nil)
	testx.False(t, called)

}

func TestSyncAsync_WriteError(t *testing.T) {
	dir := tempLogDir(t)
	app, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "sync-err.log",
		WriteMode: SyncWriteMode,
	})
	testx.RequireNoError(t, err)

	defer app.Close()
	fapp := app.(*fileAppender)
	fapp.cfg.WriteMode = AsyncWriteMode
	fapp.writeCh = make(chan []byte, 1)
	fapp.freeCh = make(chan []byte, 1)
	fapp.writeCh <- []byte("pending")

	// 关闭底层文件，强制 syncAsync 写失败
	fapp.mu.Lock()
	fapp.file.Close()
	fapp.mu.Unlock()

	if err := fapp.syncAsync(); err == nil {
		t.Error("syncAsync 写已关闭文件应返回错误")
	}
}

func TestSyncAsync_Success(t *testing.T) {
	dir := tempLogDir(t)
	app, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "sync-ok.log",
		WriteMode: SyncWriteMode,
	})
	testx.RequireNoError(t, err)

	defer app.Close()
	fapp := app.(*fileAppender)
	fapp.cfg.WriteMode = AsyncWriteMode
	fapp.writeCh = make(chan []byte, 1)
	fapp.freeCh = make(chan []byte, 1)
	fapp.writeCh <- []byte("ok")

	if err := fapp.syncAsync(); err != nil {
		t.Fatalf("syncAsync 失败：%v", err)
	}
	if m := fapp.Metrics(); m.Writes != 1 || m.WriteBytes == 0 {
		t.Errorf("syncAsync 成功写入未计数：%+v", m)
	}
}

func TestUpdateSymlink_CreateAndError(t *testing.T) {
	dir := tempLogDir(t)
	oldRemove := removePathFn
	oldSymlink := createSymlinkFn
	t.Cleanup(func() {
		removePathFn = oldRemove
		createSymlinkFn = oldSymlink
	})

	removePathFn = func(string) error { return nil }
	createSymlinkFn = func(_, _ string) error { return nil }

	var gotErr error
	fa := &fileAppender{
		symlinkPath:  filepath.Join(dir, "app.log"),
		errorHandler: func(err error) { gotErr = err },
	}

	// 成功分支
	fa.createSymlink("app-2026-01-01.log")
	testx.RequireNil(t, gotErr)

	// 失败分支
	createSymlinkFn = func(_, _ string) error { return fmt.Errorf("link fail") }
	fa.createSymlink("app-2026-01-01.log")
	testx.NotNil(t, gotErr)

}

func TestCleanup_GlobError(t *testing.T) {
	var gotErr error
	fa := &fileAppender{
		dir:           tempLogDir(t),
		basenameNoExt: "bad[",
		ext:           ".log",
		errorHandler:  func(err error) { gotErr = err },
	}
	fa.cleanup()
	testx.NotNil(t, gotErr)

}

func TestCleanup_MaxAgeRemoveError(t *testing.T) {
	dir := tempLogDir(t)
	oldFile := filepath.Join(dir, "old-2020-01-01.log")
	if err := os.MkdirAll(filepath.Join(oldFile, "sub"), 0755); err != nil {
		t.Fatalf("创建占位目录失败：%v", err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("修改时间失败：%v", err)
	}

	var gotErr error
	fa := &fileAppender{
		dir:           dir,
		basenameNoExt: "old",
		ext:           ".log",
		cfg:           FileConfig{MaxAge: 1, MaxBackups: 100},
		errorHandler:  func(err error) { gotErr = err },
	}
	fa.cleanup()
	testx.NotNil(t, gotErr)

}

func TestCleanup_StatError(t *testing.T) {
	dir := tempLogDir(t)
	if err := os.WriteFile(filepath.Join(dir, "old-2020-01-01.log"), []byte("x"), 0644); err != nil {
		t.Fatalf("创建文件失败：%v", err)
	}

	old := pathStatFn
	pathStatFn = func(string) (os.FileInfo, error) { return nil, fmt.Errorf("stat fail") }
	t.Cleanup(func() { pathStatFn = old })

	fa := &fileAppender{
		dir:           dir,
		basenameNoExt: "old",
		ext:           ".log",
		cfg:           FileConfig{MaxAge: 1, MaxBackups: 3, CompressAfter: 1},
		errorHandler:  func(error) {},
	}
	// 不应 panic；Stat 失败路径应安全跳过。
	fa.cleanup()
}

func TestCleanup_MaxAgeCurrentFileSkipped(t *testing.T) {
	dir := tempLogDir(t)
	app, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "cur.log",
		WriteMode: SyncWriteMode,
		MaxAge:    1,
	})
	testx.RequireNoError(t, err)

	defer app.Close()
	fapp := app.(*fileAppender)

	old := filepath.Join(dir, "cur-2020-01-01.log")
	if err := os.WriteFile(old, []byte("old"), 0644); err != nil {
		t.Fatalf("创建旧文件失败：%v", err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatalf("修改时间失败：%v", err)
	}

	fapp.cleanup()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("过期文件应被清理")
	}
	if _, err := os.Stat(fapp.file.Name()); err != nil {
		t.Error("当前文件不应被清理")
	}
}

func TestCleanup_MaxBackupsDeletesOldest(t *testing.T) {
	dir := tempLogDir(t)
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 5; i++ {
		p := filepath.Join(dir, fmt.Sprintf("old-%d.log", i))
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("创建文件失败：%v", err)
		}
		ts := base.Add(time.Duration(i) * time.Second)
		if err := os.Chtimes(p, ts, ts); err != nil {
			t.Fatalf("修改时间失败：%v", err)
		}
	}

	fa := &fileAppender{
		dir:           dir,
		basenameNoExt: "old",
		ext:           ".log",
		cfg:           FileConfig{MaxAge: 0, MaxBackups: 3},
		errorHandler:  func(error) {},
	}
	fa.cleanup()

	files, err := filepath.Glob(filepath.Join(dir, "old-*.log"))
	testx.RequireNoError(t, err)

	if len(files) != 3 {
		t.Errorf("MaxBackups 保留数量不符：got %d, want 3", len(files))
	}
}

func TestCleanup_MaxBackupsRemoveError(t *testing.T) {
	dir := tempLogDir(t)
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	// 最旧的“文件”用非空目录占位，使 os.Remove 失败
	blocker := filepath.Join(dir, "mb-0.log")
	if err := os.MkdirAll(filepath.Join(blocker, "sub"), 0755); err != nil {
		t.Fatalf("创建占位目录失败：%v", err)
	}
	ts := base
	if err := os.Chtimes(blocker, ts, ts); err != nil {
		t.Fatalf("修改时间失败：%v", err)
	}
	for i := 1; i <= 4; i++ {
		p := filepath.Join(dir, fmt.Sprintf("mb-%d.log", i))
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("创建文件失败：%v", err)
		}
		ts := base.Add(time.Duration(i) * time.Second)
		if err := os.Chtimes(p, ts, ts); err != nil {
			t.Fatalf("修改时间失败：%v", err)
		}
	}

	var gotErr error
	fa := &fileAppender{
		dir:           dir,
		basenameNoExt: "mb",
		ext:           ".log",
		cfg:           FileConfig{MaxAge: 0, MaxBackups: 3},
		errorHandler:  func(err error) { gotErr = err },
	}
	fa.cleanup()

	testx.NotNil(t, gotErr)

}

func TestCleanup_MaxBackupsCurrentFileSkipped(t *testing.T) {
	dir := tempLogDir(t)
	// 手动构造（无后台生命周期协程），避免测试期修改配置与后台协程产生数据竞争
	cur := filepath.Join(dir, "mb-2026-01-01.log")
	curFile, err := os.OpenFile(cur, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	testx.RequireNoError(t, err)

	t.Cleanup(func() { _ = curFile.Close() })
	fapp := &fileAppender{
		dir:           dir,
		basenameNoExt: "mb",
		ext:           ".log",
		cfg:           FileConfig{MaxAge: 0, MaxBackups: 3},
		file:          curFile,
		errorHandler:  func(error) {},
	}

	// 把当前文件时间拨到最旧，确保清理循环先经过 currentPhysical 跳过分支
	oldTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(fapp.file.Name(), oldTime, oldTime); err != nil {
		t.Fatalf("修改当前文件时间失败：%v", err)
	}

	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 4; i++ {
		p := filepath.Join(dir, fmt.Sprintf("mb-%d.log", i))
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("创建文件失败：%v", err)
		}
		ts := base.Add(time.Duration(i) * time.Second)
		if err := os.Chtimes(p, ts, ts); err != nil {
			t.Fatalf("修改时间失败：%v", err)
		}
	}

	fapp.cleanup()

	if _, err := os.Stat(fapp.file.Name()); err != nil {
		t.Error("当前文件不应被 MaxBackups 删除")
	}
	files, err := filepath.Glob(filepath.Join(dir, "mb-*.log"))
	testx.RequireNoError(t, err)

	if len(files) != 3 {
		t.Errorf("MaxBackups 保留数量不符：got %d, want 3", len(files))
	}
}

func TestCleanup_CompressViaCleanup(t *testing.T) {
	dir := tempLogDir(t)
	old := filepath.Join(dir, "old-2020-01-01.log")
	if err := os.WriteFile(old, []byte("old data"), 0644); err != nil {
		t.Fatalf("创建旧文件失败：%v", err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatalf("修改时间失败：%v", err)
	}
	gz := filepath.Join(dir, "old-2019-01-01.log.gz")
	if err := os.WriteFile(gz, []byte("already gz"), 0644); err != nil {
		t.Fatalf("创建 gz 文件失败：%v", err)
	}
	if err := os.Chtimes(gz, oldTime.AddDate(-1, 0, 0), oldTime.AddDate(-1, 0, 0)); err != nil {
		t.Fatalf("修改 gz 时间失败：%v", err)
	}

	fa := &fileAppender{
		dir:           dir,
		basenameNoExt: "old",
		ext:           ".log",
		cfg:           FileConfig{MaxAge: 0, MaxBackups: 0, CompressAfter: 1},
		errorHandler:  func(error) {},
	}
	fa.cleanup()

	if m := fa.Metrics(); m.Compressions != 1 {
		t.Errorf("压缩计数不符：got %d, want 1", m.Compressions)
	}
	if _, err := os.Stat(old + ".gz"); err != nil {
		t.Errorf("清理流程未生成压缩产物：%v", err)
	}
}

func TestCleanup_CompressCurrentFileSkipped(t *testing.T) {
	dir := tempLogDir(t)
	cur := filepath.Join(dir, "cc-2026-01-01.log")
	curFile, err := os.OpenFile(cur, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	testx.RequireNoError(t, err)

	t.Cleanup(func() { _ = curFile.Close() })

	old := filepath.Join(dir, "cc-2020-01-01.log")
	if err := os.WriteFile(old, []byte("old"), 0644); err != nil {
		t.Fatalf("创建旧文件失败：%v", err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatalf("修改时间失败：%v", err)
	}

	fa := &fileAppender{
		dir:           dir,
		basenameNoExt: "cc",
		ext:           ".log",
		cfg:           FileConfig{MaxAge: 0, MaxBackups: 0, CompressAfter: 1},
		file:          curFile,
		errorHandler:  func(error) {},
	}
	fa.cleanup()

	if m := fa.Metrics(); m.Compressions != 1 {
		t.Errorf("压缩计数不符：got %d, want 1", m.Compressions)
	}
	if _, err := os.Stat(cur); err != nil {
		t.Error("当前文件不应被压缩")
	}
}

func TestCleanup_GzFileSkipped(t *testing.T) {
	dir := tempLogDir(t)
	gz := filepath.Join(dir, "g-2020-01-01.log.gz")
	if err := os.WriteFile(gz, []byte("already gz"), 0644); err != nil {
		t.Fatalf("创建 gz 文件失败：%v", err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(gz, oldTime, oldTime); err != nil {
		t.Fatalf("修改时间失败：%v", err)
	}

	fa := &fileAppender{
		dir:           dir,
		basenameNoExt: "g",
		ext:           ".gz",
		cfg:           FileConfig{MaxAge: 0, MaxBackups: 0, CompressAfter: 1},
		errorHandler:  func(error) {},
	}
	fa.cleanup()

	if m := fa.Metrics(); m.Compressions != 0 {
		t.Errorf("已压缩文件不应再次压缩：%d", m.Compressions)
	}
	if _, err := os.Stat(gz); err != nil {
		t.Error("已压缩文件不应被删除")
	}
}

func TestCompressFile_Success(t *testing.T) {
	dir := tempLogDir(t)
	src := filepath.Join(dir, "old.log")
	if err := os.WriteFile(src, []byte("compress me"), 0644); err != nil {
		t.Fatalf("创建源文件失败：%v", err)
	}

	fa := &fileAppender{errorHandler: func(error) {}}
	fa.compressFile(src)

	if _, err := os.Stat(src + ".gz"); err != nil {
		t.Fatalf("压缩产物不存在：%v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("压缩成功后原文件应被删除")
	}
	if m := fa.Metrics(); m.Compressions != 1 {
		t.Errorf("压缩计数不符：%d", m.Compressions)
	}
}

func TestCompressFile_CopyError(t *testing.T) {
	dir := tempLogDir(t)
	src := filepath.Join(dir, "old.log")
	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatalf("创建源文件失败：%v", err)
	}

	oldCopy := ioCopyFn
	ioCopyFn = func(io.Writer, io.Reader) (int64, error) { return 0, fmt.Errorf("copy fail") }
	t.Cleanup(func() { ioCopyFn = oldCopy })

	var gotErr error
	fa := &fileAppender{errorHandler: func(err error) { gotErr = err }}
	fa.compressFile(src)
	if gotErr == nil || !strings.Contains(gotErr.Error(), "压缩-写入失败") {
		t.Errorf("写入失败未正确上报：%v", gotErr)
	}
}

type fakeGzipWriter struct {
	errOnClose error
}

func (f fakeGzipWriter) Write(p []byte) (int, error) { return len(p), nil }
func (f fakeGzipWriter) Close() error                { return f.errOnClose }

func TestCompressFile_GzipCloseError(t *testing.T) {
	dir := tempLogDir(t)
	src := filepath.Join(dir, "old.log")
	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatalf("创建源文件失败：%v", err)
	}

	oldNew := newGzipWriterFn
	newGzipWriterFn = func(io.Writer) gzipWriteCloser {
		return fakeGzipWriter{errOnClose: fmt.Errorf("gzip close fail")}
	}
	t.Cleanup(func() { newGzipWriterFn = oldNew })

	var gotErr error
	fa := &fileAppender{errorHandler: func(err error) { gotErr = err }}
	fa.compressFile(src)
	if gotErr == nil || !strings.Contains(gotErr.Error(), "压缩-关闭gzip失败") {
		t.Errorf("gzip 关闭失败未正确上报：%v", gotErr)
	}
}

func TestCompressFile_DstCloseError(t *testing.T) {
	dir := tempLogDir(t)
	src := filepath.Join(dir, "old.log")
	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatalf("创建源文件失败：%v", err)
	}

	oldClose := closeFileFn
	calls := 0
	closeFileFn = func(f *os.File) error {
		calls++
		if calls == 1 { // 第一次关闭的是 dst
			_ = f.Close()
			return fmt.Errorf("dst close fail")
		}
		return f.Close()
	}
	t.Cleanup(func() { closeFileFn = oldClose })

	var gotErr error
	fa := &fileAppender{errorHandler: func(err error) { gotErr = err }}
	fa.compressFile(src)
	if gotErr == nil || !strings.Contains(gotErr.Error(), "压缩-关闭文件失败") {
		t.Errorf("目标文件关闭失败未正确上报：%v", gotErr)
	}
}

func TestCompressFile_SrcCloseError(t *testing.T) {
	dir := tempLogDir(t)
	src := filepath.Join(dir, "old.log")
	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatalf("创建源文件失败：%v", err)
	}

	oldClose := closeFileFn
	calls := 0
	closeFileFn = func(f *os.File) error {
		calls++
		if calls == 2 { // 第二次关闭的是 src
			_ = f.Close()
			return fmt.Errorf("src close fail")
		}
		return f.Close()
	}
	t.Cleanup(func() { closeFileFn = oldClose })

	var gotErr error
	fa := &fileAppender{errorHandler: func(err error) { gotErr = err }}
	fa.compressFile(src)
	if gotErr == nil || !strings.Contains(gotErr.Error(), "压缩-关闭源文件失败") {
		t.Errorf("源文件关闭失败未正确上报：%v", gotErr)
	}
}

func TestCompressFile_RemoveSrcError(t *testing.T) {
	dir := tempLogDir(t)
	src := filepath.Join(dir, "old.log")
	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatalf("创建源文件失败：%v", err)
	}

	oldRemove := removePathFn
	removePathFn = func(p string) error {
		if p == src {
			return fmt.Errorf("remove fail")
		}
		return os.Remove(p)
	}
	t.Cleanup(func() { removePathFn = oldRemove })

	var gotErr error
	fa := &fileAppender{errorHandler: func(err error) { gotErr = err }}
	fa.compressFile(src)
	if gotErr == nil || !strings.Contains(gotErr.Error(), "压缩后删除源文件失败") {
		t.Errorf("删除源文件失败未正确上报：%v", gotErr)
	}
	if m := fa.Metrics(); m.Compressions != 0 {
		t.Errorf("删除失败不应计入压缩成功：%d", m.Compressions)
	}
}

func TestReportError_DefaultStderr(t *testing.T) {
	orig := os.Stderr
	f, err := os.CreateTemp("", "logx-stderr-*")
	testx.RequireNoError(t, err)

	os.Stderr = f
	t.Cleanup(func() {
		os.Stderr = orig
		_ = f.Close()
		_ = os.Remove(f.Name())
	})

	fa := &fileAppender{}
	fa.reportError(fmt.Errorf("stderr fallback"))

	_ = f.Sync()
	data, err := os.ReadFile(f.Name())
	testx.RequireNoError(t, err)

	if !strings.Contains(string(data), "stderr fallback") {
		t.Errorf("默认错误输出未写入 stderr：%q", data)
	}
}

// ---------------------------------------------------------------------------
// 指标
// ---------------------------------------------------------------------------

func TestLogger_Metrics(t *testing.T) {
	dir := tempLogDir(t)
	logger, err := NewBuilder().
		EnableFileLog(
			WithLogDir(dir),
			WithFilename("metrics.log"),
			WithWriteMode(SyncWriteMode),
			WithLevels(InfoLevel),
		).
		Build()
	testx.RequireNoError(t, err)

	defer logger.Close()

	logger.Info("metric entry", FieldGroup{})
	logger.Sync()

	mp, ok := logger.(MetricProvider)
	testx.RequireTrue(t, ok)

	m := mp.Metrics()
	if m.Writes == 0 {
		t.Error("Writes 应为正数")
	}
	if m.WriteBytes == 0 {
		t.Error("WriteBytes 应为正数")
	}
	if m.Cleanups == 0 {
		t.Error("Cleanups 应至少为 1（启动时立即清理）")
	}
}

func TestLogger_Metrics_ConsoleOnly(t *testing.T) {
	logger, err := NewBuilder().EnableConsole(InfoLevel).Build()
	testx.RequireNoError(t, err)

	mp, ok := logger.(MetricProvider)
	testx.RequireTrue(t, ok)

	// 控制台通道不提供指标，应安全返回零值而非 panic。
	_ = mp.Metrics()
}

func TestBuilder_UnknownAppenderType(t *testing.T) {
	b := &Builder{
		cores: []coreConfig{{appType: "unknown", minLvl: DebugLevel}},
	}
	if _, err := b.Build(); err == nil {
		t.Error("未知通道类型应返回错误")
	}
}

func TestBuilder_EnableWriterNil(t *testing.T) {
	b := NewBuilder().EnableWriter(nil, InfoLevel)
	if _, err := b.Build(); err == nil {
		t.Error("nil Writer 应返回错误")
	}
}

// ---------------------------------------------------------------------------
// 故障注入
// ---------------------------------------------------------------------------

func TestFileAppender_LogDirIsFile(t *testing.T) {
	dir := tempLogDir(t)

	filePath := dir + ".txt"
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatalf("创建占位文件失败：%v", err)
	}
	t.Cleanup(func() { _ = os.Remove(filePath) })

	_, err := newFileAppender(&FileConfig{
		LogDir:   filePath,
		Filename: "a.log",
	})
	testx.RequireError(t, err)

}

func TestFileAppender_InvalidFilename(t *testing.T) {
	dir := tempLogDir(t)
	_, err := newFileAppender(&FileConfig{
		LogDir:   dir,
		Filename: "sub/app.log", // 子目录不存在，物理文件创建必然失败
	})
	testx.RequireError(t, err)

}

func TestFileAppender_AbsError(t *testing.T) {
	old := absPathFn
	absPathFn = func(string) (string, error) { return "", fmt.Errorf("abs fail") }
	t.Cleanup(func() { absPathFn = old })

	_, err := newFileAppender(&FileConfig{
		LogDir:   tempLogDir(t),
		Filename: "a.log",
	})
	testx.RequireError(t, err)

}

func TestFileAppender_AppendSyncRotationError(t *testing.T) {
	dir := tempLogDir(t)
	app, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "rot-err.log",
		WriteMode: SyncWriteMode,
	})
	testx.RequireNoError(t, err)

	defer app.Close()
	fapp := app.(*fileAppender)

	// 注入轮转打开失败，使轮转必然失败（不依赖目录删除时序）。
	origOpen := openNewFileFn
	openNewFileFn = func(string, int, os.FileMode) (*os.File, error) {
		return nil, errors.New("模拟轮转失败")
	}
	defer func() { openNewFileFn = origOpen }()
	fapp.mu.Lock()
	fapp.file.Close()
	fapp.mu.Unlock()
	fapp.mu.Lock()
	fapp.currentSize = int64(fapp.cfg.MaxSize) * 1024 * 1024
	fapp.mu.Unlock()

	if _, err := app.Append(InfoLevel, []byte("x")); err == nil {
		t.Error("轮转失败时 Append 应返回错误")
	}
}

func TestFileAppender_AppendAfterClose(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "closed.log",
		WriteMode: SyncWriteMode,
	})
	testx.RequireNoError(t, err)

	fa.Close()
	if _, err := fa.Append(InfoLevel, []byte("x")); err == nil {
		t.Error("关闭后 Append 应返回错误")
	}
}

func TestFileAppender_CloseError_Injected(t *testing.T) {
	dir := tempLogDir(t)
	app, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "close-err.log",
		WriteMode: SyncWriteMode,
	})
	testx.RequireNoError(t, err)

	oldClose := closeFileFn
	closeFileFn = func(f *os.File) error {
		_ = f.Close()
		return fmt.Errorf("close fail")
	}
	t.Cleanup(func() { closeFileFn = oldClose })

	if err := app.Close(); err == nil {
		t.Error("文件关闭失败时应返回错误")
	}
}

func TestOpenNewFile_StatError(t *testing.T) {
	dir := tempLogDir(t)
	fa := &fileAppender{
		dir:           dir,
		basenameNoExt: "stat",
		ext:           ".log",
	}

	old := fileStatFn
	fileStatFn = func(*os.File) (os.FileInfo, error) { return nil, fmt.Errorf("stat fail") }
	t.Cleanup(func() { fileStatFn = old })

	if err := fa.openNewFile(); err == nil {
		t.Error("文件 Stat 失败时应返回错误")
	}
	if fa.file != nil {
		_ = fa.file.Close()
	}
}

func TestFileAppender_NoExtension(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "plain",
		WriteMode: SyncWriteMode,
	})
	testx.RequireNoError(t, err)

	defer fa.Close()

	files, err := filepath.Glob(filepath.Join(dir, "plain-*.log"))
	if err != nil || len(files) == 0 {
		t.Fatalf("无扩展名文件名未自动补 .log：%v", err)
	}
}

func TestFileAppender_TimeRotation(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "time-rot.log",
		WriteMode: SyncWriteMode,
	})
	testx.RequireNoError(t, err)

	defer fa.Close()
	fapp := fa.(*fileAppender)

	// 把下一次自然天轮转时间拨到过去，强制时间轮转
	fapp.mu.Lock()
	fapp.rotateAt = time.Now().Add(-time.Hour)
	fapp.mu.Unlock()

	if _, err := fa.Append(InfoLevel, []byte("after midnight")); err != nil {
		t.Fatalf("Append 失败：%v", err)
	}
	if m := fapp.Metrics(); m.Rotations == 0 {
		t.Error("时间轮转未触发")
	}
}

func TestAsyncBatchFlush(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "batch.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    1024,
		FlushInterval: time.Hour,
	})
	testx.RequireNoError(t, err)

	defer fa.Close()

	payload := bytes.Repeat([]byte("x"), 70*1024)
	if _, err := fa.Append(InfoLevel, payload); err != nil {
		t.Fatalf("Append 失败：%v", err)
	}
	fapp := fa.(*fileAppender)

	// 等待后台批量刷盘（超过 64KB 阈值立即触发）
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if fapp.Metrics().Writes >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("后台批量刷盘未执行")
}

func TestAsyncFlush_AfterCancel(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "cancel.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    16,
		FlushInterval: time.Hour,
	})
	testx.RequireNoError(t, err)

	defer fa.Close()
	fapp := fa.(*fileAppender)

	// 先入队再取消：确保数据在后台协程退出前已进入写通道
	if _, err := fa.Append(InfoLevel, []byte("pending after cancel")); err != nil {
		t.Fatalf("Append 失败：%v", err)
	}
	fapp.cancel()

	// 后台协程应在退出前排空通道
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if fapp.Metrics().Writes >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("取消后后台协程未排空通道")
}

func TestRunFlushLoop_RotationError(t *testing.T) {
	dir := tempLogDir(t)
	gotErr := make(chan error, 1)
	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "flush-rot.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    16,
		FlushInterval: time.Hour,
		ErrorHandler: func(err error) {
			select {
			case gotErr <- err:
			default:
			}
		},
	})
	testx.RequireNoError(t, err)

	defer fa.Close()
	fapp := fa.(*fileAppender)

	origOpen := openNewFileFn
	openNewFileFn = func(string, int, os.FileMode) (*os.File, error) {
		return nil, errors.New("模拟轮转失败")
	}
	defer func() { openNewFileFn = origOpen }()
	fapp.mu.Lock()
	fapp.file.Close()
	fapp.mu.Unlock()
	fapp.mu.Lock()
	fapp.currentSize = int64(fapp.cfg.MaxSize) * 1024 * 1024
	fapp.mu.Unlock()

	if _, err := fa.Append(InfoLevel, bytes.Repeat([]byte("x"), 70*1024)); err != nil {
		t.Fatalf("Append 失败：%v", err)
	}

	select {
	case <-gotErr:
	case <-time.After(5 * time.Second):
		t.Fatal("异步刷盘轮转失败未上报错误处理器")
	}
}

func TestDrainAsync_WriteError(t *testing.T) {
	dir := tempLogDir(t)
	var gotErr error
	app, err := newFileAppender(&FileConfig{
		LogDir:       dir,
		Filename:     "drain-err.log",
		WriteMode:    SyncWriteMode,
		ErrorHandler: func(err error) { gotErr = err },
	})
	testx.RequireNoError(t, err)

	defer app.Close()
	fapp := app.(*fileAppender)
	fapp.writeCh = make(chan []byte, 1)
	fapp.freeCh = make(chan []byte, 1)
	fapp.writeCh <- []byte("pending")

	fapp.mu.Lock()
	fapp.file.Close()
	fapp.mu.Unlock()

	fapp.drainAsync()
	testx.NotNil(t, gotErr)

}

func TestDrainAsync_RotationError(t *testing.T) {
	dir := tempLogDir(t)
	var gotErr error
	app, err := newFileAppender(&FileConfig{
		LogDir:       dir,
		Filename:     "drain-rot.log",
		WriteMode:    SyncWriteMode,
		ErrorHandler: func(err error) { gotErr = err },
	})
	testx.RequireNoError(t, err)

	defer app.Close()
	fapp := app.(*fileAppender)
	fapp.writeCh = make(chan []byte, 1)
	fapp.freeCh = make(chan []byte, 1)
	fapp.writeCh <- []byte("pending")

	origOpen := openNewFileFn
	openNewFileFn = func(string, int, os.FileMode) (*os.File, error) {
		return nil, errors.New("模拟轮转失败")
	}
	defer func() { openNewFileFn = origOpen }()
	fapp.mu.Lock()
	fapp.file.Close()
	fapp.mu.Unlock()
	fapp.mu.Lock()
	fapp.currentSize = int64(fapp.cfg.MaxSize) * 1024 * 1024
	fapp.mu.Unlock()

	fapp.drainAsync()
	testx.NotNil(t, gotErr)

}

func TestSyncAsync_RotationError(t *testing.T) {
	dir := tempLogDir(t)
	app, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "sync-rot.log",
		WriteMode: SyncWriteMode,
	})
	testx.RequireNoError(t, err)

	defer app.Close()
	fapp := app.(*fileAppender)
	fapp.cfg.WriteMode = AsyncWriteMode
	fapp.writeCh = make(chan []byte, 1)
	fapp.freeCh = make(chan []byte, 1)
	fapp.writeCh <- []byte("pending")

	origOpen := openNewFileFn
	openNewFileFn = func(string, int, os.FileMode) (*os.File, error) {
		return nil, errors.New("模拟轮转失败")
	}
	defer func() { openNewFileFn = origOpen }()
	fapp.mu.Lock()
	fapp.file.Close()
	fapp.mu.Unlock()
	fapp.mu.Lock()
	fapp.currentSize = int64(fapp.cfg.MaxSize) * 1024 * 1024
	fapp.mu.Unlock()

	if err := fapp.syncAsync(); err == nil {
		t.Error("syncAsync 轮转失败应返回错误")
	}
}

func TestSyncAsync_FileNilAfterClose(t *testing.T) {
	dir := tempLogDir(t)
	fa, err := newFileAppender(&FileConfig{
		LogDir:        dir,
		Filename:      "nil-sync.log",
		WriteMode:     AsyncWriteMode,
		BufferSize:    16,
		FlushInterval: time.Hour,
	})
	testx.RequireNoError(t, err)

	if err := fa.Close(); err != nil {
		t.Fatalf("Close 失败：%v", err)
	}
	if err := fa.Sync(); err != nil {
		t.Errorf("关闭后 Sync 应返回 nil：%v", err)
	}
}

func TestDrainPending(t *testing.T) {
	fa := &fileAppender{
		writeCh: make(chan []byte, 2),
		freeCh:  make(chan []byte, 2),
	}
	fa.writeCh <- []byte("a")
	fa.writeCh <- []byte("b")

	var buf bytes.Buffer
	fa.drainPending(&buf)
	if buf.String() != "ab" {
		t.Errorf("drainPending 输出不符：%q", buf.String())
	}
}

func TestLifecycle_TickerCleanup(t *testing.T) {
	old := lifecycleCheckInterval
	lifecycleCheckInterval = 20 * time.Millisecond
	t.Cleanup(func() { lifecycleCheckInterval = old })

	dir := tempLogDir(t)
	app, err := newFileAppender(&FileConfig{
		LogDir:    dir,
		Filename:  "lc.log",
		WriteMode: SyncWriteMode,
	})
	testx.RequireNoError(t, err)

	defer app.Close()
	fapp := app.(*fileAppender)

	// 启动时执行一次，之后由 ticker 周期触发
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if fapp.Metrics().Cleanups >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("生命周期 ticker 未触发周期清理")
}

func TestSortByModTime_StatError(t *testing.T) {
	old := sortStatFn
	sortStatFn = func(string) (os.FileInfo, error) { return nil, fmt.Errorf("stat fail") }
	t.Cleanup(func() { sortStatFn = old })

	paths := []string{"a.log", "b.log"}
	// 不应 panic；Stat 失败时应保持原顺序
	sortByModTime(paths)
	if paths[0] != "a.log" || paths[1] != "b.log" {
		t.Errorf("排序被错误修改：%v", paths)
	}
}

func TestCompressFile_ErrorHandler(t *testing.T) {
	dir := tempLogDir(t)
	src := dir + "\\old.log"
	if err := os.WriteFile(src, []byte("old"), 0644); err != nil {
		t.Fatalf("创建源文件失败：%v", err)
	}
	// 用同名目录占住 .gz 目标，迫使 os.Create 失败
	gzDir := src + ".gz"
	if err := os.MkdirAll(gzDir, 0755); err != nil {
		t.Fatalf("创建占位目录失败：%v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(gzDir) })

	var gotErr error
	fa := &fileAppender{errorHandler: func(err error) { gotErr = err }}
	fa.compressFile(src)
	testx.NotNil(t, gotErr)

}

// ---------------------------------------------------------------------------
// Fuzz
// ---------------------------------------------------------------------------

func FuzzJSONEncoder_Encode(f *testing.F) {
	f.Add("hello", "world")
	f.Add("a\"b\\c\nd", "k\"v")
	f.Add("中文消息", "中文字段")

	enc := NewJSONEncoder()
	f.Fuzz(func(t *testing.T, msg, field string) {
		// JSON 对非法 UTF-8 会替换为 U+FFFD，无法无损往返，跳过此类输入。
		if !utf8.ValidString(msg) || !utf8.ValidString(field) {
			return
		}
		buf := getBuffer()
		defer putBuffer(buf)
		if err := enc.Encode(buf, &Entry{
			Level:   InfoLevel,
			Time:    time.Now(),
			Message: msg,
			Fields:  Fields(String("k", field)),
		}); err != nil {
			t.Fatalf("Encode 失败：%v", err)
		}
		var out map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(buf.B), &out); err != nil {
			t.Fatalf("输出不是合法 JSON：%v", err)
		}
		if out["message"] != msg || out["k"] != field {
			t.Fatalf("往返不一致：%q %q", out["message"], out["k"])
		}
	})
}

func FuzzTextEncoder_Encode(f *testing.F) {
	f.Add("hello", "world")
	f.Add("中文消息", "字段值")

	enc := newTextEncoder(false)
	f.Fuzz(func(t *testing.T, msg, field string) {
		buf := getBuffer()
		defer putBuffer(buf)
		if err := enc.Encode(buf, &Entry{
			Level:   InfoLevel,
			Time:    time.Now(),
			Message: msg,
			Fields:  Fields(String("k", field)),
		}); err != nil {
			t.Fatalf("Encode 失败：%v", err)
		}
		if !bytes.Contains(buf.B, []byte(msg)) {
			t.Fatalf("正文丢失：%q", buf.B)
		}
	})
}
