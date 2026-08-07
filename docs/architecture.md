# 架构设计

## 1. 总体架构

```text
业务调用
   │  Logger 接口（Debug/Info/Warn/Error/Panic/Fatal + f 系列）
   ▼
logger.log ── 采样（WithSampling）→ 脱敏（WithRedact）→ 字段合并 → Entry（池化）
   │
   ▼
Core.write ── 级别过滤（原子 minLvl）→ Buffer（2MB sync.Pool）→ Encoder 编码
   │
   ▼
Appender.Append
   ├── ConsoleAppender：stdout/stderr 分流（Error 及以上 → stderr）
   ├── FileAppender：同步直写 / 异步槽位通道 + 批量刷盘 + 轮转压缩清理
   └── WriterAppender：任意 io.Writer
```

## 2. 核心组件

| 组件 | 职责 |
| --- | --- |
| `Logger` | 对外主接口：结构化 + 格式化双模式、上下文、生命周期 |
| `Builder` | 链式配置构造器，多通道独立级别，默认静默 |
| `Core` | 单通道引擎：级别过滤 → 编码 → 写入，持锁保护输出器 |
| `Encoder` | `TextEncoder`（纯文本零分配）、`JSONEncoder`（单行 JSON，手写无反射） |
| `Appender` | `ConsoleAppender` / `FileAppender` / `WriterAppender` |
| `Hook` | 异步拦截（goroutine + panic recovery），不阻塞主路径 |
| `Entry` | 日志条目载体，无 Hook 时对象池化复用 |

## 3. 零分配设计

全链路 0 allocs/op 由四层机制共同保证：

1. **Field 强类型槽位**：string/int/int64/bool 直接存类型化字段，不装箱；`Any`/`Err`/`Lazy` 走接口兜底；
2. **FieldGroup 内联容器**：≤8 个字段内联在结构体内（约 64B/字段），构造与传递零堆分配；
3. **对象池化**：`Entry`（无 Hook 时）与 `Buffer`（2MB）使用 `sync.Pool`，热路径不分配；
4. **异步槽位复用**：文件异步写入预分配 `BufferSize` 个槽位（默认 4096 × 4KB = 16MB），写盘后归还空闲池，运行时零分配、零拷贝；
5. **时间缓存**：后台协程每 100ms 预格式化时间字节，业务直接拷贝（精度约 100ms）；
6. **预编码常量**：级别名、ANSI 颜色码等硬编码为 `[]byte`。

## 4. 并发模型

| 区域 | 保护方式 |
| --- | --- |
| 通道写入 | `core.mu` 串行化 `Appender.Append` |
| 动态级别 | `core.minLvl` 为 `atomic.Uint32`，`LevelUpdater.SetLevel` 无锁热更新 |
| 文件写入与轮转 | `fileAppender.mu`；轮转前在锁内预判边界，保证单条日志原子 |
| 关闭 | `sync.Once` 幂等；`Close` 取消后台协程并 `wg.Wait` 等待退出 |
| 异步队列 | 双通道（写通道 + 空闲槽池）传递，**有界背压**：无槽时业务协程等待归还，零丢弃 |
| Hook | 每条日志每个 Hook 一个 goroutine，内置 recover；Hook 必须轻量 |

## 5. 文件管理

- 物理文件：`<basename>-2006-01-02T15-04-05.000.log`；
- 轮转：容量阈值（`当前大小 + 新日志 > MaxSize` 先轮转）+ 自然天；
- 软链：非 Windows 平台维护 `app.log → 当前物理文件`；
- 生命周期：`MaxAge` 清理、`MaxBackups` 保留数、`CompressAfter` 延迟 gzip 压缩，后台每 10 分钟扫描；
- 错误出口：所有内部错误统一走 `WithErrorHandler` 回调，未配置时降级 stderr。

## 6. 已知设计取舍

- 异步写入为有界背压（高水位阻塞），非“非阻塞丢弃”；
- Hook 异步 goroutine 无上限，Hook 慢会堆积；
- 时间缓存协程进程常驻，不可停止；
- Windows 不创建软链（权限限制），`-race` 需要 gcc（由 Linux CI 覆盖）。
