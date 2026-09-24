# 验证证据

## 通过

- `go test ./internal/handler -run 'TestLifecycleHandoff|TestCreateLifecycleHandoff' -count=1`
- `go test ./internal/service -count=1`
- `go test ./cmd/server -run '^$'`
- `go vet ./internal/handler ./internal/service ./cmd/server`
- `pnpm typecheck`
- `python3 .trellis/scripts/task.py validate .trellis/tasks/09-24-lifecycle-runtime-handoffs`
- `git diff --check`

Runtime tests exercise maintenance/unknown RCA, known and unknown bug-fix, incident mitigation gate, incident-learning idempotency, rollout unknown, and evaluator case/task handoff against the local PostgreSQL fixture.

## Known gap

`go test ./internal/handler -count=1` still fails in pre-existing project/resource and worktree fixture paths because the local test database is missing the `project.execution_squad` schema column. The failures occur before lifecycle handoff tests and are unrelated to this change; the targeted lifecycle tests pass with the same database.
