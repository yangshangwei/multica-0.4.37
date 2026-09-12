# 项目执行小队持久化与幂等准备 — implementation checklist

- [x] Read CLAUDE.md, parent contract and referenced spec manifests.
- [x] Inspect existing code and useful baseline tests.
- [x] Add meaningful failing regression tests for the new behavior.
- [x] Implement within owned files, reusing existing helpers and UI.
- [x] Run relevant tests, type/lint checks where applicable, fix observed failures.
- [x] Review against parent acceptance; report changed files, simplifications and risks.
- [x] Record exact commands/outcomes in evidence.md; leader owns final task state and commits.

## Verification commands
- `go test ./internal/handler -run 'TestProjectExecutionSquad' -count=1`
- `go test ./cmd/server -run 'ProjectExecutionSquad' -count=1`
- `go vet ./internal/handler ./internal/service`
