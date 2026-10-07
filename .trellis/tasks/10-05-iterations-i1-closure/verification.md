# CG implementation verification — 2026-10-06

Status: implemented, integration review and final regression pending. Release defaults remain disabled; atomic_handoff capability remains false until gate acceptance.

## Product implementation

- Extended the existing canonical preview/apply protocol to active end/cancel, multi-target carryover, handoff, and workspace disable. No parallel write protocol.
- Complete source R must match explicit moves. Terminal scope releases with unchanged counters. Planned destinations add exactly one server-owned rollover; original request replay does not repeat writes.
- Handoff includes target's preexisting plan members and incoming scope in its frozen original commitment; terminal choices and local calendar preview are revalidated.
- Close marker, validated immutable snapshot, ownership release, target membership, target start, settings change, durable operation result and notification enqueue share the existing transaction/fences.
- Disable loads all open plans independently of UI pagination, requires current owner/admin even on replay, rejects >2,000 complete-set members, and rolls back all mutations on failure.
- Reused existing snapshot/history and membership helpers. Start baseline capture is shared with handoff. No new dependencies, migrations, foreign keys or release enablement.
- Outbox enqueue wired to start/end/cancel/date edits; notification delivery/reminder implementation and evidence are in `notifications-verification.md`.

## Evidence

Exclusive database: `multica_i1_cg_20261006_2310` on local PostgreSQL, 566 migrations applied by the root. All Go tests invoked through `scripts/go-test-with-agent-cli-guard.sh`.

Behavioral RED: `TestIterationClosureEndFreezesAndRollsOver` reported `closure=active ... count=1` before close/snapshot application was integrated. Initial build-only RED from concurrent notification API development is not counted as behavioral proof.

- `go -C server test ./internal/service -run '^TestIteration(Closure|Lifecycle)' -count=1`: PASS, 9.430s at initial closure integration.
- `go -C server test -race ./internal/service -run '^TestIterationClosure' -count=1`: PASS, 22.514s; `/tmp/i1-cg-closure-race.log`.
- `go -C server test -race ./internal/service -run '^TestIterationClosureCompletion' -count=1`: PASS, 2.278s (real recorder completion after preview produces stale409; refreshed end freezes completed scope with null destination/count unchanged).
- Initial broader focused race run: iteration PASS 1.621s, service PASS 22.146s, router PASS 7.348s. Handler failed ONLY old revoke expectation requiring outbox deletion; pending notification lane adjustment to retained suppressed dedup tombstones. `/tmp/i1-cg-domain-regression.log`.
- `git diff --check`: PASS at implementation review.

Canonical closure tests cover end/cancel, carryover, existing+incoming handoff baseline, 103-plan whole disable, rollback after intermediate writes, stable request replay, post-close rename/delete frozen digest, exact move coverage, two-connection target-cancellation race, handoff midnight and terminal exclusion, administrator gate, 2,001-item disable rejection, late completion and terminal release. Notification tests independently cover delivery retry/revocation/reminder behavior.

## Remaining integration evidence

Final focused regression after notification changes and administrator replay test; independent review; sqlc second-generation stability; full build/vet/check; Web/Desktop E2E and P95 remain root FCG/VG responsibilities. No push, merge or deployment was performed.

## Subsequent integration and review fixes

- Final focused four-package regression passed before performance transport changes: iteration 1.953s, service 38.661s, handler 4.261s, router 2.472s (`/tmp/i1-cg-final-focused.log`). The old revoke test now verifies a suppressed dedup tombstone and removed inbox instead of deleting the dedup identity.
- Snapshot payload is frozen in memory before membership changes and inserted exactly once after all membership/handoff/outbox/settings writes. No snapshot UPDATE path exists. Final `processed_at` is sampled under locks immediately before persistence; close events retain actual sampled_at on clock rollback while occurred_at/logical end clamp separately.
- Disable reasons explicitly include `功能禁用`; one workspace-level notification replaces misleading per-plan notifications to the union recipient set.
- Current administrator authority is rechecked before disable operation replay. Date-edit recipients are resolved and locked before sampling/writes, excluding revoked coordinator/assignee identities.
- Canonical Draft decoding now targets a fresh value. A swapped-source batch exposed existing alias mutation caused by decoding sorted JSON into caller-owned pointers/slices; deterministic unit coverage preserves request immutability.
- Multi-item lifecycle membership commits use sqlc/pgx pipelines with canonical source-before-target phases, consumed statement results, participation RETURNING checks and final issue CAS. Borrowed single-item callers remain unchanged. Canonical SQL equivalence tests guard batch-query drift. Generated DBTX now includes SendBatch; static read-only test fakes explicitly reject unexpected batches.

Latest focused command (explicit exclusive DATABASE_URL and CLI guard):

```sh
go -C server test -race ./internal/service \
 -run '^(TestIterationClosure|TestIterationLifecycleBatch|TestIterationLifecycleNormalize|TestIterationLifecycleSecondMemberFailure|TestIterationLifecycleCompleteThousand)' -count=1
```

PASS 28.902s, `/tmp/i1-cg-batch-focused.log`. Includes swapped sources, multi-target handoff, real second-CAS zero-row rollback after previous batch phases, injected final snapshot insert failure, sampled/processed clocks, disable reason+single summary, and date-edit revoke/rejoin exclusion. Canonical batch SQL equivalence test also passed. `go -C server build -p 2 ./...` passed before final request-copy/date-recipient edits; final build/vet/regression is still required by FCG.

Performance: initial root 30-sample run demonstrated functional capacity but noisy closure latency under concurrent tests/builds. It is not a performance pass. The pipeline change removes thousands of sequential network round trips; root owns isolated remeasurement and frozen acceptance thresholds. No heavy test/build work should overlap that run.

## 2026-10-07 completion of concurrency/clock coverage

- Actual `TaskService.CompleteTask` and closure hold two real transactions at explicit barriers. Execution completion remains independent, does not infer issue completion, and cannot cause an additional task or another rollover. Separate management-only end/cancel/handoff/disable cases compare the complete running task record, including runtime/session/workdir, and verify unchanged project/status/assignee.
- Real successful DB commit followed by an injected transport error is recovered through the persisted original operation and same-request replay. Snapshot/end event/outbox and rollover remain exactly once. The handler lane additionally exercises HTTP operation lookup.
- Hash comparison now occurs before history/statistics projection. A catalog maintenance change after preview therefore returns stale409 before a history consistency error can mask it as503. Fresh reads still fail closed on unaudited corrupt history.
- Injected clocks normalize to PostgreSQL microsecond precision, preventing JSON snapshot nanoseconds from disagreeing with timestamptz fields. The new test reads the closed history, rather than checking only the write response.

Late focused race command across service+handler for clock, catalog, execution preservation and response loss: PASS service 4.018s, handler 2.716s (`/tmp/i1-cg-late-correctness.log`). Earlier isolated real daemon/lost-commit pair passed 3.719s. Repeated sqlc generation verified identical SHA-256 hashes for all 87 generated Go files.

The second broad focused run (`/tmp/i1-cg-final-focused-v2.log`) passed iteration/service/router but compiled a work-in-progress version of new handler race fixtures; its waiter instrumentation failures are superseded by the handler lane's corrected route-context preservation and expanded dedicated evidence. Do not present that run as an all-package pass. Final FCG regression/build/vet and isolated warmed P95 remain root-owned integration work.

## Rollover event fact correction — root FCG audit

Root review identified that target membership event `after_facts.rollover_count` was serialized before the server incremented the live issue counter. Source before-facts and the later handoff baseline were correct, masking the target event discrepancy.

A real DB regression covering end/handoff × single/batched membership reproduced all four cases: source/target 0 while live 1, and source/target 3 while live 4 (`/tmp/i1-rollover-event-red.log`). Both commit paths now compute the committed counter before serialization and copy it into target facts. Source before-facts remain unchanged. No additional SQL, query round trips, event kind, schema or migration was introduced.

Focused lifecycle/closure and batch-SQL-equivalence race regression passed: iteration 1.623s, service 14.087s (`/tmp/i1-rollover-event-green.log`). The new canonical test checks target event counters against the committed issue for both transport paths and both close operations. Existing source snapshot/rollback/replay tests also ran in that selection. `git diff --check` passed. Handler integration rerun is recorded in `/tmp/i1-rollover-handler-green.log` after completion.

Handler assignment/T1 iteration/real closure writer integration race regression passed 5.786s, exit 0 (`/tmp/i1-rollover-handler-green.log`). No active verification process remains in the CG lane; root may run the final isolated P95 against this corrected membership implementation.
