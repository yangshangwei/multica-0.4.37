# Independent Switch and shared-skill verification — 2026-10-18

Reviewed the completed Switch/backend and shared-skill invalidation slice against
`fc654e2b1`, the current continuation, PRD/design/implementation plan, E01/E02/E05/E07,
the acceptance/data contracts, `CLAUDE.md`, and the server security/spec contracts.
This is bounded review evidence; it does not complete P1–P8, M08/M11, or native
Web/Desktop acceptance. The integration owner retains those gates.

## Findings (fixed)

- Files: `server/internal/service/collaboration_switch_view.go`,
  `collaboration_queue.go`, `server/internal/handler/collaboration_switch.go`,
  and the new service/handler `collaboration_switch_recovery_test.go` files.
  Issue: `GetSwitchOperation` required current issue control access before reading
  the actor's receipt. After actual `RemoveParticipant`, a valid owner could no
  longer recover their committed Switch consent (`not_found`), violating E02 and
  design D08's own content-free recovery contract. The red regression is retained
  in `review-switch-recovery-red.log`.
  Fix: reuse `LockCollaborationUsers` and `authorizeWorkflowAccount`, then look up
  only the authenticated actor's issue/request key. Current controllers keep the
  original full receipt; loss of issue/control access produces an explicit
  redacted receipt. The HTTP body contains exactly `redacted: true`,
  `protocol_version`, `issue_id`, `request_id`, `action`, `status`, and `code`.
  It discloses no target/proposal/grant/channel/group/run or authority state.
  Proposal reads retain current ACL. Machine/disabled/stale password callers
  remain denied, and real DB/access errors are propagated rather than converted
  into successful redaction. The client owner implemented the matching strict
  union and original-key GET recovery without rebuilding private draft state.

- File: `server/cmd/migrate/migrate_collaboration_switch_test.go` (new).
  Issue: migrations 713–716 had no dedicated upgrade/replay/runner-recovery
  regression. Empty-database upgrade logs did not prove retained consent or the
  invalid concurrent consent index path.
  Fix: private-schema tests execute the actual migration files and production
  hooks, preserve consent/catalog/ledger through replay, reject duplicate
  identity, check all three indexes are valid/ready/live and no FK/inline index,
  and verify every Switch down file refuses without changing state. A deliberate
  duplicate consent makes migration 715 fail with an INVALID index; reconciliation
  of that test-only duplicate followed by the registered hook repairs the real
  index and completes the ledger. A static boundary check covers new
  709/713–722 files and their exact cleanup registration.

- Files: `server/migrations/717_collaboration_group_dependencies.up.sql` and
  `server/internal/migrations/collaboration_group_dependencies_test.go`.
  Fixed by the squad owner following this review's handoff. Its initial
  backfill could silently omit a roster member lacking an exact current grant;
  legitimate revoked grants also have a newer row epoch than the frozen roster.
  The owner now validates every exact roster resource/fingerprint, retains the
  original frozen epoch from the immutable roster, and accepts a larger current
  epoch only for revoked rows. The new private-schema regressions preserve
  revoked epoch 1/current epoch 2 and atomically refuse an incomplete roster.

## Findings (not fixed)

- Pre-717 mixed-target groups lack exact per-input configuration history; owner
  reports intentional fail-closed refusal rather than borrowing current consent.
  That upgrade boundary must remain explicit in the final deployment guidance.

- M08/M11 production code is being changed by its owner. This review checked
  717–722 index/FK/forward-boundary safety and handed concrete backfill issues to
  that owner. It does not attest squad admission, unread-input detachment,
  continuation, real machine capacity one, or native client acceptance.

## Verified behavior and simplifications

Switch stage and per-owner consent preserve live target/grant/input state. Commit
checks current proposal/config/lifecycle, owner account/participant/binding
generations, hold/control observations, coherent lane mirrors, all dependent
groups including frozen dependency/history links, and retained run/delegation
proof. Validation refusal leaves business state unchanged; required-write failure
rolls back grants, configuration, lane move, audit, and receipt together. Waiting
inputs keep relative order at the destination tail; old lane/history/fences/nonces
remain. Competing proposals, last-consent/commit ordering, withdrawal, later
configuration, original refused receipt replay, and uncertain commit recovery are
covered by the focused suites. Production pending `ReconfigureTarget` and public
`runtime-binding` paths remain removed; explicit Resume follows `switch_hold`.

Migration 709's real skill/body/file/version/plugin/deletion writers advance
enabled agent epochs and the existing durable outbox. The consumer stops exact
dependent groups, retains original proof identity, preserves waiting input and
lane controls, resumes after restart, and leaves later fresh consent intact.
Content/file timestamps, exact bundle reimport, presentation metadata, and
unchanged binding-set writes preserve authority. Skill/child and agent NOWAIT
fences prevent concurrent bind/file/admission writers from committing a partial
bundle or invalidation. The local file-set and binding-set diffs replace
delete/reinsert churn; no dependency or alternate execution model was added.

The main owner synchronized `.trellis/spec/server/collaboration-targets.md` and
the new `collaboration-switch-and-skills.md`, including the exact redacted
recovery contract. Full built-in skill/product documentation and runtime/client
gates remain with integration.

## Verification

Environment: test-owned PostgreSQL on `127.0.0.1:51974`,
`collaboration_review`, through migration 723 for final skill checks.
Go `GOTOOLCHAIN=go1.26.6`; all Go tests use
`scripts/go-test-with-agent-cli-guard.sh`. No installed AI CLI or paid account
was executed. No commit, archive, or application-schema rollback was performed.

Evidence directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-completion-20261018-l73rn78t`.

| Check | Result | Evidence |
| --- | --- | --- |
| Actual migration files + hook coverage, `-race` | Pass: 9 top-level, 4 subtests; no failures/skips | `review-migration-race.log`; migrate 2.949 s, migrations 1.883 s |
| 717 revoked history/completeness regressions, `-race` | Pass: 2 tests; no failures/skips | `review-717-history-race.log`; migrations 2.583 s |
| Atomic Switch, new service recovery, existing HTTP matrix, `-race` | Pass: 14 top-level, 33 subtests; no failures/skips | `review-switch-recovery-race.log`; service 11.435 s, handler 2.596 s |
| Exact redacted HTTP body/cache/ambient header, `-race` | Pass: 1 test | `review-switch-recovery-http-race.log`; 2.334 s |
| Shared-skill service/HTTP writers, `-race`, schema 723 | Pass: 7 top-level, 13 subtests; no failures/skips | `review-skill-race.log`; service 27.950 s, handler 3.567 s |
| Go vet (migrate, migrations, service, handler) | Pass | `review-vet-final.log` (empty, exit 0) |
| gofmt for changed review files; `git diff --check` | Pass | Executed after final edits, no output |
| Trellis task context validation | Pass | `task.py validate`, implement/check each 5 valid entries |
| Core Switch/redacted operation tests (client owner) | Pass: 4 files, 28 tests | Owner report: schema 6/API 8/recovery 11/operation 3, 16:29 |
| Core TypeCheck (client owner) | Pass | `tsc --noEmit`, owner report at 16:34 |
| Core final lint (client owner) | Pending final owner result | Earlier lint clean; final run after redacted extension underway |

One initial Switch regression run and one skill rerun were interrupted by the
active squad lane's test/production/sqlc staging and did not compile. After the
owner finished generation, the red Switch regression failed for the actual
permission defect and final focused race suites passed. Transient compile
attempts are not counted as product evidence. Do not substitute these bounded
passes for complete M08/M11, native Desktop, Linux containment/capacity, or all
97 acceptance criteria.
