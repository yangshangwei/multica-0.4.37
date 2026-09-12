# 变更日志服务与内网热更新 — 技术边界

采用父任务 [`design.md`](../09-12-desktop-changelog/design.md) 中评审后的契约，字段、发布来源与状态语义不得自行分叉。

写入边界：

- `server/internal/changelog/`
- `server/internal/handler/changelog.go`
- `server/internal/handler/changelog_test.go`

关联既有文件仅按父计划分配给执行者，跨边界修改先告知主执行者。

依赖：父方案完成评审后开始；UI 和发布器可以在契约确定后与服务端并行实现，最后由主任务集成验收。
