# LG verification — passed

2026-10-06. LG passed, including parent HTTP/T1 integration and final review.
Release remains disabled. [Integrated evidence](../10-05-iterations-i1/lg-hg-verification.md)
records commits, final checks and CG/UG/VG limits; the lane history follows.

## Initial RED

Before lifecycle implementation:

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/service -run '^TestIterationLifecycle' -count=1
```

Exit 1: missing `Create`, `Preview`, `Apply`, request DTOs and per-issue
authorization field. This establishes absent APIs, not runtime behavior.
Runtime RED cases after shared generation:

- Historical title correction incorrectly rejected an unchanged revoked
  coordinator. The existing pointer now remains untouched; only a changed
  coordinator is validated as a current member.
- Edit audit after_facts retained revision 1 instead of committed revision 2.
  The event now captures the row returned by the successful metadata update.
- A second connection holding settings FOR UPDATE did not block Create.
  Commit paths now lock settings before iteration rows; Preview keeps its RR
  snapshot read. The regression observes `pg_blocking_pids` before release.
- The first generated Cancel query retained raw processed_at under a backwards
  clock. Final central generation includes the logical-time clamp; the explicit
  cancellation regression now passes while preserving raw event sampled_at.

## Verification environment

Isolated PostgreSQL database `multica_i1_lg_20261006d`, created and migrated by
the integration owner. Every DB test explicitly supplies DATABASE_URL and runs
under the ambient agent CLI guard. No production database or release setting
is modified.

## Required checks

- [x] Create/edit replay, saved timezone, metadata validation.
- [x] Complete RR preview, same-target noop, all-or-nothing membership writes.
- [x] Fixed full O, terminal choices, current participation/reentry semantics.
- [x] Single active under two-connection barrier, today/midnight/date rules.
- [x] Planned cancellation, unused-only deletion, deletion result replay.
- [x] HTTP/T1 integration and current grant checks (parent-owned).
- [x] Focused race tests and service/domain build/vet.
- [x] Final cancellation clock regression after central query generation.
- [x] Central generation stability: 84 files unchanged.
- [x] Complete-set 1,000-item evidence and 2,000 limit refusal.

## Executed verification

The final 19-test service suite passed with the race detector, no skips:

```sh
DATABASE_URL='<isolated LG database>' bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race -p 2 -parallel 2 ./internal/service \
  -run '^TestIterationLifecycle' -count=1 -v
go -C server vet ./internal/service ./internal/iteration
go -C server build ./internal/service ./internal/iteration
git diff --check
```

Final evidence: `/tmp/i1-lg-services-accepted.log`, exit 0, 19 tests, 8.837 s.
Service/domain build and vet both exited 0 after the final generated update.
Earlier logs: `/tmp/i1-lg-services-first.log`, `/tmp/i1-lg-services-green.log`,
`/tmp/i1-lg-services-race.log`, `/tmp/i1-lg-services-final-race.log`.
Final tests include cancellation-clock ordering and an existing running task:
its session/runtime/status/full row, project and issue status remain unchanged
through start and leave. Pure execution progress does not stale the preview;
no extra task or real agent subprocess is created.

## Capacity observations

1,000 real task rows in the isolated local PostgreSQL database, one workspace,
all nonterminal, no agent subprocesses or remote service calls. Complete move
preview contained exactly 1,000 issues, atomic move retained all 1,000 pointers,
and start froze all 1,000 originals. A 2,001-item request returned 413 unchanged.

| Operation | Initial ordinary run | Final race-instrumented run |
|---|---:|---:|
| Complete move preview | 245.6 ms | 426.3 ms |
| Atomic move | 1.518 s | 1.733 s |
| Complete start preview plus commit | 1.641 s | 1.994 s |

These are local single-run observations under concurrent development, not a
P95 benchmark, production SLA or CG whole-workspace closure evidence. The
deployment limit is 2,000 affected tasks per operation; requests are rejected
rather than truncated. Wider repeated capacity and deployment verification
remain in VG.

CG end/active cancel/handoff/disable and UI remain outside this gate's scope.

## Ordinary issue assignment and response integration

Completed the follow-up authorized by the integration owner. Changes are in
`handler/issue.go`, new `handler/issue_iteration_assignment.go` and its tests,
`service/issue.go`'s borrowed after-create hook, and two response fields in
`service/task.go`. `issue_table_rows.go` has only two extra SELECT/Scan columns.
The builtin issue guidance and source map describe the exact supported envelope.

Behavior:

- Nonnull POST current_iteration_id requires expected_iteration_revision; null
  is an explicit unassociated create when assignment is available. The existing
  duplicate-409 contract stays intact; no ordinary-create request_id is added.
- Target/settings lock precedes insertion. Participation/event failure rolls
  back the new issue and issue counter. Existing assigned-agent creation still
  queues exactly one ordinary run and does not invent execution-start facts.
- Membership-only PUT requires expected_revision; actual leave/switch requires
  iteration_reason; done-to-active requires allow_completed=true. Compound
  ordinary fields return400, explicit generic batch/Plugin fields remain428.
- W01's four-attempt55P03 owner and current actor/task authorization are reused.
  Observer denial is checked before and inside the transaction. An unchanged
  private assignee needs no new invocation grant because no execution is invoked.
- All HTTP builders and background issue maps expose actual nullable membership
  and integer rollover. Handwritten list/search/grouped/table projections also
  select both fields, preventing a fabricated null/0 response.

Runtime RED recorded initial428 for confirmed create/update, then a separate
read-path test found four missing handwritten projections after builder support
was added. Detail/open passed while list/search/grouped/table returned false
null/0; after SELECT/Scan correction all six real endpoints passed.

Verification (isolated LG database and agent CLI guard throughout):

| Selection | Result | Log |
|---|---|---|
| Assignment + foundation recorder/old-field compatibility, race |28 top-level /55 including subtests,0 skips|`/tmp/i1-lg-assignment-race.log`|
| Surrounding create/current authority/CAS/attachment/agent regressions, race |39 top-level /77 including subtests,0 skips|`/tmp/i1-lg-assignment-regression.log`|
| Final assignment + DTO + list/search/grouped/table regressions, race |89 top-level /127 including subtests,0 skips|`/tmp/i1-lg-issue-fields-final.log`|
| Service deferred-create atomicity, issue quota concurrency, sanitization, race |3 passed|`/tmp/i1-lg-assignment-service-regression.log`|
| Builtin working-on-issues guidance contract |1 passed|direct tool output|

Selections overlap and must not be added as unique test counts. The final
handler command was:

```sh
DATABASE_URL='<isolated LG database>' bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race -p 2 -parallel 2 ./internal/handler \
  -run '^(TestIssueIterationAssignment|TestIssueToMap_|TestIssueTable|TestListGroupedIssues|TestListIssues|TestBuildSearchQuery|TestSearch)' \
  -count=1 -v
```

The intermediate `/tmp/i1-lg-assignment-accepted.log` captured a concurrent,
parent-owned TriageSettings struct update before its constructor was updated;
it is not a passing run. The final89-test suite compiled after that was resolved.
Final `go -C server build ./internal/handler ./internal/service ./internal/iteration`
and corresponding `go vet` both exited0; `git diff --check` passed.
Parent integration acceptance and shared generation checks have passed; see
the linked integrated evidence. Release default remains off.

### Confirmed completed creation acknowledgment

Final bounded entrypoint fix: POST /issues now passes an explicit boolean
allow_completed through the existing borrowed membership writer. A done issue
may join active only when true; omitted/false, planned target, cancelled issue,
null/nonboolean acknowledgment and acknowledgment without a nonnull target are
rejected without issue/participation/event/task leftovers. The successful case
produces exactly one participation and join event. Its assigned agent retains
the existing ordinary creation enqueue (one task), with no additional run or
execution_started event. Ordinary creation itself historically enqueues an
assigned agent even for done; this fix intentionally does not change that rule.

RED: `/tmp/i1-lg-create-completed-red.log` showed explicit true still rejected
and unsupported acknowledgment silently ignored. GREEN:
`/tmp/i1-lg-create-completed-green.log`, guarded real PostgreSQL `-race` command
with `-run '^TestIssueIterationAssignmentCreate' -count=1 -v`, exit0, no skips.
The new scenario has9 subcases; API and builtin envelope notes were updated.
