# 双闭环验收收口证据

日期：2026-09-24

## RCA 与模板验证

| 检查 | 状态 | 证据 |
|---|---|---|
| Service roster/skill/squad/autonomy | pass | `go test ./internal/service -run 'Test(AgentRoleTemplates_|RoleSkillTemplates_|SquadTemplates_|SquadLeads_|Validate|Evaluate|Governance|Diagnostician_|ReliabilityAndAgentEvaluator_|ExperienceAndMigration_)' -count=1`，通过。 |
| Handler template/squad/lifecycle | pass | `go test ./internal/handler -run 'Test(CreateLifecycleHandoff|ListAgentRoleTemplates|CreateAgentFromTemplate_DiagnosticianCopiesAndPreservesWorkspaceContent|ListSquadTemplates_ReturnsTheWholeRoster|CreateSquadFromTemplate_.*)' -count=1`，通过。 |
| Browser API template E2E | pass | 本 checkout API `http://localhost:18572`、Web `http://localhost:13492`，commit `09d06d3e7`；首次 3/4（一次 discovery 导航超时），单测重跑通过，全套重跑 `4 passed (45.2s)`。 diagnostician copy/skill 测试通过。 |

浏览器实际覆盖：diagnostician 模板创建、Contributor autonomy、并发上限 1、默认 `multica-debugging` skill、workspace skill/agent 自定义副本保护，以及本地化创建路径。

## 真实模型与生产边界

状态：`blocked-by-authorisation`，不是 pass。

当前环境未设置 `MULTICA_RUN_REAL_AGENT_SMOKE=1`，也未获 `agentintegration` 授权；因此没有运行真实模型、读取生产凭据、客户数据或调用生产系统。

授权后可重复执行的隔离命令：

```bash
(cd server && MULTICA_RUN_REAL_AGENT_SMOKE=1 go test -tags=agentintegration ./pkg/agent -run '<test-name>' -count=1 -v)
```

三类 RCA 样例（可复现、间歇性/暂不可复现、证据不足）仍不能宣称真实模型通过；本任务使用已通过的模板、路由和脱敏 runtime handoff 证据作为替代验证。

## 任务状态审计

父任务 `09-24-lifecycle-dual-loops`：`planning`，保持不变。

| 任务 | 状态 | 审计结论 |
|---|---|---|
| `09-24-rca-loop-verification` | in_progress | RCA 浏览器/模型验收仍未完全收口 |
| `09-23-debug-rca-e2e-model-eval` | planning | 真实模型三类样例未授权 |
| `09-24-incident-learning-canary` | planning | 历史任务未归档 |
| `09-24-reliability-agent-evaluator` | planning | 历史任务未归档 |
| `09-24-experience-migration-roles` | planning | 历史任务未归档 |
| `09-24-delivery-governance-recovery` | planning | 历史任务未归档 |
| `09-24-lifecycle-implementation` | in_progress | 历史任务未归档 |
| `09-24-lifecycle-closure` | in_progress | 历史任务仍有未勾选验收项 |
| `09-24-lifecycle-runtime-handoffs` | in_progress | 运行时能力已有证据但任务未归档 |
| `09-24-lifecycle-acceptance-audit` | completed (archived) | 本任务证据已收口并归档；真实模型仍是授权阻塞 |

不得把父任务标记 completed；当前状态矩阵明确存在未完成项。

## 其他验证

- `pnpm typecheck`：9 个 workspace successful。
- `go vet ./internal/handler ./internal/service ./cmd/server`：通过。
- Trellis context validation：implement 5 entries、check 4 entries，通过。
- `git diff --check`：通过。
