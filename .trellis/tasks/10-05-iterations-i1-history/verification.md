# HG history verification — 2026-10-06

Status: HG passed, including parent integration review and route acceptance.
See [integrated evidence](../10-05-iterations-i1/lg-hg-verification.md) for final
commands and commits. The release flag remains off.
No CG closure persistence or Web/Desktop acceptance is claimed here.

## Plan and ownership

Implemented the approved history slice in new `internal/iteration/history*.go`,
new `internal/handler/iteration_history*.go`, and the separately owned
`pkg/db/queries/iteration_history.sql`. The integration owner generates sqlc and
wires routes. Lifecycle consumes `CaptureHistoricalIssues`, `OriginalFacts`,
`IterationFromRow`, and `LoadHistory`.

- Complete event projection uses the immutable original commitment and the
  **start event sequence**, excluding planned changes even at equal timestamps.
- Existing `CalculateStatistics` and `BuildChart` remain the only algorithms.
- Snapshot construction owns all nested memory; closed reads require a valid
  stored payload and never repair malformed snapshots from live issue joins.
- Destinations cover every scope item. Terminal items have no target; release
  retains rollover, eligible rollover increments exactly once. Counts remain
  within the database's integer range.
- `Snapshot.Events` stays frozen. `/events` exposes append-only post-close
  metadata corrections in the same protected RR read; these never alter frozen
  scope, statistics or chart points.
- Reference capture locks workspace prefix, project, user/member, agent and
  squad display data with NOWAIT for writers. Current reads use an RR snapshot.
- Read authorization holds current membership through commit; serialization
  retries reauthorize. Issue/event keyset cursors bind tenant/entity/filter and
  both iteration versions. Page sizes never truncate statistics.

## Red evidence

The initial handler compilation failed because `OriginalFacts`, `History`,
`LoadHistory`, `CaptureHistoricalIssues`, `BuildSnapshot`, and `GetIteration`
were absent. Tests were written first. The next compile reached the shared
SQL-generation dependency; `/tmp/i1-hg-initial-red.log` records the missing
generated queries while the two SQL lanes were being integrated.

The first real DB runs exposed test harness defects: nested `withURLParam`
replaced the workspace context, replacing request context dropped Chi params,
and a same-actor concurrent writer correctly waited on the reader's subscriber
fence. Tests now preserve route context and use a second current member for the
concurrent write. The logs `/tmp/i1-hg-first-green.log`,
`/tmp/i1-hg-second-run.log`, and `/tmp/i1-hg-third-run.log` retain these failures;
their names are not passing evidence.

## Isolated environment

Root created and migrated `multica_i1_hg_20261006d` on local PostgreSQL. Commands
set its explicit `DATABASE_URL` and use `scripts/go-test-with-agent-cli-guard.sh`.
No ambient CLI or shared application database is used by this slice.

## Green evidence

| Requirement | Executed evidence |
|---|---|
| Persisted A–J and fixed O | `TestIterationHistoryPersistedCanonicalFacts`: O8/current9/effective8/completed5/original_completed4,62.5%/50%, deleted F retained |
| Real LG/HG integration | `TestIterationHistoryCanonicalThroughLifecycleAndIssueWriters`: real Create/Preview/Apply join/start/join, ordinary UpdateIssue and real DeleteIssue produce the same result and preserve F's identifier/title |
| Sequence, participation and time | Same-time pre-start delete excluded by sequence; execution-start on blocked counts; reentry resets started; Shanghai midnight chart; clock rollback clamps calculation to persisted logical facts |
| Frozen display and payload | Original project, assignee and identifier survive renames; closed payload survives project deletion, issue edits and participation deletion; nested snapshot memory is owned |
| Strict historical protocol | Malformed identity/counters/status/references/chart collections/events/cutoff/destinations rejected; unknown live status rejected; archived custom category retained; no foreign project name exposed |
| Protected coherent reads | Two-connection ordinary write preserves one RR view; revocation between snapshot and member lock retries and returns403; all endpoints reject revoked members |
| Pagination | Stable issue-ID/event-sequence pages; full total independent of page size; cross-filter/endpoint and stale-version cursors rejected |
| Display capture locking | Workspace prefix, project title and user display mutations produce55P03 under NOWAIT capture; references sorted/deduplicated |
| Post-close audit | `/events` returns later edit while Snapshot.Events, scope and statistics stay frozen |

Final commands used the explicit HG database and agent CLI guard:

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/iteration ./internal/handler -run '^(TestIterationHistory|TestHistorySnapshot|TestStatistics|TestChart|TestCalendar|TestStartDate|TestUpdateIssueIteration|TestGetIterationOperation|TestDeleteIssueIteration|TestBatchDeleteIssueIteration)' -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race ./internal/iteration -count=1 -json
go -C server build -p 2 ./...
go -C server vet -p 2 ./internal/iteration ./internal/handler
git diff --check
```

- `/tmp/i1-hg-final-race.jsonl`: handler56 and domain selection28 passed,
  including subtests; no failures/skips. Includes11 HG handler tests.
- `/tmp/i1-hg-domain.jsonl`: full iteration package103 passed, including
  subtests; no failures/skips. Overlaps the previous selection.
- `/tmp/i1-hg-lg-integration.log`: production LG→HG A–J test passed.
- Build, vet, gofmt and diff check exited0. sqlc generation is owned and
  separately verified by the integration owner. No commits created by this lane.

## Cross-review and CG responsibilities

### Historical validation and legacy identifier cleanup plan

1. Reproduce missing/null destination fields and mismatched baseline identity.
2. Require complete destination objects and validate baseline event identity.
3. Preserve the existing frozen prefix tests, then reproduce ASCII/CJK legacy
   capture failures. Extract the exact stored-prefix/name-fallback resolver into
   `internal/util`; keep thin handler delegates and the existing slug rule.
4. Read workspace name with prefix in the tenant-scoped history query, protected
   by the existing workspace SHARE lock. Regenerate SQL centrally and verify
   canonical/domain/history tests. No schema or persisted identifier rewrite.

This cleanup and validation follow-up is complete:

- `/tmp/i1-hg-validation-red.log` demonstrated missing destination target/count
  keys and null counts being accepted, plus a mismatched baseline issue UUID.
  `/tmp/i1-hg-validation-green.log` proves rejection. An explicit null target
  remains valid; integer decoding and database bounds reject unsafe counters.
- Existing frozen prefix suites passed before extraction, while ASCII/CJK
  capture incorrectly emitted `-7` (`/tmp/i1-hg-legacy-prefix-red.log`). The
  exact resolver now lives once in `util.ResolveIssuePrefix`; existing handler
  helpers delegate. `/tmp/i1-hg-legacy-prefix-green.log` proves `MYT-7`, `WS-7`,
  and stored-prefix precedence, without changing the new-workspace slug rule.
- `/tmp/i1-hg-identifier-hash-red.log` demonstrated that changing workspace
  prefix or a legacy workspace name could silently change start's frozen
  identifier. Additive `PreviewIssue.identifier` now exposes the captured
  identifier and includes it in the existing comparison hash.
- Final `/tmp/i1-hg-validation-final.jsonl`: iteration32, service30 and
  handler26 passed under race detection, including subtests; no failures/skips.
  Selection covers all history/snapshot/lifecycle tests and both frozen prefix
  suites. Full Go build and iteration/service/handler/util vet exited0; format
  and diff checks passed. Root generated the workspace-name query column.

### Approved review follow-up

Before editing, add real database regressions for coordinator NOWAIT recovery,
RR preview revocation recovery, and agent/squad/leader archival between preview
and apply. Then preserve transient PostgreSQL errors for the existing writer
retry owner, add bounded fresh RR preview attempts, and carry live reference
availability separately from immutable display summaries. Root owns central
sqlc generation; this follow-up does not enable release or change schema.

`/tmp/i1-lg-review-red.log` reproduced all three defects against real PostgreSQL:
coordinator contention returned422; preview revocation returned40001; archival
of an agent, a squad, or its leader allowed the old preview to commit. The
follow-up adds a separate availability projection, preserves PostgreSQL errors,
and retries RR previews with three fresh authorized attempts and the existing
25/75ms plus jitter schedule. Squad availability requires an existing,
unarchived leader in the same workspace; the writer captures that fact under
NOWAIT locks on the squad and deduplicated agent rows. Frozen names are retained.

Follow-up green evidence:

- `/tmp/i1-lg-review-green.log`: all4 new service tests plus4 availability
  subcases passed with `-race`. Includes real coordinator contention recovery,
  actual RR/revoke recovery, agent/squad/leader archival, a foreign leader, and
  an explicit three-attempt preview exhaustion bound.
- `/tmp/i1-lg-hg-review-final.jsonl`: service27, iteration15 and handler16
  passed with race detection, including subtests; no failures/skips. Selection
  was `^(TestIterationLifecycle|TestIterationHistory|TestHistorySnapshot)` over
  those three packages on the isolated HG database.
- Full Go build and service/domain/handler vet exited0 after the fixes and
  central SQL regeneration; gofmt and diff checks passed. No schema change,
  release flag change or commit was made by this lane.

CG owns actual end/handoff persistence, final membership release and outbox;
VG owns full lifecycle capacity, Web/Electron E2E and remote CI.
