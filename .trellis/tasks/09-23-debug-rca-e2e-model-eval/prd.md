# 浏览器 E2E 与真实模型 RCA 评测

## Goal

验证 diagnostician/multica-debugging 的浏览器模板创建链路与真实模型 RCA 行为，保留隔离证据和已知限制，并为研发交付双闭环提供可复用验收证据。

## Requirements

- 浏览器通过真实 API 创建 `diagnostician`，检查 localized picker 文案、Contributor autonomy、并发上限、默认 `multica-debugging` skill 和 workspace copy preservation。
- 覆盖可复现、间歇性/暂不可复现、证据不足三种故障输入；要求输出事实、假设、工具调用、停止条件、根因证据等级和交给实现工程师的方向。
- 真实模型运行只在显式授权环境执行，测试不得读取生产凭据、客户数据或执行生产操作；原始轨迹脱敏并记录模型/工具版本。
- 若浏览器、运行时或模型环境不可用，记录精确 blocker、已完成的替代验证和可重复的 follow-up，不把跳过写成通过。

## Acceptance Criteria

- [ ] 浏览器 E2E 真实创建 diagnostician，验证模板字段、skill、权限和已有 workspace 副本未被覆盖。
- [ ] 三类 RCA 样例均有实际模型或明确跳过证据；结论区分 confirmed/suspected/unknown，且不提交正式修复。
- [ ] 记录模型、工具、环境、输入摘要、工具调用、退出/停止原因和 authority boundary。
- [ ] 运行结果可供 `09-24-rca-loop-verification` 复用，未跟踪发布文件和凭据不纳入任务。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
