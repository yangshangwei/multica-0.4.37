# 生命周期常用场景审计

| 场景 | 当前承载 | 本轮结论 |
|---|---|---|
| 需求澄清与拆分 | 产品分析师、架构师、feature squad | 已覆盖；不新增 coordinator |
| 设计/契约兼容 | 架构决策记录、迁移审查员、release/review gate | 以旧/新契约和兼容窗口为门禁，暂不新增角色 |
| 实现与回归 | 实现工程师、测试工程师、bug-fix/feature | 已覆盖 |
| 未知故障 RCA | diagnostician + bug-fix/maintenance/incident 路由 | 本轮补真实 issue/comment/queue 证据 |
| 事故止血、恢复、复盘、防重 | incident lead、release、reliability、incident-learning skill | 本轮补幂等关联和 owner/验收信号证据 |
| 发布、canary、回滚 | release + reliability + rollout skill | 本轮补同 digest/baseline/window/approval gate |
| Agent/Skill/MCP 评测与漂移 | agent-evaluator + review-gate/release | 本轮补 case manifest 缺口的 hold/unknown |
| 关键体验与迁移 | experience-validation-engineer、migration-reviewer | 已有 workload-backed 角色，继续保留 |
| 威胁建模/供应链 | security-review、release-check | 已覆盖；MCP 需凭据/审计协议后再接 |
| 产品结果 | progress-report + release/reliability | 结果指标缺失时 unknown，不新增角色 |
| 灾备/RPO/RTO | reliability-engineering + incident/release | 以演练证据为门禁，不新增角色 |

## 仍值得补成 skill 的常用场景

| 优先级 | 场景 | 推荐组合 | 不新增默认角色的理由 |
|---|---|---|---|
| P1 | 隐私、合规、数据分类与保留期 | architect + security-reviewer + migration-reviewer + release-engineer | 交付物独立，但当前工作量还不足以拆出 privacy 专家 |
| P1 | 性能、容量与成本回归 | qa-engineer + reliability-engineer + agent-evaluator | 性能、容量和 Agent 成本分别已有 owner，先统一报告格式 |
| P1 | 许可证例外、SBOM 与供应链来源 | security-reviewer + release-engineer + technical-writer | 现有安全/发布边界能承接，缺的是许可证清单和 provenance 证据 |
| P2 | 运营就绪、支持 FAQ 与升级路径 | release-engineer + technical-writer + progress-reporter | 不是实现边界，需先证明跨团队支持交接的稳定工作量 |
| P2 | 功能开关、实验与业务结果复盘 | product-analyst + reliability-engineer + release-engineer | 需要稳定指标来源、baseline、观察窗和实验 owner；缺失时必须保持 `unknown` |
| P2 | 弃用、生命周期结束与客户迁移沟通 | migration-reviewer + technical-writer + release-engineer | 可复用迁移和文档职责，暂不单设 deprecation 角色 |

这些场景下一步应优先形成 skill/交接格式和失败样例，再根据连续两个周期的独立交付物、排队或返工证据决定是否升格为 listed role。

## 明确未接入

Git/CI、Observability、Database MCP 仍不是可直接启用的 built-in preset：来源固定、只读 scope、凭据注入、脱敏和审计协议尚未齐备。真实模型 RCA 和线上信号验证保留显式授权门禁。

候选 MCP 还包括 artifact registry 和 feature-flag 服务，但准入条件相同：官方来源与固定版本、默认只读和最小 scope、运行时凭据注入、客户数据脱敏、调用审计、超时/重试上限和单独人工审批。条件不齐时继续使用现有 issue/comment、浏览器和离线 fixture 证据。
