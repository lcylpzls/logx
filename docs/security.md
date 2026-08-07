# 安全

## 1. 依赖与供应链

- 根模块**零第三方依赖**，仅使用 Go 标准库；
- 无 `go.sum` 依赖图需要审计（`examples/bench_compare` 的 Zap/Logrus 仅为基准测试，独立模块）；
- 发布产物由 GitHub Actions 从 tag 构建，Release 自动生成。

## 2. 敏感数据

- 使用 `WithRedact("password", "token", ...)` 在编码前替换敏感字段，Lazy 字段直接跳过求值，避免敏感值进入内存；
- 注意：脱敏按 Key 精确匹配；未配置的 Key 或消息正文中的明文不会被处理；
- 错误对象（`logx.Err`）会输出 `error.Error()` 文本，确认不携带凭据。

## 3. 审计与合规

- 审计类日志必须使用 `SyncWriteMode`：调用返回即落盘，进程直接退出不丢；
- 审计链路建议同时启用 `WithCaller()` 与结构化字段，便于溯源；
- 日志目录权限：确保运行账户对日志目录拥有最小必要权限。

## 4. 已知边界

- 内部错误未配置回调时输出到 stderr，错误风暴会刷屏（建议始终配置 `WithErrorHandler`）；
- 异步模式在进程崩溃/断电时可能丢失未落盘日志，不适用于强审计场景；
- Hook 在异步 goroutine 中执行，Hook 内的敏感操作需要自行防护。

## 5. 漏洞报告

请遵循仓库根目录 `SECURITY.md` 的流程：邮件联系维护者（标题注明 `[Security]`），不要在公开 issue 中披露细节。
