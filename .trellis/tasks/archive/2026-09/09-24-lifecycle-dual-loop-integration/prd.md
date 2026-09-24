# 双闭环最终集成与验收

## Goal

按 RCA、事故学习与发布后验证、核心角色、工作量角色和治理能力顺序，完成两条生命周期闭环的运行时集成、证据核验和最终验收。

## Requirements

- 先核对并保留 `bug-fix`、`maintenance`、`incident` 的诊断路由；incident 必须先缓解，RCA 独立后置。
- incident-learning 和 rollout/canary handoff 必须继续复用现有 issue、评论和 task queue，支持重复任务关联、同 digest 观察、unknown/hold/rollback recommendation。
- reliability-engineer、agent-evaluator、experience-validation-engineer、migration-reviewer 的 roster、skill、squad 路由和 workload gate 证据必须可追溯。
- 增加统一治理 handoff 契约，覆盖契约兼容、威胁建模、供应链、产品结果和灾备；每类能力必须有必需信号，缺失为 `unknown`，明确失败为 `hold`，全部通过才是 `pass`。
- 源 issue 的生命周期 handoff 不能互相覆盖；保留最新值兼容读取，同时保存有界历史，便于跨阶段审计和防重发复核。
- 不新增数据库表、凭据型 MCP、生产发布/回滚/迁移执行器或真实模型调用；生产动作继续由人工审批边界控制。

## Acceptance Criteria

- [x] RCA 路由的定向模板/运行时测试通过，且 incident mitigation-first 仍有负向测试。
- [x] 事故学习、rollout/canary 和 Agent evaluation 的 handoff 在真实本地 issue 上可写回证据并保持幂等/失败关闭。
- [x] 五类治理能力有 service validator、handler handoff 和正负向 fixture/test；未知与 hold 不得宣称通过。
- [x] 同一源 issue 连续记录至少两个阶段后，最新 handoff 和有界历史都可读回，旧证据不丢失。
- [x] 脱敏双闭环演练覆盖研发闭环和 Agent 闭环，并明确未授权真实模型/生产信号的边界。
- [x] 定向 Go tests、`go vet`、`pnpm typecheck`、Trellis context validation、`git diff --check` 通过；完整 handler fixture 失败单独记录。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
