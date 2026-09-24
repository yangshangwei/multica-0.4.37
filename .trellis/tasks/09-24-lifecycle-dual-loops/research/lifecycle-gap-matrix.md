# 生命周期场景盘点（2026-09-24 复核）

当前默认目录已经包含 14 个 listed roles、15 个 role skills、8 个 AI 小队和 3 个无凭据 MCP 模板。结论是：主研发链路不再缺一个“通用工程师”，下一步应优先补独立交付物，再根据连续周期的真实负载决定是否升格为 listed role。

| 场景 | 当前覆盖 | 建议落点 | 独立证据 |
|---|---|---|---|
| 需求澄清/架构 | 已有角色、skill、discovery squad | 复用 | 决策标准、ADR |
| 实现/测试/代码审查 | 已有角色和 review-gate | 复用 | 回归测试、审查发现 |
| 未知故障/RCA | 已有 diagnostician，需验收 | RCA 子任务 | 根因证据、停止条件 |
| 事故止血/复盘 | 止血已有；学习已接 incident lead，且不阻塞止血 | `multica-incident-learning` | 预防任务、owner、信号、重复事故关联 |
| 发布/回滚 | 发布前已有；release 后接同产物观察 | `multica-rollout-and-canary-verification` | baseline、观察窗口、健康信号、回滚结果 |
| 运行可靠性 | 已由独立 owner 和 incident/release 席位覆盖 | `reliability-engineer` | SLI/SLO、错误预算、恢复/RPO/RTO 证据 |
| Agent 质量 | 已由版本化评测和 review/release 席位覆盖 | `agent-evaluator` | 轨迹、正确性、安全、成本、漂移、门禁建议 |
| 体验/跨端验收 | 已有 `experience-validation-engineer`、`multica-experience-validation`，并接入 review-gate/release | 复用 | 同一 artifact 的浏览器、桌面、可访问性和结果风险证据 |
| 数据迁移 | 已有 `migration-reviewer`、`multica-migration-review`，并接入 maintenance/review-gate/release | 复用 | 旧/新契约、兼容窗口、校验、幂等、重试和回滚 |
| 契约兼容 | architect skill 已补旧客户端/插件矩阵和兼容窗口 | architect/reviewer 路由 | 旧客户端/插件矩阵、`parseWithFallback` |
| 威胁建模/供应链 | security/release skill 已补信任边界、来源和 SBOM | security/release skill | 滥用路径、SBOM/来源、digest |
| 产品结果/灾备 | progress/reliability/release skill 已补结果和恢复证据 | product/reliability 路由 | baseline/观察窗口、RPO/RTO 演练 |

明确不新增：按 frontend/backend/mobile 拆角色、自动生产操作、凭据型 MCP、无独立产物的“协调员”角色。

## 本轮新增角色的复核结果

`experience-validation-engineer` 和 `migration-reviewer` 已通过最近两个发布周期的 workload gate，完成 listed roster、四语文案、权限、模板、role skill 和小队路由测试。门槛仍然保留：如果后续周期没有独立交付物，不应继续新增同类角色。

## 下一批候选（先做 skill，暂不新增默认 role）

| 优先级 | 场景 | 当前缺口 | 建议落点 | 升格条件 |
|---|---|---|---|---|
| P1 | 隐私、合规和数据治理 | 安全审查覆盖数据暴露，但没有单独的 PII/保留期/同意/地域边界产物 | 新增 `multica-privacy-and-compliance`，先附着 architect、security-reviewer、release-engineer | 连续两个周期有独立的数据分类、保留/删除或合规审批交付物 |
| P1 | 性能、容量和成本回归 | reliability 已覆盖容量，agent evaluator 已覆盖成本/延迟，但缺少统一的压测与预算输出格式 | 新增 `multica-performance-and-capacity`，先附着 qa-engineer、reliability-engineer、agent-evaluator | 每周期至少一次真实容量/延迟回归，且现有 QA 出现明确排队或遗漏 |
| P1 | 供应链、许可证和 SBOM | security skill 已要求来源、锁文件、校验和 SBOM，缺少许可证例外及归属清单 | 扩充 security/release 的交付模板，必要时再拆 `multica-supply-chain-review` | 依赖/插件发布成为固定门禁，且有独立 owner 和失败样例 |
| P2 | 运营就绪与支持交接 | release/docs 有 runbook 和 changelog，但没有已知问题、支持 FAQ、升级沟通和升级路径的统一产物 | 新增 `multica-operational-readiness`，附着 technical-writer、release-engineer、progress-reporter | 每个发布周期都需要跨团队支持交接，且缺口导致过返工或事故 |
| P2 | 业务结果与实验复盘 | progress reporter 能汇总进度，但不负责上线后的 adoption、转化或实验结论 | 扩充 product-analyst/progress skill，先不加角色 | 有稳定产品指标来源、基线和实验 owner；没有就保持 `unknown` |

不建议现在新增 frontend/backend/mobile 专家、单独协调员或“通用性能工程师”：它们没有独立边界，现有 implementer、QA、architect 和 reliability 已能承接。

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

## MCP 准入顺序

Git/CI、Observability、Database/migration、artifact registry 和 feature-flag MCP 都有价值，但目前只应保留为候选。进入 catalog 前必须同时满足：官方来源与固定版本、默认只读和最小 scope、凭据由运行时注入而非模板写入、客户数据脱敏、调用审计、超时/重试上限和人工审批边界。未满足这些条件时，继续使用现有 issue/comment、浏览器和离线 fixture 证据，不用一个“看起来能连”的 MCP 冒充生命周期覆盖。
