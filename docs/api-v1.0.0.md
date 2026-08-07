<!--
v1.0.0 API 冻结基线
生成方式：go doc -all .
用途：API 变更审计参考；未来版本可结合 golang.org/x/exp/cmd/apidiff 检测破坏性变更。
-->

package logx // import "github.com/lcylpzls/logx"

Package logx 是工业级零依赖高性能 Go 结构化日志库。

FUNCTIONS

func ReplaceStdLogger(l Logger)
    ReplaceStdLogger 将 Go 标准库 log 包的默认输出重定向到 logx Logger。 这允许已有代码中任何 log.Println
    / log.Printf 调用自动流经 logx 引擎。

    用法：

        logx.ReplaceStdLogger(myLogger)
        defer logx.RestoreStdLogger() // 可选：恢复标准库原始输出

func RestoreStdLogger()
    RestoreStdLogger 恢复标准库 log 包的默认输出到 os.Stderr。


TYPES

type Appender interface {
	// Append 将日志数据写入目标。level 用于输出器内部路由（如 stdout/stderr 分流）。
	Append(level Level, p []byte) (n int, err error)
	// Sync 强制将缓冲数据刷入底层介质。
	Sync() error
	// Close 关闭输出器并释放资源。
	Close() error
}
    Appender 输出器接口，负责将字节流落地到目标介质。 内置实现：ConsoleAppender（控制台）、FileAppender（文件）。

type Buffer struct {
	B []byte
}
    Buffer 封装一个从 sync.Pool 中分配的字节切片，用于日志编码。 单条日志生命周期：GetBuffer → Encode →
    Appender.Write → PutBuffer

type Builder struct {
	// Has unexported fields.
}
    Builder 是 logx 的链式配置构造器，支持多通道精细化级别控制。

    典型用法：

        logger, err := logx.NewBuilder().
            EnableConsole(logx.InfoLevel, logx.ErrorLevel, logx.WithColor()).
            EnableFileLog(
                logx.WithLogDir("/var/log/myapp"),
                logx.WithFilename("app.log"),
                logx.WithLevels(logx.InfoLevel),
            ).
            Build()

func NewBuilder() *Builder
    NewBuilder 创建一个新的 Builder 实例，所有通道默认静默。

func (b *Builder) Build() (Logger, error)
    Build 根据当前配置构建 Logger 实例。 若未配置任何输出通道或所有通道均为静默，返回的 Logger 不产生任何输出。

func (b *Builder) EnableConsole(opts ...ConsoleOption) *Builder
    EnableConsole 开启控制台输出通道。 接受零个或多个 ConsoleOption 参数：
      - Level 值（InfoLevel, ErrorLevel 等）指定启用的日志级别
      - WithColor() 启用 ANSI 色彩

    若未传入任何 Level，该通道保持静默。

func (b *Builder) EnableFileLog(opts ...FileOption) *Builder
    EnableFileLog 开启文件输出通道，接收文件配置项。

    必须通过 WithLevels() 指定启用的日志级别，否则该通道保持静默。

func (b *Builder) EnableWriter(w io.Writer, levels ...Level) *Builder
    EnableWriter 开启自定义 io.Writer 输出通道，便于将日志路由到网络、消息队列等目标。 与 EnableConsole
    一样，必须显式传入要启用的日志级别。

func (b *Builder) WithCaller() *Builder
    WithCaller 启用调用者追踪，日志输出中将包含源文件名和行号。 格式：file.go:42

func (b *Builder) WithEncoder(enc Encoder) *Builder
    WithEncoder 设置后续添加通道使用的编码器，默认使用纯文本编码器。 例如
    logx.NewBuilder().WithEncoder(logx.NewJSONEncoder()) 可以让后续通道输出 JSON。

func (b *Builder) WithRedact(keys ...string) *Builder
    WithRedact 配置自动脱敏的字段 key。匹配的字段（含 Lazy 字段）在编码前 会被替换为 "***"，避免手机号、密码等敏感信息落入日志。

func (b *Builder) WithSampling(maxPerSecond int) *Builder
    WithSampling 启用按秒限流采样：同一秒内最多输出 maxPerSecond 条日志，超出部分丢弃。 用于故障风暴场景保护磁盘
    IO。maxPerSecond <= 0 表示不采样。

type ConsoleOption interface {
	// Has unexported methods.
}
    ConsoleOption 是 EnableConsole 接受的配置选项。 Level 和 WithColor() 均实现此接口。

func WithColor() ConsoleOption
    WithColor 返回一个 ConsoleOption，为控制台通道启用 ANSI 色彩高亮。

type Encoder interface {
	// Encode 将日志条目编码到 buf 中。buf 由调用方从缓冲池获取。
	Encode(buf *Buffer, e *Entry) error
}
    Encoder 编码器接口，负责将 Entry 编码写入 Buffer。 内置实现：TextEncoder（纯文本）、JSONEncoder（结构化
    JSON，后续版本）。

func NewJSONEncoder() Encoder
    NewJSONEncoder 创建一个 JSON 编码器实例，可配合 Builder.WithEncoder 使用。

type Entry struct {
	// Level 日志级别
	Level Level
	// Time 日志产生时间
	Time time.Time
	// Message 日志正文
	Message string
	// Fields 结构化字段（内联容器，热路径零分配）
	Fields FieldGroup
	// CallerFile 调用者源文件名（WithCaller 启用后填充）
	CallerFile string
	// CallerLine 调用者行号（WithCaller 启用后填充）
	CallerLine int

	// Has unexported fields.
}
    Entry 表示一条待输出的日志条目，承载该条日志的完整上下文。

func (e *Entry) Context() context.Context
    Context 返回日志携带的 context.Context，若为 nil 则返回 context.Background。

type Field struct {
	Key string

	Value any

	// Has unexported fields.
}
    Field 表示一个结构化的键值对日志字段。 常用类型直接存储在类型化槽位中，避免变量装箱分配；Value 仅作兜底。

func Any(key string, val any) Field
    Any 构造一个任意类型字段。

func Bool(key string, val bool) Field
    Bool 构造一个布尔字段。

func Err(err error) Field
    Err 构造一个错误字段，key 固定为 "error"。

func Int(key string, val int) Field
    Int 构造一个整数字段。

func Int64(key string, val int64) Field
    Int64 构造一个 int64 字段。

func Lazy(key string, fn func() any) Field
    Lazy 构造一个延迟求值字段。fn 仅在日志级别通过过滤、实际编码时才被调用。 用于避免在高开销的 Debug 日志中执行不必要的计算。

        logger.Debug("user", logx.Lazy("info", func() interface{} {
            return expensiveQuery()  // 仅 Debug 启用时才会执行
        }))

func String(key string, val string) Field
    String 构造一个字符串字段。

type FieldGroup struct {
	// Has unexported fields.
}
    FieldGroup 是结构化字段的零分配容器： 前 8 个字段内联在值中（栈上或 Entry 内），超过 8 个时才按需分配。

func Fields(fs ...Field) FieldGroup
    Fields 构造一个 FieldGroup。常规数量（<=8）下零堆分配： 变参切片仅被读取（拷贝到内联数组），不会被保存，因此不逃逸。

func (g FieldGroup) At(i int) Field
    At 返回第 i 个字段（i 必须小于 Len）。

func (g FieldGroup) Len() int
    Len 返回字段数量。

type FileConfig struct {
	LogDir        string        // 日志目录
	Filename      string        // 基础文件名
	MaxSize       int           // 单文件最大容量（MB），默认 100
	MaxAge        int           // 最长保留天数，默认 180
	MaxBackups    int           // 最多保留历史文件数，默认 100
	CompressAfter int           // 超过 N 天的历史日志自动压缩为 gz，默认 0（不压缩）
	WriteMode     WriteMode     // 写入模式：异步（默认）或同步
	BufferSize    int           // 异步通道缓冲大小，默认 4096
	FlushInterval time.Duration // 异步批量刷盘间隔，默认 1 秒
	Levels        []Level       // 启用的日志级别列表
	ErrorHandler  func(error)   // 内部错误统一回调（nil=输出到 stderr）
}
    FileConfig 文件通道专属配置项。

type FileOption func(*FileConfig)
    FileOption 文件通道配置函数类型。

func WithBufferSize(size int) FileOption
    WithBufferSize 设置异步通道缓冲大小。

func WithCompressAfter(days int) FileOption
    WithCompressAfter 设置压缩延迟天数。

func WithErrorHandler(fn func(error)) FileOption
    WithErrorHandler 设置文件通道内部错误的统一处理回调。 未设置时，内部错误降级输出到标准错误流。

func WithFilename(name string) FileOption
    WithFilename 设置基础文件名。

func WithFlushInterval(d time.Duration) FileOption
    WithFlushInterval 设置异步批量刷盘间隔。

func WithLevels(levels ...Level) FileOption
    WithLevels 为文件通道指定启用的日志级别。

func WithLogDir(dir string) FileOption
    WithLogDir 设置日志目录。

func WithMaxAge(days int) FileOption
    WithMaxAge 设置最长保留天数。

func WithMaxBackups(n int) FileOption
    WithMaxBackups 设置最多保留历史文件数。

func WithMaxSize(mb int) FileOption
    WithMaxSize 设置单文件最大容量（MB）。

func WithWriteMode(mode WriteMode) FileOption
    WithWriteMode 设置文件写入模式（异步或同步）。

type Hook interface {
	// OnLog 在日志被写入后调用。entry 是已编码的日志条目。
	// 实现必须保证 OnLog 不会 panic，且应尽快返回。
	OnLog(e *Entry)
}
    Hook 定义日志拦截钩子。当一条日志通过级别过滤并被成功写入后， OnLog 会被异步调用（不阻塞日志主路径）。

    典型用途：错误报警（飞书/钉钉/Sentry）、监控打点、审计日志。

type HookedLogger interface {
	Logger
	// AddHook 注册一个日志 Hook。所有通过该 Logger 的日志都会被 Hook 拦截。
	AddHook(h Hook)
}
    HookedLogger 扩展 Logger 接口，支持添加 Hook。

type Level uint32
    Level 表示日志级别。数值越大，严重程度越高。

const (
	OffLevel   Level = iota // 0 — 关闭所有日志输出
	DebugLevel              // 1 — 调试日志，开发环境专用
	InfoLevel               // 2 — 常规业务运行日志
	WarnLevel               // 3 — 警告日志，非错误但需关注
	ErrorLevel              // 4 — 业务错误日志
	PanicLevel              // 5 — 恐慌日志，输出后触发 panic
	FatalLevel              // 6 — 致命错误日志，输出后强制退出
)
    7 级标准日志级别。OffLevel 用于关闭所有输出，默认静默。

func (l Level) String() string
    String 返回日志级别对应的英文短名称，固定 5 字符宽度。

type LevelUpdater interface {
	SetLevel(levels ...Level)
}
    LevelUpdater 是提供运行时动态调整日志级别能力的可选接口。 Logger 的默认实现支持该接口，通过类型断言使用：

        if lu, ok := logger.(logx.LevelUpdater); ok {
            lu.SetLevel(logx.DebugLevel)
        }

type Logger interface {
	// IsDebugEnabled 判断 Debug 级别是否在当前实例中启用。
	IsDebugEnabled() bool

	Debug(msg string, fields FieldGroup)
	Info(msg string, fields FieldGroup)
	Warn(msg string, fields FieldGroup)
	Error(msg string, fields FieldGroup)
	Panic(msg string, fields FieldGroup) // 输出后触发 panic
	Fatal(msg string, fields FieldGroup) // 输出后退出进程

	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
	Panicf(format string, args ...any) // 输出后触发 panic
	Fatalf(format string, args ...any) // 输出后退出进程

	// WithContext 派生一个新的 Logger，携带指定的 context.Context。
	WithContext(ctx context.Context) Logger
	// WithField 派生一个新的 Logger，默认携带一个结构化字段。
	WithField(key string, val any) Logger

	// Sync 强制刷盘所有通道的缓冲日志。
	Sync() error
	// Close 关闭 Logger 并释放所有资源。
	Close() error
	// SafeExit 优雅退出：先同步刷盘所有缓冲日志，再执行 exitFunc。
	// 用于替代直接调用 os.Exit，确保异步模式下临终日志不丢失。
	SafeExit(exitFunc func())
}
    Logger 定义了对外暴露的顶层日志接口，隔离具体实现。

    Logger 同时支持两种调用风格：
     1. 结构化字段模式：Info("msg", logx.String("key", "val"))
     2. 格式化打印模式：Infof("count=%d", 42)

type MetricProvider interface {
	Metrics() Metrics
}
    MetricProvider 是提供运行指标的可选接口。 通过类型断言使用：

        if mp, ok := logger.(logx.MetricProvider); ok {
            m := mp.Metrics()
        }

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
    Metrics 是日志库运行指标的汇总快照。 所有计数均为原子累加，可安全地在任意时刻读取。

type WriteMode int
    WriteMode 文件写入模式。

const (
	// AsyncWriteMode 异步批量模式（默认推荐）。业务协程无阻塞，后台批量刷盘。
	AsyncWriteMode WriteMode = iota
	// SyncWriteMode 绝对同步模式。每次写入立即 fsync，强可靠性。
	SyncWriteMode
)
