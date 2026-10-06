# S2 W16 real execution-start facts

Verified on 2026-10-06, branch `codex/projects-p1`. This closes the W16 writer
slice only. FG remains open; no lifecycle, closure, capability or UI completion
is implied. The preceding W13/W14 batch commit is `ef0b989e9`.

## Transaction and recorder contract

`TaskService.StartTask` now commits the actual `StartAgentTask` running
transition, current participation's first started fact, one event and its
scope revision together. Its existing admission checks, task phases and
postcommit analytics, reconciliation, deferred-escalation cleanup and task
broadcast remain in the owning service. No retry loop or autocommit fallback
was added; a transaction starter is required for issue-null tasks too.

The explicit READ COMMITTED transaction takes workspace KEY SHARE, status
catalog shared, I1 workspace fence, current iteration, issue FOR UPDATE, task
FOR UPDATE NOWAIT, then participation. The issue is reread after the fence;
an issue initially outside an iteration must also fence before its row lock.
The locked task must still reference the preflight issue. A concurrent
quick-create link on an initially issue-null task aborts this attempt before
running and can be retried against the new ownership.

`RecordExecutionStart` borrows the prepared `IssueRecord` and caller's tx. Only
false-to-true `has_started_current_participation` produces a fact; repeated
starts within that participation do not add events. Active iterations use
`execution_started`; planned iterations use `planned_activity`. The actor is
system/source `task_start` with null id/user_id, checked through the shared
identity validator. The event operation UUID is created once per invocation.
Original commitment, issue status/revision, current pointer and rollover count
remain unchanged. A later participation can acquire its own started evidence.

`StartAgentTask.started_at` uses `clock_timestamp()`: after introducing an owning
transaction, `now()` would date a task before a wait on the I1 fence. Event
sample time remains the existing lock-time sample; persisted event time is
clamped against the preceding sequence while preserving the raw sample.

Issue-null tasks produce no iteration facts. Deleted optional quick-create
sources retain their existing successful start behavior. Enqueue, claim,
waiting, failure and completion alone do not supply execution-start evidence.

Owned implementation files:

- `server/internal/service/task.go` — replaces the autocommit start call.
- `server/internal/service/task_start.go` — transaction owner.
- `server/internal/iteration/record_start.go` — borrowed recorder.
- `server/internal/service/task_start_iteration_test.go` — real DB regressions.

Root integrated `LockAgentTaskForStart`, the task timestamp SQL change, generated
queries, and shared identity validation. SQL generation evidence belongs to the
root integration run; this lane did not independently regenerate SQL.

## RED and verification corrections

Before production edits, `TestStartTaskIterationFacts/active` and `/planned`
both failed against the real service: the task entered running, but participation
started=false, events=0, scope=1 instead of true/1/2. The RED command exited 1;
its output is retained in the session, not a separate log artifact:

```sh
DATABASE_URL='postgres://multica:multica@127.0.0.1:5432/multica_i1_w16_20261006b?sslmode=disable' \
  bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -p 2 -parallel 2 ./internal/service \
  -run '^TestStartTaskIterationFacts$' -count=1
```

The first expanded run exposed an invalid fixture task status `done`; tasks
use `completed`. Only that fixture was corrected. An initial cmd/server run
could not compile concurrent W12 work in `handler/issue.go`; it ran zero tests
and was not counted as a verification result. After the W12 owner completed its
edits, the exact runtime command below passed. Those intermediate outputs were
replaced by the final logs and are described here for clarity.

## Final evidence

All DB runs target the dedicated, fully migrated PostgreSQL database
`multica_i1_w16_20261006b`, not a development/shared database. All Go execution
used the repository agent CLI guard. The JSON counts include parent tests and
subtests; none of the runs below skipped tests.

| Check | Result | Log |
| --- | --- | --- |
| W16 focused real service tests, race detector | 23 passing entries, 11 top-level; exit 0 | `/tmp/i1-w16-service-tests.json` |
| W16 plus admission/failure/quick-create service regressions, race detector | 122 passing entries, 18 top-level; exit 0 | `/tmp/i1-w16-service-regression.json` |
| Existing production start and agent-status runtime regressions, race detector | 2 passing tests; exit 0 | `/tmp/i1-w16-runtime-regression.json` |
| Existing iteration package tests | exit 0 | Session command output |
| `go vet` on service and iteration | exit 0 | Session command output |
| gofmt and diff whitespace checks | no findings; exit 0 | Session command output |

The W16 fixture matrix covers active/planned start, unchanged issue facts and
original commitment, repeated calls and later attempts, both legal start phases,
all nonstarting phases, admission refusal, already-started participation,
recorder failure rollback, publication only after commit, two-connection
join/leave fence races, issue-null-to-linked races, deleted optional quick-create
source, concurrent starts, new participation, and clock regression ordering.
The join/leave races prove actual PostgreSQL blocking with `pg_blocking_pids`,
then take the issue lock from the fence owner to detect an inverted lock order.

Final DB commands:

```sh
export DATABASE_URL='postgres://multica:multica@127.0.0.1:5432/multica_i1_w16_20261006b?sslmode=disable'

bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test \
  -race -p 2 -parallel 2 ./internal/service \
  -run '^TestStartTaskIteration' -count=1 -json \
  > /tmp/i1-w16-service-tests.json

bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test \
  -race -p 2 -parallel 2 ./internal/service \
  -run '^(TestStartTaskIteration|TestTriageAdmission|TestEnqueueQuickCreateTaskWithSourceContextIsAtomic|TestHandleFailedTasksIteration)' \
  -count=1 -json > /tmp/i1-w16-service-regression.json

bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test \
  -race -p 2 -parallel 2 ./cmd/server \
  -run '^(TestStartTaskSkipsUnchangedAgentStatusWriteAndBroadcast|TestRefreshAgentStatusFromTasks)$' \
  -count=1 -json > /tmp/i1-w16-runtime-regression.json
```

Additional commands executed successfully:

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -p 2 ./internal/iteration -count=1
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server vet ./internal/service ./internal/iteration
gofmt -l server/internal/service/task_start.go server/internal/service/task_start_iteration_test.go server/internal/iteration/record_start.go
git diff --check
```

Root's independent W16 source review found no blocker. This lane did not run the
full Go repository suite, remote CI, browser/Electron E2E, or a performance gate.
Final foundation integration still owns complete writer coverage, authorization
and settings/capability delivery, notification cleanup and FG evidence.
