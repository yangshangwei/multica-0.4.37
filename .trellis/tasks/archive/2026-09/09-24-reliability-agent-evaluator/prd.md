# 可靠性与智能体评测核心角色

## Goal

补齐运行可靠性与 Agent 变更质量的两个独立责任面，让“系统没坏”和“Agent 没漂移”都能用证据验收。

## Requirements

- 新增 `reliability-engineer` 角色及一个核心 role skill，负责 SLI/SLO、错误预算、容量/队列、降级和恢复演练证据；不直接执行未审批生产操作。
- 新增 `agent-evaluator` 角色及一个核心 role skill，负责 Agent/Skill/MCP 版本化评测集、工具调用轨迹、正确性、安全性、成本、延迟和漂移报告；不替代代码审查或发布审批。
- 两个角色必须有独立交付物、默认 autonomy、最大并发数、权限说明、四语 picker 文案和可复制模板。
- 评测结果必须可复现并区分模型变化、工具变化、提示词变化和数据变化；缺少基线时标记 unknown。
- 可靠性角色输出运行证据和改进任务，Agent 评测角色输出评测结论和门禁建议；生产变更仍由 release engineer 在人工审批下执行。

## Acceptance Criteria

- [x] 两个角色能从内置模板创建，skill 自动附着，权限、autonomy、本地化和既有 workspace copy 保护有测试。
- [x] 评测模板至少覆盖成功、工具失败、越权请求、成本/延迟超阈值和模型/工具漂移五类情况。
- [x] 可靠性演练至少覆盖一个 SLO 退化、一个队列堆积或依赖故障、一个恢复验证，并报告量化信号。
- [x] 角色不按 frontend/backend/mobile 拆分，且与现有 QA、security、release、diagnostician 的职责边界写入文档。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
