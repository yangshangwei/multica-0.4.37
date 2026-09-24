# RCA 闭环与双闭环验收证据

## 本轮通过

- `go test ./internal/service -run 'TestValidateRCAArtifact|TestValidateGovernanceEvidence|TestLifecycleHandoffFixtures' -count=1`
- `go test ./internal/handler -run 'TestCreateLifecycleHandoff(PersistsRCARepairEvidence|BugFixKnownAndUnknownRoutes|IncidentRequiresMitigation|MaintenanceCreatesDiagnosticFollowUp)' -count=1`
- `PLAYWRIGHT_BASE_URL=http://localhost:13492 NEXT_PUBLIC_API_URL=http://localhost:18572 pnpm exec playwright test e2e/localized-template-defaults.spec.ts -g diagnostician --workers=1`：1 passed
- `PLAYWRIGHT_BASE_URL=http://localhost:13492 NEXT_PUBLIC_API_URL=http://localhost:18572 pnpm exec playwright test e2e/localized-template-defaults.spec.ts --workers=1`：4 passed
- `pnpm exec playwright test e2e/localized-template-defaults.spec.ts --list`：真实 diagnostician 用例可被 Playwright 发现
- `go vet ./internal/handler ./internal/service ./cmd/server`
- `pnpm typecheck`：9/9 packages
- `python3 ./.trellis/scripts/task.py validate .trellis/tasks/09-24-lifecycle-rca-and-closeout`
- `git diff --check`

新增 runtime contract：RCA handoff 可携带 `diagnosis_ref`、`regression_test`、`conclusion`、`evidence` 和 `unknowns`，并将非敏感证据写入下游修复 issue 的 `lifecycle_*` metadata。真实 API 浏览器用例证明 diagnostician 的 template、Contributor、并发上限、默认 skill 和 workspace 副本保护。

## 未完成或受限

- 真实模型 RCA smoke 仍需要 `agentintegration` 与 `MULTICA_RUN_REAL_AGENT_SMOKE=1` 的显式授权，本轮未执行。
- 生产 observability、发布、回滚、迁移、恢复和外部通知未执行；双闭环仍使用本地脱敏 issue、fixture 和人工审批边界。
- 完整 handler suite 的既有 project/resource fixture 问题仍单独存在，不作为本轮 lifecycle 失败。
