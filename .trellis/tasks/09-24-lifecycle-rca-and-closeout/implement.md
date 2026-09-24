# 实施计划

1. 先写 handler runtime 回归测试，证明 RCA 产物会写入下游修复 issue，确认当前实现失败。
2. 扩展 lifecycle handoff request、evidence 和 follow-up metadata 写入，保持 workspace 边界、dedup 和权限复用。
3. 先写真实 API 浏览器用例，再运行验证；复用现有隔离 fixture，不调用真实 Agent CLI。
4. 更新 RCA/双闭环证据文档和本任务验收清单，明确真实模型与生产 observability 的授权 blocker。
5. 运行定向 Go tests、Playwright、`pnpm typecheck`、`go vet`、Trellis validation、`git diff --check`，最后再提交和归档。
