package logx

import (
	testx "github.com/lcylpzls/testx"
	"strings"
	"sync"
	"testing"
)

// fakeMetricSink 是外部指标接收器测试替身。
type fakeMetricSink struct {
	mu       sync.Mutex
	counters map[string][]string
	adds     map[string][]string
}

func newFakeMetricSink() *fakeMetricSink {
	return &fakeMetricSink{
		counters: make(map[string][]string),
		adds:     make(map[string][]string),
	}
}

func (f *fakeMetricSink) IncCounter(name string, labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counters[name] = append(f.counters[name], strings.Join(labels, "|"))
}

func (f *fakeMetricSink) ObserveDuration(string, float64, ...string) {}

func (f *fakeMetricSink) AddCounter(name string, delta float64, labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.adds[name] = append(f.adds[name], strings.Join(labels, "|"))
}

func (f *fakeMetricSink) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.counters[name])
}

func (f *fakeMetricSink) addCount(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.adds[name])
}

func TestBuilderWithMetricsRecords(t *testing.T) {
	sink := newFakeMetricSink()
	logger, err := NewBuilder().
		EnableConsole(InfoLevel).
		WithMetrics(sink).
		Build()
	testx.RequireNoError(t, err)

	logger.Info("测试日志", FieldGroup{})
	if got := sink.count("logx.records"); got != 1 {
		t.Errorf("records 计数不符：%d", got)
	}
	// WithField 派生 logger 应继承指标接收器。
	logger.WithField("k", "v").Warn("带字段日志", FieldGroup{})
	if got := sink.count("logx.records"); got != 2 {
		t.Errorf("派生 logger records 计数不符：%d", got)
	}
	if got := sink.counters["logx.records"][1]; got != "warn" {
		t.Errorf("records 标签不符：%s", got)
	}
}

func TestFileAppenderMetricsSink(t *testing.T) {
	sink := newFakeMetricSink()
	dir := t.TempDir()
	logger, err := NewBuilder().
		EnableFileLog(
			WithLogDir(dir),
			WithFilename("app.log"),
			WithLevels(InfoLevel),
			WithWriteMode(SyncWriteMode),
		).
		WithMetrics(sink).
		Build()
	testx.RequireNoError(t, err)

	defer logger.Close()
	logger.Info("写入测试", FieldGroup{})
	if err := logger.Sync(); err != nil {
		t.Fatal(err)
	}
	if got := sink.count("logx.writes"); got < 1 {
		t.Errorf("writes 计数不符：%d", got)
	}
	if got := sink.addCount("logx.write_bytes"); got < 1 {
		t.Errorf("write_bytes 计数不符：%d", got)
	}
	if got := sink.count("logx.cleanups"); got < 1 {
		t.Errorf("启动清理应产生 cleanups 事件：%d", got)
	}
}

func TestFileAppenderEmitHelpers(t *testing.T) {
	sink := newFakeMetricSink()
	app, err := newFileAppender(&FileConfig{
		LogDir:   t.TempDir(),
		Filename: "app.log",
	}, sink)
	testx.RequireNoError(t, err)

	fa := app.(*fileAppender)
	fa.emitCounter("logx.rotations")
	fa.emitWrite(10)
	defer fa.Close()
	if got := sink.count("logx.rotations"); got != 1 {
		t.Errorf("rotations 计数不符：%d", got)
	}
	if got := sink.count("logx.writes"); got != 1 {
		t.Errorf("writes 计数不符：%d", got)
	}
	// 未注入 sink 时全部静默。
	app2, err := newFileAppender(&FileConfig{
		LogDir:   t.TempDir(),
		Filename: "app.log",
	})
	testx.RequireNoError(t, err)

	fa2 := app2.(*fileAppender)
	defer fa2.Close()
	fa2.emitCounter("logx.rotations")
	fa2.emitWrite(10)
}

func TestFileAppenderMetricsSinkSyncWrite(t *testing.T) {
	sink := newFakeMetricSink()
	app, err := newFileAppender(&FileConfig{
		LogDir:   t.TempDir(),
		Filename: "app.log",
	}, sink)
	testx.RequireNoError(t, err)

	fa := app.(*fileAppender)
	defer fa.Close()
	if _, err := fa.appendSync([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if got := sink.count("logx.writes"); got != 1 {
		t.Errorf("同步写入 writes 计数不符：%d", got)
	}
}
