# 研发交付与 Agent 质量双闭环最终接线

## Goal

继续推进既有生命周期能力，直到“诊断 → 修复 → 发布 → 观测 → 复盘 → 防重发”和“Agent 变更 → 评测 → 发布 → 漂移监控”两条闭环都有可执行的阶段交接、独立责任和验证证据。

## Requirements

1. RCA 闭环：在 `bug-fix`、`maintenance`、`incident` 中分别验证未知原因、间歇性维护失败和事故恢复后的独立 RCA；已知原因直达实现必须记录跳过诊断的理由，事故止血不能等待 RCA。
2. 事故学习与发布后验证：让 `multica-incident-learning` 和 `multica-rollout-and-canary-verification` 的输入、输出、`unknown`、重复任务关联、同 artifact digest、观察窗口、回滚建议可被下游任务消费。
3. 核心角色：保持 `reliability-engineer` 和 `agent-evaluator` 的独立交付物、权限、skill 附着和 `incident`/`release`/`review-gate` 可达路径，并验证 workspace 自定义副本不被覆盖。
4. 专项角色：以至少两个真实发布/迁移周期的记录重算 workload gate。达标时新增 `experience-validation-engineer` 与/或 `migration-reviewer` 的完整 listed role、skill、本地化、权限、模板和负向/正向 roster 测试；未达标时保留明确的路由级能力、复评条件和不出现在默认 roster 的测试。
5. 治理能力：在既有角色和小队路由中形成契约兼容、威胁建模、供应链、产品结果、灾备的触发 → 输入 → 产物 → 验证 → 人工审批矩阵，并为每类能力保留至少一个失败证据。
6. 两条闭环演练：使用本地脱敏 issue/comment/评测 fixture 记录每个阶段的输入、产物、引用和停止条件；不得把静态 skill 文案当成运行时闭环完成证据。
7. MCP：继续只提供无凭据的浏览器/推理 MCP；Git/CI、Observability、Database MCP 只有满足来源版本、只读权限、凭据注入、脱敏和审计协议时才进入 catalog。

## Constraints

- 不读取生产凭据、客户原始数据，不执行未经批准的生产发布、回滚、迁移、恢复、通知或付费模型调用。
- 默认测试禁止调用用户安装的 Agent CLI；真实模型 smoke 只能在 `agentintegration` 与 `MULTICA_RUN_REAL_AGENT_SMOKE=1` 显式授权下运行。
- 不按 frontend/backend/mobile 拆角色；没有独立交付物不得新增 coordinator。
- 不增加 observability 数据库、事件总线或自动生产控制面；现有 issue/comment API 不足时记录最小后续设计。
- 保留既有 workspace agent/skill/squad 副本；模板更新只能影响新创建副本并记录 provenance。

## Acceptance Criteria

- [x] RCA 三条路由各有通过、跳过或停止证据；RCA 结论能被独立修复任务消费，incident 主任务不等待 RCA。
- [x] 事故学习、发布后验证和 Agent 评测 fixture 覆盖成功和缺证据失败路径，并由自动化测试验证引用、owner、验收信号、digest、观察窗口和门禁结论。
- [x] `reliability-engineer` 与 `agent-evaluator` 可从模板创建，skill、权限、本地化、并发和 squad 路由一致；既有自定义副本保持不变。
- [x] workload gate 有两个周期的可复核来源；达到阈值的专项角色完成完整上线，且有正向 roster、权限、模板和 skill 测试。
- [x] 五类治理能力各有触发、产物、验证信号、责任角色、审批边界及失败样例。
- [x] 两条闭环演练的每个阶段都有实际 task/comment/fixture/验证文件落点，且未知、阻塞、重复和漂移结果不会被成功样例覆盖。
- [ ] 相关 Go、TypeScript、模板、E2E 和 Trellis 校验通过；真实模型、生产指标和生产操作的未测试原因与复现命令被记录。
