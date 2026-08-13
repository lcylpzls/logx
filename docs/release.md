# 版本与发布

## 1. 版本策略

- 遵循语义化版本（SemVer）：`vMAJOR.MINOR.PATCH`；
- 家族约定：v1 之后破坏性变更统一走 minor 版本（不强制主版本升级），
  直至另行调整版本规范；
- 升级前查看 CHANGELOG。

## 2. 发布流程

```powershell
# 1) 确保 main 分支 CI 全绿
git push origin main

# 2) 更新 CHANGELOG：将 [Unreleased] 改为 [vX.Y.Z] - 日期

# 3) 提交定版
git add CHANGELOG.md && git commit -m "chore(release): 定版 vX.Y.Z"

# 4) 打 tag 并推送
git tag vX.Y.Z
git push origin vX.Y.Z
```

tag 推送后，`.github/workflows/release.yml` 自动执行：

1. `go test -v -race ./...`（Ubuntu）；
2. 通过后创建 GitHub Release（自动生成 release notes，`make_latest`）。

## 3. 发版检查清单

- [ ] `go test -count=1 ./...` 通过；
- [ ] `go vet ./...`、`staticcheck ./...` 零告警；
- [ ] 覆盖率 100%（`go tool cover -func coverage.out`）；
- [ ] GitHub CI 三平台矩阵全绿（含 Linux race）；
- [ ] 全部示例 `go build ./...` 通过；
- [ ] CHANGELOG 已定版并记录破坏性变更；
- [ ] tag 推送后 Release 工作流成功；
- [ ] 在临时模块验证 `go get github.com/lcylpzls/logx@vX.Y.Z`；
- [ ] 性能声明如有更新，已同步 `performance.md` 与 README。

## 4. 历史版本

| 版本 | 说明 |
| --- | --- |
| v1.5.2 | 新增 NewNopLogger（no-op Logger） |
| v1.5.1 | 文档同步与历史清理（纯文档/版本元数据变更） |
| v1.5.0 | 主体下沉 internal/core、根包薄转发；依赖升级 errx v1.6.0 |
| v1.4.x | 家族依赖对齐与指标外置 |
| v1.0.0 | API 冻结：移除失效的 `WithOnDropped` / `Metrics.Drops`，工业化基线 |
| v0.12.0 | 工业化基线（历史）：全链路零分配、Field 强类型化、有界背压、CI 三平台验证 |
| v0.11.0 / v0.10.0 | 早期版本（旧 API，CI 未验证通过） |

## 5. 发布后

- 检查 Release 页面与 `go get` 安装；
- 破坏性变更必须在 CHANGELOG 中显著标注；
- 如发布失败，删除/重打 tag 后重新推送（release.yml 幂等）。
