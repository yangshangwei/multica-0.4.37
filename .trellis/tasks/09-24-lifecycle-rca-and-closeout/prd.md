# RCA 闭环与双闭环最终验收

## Goal

继续推进研发交付与 Agent 质量双闭环，补齐 RCA bug-fix/maintenance/incident 的真实 API/E2E 验收、RCA 到修复交接证据，并完成两条闭环的逐项完成审计。

## Requirements

- 为 `bug-fix`、`maintenance`、`incident` 补齐真实 lifecycle handoff 的证据检查：未知原因进入诊断，已知原因直达修复必须有 bypass reason，incident 必须先止血并创建独立 RCA 后续。
- 让 RCA 交接可被修复任务消费：支持脱敏的 `diagnosis_ref`、`regression_test`、结论/证据/未知项，并将其写入下游修复 issue 的生命周期 metadata；不提交正式修复，不改变生产权限边界。
- 增加真实 API 浏览器验收，创建 `diagnostician` 并验证默认 `multica-debugging`、Contributor、并发上限、localized copy，以及已有 workspace agent/skill 副本不会被覆盖。
- 复核 incident-learning、rollout/canary、Agent evaluation、治理 handoff 和有限历史，补一份逐项完成审计；任何真实模型、生产 observability 或生产操作仍必须标记为未授权/未执行。
- 保持现有 issue/comment、metadata、task queue 和 dedup 语义；不新增表、凭据型 MCP、通用 coordinator 或技术栈角色。

## Acceptance Criteria

- [x] RCA 三条路由各有正向和负向 runtime 断言，且 incident mitigation-first 不被 RCA 阻塞。
- [x] 带 RCA 产物的下游修复 issue 可读回 `diagnosis_ref`、`regression_test`、结论/证据/未知项；重复提交不覆盖 workspace 自定义内容。
- [x] 浏览器 E2E 通过真实 API 创建 diagnostician，检查模板字段、skill/autonomy 和 workspace copy preservation。
- [x] 两条闭环的每个阶段都有代码、fixture、task/comment 或证据文件落点；缺证据仍返回 `unknown`/`hold`/关联既有任务。
- [x] 定向 Go、浏览器、TypeScript、`go vet`、Trellis validation 和 diff 检查通过；真实模型 smoke 和生产指标连接按授权限制记录为未测试。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
