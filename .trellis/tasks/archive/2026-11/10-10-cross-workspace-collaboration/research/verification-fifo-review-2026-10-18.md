# Independent FIFO and configuration review — 2026-10-18

Reviewed the tracked and untracked continuation changes from `6356cd9cf` under
the existing task. Read the full saved Trellis hook, PRD/design/implementation
plan, current data/execution contracts, CLAUDE.md, security boundaries and
current server/core specs. No new task, dependency, commit or execution gate
was introduced by this review.

The review traced channel sequence allocation, waiting-input selection and
staging limits, immutable consumed-input history, E03 return, pending target
reconfiguration, actual configuration writers and their durable consumer,
migrations 699–708/index cleanup registration, and the v4 service/API/core
boundary. The added Linux route regressions were reviewed in source; their
actual native execution belongs to the main session's separate verification.

## Findings (fixed)

### F01 — Contradictory settled proof could release an active group

- File: `server/internal/service/collaboration_groups.go`.
- Issue: `collaborationRunProvenEnded` existed, but `validateGroup` never called
  it. A retained run with `settled_at`, `never_launched_proven` and
  `progress_state='running'` passed Stop, allowing settlement SQL to clear the
  group and release its physical reservation. The database settlement queries
  alone did not reject that contradiction.
- Reproduction: `TestCollaborationNonlaunchContradictoryProgressKeepsBarrier`
  failed on the exclusive review database with “Stop accepted a settled
  nonlaunch proof that still claims running”.
- Fix: validate every already-settled run with the existing proof helper before
  the shared group path can release any reservation or settle the group. This
  also rejects a missing/incomplete seal and a started/success contradiction
  in nonlaunch evidence.
- Verification: the existing regression is green in `review-fifo-race.jsonl`
  and `review-projection-green.jsonl`.

### F02 — Late proof skipped that guard after group settlement

- Files: `server/internal/service/collaboration_runtime_capabilities.go` and
  `server/internal/service/collaboration_fifo_review_test.go`.
- Issue: `AcceptRuntimeProof` has a reservation-release call outside
  `settleProvenGroup`. When the group was already settled, its path checked a
  retained run's proof enum/sequence but did not reject contradictory progress.
- Reproduction: the new
  `TestCollaborationReviewLateProofRejectsContradictorySettledRun` failed before
  the repair; `review-late-proof-red.jsonl` retains the failing output.
- Fix: validate the retained/newly recorded run proof before confirming the
  termination receipt or releasing capacity. Historical termination authority
  and already-confirmed receipt replay retain their existing behavior; this
  adds no current-resource authorization requirement.
- Verification: the regression and existing stop/proof/replay/resource-removal
  cases pass with the race detector in `review-proof-config-race.jsonl`.

### F03 — v4 accepted incomplete evidence for returned input

- Files: `server/internal/service/collaboration_execution_view.go`, existing
  projection fixtures, and the review regression file above.
- Issue: the projection's `allRunsNeverLaunched` predicate checked a proof
  enum and settled marker without checking `started_at` or the sealed versus
  authorized attempt boundary. It could present returned input as valid when
  the same retained evidence was insufficient for a legal requeue.
- Reproduction: `TestCollaborationReviewProjectionRequiresCompleteNonlaunchEvidence`
  failed for all three cases: present `started_at`, missing seal, and a seal
  below the authorized attempt. Each case first verifies a complete baseline.
  The failures remain in `review-projection-red.jsonl`.
- Fix: the projection owner reused `collaborationRunProvenEnded` for settled
  evidence and nonlaunch classification, and supplied actual seal/attempt
  facts in the existing test fixtures. No additional private wire fields were
  introduced. The reviewer kept the independent regressions and reran them.
- Verification: all three cases, historical movement/requeue and current
  projection cases pass in `review-projection-green.jsonl`.

### F04 — SQL source comment described an obsolete lock order

- File: `server/pkg/db/queries/collaboration_batches.sql`.
- Issue: the `CreateCollaborationChannel` comment said the new destination was
  inserted before the sorted existing-channel locks, contrary to the current
  `moveReconfiguredTarget` path.
- Fix: aligned that comment with the implementation: existing channels are
  already locked, and only a missing destination is inserted. SQL is unchanged;
  the main session regenerated the matching Go comment before the final run.

## Findings (not fixed)

No unresolved functional defect remains from this independent review of the
current slice. The configuration owner separately repaired the fingerprint
versus child-trigger mismatch and test fixture grant cleanup; those changes
were inspected without weakening production missing-source refusal.

The current scope intentionally remains short of complete product acceptance:
full frozen multi-target grants, consent-ready atomic Switch, M08/M11, shared
skill-content invalidation, the remaining provider/OS capability matrix and
full Web/Desktop flows are still pending. `ReconfigureTarget` only prepares a
held configuration and moves unread input. The reviewer did not enable squad
launch, replace pending acceptance evidence, or claim that synthetic squad
rows prove a working squad runtime.

Lint retains 31 warnings in unchanged desktop/UI/views files (zero errors).
They are outside this task's write scope and were not changed.

## Verification

Reviewer database: `postgres://postgres@127.0.0.1:53513/collaboration_review?sslmode=disable`,
created independently from empty through current migrations. Database tests
were serialized where they share the singleton gate; migration tests use
private fixture schemas. Go used `GOTOOLCHAIN=go1.26.6` and
`scripts/go-test-with-agent-cli-guard.sh`. No user-installed AI CLI was invoked.

Evidence directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-fifo-20261018-21zqy8p8/`.

| Check run by reviewer | Result | Evidence |
| --- | --- | --- |
| `pnpm lint` | Pass; 6 cached tasks, zero errors | `review-lint-final.log` |
| `pnpm typecheck` | Pass; 9 cached tasks | `review-typecheck-final.log` |
| `pnpm --filter @multica/core test collaboration` | Pass; 476 tests / 21 files | `review-core.log` |
| FIFO, pending Switch and E03 service tests, `-race` | Pass; 15 top-level + 12 subtests | `review-fifo-race.jsonl` |
| Proof/control/configuration service tests, `-race` | Pass; 39 top-level + 48 subtests | `review-proof-config-race.jsonl` |
| Final review/projection/nonlaunch tests, `-race` | Pass; 16 top-level + 39 subtests | `review-projection-green.jsonl` |
| Channel-input/configuration migrations, `-race` | Pass; 6 top-level tests | `review-migrations.jsonl` |
| Concurrent-index cleanup registry | Pass; 3 tests | `review-index-registry.jsonl` |
| `go vet ./internal/service ./internal/handler ./cmd/server ./cmd/migrate` | Pass | `review-vet-final.log` |
| Reviewer Go formatting and `git diff --check` | Pass | Tool output |

The service suites overlap; their counts must not be added as unique tests.
The configuration owner also reported a separate fresh-database full
`^TestCollaboration` service race run (240 top-level + 431 subtests, no failures
or skips). The main session owns the final integrated Go/SQL generation and
actual ARM64 Linux execution evidence; those are not claimed as reviewer-run
checks here.

## Final integration follow-up

The main session requested a bounded second pass over changes made after the
review above. No new defect, weakened assertion or authorization bypass was
found. This follow-up changed only this report.

- `agent_access_test.go`: cleanup selects the test's newly created agent UUID
  within its workspace. The positive owner assignment and both unauthorized
  assignment assertions remain intact; cleanup executes after those assertions.
- Handler/service `collaboration_config_invalidation_test.go`: drains still
  invoke the real global consumer with its normal ordering. Only the completion
  condition is scoped to fixture-owned resource IDs, with a 30-second deadline.
  Query errors, consumer errors and pending work without progress still fail.
  Hold/grant/history/no-op and direct fairness/concurrency assertions remain.
- `collaboration_lifecycle_routes_test.go`: the fixture's sequence counter moved
  to its channel, matching the production schema, without relaxing lifecycle
  or authority assertions.
- `TestCollaborationHeldSquadAllowsDirectAgentRoutesLinux`: the public comment
  path must return `squad_execution_unavailable`, no command ID, and the initial
  hold. The separate waiting fixture uses a real participant/password identity
  and the authorized internal queue service. The actual Runner must consume
  only A's command; S must remain held/waiting without consumed membership or
  another run. This proves retained-input FIFO behavior and public rejection,
  not public squad submission or squad execution.
- Built-in skill compaction: `SKILL.md` is 499 lines and explicitly directs
  readers to `references/collaboration-workflow.md` before shared actions. The
  reference retains the v4/gate, private-content, hold/proof and original-key
  restrictions and links the existing source map. The existing recursive
  `go:embed`/`loadBuiltinSkill` loader includes this supporting file.
- The generated `CreateCollaborationChannel` comment now matches its query
  source; no SQL behavior changed.

Inspected the main session's completed `go-final-race.jsonl`: 1,191 top-level
tests plus 1,211 subtests passed across five packages, with no failures. Its
five skips are four Linux-only cases covered by the native run and the separate
live-GitHub opt-in case. `linux/routes.log` and `linux/controller.log` both end
in PASS (six route tests and four controller tests). The built-in skill check
has ten passing cases. Main-session final evidence additionally records 3,532
core tests, lint/typecheck/vet, reproducible sqlc output and a clean schema-708
database check. These remain main-session executions; no broad suite was rerun
for this bounded review. `git diff --check` passed during the follow-up.
