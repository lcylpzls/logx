# logx

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.21-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-zero-brightgreen.svg)](go.mod)

**工业级 · 零依赖 · 极致性能 · 可插拔** 的 Go 语言结构化日志库。

logx 兼具 [Zap](https://github.com/uber-go/zap) 的极致性能与 [Logrus](https://github.com/sirupsen/logrus) 的易用性，通过链式 Builder API 实现业务与底层日志逻辑的完美解耦。纯标准库实现，不引入任何第三方依赖。

---

## 快速开始

### 安装

```bash
go get github.com/lcylpzls/logx
```

### 最小示例

```go
package main

import "github.com/lcylpzls/logx"

func main() {
    logger, _ := logx.NewBuilder().
        EnableConsole(logx.InfoLevel).
        Build()

    logger.Info("服务启动成功", logx.Int("port", 8080))
}
```

输出：

```
2026-06-06 15:04:05.000 INFO   服务启动成功  {port=8080}
```

---

## 核心特性

- ⚡ **零分配编码** — 2MB 缓冲池 + 预编码常量 + 时间缓存，单条日志 **0 B/op, 0 allocs**
- 🔌 **可插拔架构** — Encoder / Appender 接口完全解耦，支持自定义扩展
- 📝 **双模式 API** — 同时支持结构化字段（`Info("msg", logx.Int("k", v))`）和传统格式化（`Infof("k=%d", v)`）
- 🎨 **控制台色彩** — 按级别高亮（Debug 蓝 / Info 绿 / Warn 黄 / Error 红），落盘自动剥离
- 📁 **工业级文件管理** — 原子切割（大小+时间）、Symlink 软链接、Gzip 压缩、自动清理
- ⚙️ **双引擎写入** — 异步批量（高吞吐）与绝对同步（强可靠）随意切换
- 🛡️ **默认静默** — 所有通道/级别默认关闭，必须显式传入级别参数，避免隐式开销
- 🔗 **零外部依赖** — 纯 Go 标准库，不引入任何第三方模块
- 📍 **调用者追踪** — 一键 `WithCaller()` 定位日志源码位置，零性能开销

---

## 使用指南

### 1. 链式 Builder 配置

logx 强制使用 Builder 模式初始化，支持多通道精细化控制。**所有输出默认静默，必须显式开启。**

```go
logger, err := logx.NewBuilder().
    WithCaller(). // 开启调用者追踪
    // -------- 控制台通道 --------
    EnableConsole(logx.DebugLevel, logx.WithColor()).
    // -------- 文件通道 --------
    EnableFileLog(
        logx.WithLogDir("/var/log/myapp"),
        logx.WithFilename("app.log"),
        logx.WithMaxSize(100),       // 单文件最大 100MB
        logx.WithMaxAge(180),        // 保留 180 天
        logx.WithCompressAfter(7),   // 7 天后 gzip 压缩
        logx.WithWriteMode(logx.AsyncWriteMode),
        logx.WithLevels(logx.InfoLevel),
    ).
    Build()
if err != nil {
    panic(err)
}
defer logger.Close()
```

### 2. 日志输出

```go
// 结构化 API（零反射，推荐）
logger.Info("用户登录", logx.String("user", "admin"), logx.Int("attempt", 3))
logger.Debug("SQL 查询", logx.String("sql", "SELECT * FROM users"), logx.Int64("elapsed_ms", 12))
logger.Error("连接失败", logx.Err(err))

// 格式化 API
logger.Infof("第 %d 次重试", retry)
logger.Errorf("处理失败：%v", err)

// 特殊级别
logger.Panic("不可恢复的错误")  // 刷盘后触发 panic
logger.Fatal("致命错误")       // 刷盘后 os.Exit(1)
```

### 3. 结构化字段

```go
logx.String("key", "value")      // 字符串
logx.Int("count", 42)            // int
logx.Int64("id", 123456789)      // int64
logx.Bool("enabled", true)       // 布尔
logx.Err(err)                    // 错误（key 固定为 "error"）
logx.Any("data", myStruct)       // 任意类型（后备方案）
```

### 4. 延迟求值 Lazy

当 Debug 未开启时，避免执行昂贵的计算：

```go
logger.Debug("用户详情", logx.Lazy("info", func() any {
    return expensiveDBQuery()  // 仅在 Debug 启用时才执行
}))
```

### 5. 派生 Logger

```go
// 携带链路上下文
ctx := context.WithValue(context.Background(), "trace_id", "abc123")
traceLogger := logger.WithContext(ctx)

// 固定字段
userLogger := logger.WithField("user_id", "10086")
userLogger.Info("操作成功") // 自动携带 user_id=10086
```

### 6. 写入模式

| 模式 | 吞吐 | 可靠性 | 适用场景 |
|------|:--:|:--:|------|
| `AsyncWriteMode`（默认） | 极高 | 进程崩溃可能丢失少量 | 高并发 API、微服务 |
| `SyncWriteMode` | 受限于磁盘 | 强保证，不会丢失 | 金融、审计、核心事务 |

```go
EnableFileLog(
    logx.WithWriteMode(logx.SyncWriteMode),
    logx.WithLevels(logx.InfoLevel),
)
```

### 7. 优雅退出 SafeExit

在异步模式下，`os.Exit` 会导致缓冲日志丢失。使用 `SafeExit` 确保临终日志落盘：

```go
// ❌ 错误：异步日志会丢失
os.Exit(1)

// ✅ 正确：先刷盘再退出
logger.SafeExit(func() {
    os.Exit(1)
})
```

`Fatal` 和 `Panic` 级别内部已集成刷盘逻辑，无需额外处理。

### 8. 调用者追踪 WithCaller

开启后自动在日志中追加源文件名和行号：

```go
logger, _ := logx.NewBuilder().
    WithCaller().                    // 开启调用者追踪
    EnableFileLog(
        logx.WithLogDir("./logs"),
        logx.WithFilename("app.log"),
        logx.WithLevels(logx.InfoLevel),
    ).
    Build()

logger.Info("服务启动")
// 输出：2026-06-06 15:04:05.000 INFO  main.go:15  服务启动
```

路径格式为 Zap `TrimmedPath()` 同款——保留包目录/文件名两级，零额外内存分配。

### 9. 劫持标准库 log

一行代码将老项目中所有的 `log.Println` 路由到 logx：

```go
logx.ReplaceStdLogger(logger)
defer logx.RestoreStdLogger()

// 标准库调用自动流经 logx 引擎
log.Println("这条日志由 logx 接管")
```

### 10. Hook 扩展

注册 Hook 实现告警、监控等自定义逻辑：

```go
type AlertHook struct{}

func (h *AlertHook) OnLog(e *logx.Entry) {
    if e.Level >= logx.ErrorLevel {
        sendAlert(e.Message) // 飞书/钉钉/邮件...
    }
}

logger.(logx.HookedLogger).AddHook(&AlertHook{})
```

Hook 以异步 goroutine 执行，内置 panic recovery，绝不阻塞日志主路径。

---

## 性能基准

> 测试环境：Go 1.21+, 单条日志含时间 + 级别 + 消息 + 3 个字段

### 微基准

| 指标 | logx | 说明 |
|------|:--:|------|
| TextEncoder 简单日志 | **18.6 ns/op, 0 B, 0 allocs** | 零分配纯字节拼接 |
| TextEncoder 含字段 | **40.5 ns/op, 0 B, 0 allocs** | 无反射类型分发 |
| 未启用级别过滤 | **52.7 ns/op** | 仅级别判断开销 |

### 吞吐量（`examples/bench`）

| 场景 | 吞吐量 |
|------|--:|
| 文件异步写入 | **4,003,544 条/s** |
| 文件同步写入 | 402,047 条/s |
| 文件异步 + 3 字段 | 2,669,233 条/s |
| 未启用级别过滤 | **24,793,070 条/s** |

---

## 设计原则

| 原则 | 说明 |
|------|------|
| **默认静默** | 所有通道和级别默认关闭，必须显式指定启用的日志级别 |
| **链式构建** | 通过 Builder 模式实现声明式、类型安全的配置 |
| **通道隔离** | Console / File 通道级别独立控制 |
| **零依赖** | 纯标准库，零外部 import |
| **绝不 panic** | 除 `Panic()` 级别方法外，库内部绝不允许 panic |
| **原子切割** | 单条日志绝不跨文件，切割前预判边界 |
| **并发安全** | 所有共享状态由 `sync.Mutex` 保护 |

---

## 日志格式

### 控制台（纯文本 + 色彩）

```
2026-06-06 15:04:05.000 INFO   服务启动成功  {port=8080}
2026-06-06 15:04:05.001 DEBUG  SQL 查询  {sql=SELECT ..., elapsed_ms=12}
2026-06-06 15:04:05.002 ERROR  连接失败  {error=connection refused}
```

### 控制台 + WithCaller

```
2026-06-06 15:04:05.000 INFO   main.go:15  服务启动成功  {port=8080}
2026-06-06 15:04:05.001 DEBUG  service/handler.go:42  SQL 查询  {sql=SELECT ...}
2026-06-06 15:04:05.002 ERROR  db/conn.go:108  连接失败  {error=connection refused}
```

### 文件（纯文本，无 ANSI）

```
2026-06-06 15:04:05.000 INFO   服务启动成功  {port=8080}
```

### 文件物理布局

```
/var/log/myapp/
├── app.log                           → app-2026-06-06T15-04-05.000.log  (Symlink)
├── app-2026-06-06T15-04-05.000.log   ← 当前写入文件
├── app-2026-06-05T00-00-00.000.log   ← 历史归档
├── app-2026-05-30T00-00-00.000.log.gz ← 已压缩归档
└── ...
```

---

## 项目结构

```
logx/
├── logger.go             # Logger 顶层接口
├── level.go              # 7 级日志级别
├── field.go              # 结构化字段 + Lazy 延迟求值
├── entry.go              # 日志条目
├── encoder.go            # Encoder 接口
├── appender.go           # Appender 接口
├── core.go               # 核心引擎（级别过滤 + 编码 + 写入）
├── builder.go            # Builder 链式构造器
├── text_encoder.go       # 零分配纯文本编码器
├── console_appender.go   # 控制台输出器（stdout/stderr 分流）
├── file_appender.go      # 文件输出器（同步/异步 + 轮转）
├── buffer.go             # 2MB sync.Pool 缓冲池
├── time_cache.go         # 100ms 时间缓存
├── color.go              # ANSI 色彩常量
├── stdlog.go             # 标准库 log 劫持
├── hook.go               # Hook 扩展接口
└── logx_test.go          # 测试与 Benchmark
```

---

## 示例

完整可运行的示例位于 [examples/](examples/) 目录：

| 示例 | 说明 |
|------|------|
| [basic](examples/basic/) | 最简控制台输出 |
| [file_output](examples/file_output/) | 文件输出 + 轮转 + 压缩 |
| [advanced](examples/advanced/) | 完整功能：双通道 + 色彩 + Hook |
| [stdlog](examples/stdlog/) | 劫持标准库 log |
| [bench](examples/bench/) | 压力测试：吞吐量测量 |

---

## License

MIT © [lcylpzls](https://github.com/lcylpzls)
