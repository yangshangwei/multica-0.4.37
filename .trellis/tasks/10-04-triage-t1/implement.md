# Triage T1 implementation plan

> Execution uses native executor subagents for bounded independent lanes, with leader integration and review. This is the current session's authorized implementation, not a request to start a new session.

**Goal:** Deliver every reviewed T1 triage behavior and verify the real shared Web/Desktop flow.

**Architecture:** Existing Issue owns content, separate server-owned admission and triage annotation own review. Transactional action/import ledgers own concurrency, replay and execution intent. Shared core queries/API and views own the platform-independent experience.

**Tech stack:** Go/Chi/pgx/sqlc/PostgreSQL; TypeScript/React/TanStack Query/Base UI/Next/Electron; existing Vitest/Playwright. No new dependencies.

## Lane 0 — prerequisites and frozen contracts (leader)

- [x] New branch/worktree `codex/triage-t1`, Trellis task and source PRDs.
- [x] Read scoped specs and inventory storage/dispatch/frontend boundaries.
- [x] Core baseline 201 suites / 2403 tests and handler Create/UpdateIssue baseline pass with live isolated DB; migrations through511 apply.
- [x] Write PRD, design, API contract and complete T1 acceptance ledger.
- [x] Independent design review identified history/rounds, batch whitelist, execution fingerprint, identity/lock order and deletion replay gaps; all incorporated before start.
- [x] Context manifests validated; parent and five child tasks started after design gate PASS.

## Lane 1 — storage and manual review backend (backend executor)

Files: `server/migrations/513_*` onward, `server/pkg/db/queries/triage.sql`, `server/internal/handler/triage*.go` (excluding CSV), router, new triage service helpers, workspace cleanup.

1. Write PostgreSQL failing tests for disabled/default config, admin gate, intake with no formal candidates, action revision/idempotency, invalid acceptance rollback, reject/duplicate/snooze/reopen and disable-vs-intake. Run with isolated DATABASE_URL and confirm assertion failure.
2. Add server-owned schema and concurrent indexes, sqlc queries; regenerate, migrate private DB and implement transactional settings/intake/actions plus history and selected-only batch preflight/commit.
3. Add failing accept-and-execute retry/terminal-task/membership tests; implement durable action-to-task insertion with prepared enqueue, preserve accepted on startup failure.
4. Add notification recipient/dedup/import-summary support and cleanup/manifest tests. Share reusable helpers and frozen names with CSV lane.
5. Run targeted backend tests; update ledger with command/results. Never claim CSV/guard/UI completion for this lane.

## Lane 2 — formal scope and execution boundaries (boundary executor)

Files: `server/internal/triage` or existing admission helper, existing task service/dispatch/comment/update/lifecycle/plugin entry points, existing issue/project/dashboard/search SQL, dynamic table/list SQL, IssueResponse/util mappers and tests. Coordinate generated SQL regeneration with lane1. Reserve migrations535+ for canonical task fence if required.

1. Write failing service/handler tests proving pending/rejected/duplicate tasks cannot enqueue, merge, retry, claim/start or change execution state through old HTTP/CLI/machine paths. Keep formal control tests.
2. Implement authoritative shared formal predicate and all audited call sites; no stale-struct-only check. Explicitly cover direct SQL paths and plugin comments/actions.
3. Add failing tests for ordinary list/table/group/facet/children/search/count/project exclusion. Implement matching SQL predicates without filtering explicit triage/history/detail.
4. Add HTTP/realtime admission_status fields; keep response key parity tests. Ensure ordinary acceptance never becomes an implicit plugin automation trigger.
5. Run full audited boundary regression matrix. Hand any shared-file conflicts back to leader.

## Lane 3 — shared typed client and cache (core executor)

Files: `packages/core/types/triage.ts`, `types/issue.ts`, `types/events.ts`, API client/schema modules, `triage/*`, exports, realtime. Do not edit paths registry (UI lane).

1. Add failing schema-malformation/API tests for every DTO class and mutation; implement the frozen contract with unknown JSON parsing.
2. Add workspace-scoped queries and nonoptimistic mutations. All request IDs survive retry; malformed success never advances UI.
3. Test cache updates, failed/conflicted input preservation, authoritative queue/count invalidation, reconnect and due-time refetch. No server data in Zustand.
4. Export hooks/types for UI lane and publish exact names promptly.
5. Run affected core tests/lint/typecheck.

## Lane 4 — Web/Desktop interface (UI executor)

Files: `packages/views/triage/*`, direct IssueDetail admission affordances, settings/layout/sidebar, bilingual locales, `packages/core/paths/*`, Web page, Desktop route and package exports. CSV UI may be split into a separate file once contract is stable; do not invent backend methods.

1. Read incumbent Inbox/IssueDetail/settings layouts and impeccable craft floor; preserve the established visual system and specified split interface.
2. Add failing component/navigation tests; wire enabled sidebar count, settings and routes. Verify owner/admin vs member and unsupported backend handling.
3. Build pending creation, filter/sort/counters/list-detail, atomic accept form, duplicate/reject/snooze/reviewer dialogs, history/reopen, explicit execute recovery, keyboard/compact behavior. Reuse comment/editor/attachment primitives.
4. Build selected-only batch preflight/results/retry and CSV file/mapping/preview/row selection/progress/results/download. Preserve drafts and batch IDs on error.
5. Run component/localization/path tests, lint/typecheck, then screenshots/visual verdict before corrective edits.

## Lane 5 — CSV server pipeline (leader or separate executor after lane1 helpers)

Files: `server/internal/handler/triage_import*.go`, CSV parser/resolver helper tests, import queries owned/coordinated with lane1; no edits to generic task routes.

1. Fail parser tests for quoted cells/newlines/BOM/invalid UTF8/limits/mapping/date/ambiguous lookup and warnings for ignored status/iteration.
2. Implement preview without issue/notification writes, durable batch authorization and content identity.
3. Fail real-DB tests for row rollback/partial success/external-ID default skip/explicit override/retry/batch summary dedup; implement row atomic commit with common intake helper and scoped external-ID lock.
4. Add failed-row download escaping and permissions tests; benchmark 1000 rows and record timing/limits.

## Integration, evidence and finish (leader + independent verifier)

- [x] Reconcile every TRI-AC T1 row and relevant WM-AC against actual source/tests; missing proof means remaining work.
- [x] Run targeted then full `pnpm lint`, `pnpm typecheck`, `pnpm test`, `make test`, `go vet`, boundary/static checks; distinguish baseline failures from regressions.
- [x] Run fresh migration and private-schema down refusal/empty rollback. Confirm no foreign keys, cascades or nonconcurrent new indexes.
- [x] Exercise production Web and shared Desktop route: enable/create/review/history/snooze/reopen/CSV/batch/conflict; old-client/direct route blocked; formal controls still work. Fake runtimes only.
- [x] Persist browser screenshots and visual verdict, responsive/keyboard proof, performance samples.
- [x] Update built-in skill/API behavior docs and scoped Trellis specs, user guide and acceptance ledger.
- [x] Independent full-scope review, fix findings, rerun affected checks, commit task-owned files under Lore protocol. Leave branch reviewable; no merge or deployment.

Rollback checkpoints: task planning commit; schema+backend commit; guard/query commit; core/UI commit; integration/docs commit. Feature remains default off. Never roll back populated audit/import/admission tables destructively; disable only after pending queue is resolved, preserve history and deploy forward correction.


Completed 2026-10-05. See verification.md for actual commands, named proof and limitations. Git commits and final task metadata follow the verified source delivery; no merge or deployment.
