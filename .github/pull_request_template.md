## 变更说明

（简述本次变更解决了什么问题、如何实现）

## 关联项

- Closes #
- 对应计划条目：`docs/开发计划文档.md` 中的阶段/任务编号

## 验证

- [ ] `go vet ./...` 通过
- [ ] `staticcheck ./...` 通过
- [ ] `go test -race -coverprofile=coverage.out ./...` 通过，覆盖率 100%
- [ ] 涉及性能时附 `go test -bench=. -benchmem ./...` 对比数据

## 兼容性

（是否破坏现有 API；是否需要同步更新 README 与示例）
