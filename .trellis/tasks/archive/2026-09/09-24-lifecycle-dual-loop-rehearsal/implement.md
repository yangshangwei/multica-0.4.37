# 执行计划

1. 完成 PRD、设计、context manifests，并启动 Trellis 子任务。
2. 在 `lifecycle_handoff_runtime_test.go` 增加研发连续闭环测试，覆盖 RCA、rollout、incident-learning、prevention metadata 和 history。
3. 增加 Agent 连续闭环测试，覆盖六类评测、同 digest rollout、history 顺序和 digest 一致性。
4. 运行定向 Go 测试、`go vet`、`pnpm typecheck`、浏览器模板 E2E、`git diff --check` 和 Trellis validation。
5. 只读审计父任务和历史子任务状态，将结果写入 rehearsal evidence；不改变仍未完成任务的状态。
6. 更新相关 spec/research（如验证发现新约束），提交 Lore commit 并归档本子任务。

## 验证命令

```bash
go test ./internal/handler -run 'TestCreateLifecycleHandoff(FullDevelopmentLoop|AgentEvaluationRollout)' -count=1
go test ./internal/handler -run 'TestLifecycleHandoff|TestCreateLifecycleHandoff' -count=1
go vet ./internal/handler ./internal/service ./cmd/server
pnpm typecheck
git diff --check
python3 ./.trellis/scripts/task.py validate .trellis/tasks/09-24-lifecycle-dual-loop-rehearsal
```
