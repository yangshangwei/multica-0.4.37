# 验证证据

## 本轮通过

- `go test ./internal/service -run 'TestValidateGovernanceEvidence|TestLifecycleHandoff|TestEvaluateRolloutEvidence|TestValidateAgentEvaluation' -count=1`
- `go test ./internal/service -count=1`
- `go test ./internal/handler -run 'TestCreateLifecycleHandoff|TestLifecycleHandoffComment' -count=1`
- `go test ./internal/service -run 'TestBuiltinAgentTemplates|TestBuiltinAgentAutonomy|TestBuiltinSquad|TestRoleSkill' -count=1`
- `go test ./cmd/server -run 'Test.*Skill.*Template|Test.*Agent.*Template' -count=1`
- `go vet ./internal/handler ./internal/service ./cmd/server`
- `pnpm typecheck`
- `python3 ./.trellis/scripts/task.py validate .trellis/tasks/09-24-lifecycle-dual-loop-integration`
- `git diff --check`

覆盖内容：五类治理 capability 的 pass/hold/unknown；governance handler 写回；incident-learning、rollout/canary、Agent evaluation handoff；RCA 与 rollout 连续写入；有界 `lifecycle_handoff_history`；已有 prevention 引用和 follow-up parent/workspace 边界。

## 既有证据复用

- 角色 roster、四语模板、skill 附着、squad 路由和 workload gate 见父任务 `lifecycle-gap-matrix.md`、`requirements-evidence.md`、builtin template tests。
- 脱敏双闭环输入与未授权边界见本任务 `research/dual-loop-rehearsal.md`。

## 限制

完整 `go test ./internal/handler -count=1` 仍受本地既有 project/resource fixture 缺少 `project.execution_squad` 列影响；未因本轮 handler 改动产生新的 lifecycle 失败。未执行生产发布、回滚、迁移、恢复、通知、生产指标读取或真实模型调用。
