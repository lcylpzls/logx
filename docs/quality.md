# 质量保证

## 1. 质量门禁（硬性）

| 门禁 | 要求 |
| --- | --- |
| 单元测试 | `go test -count=1 ./...` 全绿 |
| 覆盖率 | 语句覆盖率 100%（`go tool cover -func coverage.out`） |
| 静态检查 | `go vet ./...`、`staticcheck ./...` 零告警 |
| 竞态检测 | Linux 上 `go test -race -count=1 ./...` 全绿 |
| 示例构建 | 全部 `examples/*` 模块 `go build ./...` 通过 |

## 2. CI 矩阵（.github/workflows/ci.yml）

- 操作系统：Ubuntu / Windows / macOS；
- Go 版本：1.21.x 与 1.26.x；
- 步骤：vet → staticcheck → test → race（仅 Linux）→ coverage（仅 Linux）→ 上传覆盖率产物；
- 单独 job 构建全部示例模块。

## 3. 测试体系

- **组件拆分**：`logx_test.go`（核心）、`file_appender_test.go`、`encoder_test.go`、`extensions_test.go`、`features_test.go`；
- **表驱动**：级别、字段、编码器、配置项；
- **并发测试**：并发写入、动态级别热更新、并发关闭；
- **故障注入**：只读目录、写失败、轮转失败、清理失败、压缩失败、槽位池满、超长日志；
- **Fuzz**：`FuzzTextEncoder_Encode`、`FuzzJSONEncoder_Encode`；
- **死锁防护**：异步背压、槽位归还非阻塞、Close 幂等均有专项测试。

## 4. 性能回归

- `examples/bench_compare`：与 Zap / Logrus 的端到端对比（io.Discard、文件异步、文件同步）；
- 性能声明必须以本仓库基准命令复现，并注明环境；
- 0 allocs/op 是硬性承诺：新增代码路径不得引入热路径分配。

## 5. 变更流程

- 代码变更必须同步更新 README、docs、CHANGELOG；
- 提交使用 Conventional Commits（中文描述）：`feat` / `fix` / `perf` / `style` / `docs` / `chore`；
- 发版前按 `release.md` 检查清单执行。
