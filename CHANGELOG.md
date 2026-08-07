# Changelog

本项目遵循语义化版本（SemVer）。值得记录的变更统一维护在此文件。

## [Unreleased]

### 新增

- JSON 编码器：`NewJSONEncoder()` + `Builder.WithEncoder()`，对接 ELK/Loki 采集；
- 自定义 `io.Writer` 输出通道：`Builder.EnableWriter()`；
- 动态级别热更新：`LevelUpdater.SetLevel()` 可选接口；
- 按秒限流采样：`Builder.WithSampling()`；
- 敏感字段自动脱敏：`Builder.WithRedact()`；
- 运行指标：`MetricProvider.Metrics()`（写入/字节/丢弃/轮转/压缩/清理计数）；
- 性能：**全链路零分配**——`Field` 强类型化（消除变量装箱）+ `FieldGroup` 内联容器 + `Entry` 池化 + 异步槽位复用，控制台/同步文件/异步文件路径均实测 `0 allocs/op`；
- 文件通道错误回调与异步丢弃回调：`WithErrorHandler()` / `WithOnDropped()`；
- 示例：`examples/graceful_shutdown`（信号优雅退出）、`examples/bench_compare`（Zap/Logrus 对比基准）；
- Fuzz 测试与故障注入测试，语句覆盖率 100%；
- CI 质量门禁：三平台矩阵执行 `go vet` / `staticcheck` / 测试 / 竞态 / 覆盖率。

### 修复

- Windows 下压缩完成后原文件删除失败（先关闭源文件再删除）；
- go.mod 与 CI、README 的 Go 版本不一致（统一最低版本 Go 1.21）；
- 异步槽位归还可能因空闲池满而阻塞（并发新建槽时）导致死锁，改为非阻塞归还。
- 异步写入由“非阻塞丢弃”改为**有界背压**（无空闲槽时等待归还），严格零分配、零丢弃。

### 破坏性变更

- 结构化日志方法参数由 `...Field` 改为 `FieldGroup`，构造方式为 `logx.Fields(field...)`；
- 异步文件通道启动时预分配 `BufferSize × 4KB` 槽位内存，换取运行时零分配。
- `Field` 改为强类型槽位存储（`Value` 接口保留作 Any/Err/Lazy 兜底），`Field{Key, Value}` 字面量仍可用。

### 工程

- 新增 MIT LICENSE；
- 测试文件按组件拆分；
- 发布脚本补充 PowerShell 版 `git_push.ps1`。
