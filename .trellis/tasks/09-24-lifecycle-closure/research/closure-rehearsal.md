# 双闭环产物映射演练

本文件不是成功声明，而是要求每个阶段指出实际产物落点；缺失处必须创建后续任务或标记 `unknown`。

## 研发交付闭环

| 阶段 | 输入 | 产物落点 | 当前状态 |
|---|---|---|---|
| 失败/分诊 | issue、复现信号、严重度 | bug-fix issue + QA 最小复现评论 | 已有模板规则，待 fixture |
| RCA | 脱敏时间线、失败样例、变更范围 | diagnostician 独立 issue/comment，结论 `confirmed/suspected/unknown` | 有 role/skill；fixture 的 RCA comment 被修复 issue 的 `diagnosis_ref` 消费 |
| 修复 | RCA 位置/机制/建议 | implementer 分支、回归测试、QA 报告 | 复用现有 feature/bug-fix 流程 |
| 发布 | 已审查候选 artifact | release issue、审批记录、rollback plan | 有 release skill，生产动作不执行 |
| 观测 | 同一 artifact digest、baseline、window | rollout/canary report：signals、decision、rollback outcome | 脱敏 fixture 已覆盖同 digest、观察结束、未获批不执行回滚及失败路径；缺真实指标接入 |
| 事故学习 | 恢复时间线、影响、缓解和未知 | incident-learning 后续 issue：owner、验收信号、关联事故 | 脱敏 fixture 已覆盖事实/推断/unknown、owner/验收信号和重复任务关联 |
| 防重发 | 预防任务完成和验收信号 | regression/gate task + 后续发布检查 | 只能静态演练，未有自动关联器 |

## Agent 质量闭环

| 阶段 | 输入 | 产物落点 | 当前状态 |
|---|---|---|---|
| Agent 变更 | Agent/Skill/MCP 版本差异 | change issue + case manifest | evaluator skill 已定义 |
| 评测 | 固定模型/工具/权限/脱敏数据 | 每 case 轨迹、停止原因、correctness/safety/cost/latency/drift | 脱敏 fixture 已覆盖六类 case 和缺轨迹 unknown；真实模型未授权 |
| 发布门禁 | evaluator + code/security/QA 结论 | review-gate/release issue verdict | squad route 已接入 |
| 发布 | 已批准 immutable artifact | release artifact + human approval | 生产动作不执行 |
| 漂移监控 | 同 artifact 的成本/延迟/安全/工具信号 | hold/rollback recommendation 或新增 case issue | 有规则，缺 observability 连接 |

## 演练规则

1. 任何没有实际输入或产物的格子不得标记通过。
2. 观察窗口未结束、artifact digest 不一致、缺 baseline、缺真实轨迹或重复事故无法关联时，输出 `unknown` 并创建补证据任务。
3. 创建任务/评论只使用本地脱敏 fixture；不调用生产 API，不读取凭据，不发送外部通知。
