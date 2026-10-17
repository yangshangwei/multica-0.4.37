# Revision review: legacy execution fence and execution projection

- Date: 2026-10-17.
- Reviewer: dispatched `trellis-check`; no further agents or commits.
- Authorization: root confirmed current implementation authorization and assigned S0 first, then the completed projection and two built-in skill documentation edits. Other foundation work remains outside this first stage.
- Environment: task-owned `check.env` database, migrated through 620–642 and 650. Migration down rehearsals use private schemas only. No application database, provider CLI, paid AI account, or native execution was used.

## S0 stage: ordinary execution exclusion

**Verdict: no defect found in the seven-file scope. No reviewer code changes.**

Reviewed `server/internal/service/issue_admission.go`, `issue_task_transaction.go`, `task.go`, `legacy_execution_boundary_test.go`, `server/internal/migrations/legacy_execution_boundary_test.go`, and both `server/migrations/650_collaboration_legacy_execution_fence` SQL files. Each file's SHA-256 matched the supplied isolated-checkout manifest before independent testing. The isolated service/handler checkout is baseline `ff6bb36e3c5c9fd7cb8094f2c67652d09d7b2a33` plus those seven files; it intentionally excludes in-flight foundation edits.

Actual-path checks:

- Persisted collaboration identity is checked before ordinary assignment/mention/thread/squad/deferred preparation, including the borrowed handoff enqueue path. Authority lookup errors fail closed rather than becoming ordinary work.
- The shared SQL fence protects insert, claim, batch/runtime claim, retry, deferred promotion, stale reclaim, issue linking, and start. Direct issue ownership, quick-create captured source, and trigger/coalesced comment owners are checked. A plain parent relation remains independent of content ownership, preserving R51 ordinary children and pure quick-create parent links. Existing triage admission and pending-comment checks remain intact.
- Finalization requires a runtime snapshot, locks the persisted delivery, checks issue/context/runtime/claim identity, and checks both the original comment plan and newly persisted trigger/coalesced comments before minting task or broker credentials. It retains the original payload and receipt CAS; late ordinary comments are not acknowledged as delivered merely because they entered the queue plan.
- Migration 650 is forward-only relative to the existing migration files and creates no indexes or foreign keys. Down takes all five relevant table locks in deterministic name order with `ACCESS EXCLUSIVE NOWAIT` before the retained-marker check and restores the old function in the same `DO` block. Concurrent creation/marker/queue writers are refused without disrupting their commits.
- The built-in `multica-working-on-issues/SKILL.md` and its source map correctly describe the ordinary fence and preserved parent references. Their v2 projection wording is included in the next stage below.

Independent verification (all passed, no skipped tests):

| Check | Result |
| --- | --- |
| Boundary service tests with `-race` | 8 top-level tests |
| Existing triage/managed/finalization regressions with `-race` | 11 top-level tests |
| Existing handler comment delivery, receipt rollback, deleted-trigger provenance and lifecycle enqueue/reuse regressions with `-race` | 8 top-level tests |
| Actual migration 650 private-schema tests with `-race` | 5 top-level tests |
| `go vet ./internal/service ./internal/migrations` in the isolated checkout | Pass |
| `gofmt -l` on the five reviewed Go files | Pass, empty output |
| Scoped `git diff --check` | Pass |

Logs are under `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-collab-revision-20261017-hgtrswox/`: `check-boundary-service.log`, `check-boundary-ordinary.log`, `check-boundary-handler.log`, `check-boundary-migrations.log`, and `check-boundary-vet.log`. The supplied `boundary-isolated-final-manifest.json` records the exact seven hashes.

## Projection stage

**Verdict: no defect found in the assigned projection scope. No reviewer code changes.**

Reviewed the service execution view and its test, handler collaboration response and execution tests, core collaboration schema/index/fixtures/version/execution schema and tests, API collaboration tests, and the two built-in skill documentation edits. Exact hashes for these 13 files are in `check-projection-manifest.json` beside the logs above.

- Handler response assembly re-resolves human access inside the existing repeatable-read, read-only transaction and supplies those same transaction-bound queries to `ExecutionView`. Issue/session/control identity checks reject a mixed snapshot. The controlled concurrent update/revocation test proves the response does not splice new target state into old authorization; a subsequent revoked read is denied.
- The view is explicitly protocol 2, unavailable, and has complete initialized collections. It omits command inputs, resource rosters, grants, runtime/installation identities and credentials. Historical unresolved command/run IDs remain visible without an invented target; their old active-run pointer must resolve to retained legacy work.
- API affordances are masked for the absent execution implementation, while the internal participant authority and unrelated human capabilities remain unchanged. The mask is not conditional on a valid grant; it does not create a future grant prerequisite for Stop.
- The canonical core execution parser rejects unknown/malformed present protocols, duplicate IDs and typed targets, duplicate target sequence/input positions, invalid/cross-group lineage, cycles, contradictory active pointers and missing terminal root evidence. All version comparisons remain exact signed-64-bit decimal comparisons. Missing old-server execution data becomes explicit null with human workflow intact; a malformed present protocol becomes unavailable rather than ordinary executable work.
- Core detail and creation transports both use the new parser. Private extra properties are stripped, and redacted diagnostics contain no private sentinels or received payloads. The built-in skill/source map accurately describe these implemented limits without advertising positive execution or control APIs.

Independent verification (all passed, no skipped tests):

| Check | Result |
| --- | --- |
| Core execution-schema and collaboration API tests | 67 tests in 2 files |
| Shared-checkout Go execution projection and authorized-snapshot tests with `-race` | 5 top-level tests |
| Shared-checkout `go vet ./internal/service ./internal/handler ./internal/migrations` | Pass |
| Scoped Go formatting and tracked diff whitespace checks | Pass |
| ESLint on the seven changed core TypeScript files | Pass |
| `pnpm --filter @multica/core typecheck` | Pass |

Logs: `check-projection-core.log`, `check-projection-go.log`, `check-integrated-vet.log`, `check-projection-eslint.log`, and `check-projection-typecheck.log` in the same task-owned temporary directory. Root owns full-workspace TypeScript lint/typecheck and all-core testing; those broad results are not claimed by this stage report.

## Final integrated foundation review

**Verdict: two local integrity defects fixed; no unresolved defect found in the delivered foundation scope.**

The final review includes migrations 620–642, typed execution identities, target registration/input authorization, scoped controls, group settlement, owner/author/account invalidation, human workflow/lifecycle adaptations, their SQL/generated bindings and tests. The S0 and projection file hashes still match their reviewed manifests. The implementation continues to expose no positive Resume, promotion, run insertion, launch or proof-ingestion API.

### Findings fixed

1. **Whole-issue Stop/Complete silently omitted retained work when its target row was missing.** `stopTargets` iterated current target rows without rejecting an orphaned group or run. Both whole Stop and Complete returned success in a corrupted fixture with a retained permitted run, without writing that run's stop intent. The new preflight in `server/internal/service/collaboration_control.go` rejects missing selected targets and unclassifiable typed work for whole-issue/legacy scope. Source/session/lifecycle changes, operation receipts and events roll back. The regression also covers an orphaned run without its group and preserves the existing target-scope `stale_scope` refusal. Before-fix evidence: `check-missing-target-red.log`.

2. **Whole Stop reported `already_ended` for a dangling legacy active-run pointer.** The prior single-session helper rejected this mismatch, but the new enumeration omitted that check. A retained `active_run_id` without its run produced HTTP-style status 200 and advanced control versions. The same preflight now requires that pointer to identify a retained unresolved legacy run. Failure preserves the pause, pointer, control version and event history. Before-fix evidence: `check-missing-legacy-red.log`.

`server/internal/service/collaboration_targets_review_test.go` is the new reviewer-owned regression file. It also closes important coverage gaps: every squad member requires current consent; changed owner epoch, runtime owner, enrollment or leader refuses input; complete offline consent retains waiting input; missing/cyclic parents and wrong root-command identity cannot cause partial stop or fabricated settlement. All fixtures use synthetic database resources and no provider process.

### Findings not fixed / forward requirements

- No remaining implementation defect was found within this foundation's supported paths.
- Current Stop locks groups/runs before validating commands, under the common source/session/target serialization. There is no independent runtime writer yet. Alignment with the stricter command/group/run lock contract and cross-writer race tests remains required before adding claim, delegation or proof writers. This is outside the current foundation; root corrected `collaboration-targets.md` to describe the actual current order and the forward requirement.

### Integrated checks

- Immutable input/target identity and per-target sequence/deduplication remain distinct from physical scheduling. Runtime support is not inferred from seeded groups.
- Complete current owner grants are checked for every roster resource; owner/author identity stays separate. Registration creates no grants or participants. Target registration and input do not lift either pause.
- Target Stop leaves unrelated targets and global control untouched; issue Stop checks the complete observed set and holds the global pause over later additions. Parent proof alone cannot settle a permitted member. All-run proof boundaries and delegation disposition precede command/group completion.
- Removal/exit, resource withdrawal, account reconciliation and lifecycle close use the same scoped stop owner. Old invalidations cancel old inputs/grants while preserving fresh consent and a fresh group on the same typed target. Required-write failures retain the reconciliation cursor and roll back source effects together.
- Migration 620 quarantines all old inputs/grants/runs without guessing a target, retains identities/nonces/unknown proofs and closes gates. The 22 following indexes are each a single concurrent build. Old targetless/forged-legacy writers fail. All 23 populated and busy rollback cases refuse; empty maintenance rollback restores the original columns/indexes.
- The new current-contract spec and built-in skill/source map now match the delivered foundation, including the two integrity fixes. No spec was edited by this reviewer.
- The three final real-router fixture adaptations retain the existing human workflow assertions and seed explicit typed waiting inputs. The handoff assertion now correctly separates preserved human comment rights from the deliberately unavailable execution affordance.

### Final independent verification

| Check | Result |
| --- | --- |
| Target/control/owner/account/lifecycle service regressions with `-race` | 15 top-level tests passed, zero skips |
| Corrupt descendant lineage regression with `-race` | 1 top-level test, 3 cases passed |
| Migration 620–642 upgrade/index/down rehearsal with `-race` | 1 top-level test covering all 23 files passed |
| Go vet on service/handler/collaboration/migrations, plus final service vet after all test edits | Pass |
| Reviewer Go formatting and diff whitespace checks | Pass |
| sqlc v1.31.1 regeneration into a temporary directory | All 6 collaboration/model generated files byte-identical |
| Previously reviewed S0 and projection hashes | Unchanged |

Final logs in the same temporary directory: `check-foundation-service.log`, `check-lineage-review.log`, `check-foundation-migrations.log`, `check-foundation-vet.log`, `check-review-final-vet.log`, and `check-sqlc.log`. The generator comparison writes only temporary output, not shared generated files. The earlier targeted core lint/typecheck and 67 tests remain valid because those code hashes are unchanged.

Root's completed merged verification, reported after the reviewer fixes and the three route fixture adaptations: 323 top-level Go tests plus 638 subtests passed across all six selected packages, with only two unrelated opt-in triage skips; `go vet ./...` passed. Full-workspace lint/typecheck and all-core tests passed (255 files, 3,452 tests). Root also compared all 97 generated SQL files with zero diff and confirmed migration source hashes against both task-owned databases. These broader runs are root-owned evidence; the independent reviewer runs and their exact logs are listed above. **No blocking finding remains for this foundation slice.**

## Scope and remaining work

- This is staged verification, not release approval or completion of any full product acceptance criterion. Explicit task purpose, scoped runtime credentials, native containment/proofs, scheduling, positive Resume, mentions and dual-client workflows remain the later implementation scope described by the task.
- The implementation owner completed the integrated foundation spec update. Per dispatch, this reviewer did not edit specs or task metadata; the only reviewer product-code change is the local control preflight, accompanied by its regression tests.


## Root toolchain follow-up

The final source was also verified with the repository-declared Go 1.26.6 toolchain: all six selected packages passed with race detection, 323 top-level tests and 638 subtests passed, and only the same two opt-in triage cases skipped. `go vet ./...` also passed on Go 1.26.6. Counts use the latest complete run of each package; random recipient-fixture names across reruns are not counted twice. These are root-owned results in `go126-race.jsonl` and `go126-vet.log`; Go/TypeScript source remained unchanged after this verification. Root subsequently normalized only extra EOF blank lines in 45 new migration files, verified identical SQL content, reran the full migration rehearsal with Go 1.26.6 race detection, and rechecked all 97 sqlc outputs with no changes.
