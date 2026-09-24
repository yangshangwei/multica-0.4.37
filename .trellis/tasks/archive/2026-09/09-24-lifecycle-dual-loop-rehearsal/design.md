# 技术设计

## 边界

复用现有 `CreateLifecycleHandoff` API、service validator、IssueService 和 metadata 存储，不新增数据库表、实体类型、MCP 或生产控制面。测试只使用本地数据库 fixture 和脱敏 issue。

## 连续演练

研发闭环使用一个 source issue：

1. `rca`（known bug-fix）创建修复 follow-up，并写入 RCA 证据。
2. `rollout` 使用修复 artifact，验证 digest、baseline、观察窗口和健康信号。
3. `incident-learning` 写入事实/推断/未知，并创建带 owner 与 acceptance signal 的 prevention issue。

Agent 闭环复用同一个 source issue：

1. `agent-evaluation` 提交六类版本化 case，并记录 immutable artifact digest。
2. `rollout` 使用相同 digest，验证评测产物与发布产物未漂移。
3. 读取 history，断言顺序、source issue 和 digest 可追踪。

## 审计策略

使用现有 `task.py list` 和任务 JSON 只读审计父任务及历史子任务。发现仍处于 planning/in_progress 的任务时，记录为未完成证据，不擅自归档或把父任务标记为 completed。
