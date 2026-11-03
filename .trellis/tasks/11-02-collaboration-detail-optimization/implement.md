# Collaboration Detail Implementation Plan

**Goal:** Complete R01–R17 and A01–A17 from the PRD.
**Architecture:** Server contracts → Core schema/actions → shared presentation →
real production Web and Electron verification. Reuse handoffs/events/files/drafts.
**Tech Stack:** Go/sqlc/PostgreSQL, TypeScript/React/TanStack Query, Vitest,
Playwright and existing Electron fixtures. No new dependencies.

## Planning/activation gate

- [x] Read AGENTS.md, CLAUDE.md, source plan, current branch and dirty diff.
- [x] Create this authorized Trellis planning task and capture shared-file baseline.
- [x] Persist requirements, decisions, state/role/action table and full acceptance matrix.
- [x] Review PRD/design/implementation contracts, especially exact history linkage,
  all completion callers, manual admission and retention.
- [x] Curate actual spec/research entries in implement.jsonl and check.jsonl.
- [x] Activate through task.py start only after review; user authorization for
  reviewed activation and execution is already in the active goal.

## 1. Protect contracts before changing behavior

- [x] Read relevant server/Core/Views specs via trellis-before-dev and task manifests.
- [x] Add canonical failing tests for result/basis requirements, exact acceptance,
  revision/eligibility/history, self completion, status bypass and manual policy.
- [x] Use existing workflow fixtures and testutil. Run narrow suites in a private
  database and retain failing assertions proving the missing behavior.
- [x] Preserve existing lifecycle/run/control/notification tests by adapting their
  setup to actual formal delivery rather than weakening their assertions.

Files: server/internal/service/collaboration_handoff_test.go,
collaboration_lifecycle_test.go plus a focused collaboration_delivery_test.go;
handler tests for strict HTTP bodies/route boundaries; Core action/schema tests.

## 2. Server delivery/decision API and policy

- [x] Implement typed immutable delivery/decision event payloads and issue-scoped
  current/history projection in collaboration_delivery.go; query through exact
  handoff/event IDs and bounded sequence cursors in collaboration_delivery.sql.
- [x] Extend handoff/lifecycle/workflow owner and account handlers/routes. Bind
  result attachments through the existing authorized draft mechanism. Persist
  self delivery without a responsibility change.
- [x] Add return_acceptance directed catalog options and fixed refusal codes;
  enforce review/completion/transfer restrictions at service entry points.
- [x] Add ai_policy migration (next free number), access/creation/policy writer
  and centralized new-command/manual launch admission. Reuse Stop machinery;
  preserve existing controls, inputs and proof state.
- [x] Regenerate sqlc and review generated diff, preserving pending source-field SQL.
- [x] Update built-in collaboration workflow skill/source map and affected specs.
- [x] Run narrow Go service/handler/router/migration tests plus gofmt/go vet.

Primary files: server/internal/service/collaboration_handoff.go,
collaboration_lifecycle.go, collaboration_workflow.go, collaboration.go,
collaboration_control.go, collaboration_mentions.go, collaboration_runtime.go;
server/internal/handler/collaboration*.go; server/cmd/server route wiring;
server/pkg/db/queries/collaboration.sql and new delivery query;
server/migrations/next_collaboration_ai_policy.{up,down}.sql; generated sqlc files.

## 3. Core schemas, actions, derivation and recovery

- [x] Add strict compatible delivery/policy summary/history schema and API methods.
- [x] Extend action/status/lifecycle/creation input and original-key draft variants;
  settle any validated final refusal while retaining user-entered unsent content.
- [x] Add current stage/primary action and AI-panel presence helpers with canonical
  pure tests; unknown/legacy evidence must not invent a result or acceptance.
- [x] Verify malformed responses, changed identities, original-key replay and
  stale preflight/draft intent before wiring UI.

Files: packages/core/collaboration/{schema,action,operation,creation,content-schema,
content-queries,comment,index}.ts; new delivery schema/query/presentation helper;
packages/core/api/client.ts and adjacent collaboration API tests.

Commands:

```bash
pnpm --filter @multica/core exec vitest run collaboration/action.test.ts collaboration/operation.test.ts collaboration/comment.test.ts api/collaboration-workflow.test.ts api/collaboration-lifecycle.test.ts
pnpm --filter @multica/core typecheck
```

## 4. Shared detail/actions/discussion and auxiliary structure

- [x] Read Impeccable craft-floor before UI editing; preserve established Operate
  vocabulary. Record the spatial thesis in design.md; no brand redesign.
- [x] Add result/basis/decision/history/evidence presentation and comment-to-result
  draft preparation requiring explicit submission.
- [x] Restructure workflow into one role/stage primary action, small secondary
  actions and More; keep source editing/pickers and recovery/focus behavior.
- [x] Recompose detail with requirements → result → discussion/progress, compact
  metadata/participants and responsive stacking. Preserve source/account identity,
  repeated comment/run/memory anchors and restored scroll.
- [x] Publish ordinary comments in one user action using validated preflight and
  captured-intent comparison; keep explicit AI impact confirmation and uploads.
- [x] Reduce empty auxiliary UI while ensuring errors/pending operations/history
  are accessible; show current permission context beside participants.
- [x] Update both en/zh-Hans locales by merging with baseline source-picker keys.
- [x] Add focused Views wiring/accessibility tests and run existing named suites.

Files: packages/views/collaboration/{collaboration-detail,workflow,field-summary,
comment-composer,comment-list,activity,execution-panel,memory-panel,
knowledge-panel,creation-form}.tsx; result component; locales and adjacent tests.

```bash
pnpm --filter @multica/views exec vitest run collaboration/workflow.test.tsx collaboration/collaboration-detail.test.tsx collaboration/comment-composer.test.tsx collaboration/field-editing.test.tsx collaboration/creation-form.test.tsx
pnpm --filter @multica/views typecheck
```

## 5. Production Web and actual Desktop acceptance

- [x] Inspect make/check/dev environment scripts and retained fixtures. Allocate
  private API and Go databases; do not stop mismatched/unowned listeners.
- [x] Capture baseline layout, then build one current production Web and actual
  Electron renderer with matching API/origin/source provenance.
- [x] Add focused task-owned Web/Desktop full-chain specs with TestApiClient
  setup/cleanup and existing account/native/source-entry helpers.
- [x] Execute A01–A17 at appropriate canonical layers; native and Web each cover
  the complete manual information/submission/revision/resubmission/acceptance chain.
- [x] Capture both locales, long/narrow states, keyboard actions, retained anchors/
  scroll and source identity. Run visual-verdict for each bounded capture pass;
  persist verdict JSON and evidence in this task's research/verification files.
- [x] Run Impeccable mechanical detector once after UI completion; fix real
  findings in one batch and confirm with at most one additional visual round.
- [x] Record commands, actual results, live handles, output, build IDs and cleanup.

## 6. Quality/review/finish

- [x] Run trellis-check review against task artifacts and spec contracts; fix
  verified findings. If native model routes remain unavailable, retain the actual
  error and use a separate recorded local review pass without claiming delegation.
- [x] Run pnpm lint, pnpm typecheck, pnpm test, make test and make check as
  appropriate through their bounded isolated environment entry points. Record
  any unrelated existing failure with exact identity; keep required acceptance open.
- [x] Inspect final diff against owned-file snapshots; confirm other sessions'
  original hunks and files survive; update executable contracts and progress.
- [ ] Commit only verified owned changes with Lore trailers when they can be
  isolated safely; never sweep the dirty worktree into a commit.
- [ ] Requirement-by-requirement completion audit with current evidence, then
  Trellis finish/journal/archive and goal completion only when no required item remains.

## Live execution notes

Initial native research attempts failed terminally (stream disconnections;
specialist route also reported 503/no model channel). They produced no product
edits or verification. Primary agent continues the authorized evidence work;
no failed helper is treated as a running wait or successful independent review.

### Continuation — 2026-11-03

The existing in-progress task was selected through `task.py start`, without
creating another task. Current HEAD and dirty-file hashes were recorded in
`research/continuation-20261103/baseline.json`; older baseline files remain intact.
Native Trellis implementers own Server, Core and shared Views separately. Initial
Server/Core turns disconnected; a later Server continuation failed with an Azure
resource-identity error, so a fresh Server agent with no inherited conversation
items took over the same file ownership. Failed agents are terminal, not waits.
The shared Server/Core wire shapes live in `research/server-wire-contract.md`
and `research/core-ui-contract.md`. Private package/API databases were allocated
through the configured PostgreSQL server; names and configuration paths are in
`research/continuation-20261103/database-allocation.json`. No existing app database
or unknown listener is a verification target.

An independent Core check traced client form records through `delivery-draft.ts`
and `platform/storage.ts`: their JSON is persisted through the existing synchronous
localStorage adapter. The source brief promises recoverable scoped intent, not
a new client encryption scheme. The inaccurate planning phrase "encrypted
delivery form" was corrected; encrypted uploaded draft/file bytes, all access
erasure and original-key recovery requirements remain unchanged.

### Final quality evidence — 2026-11-03

R01–R17 and A01–A17 passed; see `verification.md`. Full TS static/tests, all
82 Go packages under guarded race (fresh DBs for DB-backed packages), actual
migration 764 up/down/NOWAIT, whole main-source vet, script/UI exports and private
sqlc comparison pass. Final production Web4/actual Electron2 each pass 19 framed
captures and 620/620 restoration; visual verdict 95/pass. The aggregate wrappers
were not literally run. Prior failures, conditional skips, the known built-in
document wrapping delta and native/provider scope limits remain explicit.

The code/spec and E2E/evidence commits exclude parallel field-input files, their
stage-hint locale keys and prior shared-spec/journal hunks. Commit/archive and
owned-runtime cleanup are the remaining finalization steps.
