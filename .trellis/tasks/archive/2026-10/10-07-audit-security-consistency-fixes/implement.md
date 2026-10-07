# Audit security and consistency fixes implementation plan

**Goal:** Close all six confirmed findings with executable regressions while preserving legitimate clients.
**Architecture:** Reuse principal guards, transaction fences, draft storage and command recovery; add a native Desktop file-request boundary.
**Tech stack:** Electron 39, React/TanStack Query, Vitest, Go/Chi/pgx, PostgreSQL.

## Authorization and review

The user requested fixes, approved this existing task and explicitly asked to continue on 2026-10-07 after receiving its planning/implementation status. Proceed within the six-finding scope. PRD/design were reviewed against audit evidence and lane research. No new task or duplicate authorization is needed.

## Boundaries and local simplification plan

- Desktop owner: apps/desktop/src/main/renderer-file-access.ts and tests, index.ts wiring, renderer-web-preferences.ts comments, apps/desktop/scripts/verify-renderer-file-access.mjs. Preserve normal previews and existing request handlers.
- Backend owner: server/cmd/server/router.go and plugin_action_routes_test.go; server/internal/handler/plugin_action.go, project_resource.go and corresponding tests. Reuse locking/permission/label helpers. No schema or API migration.
- Client owner: packages/views/iterations form/detail and tests; project-detail.tsx overview call-site and navigation regression; necessary English/Chinese projects locale strings. Keep any pure state helper colocated.
- Main: task records, integration checks, relevant specs and commits. Preserve unrelated existing modifications.
- For local refactoring in R3/R5/R6, record behavioral regression failures first. Change only state/read/merge lifetime; retain existing compatibility regressions. No broad cleanup, dependencies, data repair, release or deployment.

## Ordered execution

### 1. Desktop R1 (parallel lane)

- [x] Read Desktop specs and preserved probes; confirm production entry points and request ownership.
- [x] Add failing native regression using fake files/temporary profile and production preference/preview path, covering inline, attachment and full-page input.
- [x] Install one file policy per session before load. Resolve live main-frame identity per request; deny unknown/subframe contexts.
- [x] Validate trusted scripts, dynamic imports, workers, HTTP interactive previews, image/PDF controls, existing headers and window/session lifetimes.
- [x] Run native regression, Desktop tests, lint/typecheck. Record red/green evidence and limitations.

### 2. Backend R2/R5/R6 (parallel lane)

- [x] Read research/backend-fix-plan.md and server specs. Create/verify a unique disposable database and explicit DATABASE_URL. Never use application DB or real agent accounts.
- [x] Add real-router machine-credential and deterministic concurrent resource regressions; record failures before edits.
- [x] Add RequireHumanActor bridge middleware and handler backstop; retain human and dedicated plugin bearer/callback behavior.
- [x] Use runProjectTransactionAtIsolation with READ COMMITTED plus exclusive project lock before fresh reads, validation, partial merge and write; publish only after commit.
- [x] Run auth/router, resource/association, middleware and execution-snapshot suites with race detection; gofmt and go vet. Clean only the created database after tests.

### 3. Clients R3/R4 (parallel lane)

- [x] Read research/client-fix-plan.md, iteration operations and views specs; conventions before locale changes.
- [x] Add old-draft/new-revision and cached A-to-B navigation regressions, recording failures.
- [x] Keep values with baseline revision, patch only changed fields, offer explicit server/rebase conflict resolution; preserve useIterationCommand request recovery.
- [x] Advance baseline only after authoritative refresh; prevent duplicate writes after committed-but-refresh-failed result. Keep editor for transient errors, hide on denied/deleted resources.
- [x] Key overview by workspace/project; prove outgoing debounce flush saves under A, B uses only B, returning restores A.
- [x] Run canonical views regressions, relevant core tests, views lint/typecheck.

### 4. Integration and independent review

- [x] Dispatch fresh trellis-check review of all acceptance criteria and changes; repair concrete issues.
- [x] Run pnpm typecheck, pnpm lint, pnpm test (bounded concurrency), relevant Go race suites, go -C server vet -p 2 ./..., final native Electron verification and git diff --check.
- [x] Update specs for native file boundary, human plugin bridge, fresh locked resource reads, draft/revision lifetime.
- [x] Commit only task-owned source/tests/docs with Lore intent/trailers; no push or deployment. Record commit and archive when verified.

## Commands and evidence

Detailed commands are in research/backend-fix-plan.md and research/client-fix-plan.md. Focused frontend: pnpm --filter @multica/views exec vitest run <files> --maxWorkers 2. Desktop equivalent uses @multica/desktop; native: node apps/desktop/scripts/verify-renderer-file-access.mjs. Backend: bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 <packages> -run <pattern> -count=1 -v with preflighted disposable DATABASE_URL. Require real test pass events; reject silent DB skips.

Each owner writes research/<lane>-implementation.md with files, red/green commands/results, limitations. Original research/verification.json remains audit baseline, not remediation evidence.

## Rollback

Keep fixes reviewable and commits small. Revert only the relevant task fix if needed, never reset user changes. No production operations or automatic repair of existing duplicate records.

## Completion

All six requirements were independently reviewed with no unresolved functional findings. The review additionally fixed HTTP 408/429 draft loss. Five scoped work commits precede task archival; see research/remediation-verification.json and research/final-review.md.
