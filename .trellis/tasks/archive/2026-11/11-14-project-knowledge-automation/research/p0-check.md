# P0 independent check

Date: 2026-11-14. Reviewer: native `trellis-check` child `p0_check`.
Scope: the approved `2026-11-14-v1` P0 gate only. This review did not modify
production code, tests, the launcher or unrelated task work; its only write is
this report. The complete native-injection output and the current P0 artifacts
were consulted.

## Verdict

P0 admits P1. The expected failures identify unimplemented approved behavior;
they are not a completed product check. There is no blocking scope, isolation or
regression-test issue in the reviewed P0 changes.

The implementation approval is recorded separately from historical planning
validation. The active task remains the existing task. No new task, real-account
model request, deployment, commit or database operation was performed by this
reviewer.

## Tests and evidence reviewed

- `server/cmd/server/task_knowledge_config_test.go` exercises actual router
  construction and its `TaskService` gate, with a truly unset environment case,
  empty value, literal true/false and invalid nonempty value. The two new-default
  cases fail on the current production default, while the three explicit-value
  cases pass. Raw inventory in `p0-default-red-c10050f1.json` has six named
  events: three passes and three failures including the parent, zero skips.
- `server/pkg/protocol/collaboration_knowledge_automation_test.go` decodes through
  existing production DTOs and validates actual runtime requests. Its v2,
  delivery and validity tests fail for the intended reasons. Manifest validity
  currently disappears on decode, leaving the same hash after reacceptance;
  malformed and v1-labelled validity is also silently discarded. The tests
  require exact round-trip preservation, unknown-version denial, canonical
  delivery identity and refusal of client-selected system authority.
  `p0-server-protocol-red.jsonl`: seven top-level tests, five failures and two
  passes; 22 events including subtests, 10 passes / 12 failures / zero skips.
- The separately selected existing Go knowledge protocol baseline has 15
  top-level tests and 47 passing events including subtests, zero failures/skips
  (`p0-server-protocol-legacy.jsonl`).
- `packages/core/collaboration/knowledge-automation-schema.test.ts` calls the
  existing strict production schemas/readers. Four positive v2 behaviors are
  rejected; the existing v1 suite remains green. Nullable system actors, exact
  delivery references, separate approved/paused state, v1 representation limits
  and malformed v2 fields are covered. The combined run has 25 tests: four
  expected failures and 21 passes (`p0-core-red.log`). The source/target combined
  positive test does not reach its second assertion while red; the author
  accurately records that limitation.
- Existing service regression evidence
  `p0-service-baseline-cc390f65.json` is complete: 19 passing named events including
  subtests, zero failures/skips. It covers existing approval/withdrawal commit
  orders, original-receipt identity, source revocation ordering and private
  failure-log behavior. It does not prove the new automatic reopen path.

## Environment review

`run-owned-go.py` selects one exact permitted package, constructs a private
allowlisted environment, allocates a fresh uniquely named database, and checks
its actual identity before migrations and again before tests. Its PostgreSQL
administrative connection is explicitly `postgres`; migration/test subprocesses
receive only the new database URL. Database URLs are parsed as data, restricted
to local hosts and never emitted into task evidence. The launcher does not drop
or reuse databases.

External mode0700 HOME/TMP directories are checked to be outside Git; Go caches
are pinned explicitly. Model/account variables and inherited PostgreSQL settings
are not forwarded. The repository agent-CLI guard wraps tests. A nonblocking
lock serializes package runs and each allocation checks cluster connection
headroom. Both reviewed database runs report source stability, successful
migrations through843 and zero owned connections after exit.

This is suitable for bounded iterations. It is not the full repository check
launcher or a browser deployment proof. Its exit status does not enforce
`source_stable`, and its `state=complete` only combines process success with a
nonempty run inventory. The coordinator must inspect source stability, exact
selected names and skips before accepting a run, as its documentation already
requires. A final task gate must freeze writers or re-run against a stable
snapshot. No broader clearance is inferred here.

## Nonblocking follow-ups

1. The Core positive automatic target fixture inherits a collection revision
   below the candidate's published revision, and an automatic root candidate
   inherits the generic fixture's large version with no predecessor. These are
   not the cause of the current red failures, but should be made internally
   consistent before the green check: use version1 for the new root and a target
   revision at least its published revision. Large delivery event sequences
   continue to exercise decimal precision. This was sent to the Core owner;
   the reviewer did not race its test edits.
2. The baseline comparison observed changes in
   `server/internal/service/collaboration_stage_execution_fixture_test.go` and
   `collaboration_stage_execution_test.go` outside P0's assigned files. They
   were reported to the coordinator and left intact; they must not be attributed
   to or included with this task's changes.
3. P1/P2 still need strict retained-record dual readers, unchanged v1 receipt/hash
   bytes, stable collaboration slots, current source-lock validity checks and
   actual reopen/prepare/authorize ordering tests. P0's manifest regression is
   necessary groundwork, not that transactional proof. Product ACs remain open.

## Checks executed by this reviewer

- Scoped Core ESLint on the new schema test: pass.
- `pnpm --filter @multica/core typecheck`: pass.
- `gofmt -l` on the two new/changed Go P0 test files: no output, pass.
- `go -C server vet ./pkg/protocol`: pass.
- Python AST parse of `run-owned-go.py`: pass, no bytecode/cache writes.
- Existing-product source comparison against the recorded P0 SHA256 baseline:
  only the assigned default test plus the two separately reported concurrent
  stage test changes were observed in preexisting Go/TS/TSX/SQL files under
  server/Core/Views at review time. The two new protocol/Core test paths were
  absent from that baseline.

No database suite, real model, browser or full lint/test pipeline was re-run by
this reviewer. Red and legacy test claims above were checked against retained
raw evidence; P0 intentionally does not turn those new failures green.
