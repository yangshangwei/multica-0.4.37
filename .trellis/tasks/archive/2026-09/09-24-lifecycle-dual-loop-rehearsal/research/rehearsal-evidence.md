# 双闭环运行时演练证据

## 已验证

| 闭环 | 测试 | 证据 |
|---|---|---|
| 研发交付 | `TestCreateLifecycleHandoffFullDevelopmentLoop` | 同一 source issue 依次写入 `rca`、`rollout`、`incident-learning`；history 长度为 3，阶段顺序和 source id 保持；prevention issue 具有 owner、acceptance signal 和 `lifecycle_source_issue`。 |
| Agent 质量 | `TestCreateLifecycleHandoffAgentEvaluationRolloutKeepsArtifactIdentity` | 同一 source issue 依次写入 `agent-evaluation`、`rollout`；六类评测通过；两个 evidence 的 `artifact_digest` 与 approved digest 均为 `sha256:agent-release-3`。 |
| Fail-closed | 现有 rollout/governance/evaluation handler 与 service tests | digest、baseline、观察窗口、评测 case 或治理信号缺失时为 `unknown`；失败信号为 `hold` 或 rollback recommendation，不执行生产动作。 |

## 状态审计（2026-09-24）

父任务 `09-24-lifecycle-dual-loops` 仍为 `planning`，不能标记完成。当前任务树仍有 planning/in-progress 子任务，历史任务状态和验收证据尚未全部收口；本演练只记录事实，不重写这些状态。

## 未验证 / 明确不执行

- 未运行真实模型 smoke；需要显式 `MULTICA_RUN_REAL_AGENT_SMOKE=1` 和 `-tags=agentintegration` 授权。
- 未连接生产 Observability、Git/CI、Database、artifact registry 或 feature-flag MCP。
- 未执行生产发布、回滚、迁移、恢复、通知，也未读取客户原始数据或生产凭据。
- 本地测试证明 metadata/handoff 合同，不证明真实生产 SLO、客户结果或外部供应商 API 可用性。

## 运行命令

```bash
go test ./internal/handler -run 'TestCreateLifecycleHandoff(FullDevelopmentLoop|AgentEvaluationRollout)' -count=1
go test ./internal/handler -run 'TestLifecycleHandoff|TestCreateLifecycleHandoff' -count=1
go test ./internal/service -count=1
go vet ./internal/handler ./internal/service ./cmd/server
pnpm typecheck
git diff --check
```
