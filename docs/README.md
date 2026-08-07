# logx 工业化文档

logx 是工业级、零依赖、全链路零分配的高性能 Go 结构化日志库（模块 `github.com/lcylpzls/logx`）。
本目录是随仓库维护的正式文档集，与代码同步更新。

## 文档索引

| 文档 | 内容 | 适用读者 |
| --- | --- | --- |
| [architecture.md](architecture.md) | 总体架构、核心组件、数据流、零分配与并发模型 | 维护者、二次开发者 |
| [performance.md](performance.md) | 性能基准、竞品对比、零分配成本与调优 | 架构师、性能负责人 |
| [operations.md](operations.md) | 生产接入、配置项、监控、优雅退出、容量规划、故障排查 | 业务接入方、SRE |
| [security.md](security.md) | 依赖与供应链、敏感数据、审计、漏洞报告 | 安全负责人、接入方 |
| [quality.md](quality.md) | 质量门禁、测试体系、CI 矩阵、回归策略 | 维护者、QA |
| [release.md](release.md) | 版本策略、发布流程、CHANGELOG 规范、发版检查清单 | 维护者、发布负责人 |

## 建议阅读顺序

1. 接入使用：先读 `operations.md`；
2. 理解性能承诺：读 `performance.md`；
3. 理解实现机制：读 `architecture.md`；
4. 上线前：读 `security.md` 与 `quality.md`；
5. 负责发版：读 `release.md`。

## 文档维护约定

- 代码行为变更时，必须同步更新对应文档；
- 性能数字必须来自本仓库 `bench_compare` 的实测，标注环境与命令；
- 配置项、默认值、指标字段以本文档为权威描述。
