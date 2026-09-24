# 双闭环跨阶段运行时演练与父任务审计

## Goal

在同一脱敏 issue 上连续验证研发交付闭环和 Agent 质量闭环的 handoff/history/artifact 关联，并审计父任务与历史子任务的完成状态。

## Requirements

- 在同一个脱敏 source issue 上连续提交研发闭环 handoff：RCA、rollout、incident-learning，并验证 prevention issue 回链到 source issue。
- 在同一个 source issue 上连续提交 Agent 闭环 handoff：agent-evaluation、rollout；两阶段必须携带同一个 immutable `artifact_digest`。
- 查询 issue metadata，验证 `lifecycle_handoff` 始终是最新阶段，`lifecycle_handoff_history` 保留阶段顺序、source issue、决策和 artifact digest。
- 覆盖至少一个 unknown/hold 结果，证明缺失或不一致证据不会被误判为通过。
- 审计父任务及其历史子任务状态和验收证据；不得通过改写历史任务状态来掩盖未完成工作。
- 不执行真实模型、生产发布、回滚、迁移、恢复、通知或读取生产数据；所有输入必须是脱敏 fixture。

## Acceptance Criteria

- [x] 研发闭环测试通过：RCA -> rollout -> incident-learning，prevention issue 带 owner、acceptance signal 和 source incident 关联。
- [x] Agent 闭环测试通过：agent-evaluation -> 同 digest rollout，历史阶段顺序和 digest 一致，评测与发布决策均可读。
- [x] history 的每条记录保留 `source_issue_id`，最新记录与 history 尾项一致，且 history 上限逻辑未回归。
- [x] unknown/hold 分支通过，且没有产生未经证据支持的成功决策或生产动作。
- [x] 父任务审计记录 active/planning/in-progress 子任务及其证据缺口；父任务保持 planning，直到所有验收项有证据。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
