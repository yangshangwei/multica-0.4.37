# 可靠性与 Agent 评测角色证据

日期：2026-09-24

## 通过

- `go test ./internal/service -run 'Test(AgentRoleTemplates_(WorkloadBackedSpecialistsAreListed|AutonomyDefaults|DefaultRoleSkills|ReliabilityAndAgentEvaluator_EvidenceContracts)|RoleSkillTemplates_(RosterIsFifteen|FilesMatchSource|EveryEmbeddedSkillIsRegistered)|SquadTemplates_(RosterIsCoherent|LifecycleEvidenceRouting|MaxAutonomyMatchesRoster))' -count=1`：通过。
- `go test ./internal/handler -run 'Test(CreateAgentFromTemplate_WorkloadBackedSpecialistsCopyTheirContracts|CreateAgentFromTemplate_ReusesAnEditedSkill|CreateAgentFromTemplate_SecondAgentSharesTheSkill|ListAgentRoleTemplates_ReturnsTheRosterWithInstructions)' -count=1`：通过。
- 真实 API Playwright `diagnostician API creation preserves workspace skill and agent copies`：通过，证明模板创建、副本保护、Contributor autonomy、并发 1 和默认 skill。
- Agent evaluation runtime fixture 覆盖 correctness、tool-failure、safety/越权、cost、latency、drift 六类 case；缺轨迹/版本/基线为 `unknown`，安全 case 非 blocked 为 `hold`。
- Reliability skill/instructions 输出 SLI/SLO、错误预算、队列/容量、降级、恢复和 RPO/RTO 证据；角色不执行生产操作。

## 未授权边界

真实模型轨迹、真实 SLO/队列/备份信号和生产工具调用未执行。需要显式 `agentintegration` 与 `MULTICA_RUN_REAL_AGENT_SMOKE=1`；当前只证明角色契约和脱敏 fixture。
