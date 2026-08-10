# Changelog

本项目遵循语义化版本（SemVer）。值得记录的变更统一维护在此文件。

## [v1.3.4] - 2026-08-10

### 变更

- 依赖升级：errx v1.5.6、testx v1.4.4（go get -u -t all）。

## [v1.3.3] - 2026-08-10

### 变更

- 家族正式基线锁定：依赖统一指向 v1 基线已发布版本（errx v1.5.5 / logx v1.3.2 / testx v1.4.3 / validx v1.2.4 / cryptox v1.0.2 / confx v1.0.2 / webx v1.5.4 等），此后家族依赖不再前进。

### 质量

- 全部库包语句覆盖率保持 100%；race / vet / staticcheck / fuzz / govulncheck 全绿。

## [v1.3.2] - 2026-08-10

### 变更

- 家族依赖最终对齐到 v1 正式版基线（errx v1.5.4 / logx v1.3.1 / testx v1.4.2 / validx v1.2.3 / confx v1.0.1 / cryptox v1.0.1 等），无 API 变更。

### 质量

- 全部库包语句覆盖率保持 100%；race / vet / staticcheck / fuzz / govulncheck 全绿。

## [v1.3.1] - 2026-08-10

### 变更

- 家族依赖统一对齐到最新基线（errx v1.5.4 / logx v1.3.0 / testx v1.4.1 / validx v1.2.2 等），无 API 变更。

### 质量

- 全部库包语句覆盖率保持 100%；race / vet / staticcheck / fuzz / govulncheck 全绿。

## [v1.3.0] - 2026-08-10

### 变更

- 错误统一 errx 化：新增 CodeInvalidConfig / CodeIOFailed / CodeClosed，Builder、文件输出器与内部错误上报全部走 errx 结构化错误；errx 升级为直接生产依赖。

### 质量

- race / vet / staticcheck / govulncheck 全绿；覆盖率维持基线。

## [v1.2.5] - 2026-08-10

### 变更

- go 指令与 CI/Release 工作流统一为 Go 1.26.5；
- README Go 版本徽章同步更新。

## [v1.2.4] - 2026-08-10

### 变更

- go 指令与 CI/Release 工作流统一为精确版本 `1.21.0`；
- 依赖升级 `testx v1.2.3`（testx 同步为 1.21.0）；
- 全部示例子模块 go 指令同步 1.21.0 并 tidy 对齐。

## [v1.2.3] - 2026-08-10

### 变更

- 家族统一 Go 1.21：全部 go.mod 与 CI/Release 工作流版本号对齐 1.21。

## [v1.2.2] - 2026-08-10

### 修复

- 依赖升级 `testx v1.2.1` 后 go 指令正式恢复 `go 1.21`
  （此前被 v1.2.0 的 go 1.26.5 门槛顶回），CI Go 1.21 矩阵可用。

## [v1.2.1] - 2026-08-10

### 修复

- go.mod 的 go 指令恢复 `go 1.21`（testx v1.2.1 已同步降级），
  修复 CI Go 1.21 矩阵 staticcheck 失败；
- 全部示例模块 go.mod 与最新依赖对齐（go mod tidy）。

## [v1.2.0] - 2026-08-10

### 变更

- 家族测试底座接入：全部测试改用语义等价的 testx 断言
  （含 Require* 致命断言）；
- 测试依赖新增 `testx v1.2.0`，errx 同步升级 v1.4.0。

### 质量

- 根包语句覆盖率 100%；race / vet / staticcheck 全绿。

## [v1.1.0] - 2026-08-10

### 新增

- `Builder.WithMetrics(MetricSink)`：外置指标接收器（metricsx 等家族
  底座天然满足），日志记录、文件写入/字节量、轮转、压缩、清理事件
  全部转发；未注入时仅保留内部 `Metrics()` 快照，零额外开销；
- 派生 logger（`WithField` / `WithContext`）自动继承指标接收器。

### 质量

- 新增外部指标转发测试（含无接收器降级）；覆盖率保持 100%，
  race / vet / staticcheck 全绿。

## [v1.0.0] - 2026-08-08

### 破坏性变更

- 移除 `WithOnDropped` 与 `Metrics.Drops`：异步写入采用有界背压后日志不会丢弃，原回调永不触发，属失效 API；
- **API 冻结**：自本版本起，破坏性变更必须提升主版本号（v2.0.0）。

### 工程

- 移除调试用对照基准 `BenchmarkLogxAsyncFileConst`；
- 工业化文档集入库（架构/性能/运维/安全/质量/发布）。

## [v0.12.0] - 2026-08-08

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
