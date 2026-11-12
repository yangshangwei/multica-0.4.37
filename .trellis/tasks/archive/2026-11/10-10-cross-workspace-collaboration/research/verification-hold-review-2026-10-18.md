# Independent hold/control review — 2026-10-18

Status: **scoped review passed; no unresolved current-slice blockers**. Baseline: `5a0817b8a`. The review covers tracked and untracked production/test/migration files in this continuation, including the final follow-up fixes. Full multi-target and client acceptance remains pending.

## Context and scope

Read the full saved Trellis check injection, current PRD/design/implementation plan, acceptance and data contracts, the full execution contract, the continuation plan, server security/managed-installation/collaboration specs and core schema/state/auth-epoch/recovery guidelines. The continuation and execution contract matched the full injected copies. Reviewed the final server/core spec indexes, `collaboration-targets.md`, and built-in working-on-issues skill/source-map updates against the implemented scope.

Inspected these boundaries against actual callers:

- `collaboration_control.go`, `collaboration_targets.go`, `collaboration_authority.go`, queue transaction/recovery, full-group proof checks, and hold/version SQL.
- Managed runtime candidate selection, promotion, claim/permit admission and original proof identity; full-batch account-author discovery and owner/author invalidation separation.
- Resource invalidation’s exact runtime/binding dependency discovery, source-side NOWAIT locks, retained runs without current grants, and the shared whole-transaction retry wrapper. Callers inspected: ordinary runtime deletion, confirmed deletion, profile teardown, legacy runtime merge, managed bind/renew and retention GC.
- v3 channel/target/gate projection, strict hold/control JSON, route registration, core control request/receipt parsing, original-key GET recovery, draft persistence, account transport and access-epoch fencing.
- Migration 698 backfill/defaults/constraints/replay/down guards, legacy target-control repair, private-schema regressions and an actual registered-target upgrade followed by HTTP GET.

The reviewer wrote only this report and temporary regression/evidence files outside the repository. Production/test fixes below were made by the designated implementation owners. No task creation, commit, sqlc regeneration or spec edits were performed by this reviewer.

## Findings (fixed)

### H01 — Promotion detected direct-agent configuration drift without persisting hold

Files: `server/internal/service/collaboration_runtime.go`, `server/internal/service/collaboration_hold_test.go`.

Before the fix, `promoteCollaborationRuntimeBatch` returned an error immediately after `authorizeTargetInput` rejected the current configuration; the transaction wrapper rolled back, losing the required durable hold.

Independent reproduction used a real managed fixture on the reviewer’s disposable PostgreSQL database: append a direct-agent input, explicitly Resume, update `agent.instructions` (advancing the real configuration epoch), then call `ListRuntimeRuns`. Polling returned no run, but the target retained `hold_reason=NULL, hold_version=3`; expected `config_invalid, 4`. This did not involve a squad or mixed batching.

Fix: a private promotion result carries only an allowlisted authoritative refusal after the hold and audit succeed in the original transaction. The caller handles that refusal after commit. Arbitrary errors still roll back; offline/capacity conditions do not become holds. No channel pause, run or reservation is created. The common authorization helper remains read-only. Unknown commits are not retried.

Independent evidence: `check/independent-promote-regression.jsonl` records the original failure; `check/independent-promote-regression-green.jsonl` records the same overlay regression passing. The final race run also includes that independent regression and the owner’s additional grant/configuration, duplicate-poll, audit, write/commit rollback, unknown-commit and transient-availability cases.

### H02 — Upgrade preserved old Reconfigure-only control mirror mismatches

Files: `server/migrations/698_collaboration_target_holds.up.sql`, `server/internal/migrations/collaboration_target_holds_test.go`, `server/cmd/server/collaboration_execution_routes_test.go`.

The baseline Reconfigure SQL advanced target control/configuration epochs and set target `paused/configuration_changed` without updating its channel. The initial 698 backfill preserved that mismatch, and the new correctly strict reader refused the affected issue. The original migration fixture lacked channels and could not expose this upgrade failure.

Fix: 698 repairs only the recognizable legacy reconfiguration footprint, including repeated local reconfigurations bounded by configuration history. It preserves hold reason/version, channel control, input identity and retained group/run/delegation records. Active pointers, unexplained generations/control deltas and unresolved/mismatched retained execution prevent repair. Unknown mirrors are not healed and the reader is not relaxed.

Independently reviewed the SQL guards and ran the migration regression under race, including idempotent rerun, prior backfill recovery with a large hold version, retained history, negative cases and guarded rollback. The actual HTTP upgrade regression starts with a registered/consented target and retained waiting input, replays the old writer, applies 698, then verifies the strict GET projection and unchanged history.

### H03 — Existing human-route fixtures independently seeded conflicting pause defaults

Files: `server/cmd/server/collaboration_lifecycle_routes_test.go`, `server/cmd/server/collaboration_execution_routes_test.go`.

The main session’s broad race run exposed three initial GET 503s in edit, handoff and lifecycle route tests. The shared seed helper independently inserted a target and channel with different defaults (`new_target` versus `new_channel`), violating the newly validated mirror contract.

Fix: the fixture uses the real `BindCollaborationTargetChannel` operation. Existing human authorization, status-wire, handoff and lifecycle assertions remain unchanged. New route coverage verifies real registration/consent/waiting reads, and explicitly corrupts a mirror to confirm that the strict reader still returns unavailable.

The reviewer independently reran all three previously failing human-route tests with race and then the new registration and actual-upgrade routes. All passed, with no skips. No production reader change was needed.

## Findings (not fixed)

None within the implemented hold/control/resource slice. No mechanical lint/type errors or unresolved additional regressions were found. The explicitly deferred features below remain outside this review’s completion claim.

## Verification performed independently

Go commands used `check/run.py`, which supplies a reviewer-owned disposable database and `GOTOOLCHAIN=go1.26.6`. Migration rehearsals use private schemas. No application database or user-installed AI CLI was used.

Evidence root: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-hold-20261018-nvb9nij7/`.

| Check | Result | Evidence / precise scope |
| --- | --- | --- |
| Core lint | pass | `pnpm --filter @multica/core lint`, entire core package, exit 0 |
| Core TypeCheck | pass | `pnpm --filter @multica/core typecheck`, exit 0 |
| Core control/content/execution/API tests | pass | 5 files, 82 tests |
| Initial focused Go race | pass | 28 top-level tests, 65 pass records including subtests; `check/independent-focused-race.jsonl` |
| H01 independent red/green | pass after fix | Same overlay reproducer, original red retained; `check/independent-promote-regression{,-green}.jsonl` |
| Restored human-route race | pass | 3 tests; `check/independent-human-route-race.jsonl` |
| Final hold/migration/HTTP race | pass | 9 top-level tests, 31 pass records including subtests; `check/independent-final-hold-race.jsonl` |
| Final Go vet | pass | service, handler, router and migrations; `check/independent-final-go-vet.log`, exit 0 |
| Go formatting | pass | `gofmt -l` over 44 changed/untracked Go files, no output |
| Diff whitespace | pass | `git diff --check`, exit 0 |

All listed Go pass runs had **zero skipped and zero failed tests**. The initial run precedes H01/H02; the final run specifically verifies their corrected paths. Core source did not change after the independent lint/type/test run.

Core command:

```sh
pnpm --filter @multica/core test collaboration/control-schema.test.ts collaboration/control.test.ts collaboration/execution-schema.test.ts collaboration/content-schema.test.ts api/collaboration-control.test.ts
```

Initial Go selection:

```sh
go test -race -count=1 -json -p=1 -timeout=240s ./internal/service ./internal/handler ./internal/migrations -run '^TestCollaboration(TargetHoldMigration|TargetConsentRetains|ResourceHold|UnusedAliasHold|TargetResume|AccountInvalidationFindsNonRoot|ReconsentHold|RuntimeHold|ResumeRejectsHold|ResumeAllMaterializes|HoldControl|ControlOperation|ReconfigureRetains|RuntimeTeardown|RuntimeRetentionGC|ManagedBinding|ControlHTTP)'
```

Final Go selection, with `-overlay=<evidence-root>/check/promote-review-overlay.json`:

```sh
go test -race -count=1 -json -p=1 -timeout=240s ./internal/service ./internal/migrations ./cmd/server -run '^TestCollaboration(Promote|TargetHoldMigration|RegisteredTargetExecutionProjection|UpgradedTargetExecutionProjection|ReviewPromote)'
```

## Simplifications and verified limits

The final implementation separates target eligibility from channel pause, reuses the existing source transaction and group-stop machinery, and consolidates resource transaction retries without adding dependencies. Proof identity and retained reservations remain authoritative; ordinary task termination and missing resource rows are not treated as process proof. Control recovery reuses original observations and refetches live state instead of writing an old receipt into Query.

These checks establish the scoped hold/control behavior, resource invalidation atomicity, proof preservation, original control receipts and schema/access boundaries. They do not certify all 97 acceptance cases. The main session owns broader Go/core/global lint/type-check, sqlc consistency, live-database migration replay and real Linux route evidence; those results must be reported from their own logs.

Mixed same-channel A/S eligibility and FIFO rebatching remain pending: a pre-coalesced held squad can still block later valid direct-agent input. Frozen multi-target/squad resource sets, complete Switch movement, M08/M11 delegation/continuation, external agent/squad configuration-writer stop convergence, legacy runtime enrollment, generic provider adapters, full issue-body/attachment delivery and complete Web/Desktop UI remain outside this slice. Synthetic child runs establish control/proof accounting only. Read protocol 3 and control/content/runtime protocol 2 do not enable client creation/execution affordances.

The first root Linux route log used a Linux ARM kernel with an AMD64 executable through configured emulation; it is not native AMD64-host evidence. Matching ARM64 execution and final-source Linux evidence are owned by the root session. Native macOS/Windows execution remains unsupported. No release/deployment readiness is claimed.

Final spec/index and built-in skill/source-map updates correctly describe the implemented hold/control/resource boundaries and the remaining mixed-FIFO, Switch, M11, external-configuration and UI limitations.
