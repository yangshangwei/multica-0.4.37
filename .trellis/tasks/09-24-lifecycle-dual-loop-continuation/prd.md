# 生命周期双闭环续作

## Goal

把已经存在的生命周期契约继续收口为可重放的运行时闭环：事故学习可以链接既有预防工作，发布后验证保存完整观察窗口，Agent 评测绑定不可变评测产物，并且任何复用的后续 issue 都必须仍然属于源 issue 的关系链。

## Requirements

- 保持现有 workspace 权限、agent autonomy、issue service、task queue 和人工生产操作边界。
- incident-learning 支持“引用已有预防任务”与“创建/复用预防任务”两条路径；证据不足仍返回 unknown/错误，不创建无主副本。
- rollout/canary 记录 `observation_window.started_at`、`ended_at` 和完成状态；窗口缺失、反向或未完成时只能返回 unknown。
- agent-evaluation 记录不可变评测 artifact digest，并要求版本、digest 和六类最小 case 同时存在；缺证据不得通过。
- `follow_up_issue_id` 复用路径必须校验目标 issue 与源 issue 的 parent 关系和 workspace 边界；跨 workspace 或无关联目标拒绝。
- 每条运行时交接继续写回源 issue metadata 和审计评论，重复请求保持幂等，不能执行真实发布、回滚、迁移或模型调用。

## Acceptance Criteria

- [ ] 既有 prevention issue 可以被引用并记录，不会因为没有新 prevention 对象而失败或复制。
- [ ] rollout 的完整窗口、摘要和回滚结论可从源 issue metadata 读回，窗口无效时为 unknown。
- [ ] Agent evaluation 的 artifact digest 被保存，缺失或变更会阻止 pass。
- [ ] 已有关联 follow-up 可复用；同 workspace 但无 parent 关联、跨 workspace 和不存在目标均被拒绝。
- [ ] 定向 handler/service 测试、`go vet`、Trellis validate 和 `git diff --check` 通过；既有 fixture 环境失败单独记录。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
