# 实施计划

1. 为 service 增加五类治理 capability 的 fail-closed validator 和单元/fixture 测试。
2. 为 lifecycle handler 增加 `governance` 请求分支，复用现有 metadata/comment/follow-up 语义。
3. 将 handoff metadata 写入改为 latest + 有界 history，补跨阶段连续写入和历史保留测试。
4. 复核 RCA、incident-learning、rollout、Agent evaluator、四个角色和 squad 路由的既有测试证据，补一份脱敏双闭环 rehearsal。
5. 运行定向 Go tests、`go vet`、`pnpm typecheck`、Trellis validation 和 diff 检查；记录完整 handler suite 的既有 fixture 限制。
6. 更新本任务 evidence，提交只包含本轮实现和任务材料的 Lore commit，保留其它用户改动不变。
