# 生产接入与运维

## 1. 快速接入

```go
logger, err := logx.NewBuilder().
    WithCaller().
    EnableConsole(logx.InfoLevel, logx.WithColor()).
    EnableFileLog(
        logx.WithLogDir("/var/log/myapp"),
        logx.WithFilename("app.log"),
        logx.WithMaxSize(100),          // 100MB 切割
        logx.WithMaxAge(180),           // 保留 180 天
        logx.WithMaxBackups(100),       // 最多 100 个历史文件
        logx.WithCompressAfter(7),      // 7 天后 gzip
        logx.WithWriteMode(logx.AsyncWriteMode),
        logx.WithBufferSize(4096),      // 槽位数（预分配 16MB）
        logx.WithFlushInterval(time.Second),
        logx.WithLevels(logx.InfoLevel, logx.ErrorLevel),
        logx.WithErrorHandler(func(err error) { /* 告警 */ }),
        logx.WithOnDropped(func() { /* 预留，背压模式不触发 */ }),
    ).
    Build()
```

## 2. 配置项全表

| 配置 | 默认 | 说明 |
| --- | --- | --- |
| `WithLogDir` | 必填 | 日志目录，自动创建 |
| `WithFilename` | 必填 | 基础文件名 |
| `WithMaxSize` | 100 MB | 单文件容量阈值（原子边界预判） |
| `WithMaxAge` | 180 天 | 过期清理 |
| `WithMaxBackups` | 100 | 保留历史文件数 |
| `WithCompressAfter` | 0（不压缩） | N 天后 gzip 压缩 |
| `WithWriteMode` | Async | 异步批量 / 同步直写 |
| `WithBufferSize` | 4096 | 异步槽位数（内存 = × 4KB） |
| `WithFlushInterval` | 1s | 异步批量刷盘间隔 |
| `WithLevels` | 无（静默） | 文件通道启用级别 |
| `WithErrorHandler` | stderr | 内部错误统一出口 |
| `WithOnDropped` | 无 | 丢弃回调（背压模式下不触发） |
| `WithSampling` | 关闭 | 每秒最多 N 条 |
| `WithRedact` | 关闭 | 指定 Key 自动脱敏 |
| `WithEncoder` | 文本 | 切换 JSON 等编码器 |
| `WithCaller` | 关闭 | 调用者文件:行号 |

## 3. 生产建议

- **级别策略**：线上 Info 起步，排障时用 `LevelUpdater.SetLevel(DebugLevel)` 热切换，无需重启；
- **可靠性分级**：审计/金融链路用 `SyncWriteMode`；普通高吞吐用异步；
- **监控**：通过 `MetricProvider.Metrics()` 采集 Writes / WriteBytes / Rotations / Compressions / Cleanups；
- **告警**：`WithErrorHandler` 接入轮转失败、写入失败、清理失败；
- **脱敏**：`WithRedact("password", "token", "phone")`，敏感字段在编码前替换为 `***`；
- **风暴保护**：`WithSampling(1000)` 限制每秒输出量。

## 4. 优雅退出

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
<-ctx.Done()
logger.Sync()  // 异步模式先刷盘
logger.Close()
```

或使用 `logger.SafeExit(func() { os.Exit(1) })`：先清空缓存再执行退出函数。
`Fatal` / `Panic` 级别已内置刷盘。

## 5. 容量规划

- 内存：每异步实例约 `BufferSize × 4KB`（默认 16MB）+ 2MB Buffer 池（进程级）+ Entry 池；
- 磁盘：`MaxSize × (MaxBackups + 当前文件)` 上限；`CompressAfter` 降低冷数据占用；
- goroutine：每异步实例 2 个后台协程（刷盘 + 生命周期），进程级 1 个时间缓存协程；
- Hook：每条日志每 Hook 一个 goroutine，按 Hook 耗时估算。

## 6. 故障排查

| 现象 | 排查方向 |
| --- | --- |
| 异步日志延迟高 | 检查写盘速度、`FlushInterval`、磁盘 IO；有界背压会传导到业务 |
| 日志丢失 | 异步模式进程崩溃/断电会丢未落盘数据；关键链路改同步 |
| 写入失败 | 检查 `WithErrorHandler` 回调、磁盘满、权限 |
| 轮转不生效 | `MaxSize` 单位是 MB；确认配置生效 |
| 性能回退 | 字段数是否 >8、是否使用 `Any`、GC 后池重建 |
| 时间戳不准 | 时间缓存精度约 100ms，属设计行为 |
