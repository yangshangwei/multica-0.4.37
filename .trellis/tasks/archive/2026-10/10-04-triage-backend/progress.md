# T1 backend delivery

Backend ownership slice implemented and verified. No commit. Source frozen after this report; root owns any further changes and full integration/browser/release gates.

## Delivered

- Migration513 adds default-not_required admission plus7 server-owned tables.514–534 each create one CONCURRENTLY index; no FK or implicit primary-key index. Root owns final guarded downgrade tests/review.
- Settings are default-off and server-owned. Human owner/admin settings writes, machine+resolved-agent review denial, Contributor autonomy for explicit agent intake, current member/role recheck, subscriber-revocation guard and shared enable/disable fence.
- Manual and CSV helper intake are inert. Candidate project/assignee stay separate; active member/visible agent/squad/project/label/date/attachment validation happens in the transaction. Actual member creators subscribe through the existing tombstone-preserving helper; agent creators do not infer human subscribers.
- Atomic acceptance/reject/duplicate/snooze/unsnooze/reopen/reviewer actions; revision CAS; immutable round/source/field history; consumed request/action tombstones after issue deletion. Closed formal duplicate targets work without mutating target/comments; foreign/nonformal/self/deleted targets fail without disclosure.
- Accept-and-execute persists its original human authorization, full relevant agent config, lifecycle status, selected project/resource fingerprint and durable task identity. Changed context cannot retry. Admission/enqueue lost-commit-response tests prove recovery without a second action/task, including retries after terminal task completion. Boundary owner added project-resource parent fences.
- Exact selected-item batch whitelist, nonmutating preview, per-item outcomes, duplicate-selection prevalidation and stable sorted request-key operator summaries.
- Queue filters/counts/order/pagination and immutable global action/import history. Date-only upper bounds include the full day via next-day-exclusive comparison; RFC3339 bounds remain exact inclusive instants.
- Durable notifications for configured responsibility, results/watchers, reassignment and snooze due; idempotent recipient/event identity, retry/clock-safe delivery, current membership checks and personal inbox events after commit. Reassignment reschedules due delivery for the current reviewer. Explicit future deadlines preserve fractional precision.
- Workspace deletion query+manifest sweep all7 tables. Canonical issue cleanup is owned by boundary lane and preserves request/action/import provenance.
- Routes include all frozen API paths and the4 CSV handlers from import lane. Root wired the15-second notification recovery loop.

## Verification

`verification.ndjson` and `verification-summary.json`:63 named tests/subtests passed,0 failed,8.768s, using reachable isolated PostgreSQL and the shared fcntl lock. Includes import/boundary tests, populated7-table workspace cleanup, role/autonomy checks, stale-membership and disable races, history fault rollback, duplicate target matrix, project+agent/default-squad acceptance counts/no execution, date boundaries, lost commits, changed MCP/instructions/cancelled status retry denial, terminal task retry, snooze replay/precision/reassignment, outbox guard cycle and batch summary dedup.

`go -C server vet ./internal/handler ./cmd/server` passed. `gofmt -l` produced no files; `git diff --check` passed. Full repository tests and browser integration remain root-owned.

Notification timing investigation found initial outbox scans selected0 rows, not guard contention: immediate Go-clock deadlines could be in the future of PostgreSQL's transaction clock. The direct DB-clock regression failed before the fix. Immediate deadlines now use SQL now(); explicit future snoozes are unchanged. Real creator/comment/result delivery and clock tests passed30 repetitions each (60 total), with the original immediate delivery assertion retained. See `notification-clock-verification.log`.

Actual handler DTOs are exported in `wire-fixtures.json`. Core owner validated settings/item/actionResult/items/history through strict client schemas; latest export also includes batch preview/result for final integration validation.

## Local performance baseline

Opt-in command: `MULTICA_TRIAGE_PERF=1 TRIAGE_PERF_REPORT=/tmp/triage-queue-baseline.json python3 /tmp/multica-triage-run-locked.py bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/handler -run '^TestTriageQueueScaleBaseline$' -count=1 -v` after exporting `.env.worktree`.

Fixture:10000 formal tasks +2000 pending tasks, page50,10 samples each. P95 local handler+PostgreSQL: list97.333ms, filtered list95.966ms, ordinary accept18.558ms, all below the2000ms target. See `queue-baseline.json` for every sample. These exclude browser and HTTP network latency and are a local baseline, not a production SLA. CSV1000-row sample is recorded in the final verification log; import lane owns parser benchmark evidence.

## Remaining integration limits

No real authenticated agent CLI was launched; queue/fake-runtime fixtures verify dispatch persistence and guards. Browser accessibility/keyboard/rendering, production Web/Desktop build, full repository test/migration rehearsal and final PRD acceptance ledger are root-owned. No production deploy, push, merge or release performed.


## Final evidence location

Raw logs and exported fixture files mentioned above are retained under `.omx/triage-t1/evidence/lanes/10-04-triage-backend` (ignored runtime evidence). The parent verification ledger is authoritative for final whole-feature results.
