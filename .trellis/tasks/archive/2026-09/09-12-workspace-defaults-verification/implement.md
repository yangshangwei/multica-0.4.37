# 开箱即用流程集成验证与文档同步 — implementation checklist

- [x] Read CLAUDE.md, parent contract and referenced spec manifests.
- [x] Inspect existing code and useful baseline tests.
- [x] Add meaningful failing regression tests for the new behavior.
- [x] Implement within owned files, reusing existing helpers and UI.
- [x] Run relevant tests, type/lint checks where applicable, fix observed failures.
- [x] Review against parent acceptance; report changed files, simplifications and risks.
- [x] Record exact commands/outcomes in evidence.md; leader owns final task state and commits.

## Verification commands
- `corepack pnpm typecheck`
- `corepack pnpm lint`
- `corepack pnpm test`
- `make test`
- `go vet ./... (server)`
- `targeted Playwright suite`
- `git diff --check`
