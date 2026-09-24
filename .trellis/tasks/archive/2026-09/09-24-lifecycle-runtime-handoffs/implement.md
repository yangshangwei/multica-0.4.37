# 实现计划

1. 为 request/response 建立 handler 边界类型，复用 `ValidateRCARoute`、`ValidateIncidentLearning`、`EvaluateRolloutEvidence` 和 `ValidateAgentEvaluation`。
2. 实现后续 issue 创建/复用、metadata 写回、审计评论和 task queue 触发。
3. 注册 issue 子路由，补 unknown/hold/duplicate/跨 workspace 失败测试。
4. 运行定向 Go tests、`go vet`、Trellis validate 和 diff 检查。
