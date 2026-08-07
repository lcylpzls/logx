# 性能与零分配

## 1. 基准环境与命令

实测环境：Windows 10 22H2 / AMD Ryzen 5 7600（6C12T）/ Go 1.26.5 amd64。

```bash
# logx 内部微基准
go test -bench=. -benchmem -benchtime=2s ./...

# 竞品对比与文件路径（io.Discard / 文件写入）
cd examples/bench_compare && go test -bench=. -benchmem -benchtime=2s ./...

# 文件写入吞吐量（1 秒压测）
cd examples/bench && go run .
```

## 2. 微基准（变量字段，非优化常量）

| 场景 | 耗时 | 内存 |
|---|---:|---:|
| TextEncoder 简单日志 | 22.1 ns/op | 0 B / 0 allocs |
| TextEncoder 含 3 字段 | 54.2 ns/op | 0 B / 0 allocs |
| 端到端（io.Discard，3 字段） | 122.8 ns/op | 0 B / 0 allocs |
| 文件异步写入（3 字段） | 230.6 ns/op | 0 B / 0 allocs |
| 文件同步写入（3 字段） | 2927 ns/op | 0 allocs |
| 未启用级别过滤 | 55.9 ns/op | 0 B / 0 allocs |

## 3. 竞品对比（同消息、同 3 字段、纯文本、io.Discard）

| 库 | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| **logx** | **122.8** | **0** | **0** |
| Zap | 382 | 216 | 3 |
| Logrus | 1200 | 1190 | 16 |

## 4. 文件写入吞吐量

| 场景 | 条/s |
|---|---:|
| 文件异步写入 | 4,952,640 |
| 文件同步写入 | 370,622 |
| 文件异步 + 3 字段 | 3,787,414 |
| 未启用级别过滤 | 20,118,049 |

## 5. 零分配的边界

- 字段 ≤ 8 个：零堆分配；超过 8 个退化为按需分配（罕见路径）；
- `Any`/`Err`/`Lazy` 字段：走接口兜底，`Any` 由调用方值决定是否分配；
- `Infof` 系列：`fmt.Sprintf` 固有 2 次分配，与日志引擎无关；
- GC 后 `sync.Pool` 重建会偶发分配（B/op 平均约 1–8 B），非日志主路径；
- 同步文件路径写盘系统调用耗时为主，allocs 仍为 0。

## 6. 性能调优

| 手段 | 效果 |
| --- | --- |
| `WithWriteMode(SyncWriteMode)` | 强可靠，吞吐约 37 万条/s（受磁盘限制） |
| 默认 `AsyncWriteMode` | 吞吐约 495 万条/s，预占 `BufferSize × 4KB` 内存 |
| `WithBufferSize` | 调小降低内存（如 1024 → 4MB），调大降低背压概率 |
| `WithFlushInterval` | 调大减少刷盘次数、提升批量，但增加落盘延迟 |
| 字段数量 | ≤8 保持零分配；高频路径避免 `Any` 与超长消息 |
| `WithSampling` | 故障风暴时保护磁盘 IO 与业务 |

> 数字与机器环境强相关，发布性能声明时请在本机复跑并注明环境。
