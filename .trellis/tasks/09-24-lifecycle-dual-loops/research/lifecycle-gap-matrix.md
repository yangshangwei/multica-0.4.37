# 生命周期场景盘点

| 场景 | 当前覆盖 | 建议落点 | 独立证据 |
|---|---|---|---|
| 需求澄清/架构 | 已有角色、skill、discovery squad | 复用 | 决策标准、ADR |
| 实现/测试/代码审查 | 已有角色和 review-gate | 复用 | 回归测试、审查发现 |
| 未知故障/RCA | 已有 diagnostician，需验收 | RCA 子任务 | 根因证据、停止条件 |
| 事故止血/复盘 | 止血已有；学习已接 incident lead，且不阻塞止血 | `multica-incident-learning` | 预防任务、owner、信号、重复事故关联 |
| 发布/回滚 | 发布前已有；release 后接同产物观察 | `multica-rollout-and-canary-verification` | baseline、观察窗口、健康信号、回滚结果 |
| 运行可靠性 | 已由独立 owner 和 incident/release 席位覆盖 | `reliability-engineer` | SLI/SLO、错误预算、恢复/RPO/RTO 证据 |
| Agent 质量 | 已由版本化评测和 review/release 席位覆盖 | `agent-evaluator` | 轨迹、正确性、安全、成本、漂移、门禁建议 |
| 体验/跨端验收 | 分散在 QA | workload gate 后专项角色 | 浏览器与可访问性证据 |
| 数据迁移 | 规则在文档，缺审查产物 | workload gate 后专项角色 | 兼容窗口、校验、回滚 |
| 契约兼容 | architect skill 已补旧客户端/插件矩阵和兼容窗口 | architect/reviewer 路由 | 旧客户端/插件矩阵、`parseWithFallback` |
| 威胁建模/供应链 | security/release skill 已补信任边界、来源和 SBOM | security/release skill | 滥用路径、SBOM/来源、digest |
| 产品结果/灾备 | progress/reliability/release skill 已补结果和恢复证据 | product/reliability 路由 | baseline/观察窗口、RPO/RTO 演练 |

明确不新增：按 frontend/backend/mobile 拆角色、自动生产操作、凭据型 MCP、无独立产物的“协调员”角色。

## 专项角色门槛

`experience-validation-engineer` 只有在连续两个发布周期各有至少两条需要真实浏览器/可访问性/跨端证据的关键路径，且现有 QA 交付出现可复核排队或覆盖缺口时，才进入 listed roster。`migration-reviewer` 只有在连续两个周期各有至少一项 schema/API/客户端兼容迁移，并且迁移审查已经产生独立的兼容窗口、校验或回滚交付物时，才进入 listed roster。当前仓库没有这两项 workload 证据，因此保持为路由建议，不新增默认角色；达到阈值后需重新完成权限、本地化、模板和负向 roster 测试。

## 推荐小队组合

| 小队 | 组合 | 触发 | 主要交付 |
|---|---|---|---|
| Incident Learning / Canary | incident lead + diagnostician + reliability engineer + release engineer + technical writer | 事故恢复后或发布观察窗结束 | 脱敏复盘、预防任务、同产物健康结论、runbook |
| Reliability Review | reliability engineer + QA engineer + architect + product analyst | SLO 退化、队列/依赖风险、灾备演练 | SLI/SLO、容量/降级、RPO/RTO 和改进任务 |
| Agent Quality Gate | agent evaluator + QA engineer + security reviewer + code reviewer + release engineer | Agent/Skill/MCP 版本变更 | 版本化评测、越权/安全、回归、门禁建议 |
| Migration Review | migration reviewer（达到 workload gate 后） + architect + security reviewer + release engineer | schema/API/客户端兼容迁移 | 兼容窗口、校验、回滚/重试、审批点 |
| Experience Validation | experience validation engineer（达到 workload gate 后） + QA engineer + product analyst | 关键用户路径或跨端改动 | 浏览器/可访问性/跨端证据和结果风险 |

## MCP 取舍

| MCP 候选 | 价值 | 当前动作 | 原因 |
|---|---|---|---|
| Chrome DevTools / Playwright | 浏览器 E2E、体验和发布后验证 | 已内置 | 本地、无凭据模板；可由 E2E 使用 |
| Sequential Thinking | 多阶段规划和复盘 | 已内置 | 本地、无外部数据 |
| Git provider / CI | 变更、PR、构建证据 | 规划候选 | 需官方包版本固定、令牌注入、最小 scopes 和审计 |
| Observability / Sentry / OTel | SLO、事故和 canary 信号 | 规划候选 | 需脱敏、租户边界、只读默认和生产授权 |
| Database / migration | 迁移校验和数据质量 | 暂不内置 | 凭据和破坏性查询风险高，应先有只读代理和审批 |
