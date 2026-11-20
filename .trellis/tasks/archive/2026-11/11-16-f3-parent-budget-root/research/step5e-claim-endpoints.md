# Step 5e: the claim endpoints of the budget step (handler half)

Lane 5e, 2026-11-19. Worktree `/Volumes/artisan/code/2026/mc-f3-budget-5e-20261119`
(a scratch copy of the task's tree taken at 14:33; nothing committed).

## 1. State

Done. One file was added, `server/internal/handler/daemon_claim_budget_test.go`:
12 tests (30 with subtests) for H01 to H11 plus one addition (H06b). They pass
on both toolchains and under `-race`, with no skip. The whole handler package
passes with them: 3040 passed, 0 failed, 53 skipped, which is the baseline
plus these 12. All seven mutants were killed (the sixth as two, and three
more) and the files restored. No defect was found in the handler lines and no
production code was changed. The two inventories are in sections 4 and 5; the
laps were measured three times after 17:45 (section 6). Four points need a
ruling (section 8). The three databases of this lane are dropped. Nothing was
committed.

## 2. Tests

One file, `server/internal/handler/daemon_claim_budget_test.go` (1,054 lines,
SHA-256 `bf7626d5…7d67f38`): 12 top-level tests, 30 with subtests. Every test
builds its own workspace, runtime, agents, issues and tasks through
`internal/testutil`, enables its root through `IssueBudgetService.Enable`,
and drives the real handler with `testutil.Call(…).Want(…)`. Expectations were
written from the brief and design 10.10 before the first run; none was changed
after a run.

| Id | Test | What is asserted |
| --- | --- | --- |
| H01 | `TestClaimBudgetSingleRuntimeDeclaredProtocol` | 200 with the queued task; task `dispatched`; the payload's `budget_attempt_id` is the id of the one attempt row of (task, claim generation); the task has one attempt row; `protocol_version` 1; no `legacy_protocol` observation. |
| H02 | `TestClaimBudgetSingleRuntimeUndeclaredProtocol` | No header: the payload still carries the attempt id; `protocol_version` NULL; exactly one `legacy_protocol` observation, and it is a `coverage_gap` naming the attempt, the task and the claim generation. |
| H03 | `TestClaimBudgetBatch` (`declared`, `not declared`) | A root-issue task and a child-issue task in one batch: two items, both `dispatched`, each payload names its own attempt row and the two ids differ; `protocol_version` 1 / NULL; `legacy_protocol` gaps 0 / one per claim. |
| H04 | `TestClaimBudgetOutsideEveryRoot` (single runtime and batch, each declared and not) | The workspace has a root on another issue. The payload, decoded into `map[string]json.RawMessage`, has no `budget_attempt_id` key; the row counts of the six budget tables are unchanged. |
| H05 | `TestClaimBudgetCapabilityHeader` (5 rows) | `a, budget-usage-v1 ,b` gives `protocol_version` 1; `budget-usage-v10`, `budget-usage`, `Budget-Usage-V1` and an empty header give NULL. Single-runtime endpoint, read off the attempt row. |
| H06 | `TestClaimBudgetSingleRuntimeBindingInProgress` | With the binding lock held by a second connection (seen in `pg_locks`): 200, body byte-equal to the endpoint's no-task answer `{"task":null}`; task `queued`, its claim generation one further (a claim was made and handed back); no budget row and no task token written. After the lock is released the next claim returns the task, one generation later, `dispatched`, with the `budget_attempt_id` of its attempt row; the task has exactly one attempt row. |
| H06b | `TestClaimBudgetBindingInProgressInvalidatesEmptyVerdict` (an addition, see section 8) | The same with an empty-claim cache installed: an empty verdict stored for the version sampled before the refusal does not hide the requeued task from the next claim. |
| H07 | `TestClaimBudgetBatchBindingInProgress` | Two tasks (a member issue, an issue outside every root), lock held: 200, body byte-equal to the empty batch `{"tasks":[]}`; both `queued` one generation further; no budget row, no token. After release the next batch returns both, `dispatched`; the member item names its attempt row, the other has no key; one attempt row in total. |
| H08 | `TestClaimBudgetFinalizeFailure` (`single runtime`, `batch`) | An attempt row planted at (task, next claim generation) makes the budget step's insert fail. Single runtime: 500 `failed to finalize task claim`. Batch: 200 with no task. Both: task `queued` one generation further, budget row counts unchanged, the only attempt of that generation is the planted one, no token. |
| H09 | `TestClaimBudgetDaemonRPC` (3 rows) | `DaemonRPCHandler(ctx, identity, "tasks.claim", body)`: capabilities `admin_cancel_ack_v1, budget-usage-v1` give an attempt with `protocol_version` 1; other capabilities only, or none, give NULL; the payload names the attempt row each time. |
| H10 | `TestClaimBudgetClosedRoot` | Root closed through `IssueBudgetService.Close`: 200 with the task, `dispatched`, `budget_attempt_id` = its attempt row. Premise checked: a `would_refuse` observation names that attempt. |
| H11 | `TestClaimBudgetBindingInProgressLogsAtInfo` (`single runtime`, `batch`) | `slog` captured: exactly one record says "budget binding in progress" for the task and it is `level=INFO`; no record naming the task is `level=ERROR`. |

## 3. Mutants

One at a time (`/tmp/f3-s5e/mutants.py`): the mutant is written into the file,
`-run '^TestClaimBudget'` runs, the file is restored from a copy and its
SHA-256 compared. All were killed. The three production files have their
recorded hashes again (`shasum -a 256 -c /tmp/f3-s5e/pristine.sha256`: OK,
checked again after the last run); only their modification times are today's.

| # | Mutant | Tests that failed |
| --- | --- | --- |
| 1 | `claimBudgetProtocol` always "none" | H01; H03 `declared`; H05 `among others, with spaces`; H09 `declared among others` |
| 2 | `claimBudgetProtocol` always V1 | H02; H03 `not declared`; H05, the four undeclared rows; H09 `other capabilities only` and `none` |
| 3 | Batch endpoint leaves `budget_attempt_id` out | H03, both; H07; H09, all three |
| 4 | Single-runtime endpoint leaves it out | H01; H02; H06; H10 |
| 5 | Binding branch of the single-runtime endpoint answers 500 | H06; H06b; H11 `single runtime` |
| 6a | Binding branch of the single-runtime endpoint does not requeue | H06; H06b; H11 `single runtime` |
| 6b | Binding branch of the batch endpoint does not requeue | H07; H11 `batch` |
| 7 | `omitempty` removed from the field | H04, all four; H07 |

Three more, not asked for: the single-runtime refusal logged at error (H11
`single runtime` failed), the batch refusal logged at error (H11 `batch`),
and the RPC claim not copying the connection's capabilities,
`daemon_rpc.go:112` (H09 `declared among others`).

H06b was also checked against its own fake: with the requeue's version bump
dropped by the fake, the last claim found nothing and the test failed.

## 4. Entry-point inventory

Paths are under `server/`. Found by searching for the callers of
`FinalizeTaskClaim`, of the service's claim functions and of
`ClaimCollaborationRuntimeRun`, the daemon and collaboration route groups,
what the WebSocket hub can push, and outbound HTTP calls. No code was changed.

**Through `FinalizeTaskClaim`.** It has two callers and no other
(`internal/handler/daemon.go:1921` and `:3689`); three ways lead to them.

| # | Entry point | Where | Class |
| --- | --- | --- | --- |
| 1 | `POST /api/daemon/runtimes/{runtimeId}/tasks/claim`, one task per request | `cmd/server/router.go:1573`; `ClaimTaskByRuntime` `daemon.go:3551`; claim `:3596` (`ClaimTaskForRuntime`, `internal/service/task.go:3584`); finalize `:3689` | (a) or (b), by the task |
| 2 | `POST /api/daemon/tasks/claim` and its alias `POST /api/daemon/claim`, a batch | `router.go:1577`, `:1578`; `ClaimTasksByRuntime` `daemon.go:1750`; claim `:1851` (`ClaimTasksForRuntimes`, `task.go:3934`); finalize `:1921`, once per task | (a) or (b), per task |
| 3 | WebSocket RPC `tasks.claim` | `router.go:1328`; `internal/handler/daemon_rpc.go:48`, case `:59`, `rpcClaimTasks` `:66`, calls `ClaimTasksByRuntime` at `:119`, so the finalize call of row 2. Capabilities: captured at connect (`daemon_ws.go:73`), copied at `daemon_rpc.go:112` | as row 2 |

A claim whose response was lost is handed out again by the same service calls
(`task.go:3634`, `:4018`). Both reclaim queries advance `claim_generation`
(`pkg/db/queries/agent.sql:985`, `:1059`), so that hand-out passes
`FinalizeTaskClaim`, and with it the budget step, a second time for the same
task under a new generation. What the budget step does with the task's earlier
attempt is the service's side (step 5d); not looked at and not tested here.

Which class a task is, is decided in one place, `claimSubject`
(`internal/service/issue_budget_finalize.go:187`): (a) a task with an issue
(`:188`), or a quick-create task whose context names a parent issue
(`:191`-`:207`); (b) everything else (`:192`), for which the budget step
returns at `:222` before the try lock, so a class (b) claim is never refused
for a binding. By the query that queues the task:

| Query | Where | Class |
| --- | --- | --- |
| `CreateAgentTask`, `CreateDeferredChannelIssueTask`, `CreateDeferredAgentTask` | `agent.sql:337`, `:392`, `:491` | (a): `issue_id` is a parameter. Their callers were not walked; one that passes NULL would be (b). |
| `CreateQuickCreateTask`, `CreateManualQuickCreateRetryTask` | `agent.sql:459`, `:662` | No issue; (a) with a context parent, (b) without |
| `CreateChatTask` | `chat.sql:1071` | (b): `issue_id` NULL |
| `CreateAutopilotTask` (the run-only autopilot) | `autopilot.sql:576` | (b): `issue_id` NULL |
| `CreateRetryTask` | `agent.sql:546` | That of its source: copies `issue_id`, `chat_session_id`, `autopilot_run_id`, `context` |

**Not through `FinalizeTaskClaim`: the known cases, class (c).**

| # | Entry point | Where |
| --- | --- | --- |
| 4 | Collaboration run claim, `POST /api/collaboration/runtime/runs/{runId}/claim`. Host mode (`host_process_supervision_v1`, protocol 3) and contained mode (`linux_pid_namespace_v1`) share the route; the mode is a field of the request. | `router.go:1447`; `internal/handler/collaboration_runtime.go:163`; `CollaborationService.ClaimRuntimeRun` `internal/service/collaboration_runtime_capabilities.go:125`, the claim query at `:249` |
| 5 | Its second step, `POST …/runs/{runId}/permit` (the launch permit) | `router.go:1448`; handler `:205`; `PermitRuntimeRun` `:334`, host mode branches at `:336`, the contained attempt row is written at `:402` |
| 6 | A plugin's external call: `PluginService.InvokeHook` posts to the plugin's endpoint | `internal/service/plugin_hook.go:197`, `callHookEndpoint` `:245`, the request at `:314`/`:329`. Four callers: `internal/handler/plugin_hook.go:85` (a person, `router.go:1637`); `internal/handler/plugin_agent_hook.go:60` through `plugin_agent_tools.go:138` (an agent's tool call during a task, `router.go:1568`); `internal/scheduler/jobs_plugin_hook.go:274`; `internal/service/plugin_event_dispatch.go:306` |

**Fits none of the known cases. Reported, not classified.**

| # | What | Where |
| --- | --- | --- |
| 7 | Work handed to a daemon in the heartbeat answer, outside the task queue: a CLI update, a model list, a local-skill list, local-skill imports. No task row and no claim. | `processHeartbeat` `daemon.go:1406` (`:1434`, `:1464`, `:1492`, `:1524`-`:1543`), answered over HTTP (`:1291`-`:1304`) and over the socket (`:1323`); the nudge is `requestDaemonPendingWork` `internal/handler/runtime_models.go:445` |
| 8 | A command run on a cloud node: `POST /api/cloud-runtime/nodes/exec` is forwarded to the cloud runtime service with the user's id. No task row. | `router.go:2667`; `internal/handler/cloud_runtime.go:109`, forwarded at `:145` |
| 9 | Model calls the server makes itself, with no daemon, runtime or task | `internal/handler/issue_creator_recommendation.go:135`; `issue_description_assist.go:117`, `:131`; `chat_title.go:165`; the collaboration knowledge automation worker (`internal/service/collaboration_knowledge_automation_*.go`, not opened) |
| 10 | `GET /api/daemon/runtimes/{runtimeId}/tasks/pending` returns a runtime's queued and dispatched rows without claiming: no state change, no token. Nothing under `server/` calls it. | `router.go:1581`; `daemon.go:3872` |
| 11 | `TaskService.ClaimTask`, a claim by agent with no runtime. No caller outside tests. | `task.go:3415` |

Rows 9 to 11 hand nothing to a daemon; they are listed because a search for
"an execution starts here" finds them. The hub pushes only wake-ups and
change notices, never a task (`internal/daemonws/hub.go:426`-`:446`; the
relay carries the same four, `internal/daemonws/notifier.go:23`-`:108`).

## 5. Requeue call sites

The error has one producer, the budget step (`issue_budget_finalize.go:236`,
`:248`), which only `FinalizeTaskClaim` calls (`task.go:3871`). The handler
matches it at `daemon.go:1932` and `:3700` and nowhere else. So it can reach a
requeue only after one of the two finalize calls, and there the binding branch
takes it first.

| Line | Belongs to | Requeues when | Binding in progress reaches it |
| --- | --- | --- | --- |
| 1900 | `ClaimTasksByRuntime` (batch) | the task token could not be generated | No: before the finalize call (`:1921`) |
| 1910 | batch | the Remote MCP daemon token could not be generated | No: before the finalize call |
| 1939 | batch, the binding branch (`:1932`-`:1944`) | the finalize call returned the binding error | **Yes.** This is its site: info log at `:1937`, requeue, the task is left out (H07, H11; mutant 6b) |
| 1950 | batch | any other finalize error that is not a knowledge refusal | No: the branch above leaves the loop body at `:1943` |
| 2137 | `failClaimedTaskBeforeLaunch` (`:2122`) | failing a rejected task itself failed | No. Its callers are ten rejections inside `buildClaimedTaskResponse` (`:2277` to `:3496`), all before the finalize call, and `settleKnowledgeFinalizeRefusal` (`:2169`), which is called after the binding branch (`:1945`, `:3712`) and acts only on a `*service.CollaborationError` |
| 2286 | `buildClaimedTaskResponse` (`:2218`) | the task's agent could not be loaded | No: the builder runs before the finalize call (called at `:1879` and `:3630`) |
| 3193 | builder, quick-create | the captured source context could not be loaded | No, same |
| 3330 | builder | the knowledge history load failed for a transient reason | No, same |
| 3473 | builder | the cancel after the worktree version gate failed | No, same |
| 3638 | `requeueFailedClaim`, a closure of `ClaimTaskByRuntime` (`:3637`) | used four times: | |
| 3676 | single runtime | the task token could not be generated | No: before the finalize call (`:3689`) |
| 3685 | single runtime | the Remote MCP daemon token could not be generated | No: before the finalize call |
| 3708 | single runtime, the binding branch (`:3700`-`:3711`) | the finalize call returned the binding error | **Yes.** This is its site: info log at `:3706`, requeue, 200 `{"task":null}` at `:3709` (H06, H06b, H11; mutants 5, 6a) |
| 3724 | single runtime | any other finalize error that is not a knowledge refusal | No: the branch above returns at `:3710` (H08 pins this path's 500) |

All of them call the same `RequeueTaskAfterClaimFailure` (`task.go:3891`),
which puts the exact claim back to `queued` without taking the claim
generation back, and then invalidates the runtime's empty verdict and wakes
the daemon (`task.go:7014`).

## 6. Laps

**These figures are an upper bound.** The loop calls the handler in process
and claims again the moment it is answered: no network, no daemon, no wake-up
to wait for. Go toolchain: go1.27.0 darwin/arm64 (the default). The database
is the shared PostgreSQL container, an x86_64 image running under emulation on
an arm64 host, with other lanes' tests on it. The handler suite has no
empty-claim cache (section 8), so every claim goes to the database.

How it was measured: `/tmp/f3-s5e/zz_laps_test.go` (174 lines), compiled into
the handler package with `go test -overlay /tmp/f3-s5e/overlay.json` and never
written into the tree. Per run: a new workspace, runtime and root; the tasks
queued (with two, the second on a child issue of the root's issue); a second
connection takes the binding lock, a goroutine ends its transaction after the
hold; the loop claims until every task is handed out. A lap is a request
answered with no task. Afterwards every task must be `dispatched`, and on the
single-runtime endpoint the claim generations must have advanced by exactly
laps plus tasks, so every empty answer was a claim made and handed back. Wall
time runs from the lock being taken to the last task handed out. The lock was
held 10.2 to 11.8 ms, 100.3 to 101.3 ms and 1000.3 to 1001.8 ms.

| Series | Start to end (the whole command, compile included) | Load averages at start | At end | Log |
| --- | --- | --- | --- | --- |
| 1 | 19:52:17 to 19:52:45 | 4.74 6.45 6.42 | 5.27 6.45 6.42 | `/tmp/f3-s5e/logs/laps-1.log` |
| 2 | 20:03:51 to 20:05:15 | 9.79 7.76 7.13 | 11.73 9.55 7.91 | `laps-2.log` |
| 3 | 20:55:41 to 20:56:16 | 5.61 4.83 5.28 | 4.66 4.68 5.21 | `laps-3.log` |

Series 1 is the measurement as briefed. Series 2 and 3 first run six
unmeasured one-second runs (two per endpoint and queue length) and add a row
for the batch endpoint (section 8). All three: exit 0, five runs per cell.

Laps, minimum / median / maximum of five runs:

| Endpoint, tasks queued | Lock held | Series 1 | Series 2 | Series 3 |
| --- | --- | --- | --- | --- |
| single runtime, 1 | 10 ms | 0 / 0 / 0 | 0 / 1 / 1 | 1 / 1 / 1 |
| single runtime, 1 | 100 ms | 2 / 3 / 5 | 5 / 7 / 8 | 9 / 9 / 10 |
| single runtime, 1 | 1 s | 72 / 89 / 96 | 65 / 70 / 80 | 90 / 91 / 95 |
| single runtime, 2 | 10 ms | 0 / 0 / 1 | 0 / 1 / 1 | 0 / 1 / 1 |
| single runtime, 2 | 100 ms | 10 / 10 / 11 | 7 / 7 / 8 | 7 / 9 / 10 |
| single runtime, 2 | 1 s | 84 / 95 / 99 | 54 / 77 / 81 | 71 / 91 / 92 |
| batch, 2 | 10 ms | not run | 0 / 0 / 0 | 0 / 0 / 0 |
| batch, 2 | 100 ms | not run | 2 / 4 / 4 | 3 / 4 / 4 |
| batch, 2 | 1 s | not run | 28 / 32 / 40 | 41 / 43 / 44 |

On the batch endpoint one lap hands back every queued task of the runtime.
Task claims handed back there: 100 ms, 5 / 8 / 8 and 6 / 8 / 8; 1 s,
57 / 65 / 80 and 83 / 86 / 89 (series 2 and 3).

Wall time in ms, minimum / median / maximum:

| Endpoint, tasks queued | Lock held | Series 1 | Series 2 | Series 3 |
| --- | --- | --- | --- | --- |
| single runtime, 1 | 10 ms | 24.9 / 29.6 / 165.4 | 16.3 / 23.0 / 24.9 | 20.4 / 23.1 / 24.8 |
| single runtime, 1 | 100 ms | 107.9 / 114.3 / 118.2 | 105.5 / 114.6 / 116.3 | 103.8 / 106.4 / 112.6 |
| single runtime, 1 | 1 s | 1005.5 / 1009.7 / 1015.0 | 1005.3 / 1010.2 / 1022.1 | 1004.6 / 1010.6 / 1012.3 |
| single runtime, 2 | 10 ms | 28.1 / 33.9 / 42.2 | 26.6 / 39.5 / 48.8 | 26.1 / 35.9 / 37.3 |
| single runtime, 2 | 100 ms | 116.0 / 124.7 / 125.7 | 122.7 / 124.2 / 135.0 | 117.6 / 125.2 / 128.8 |
| single runtime, 2 | 1 s | 1015.6 / 1021.0 / 1028.2 | 1018.7 / 1021.0 / 1022.7 | 1022.2 / 1024.5 / 1029.4 |
| batch, 2 | 10 ms | not run | 25.7 / 26.1 / 30.8 | 25.6 / 29.2 / 33.1 |
| batch, 2 | 100 ms | not run | 120.0 / 124.8 / 135.2 | 114.0 / 119.6 / 134.5 |
| batch, 2 | 1 s | not run | 1021.7 / 1026.1 / 1030.4 | 1014.1 / 1026.4 / 1031.1 |

What the figures say, and no more:

- The laps do not multiply with the queue on the single-runtime endpoint: 91
  a second with one task and 91 with two (series 3, medians). One request
  claims one task, so two tasks take turns.
- On the batch endpoint the requests halve and the task claims handed back
  stay about the same: 43 laps and 86 claims a second (series 3, medians).
- One lap took about 11 ms at a load near 5 (series 3) and 13 to 14 ms at a
  load near 10 (series 2).
- The 100 ms cells of series 1 differ between one task and two (median 3 and
  10), and the first run of its first cell took 165 ms. The first claims of a
  process are slower (each pool connection prepares each statement once), the
  one-task cells ran first, and the 10 ms cells before had no lap, so the
  refusal path ran for the first time in the 100 ms cell. With the warm-up
  the two cells agree (7 and 7; 9 and 9). That the warm-up is the cause is my
  reading; the agreement is measured.
- Each lap is, per task: a claim that advances `claim_generation`, a finalize
  transaction rolled back, a requeue, an agent status reconcile, a
  `task:queued` broadcast and a wake-up (`task.go:3891`-`:3904`), and one info
  log line. A task refused for one second went 90 generations on.

## 7. Verification

All from `server/` of the worktree, with the briefed `TMPDIR`, one command at
a time. "Default" is go1.27.0 darwin/arm64; "1.26.9" is
`GOTOOLCHAIN=go1.26.9 GOPROXY=off` (go1.26.9 darwin/arm64). Counts are
`--- PASS` / `--- FAIL` / `--- SKIP` lines of the log, top level first, then
with subtests.

| # | Command | Toolchain | Exit | Result |
| --- | --- | --- | --- | --- |
| 1 | `gofmt -l internal/handler/daemon_claim_budget_test.go` | default | 0 | prints nothing |
| 2 | `go build ./...` | default | 0 | no output |
| 2 | `go vet ./internal/handler/` | default | 0 | no output |
| 2 | `go build ./...` | 1.26.9 | 0 | no output |
| 2 | `go vet ./internal/handler/` | 1.26.9 | 0 | no output |
| 3 | `go test ./internal/handler/ -run '^TestClaimBudget' -count=1 -v`, fresh clone `f3_budget_s5e_b` | default | 0 | `ok` 3.459s; 12 / 0 / 0; with subtests 30 / 0 / 0 |
| 3 | the same, same clone | 1.26.9 | 0 | `ok` 8.259s; 12 / 0 / 0; 30 / 0 / 0 |
| 3 | the same with `-race`, same clone | default | 0 | `ok` 13.620s; 12 / 0 / 0; 30 / 0 / 0; no `DATA RACE` |
| 4 | `go test ./internal/handler/ -run . -count=1 -v -timeout 45m`, fresh clone `f3_budget_s5e_c` | default | 0 | `ok` 329.109s; 3040 / 0 / 53; with subtests 5985 / 0 / 53 |

- Rows 1 and 2 ran 20:06:49 to 20:07:16. Row 3: 20:08:37, 20:09:28, 20:10:21.
- Row 4 ran 20:14:40 to 20:20:25, load averages 15.77 16.49 11.65 at the
  start and 10.14 12.47 11.37 at the end. `-run .` is the run script's way of
  selecting everything. 3040 is the baseline's 3028 plus this lane's 12; the
  skips are the baseline's 53, none of them mine (mine: 12 passed, 30 with
  subtests, 0 skipped). The three tests known to fail on this machine passed
  in this run. No `too many clients` in the log.
- Logs: `/tmp/f3-s5e/logs/verify-static.log` (empty), `verify-focused-default.log`,
  `verify-focused-go1269.log`, `verify-focused-race.log`, `verify-whole.log`.
- The test file after the last run: SHA-256
  `bf7626d55f4ba30e430cdd60340c6c7f35e7bfc445a37dbd27f1fd1c67d67f38`. `git
  status` of the worktree shows one path of this lane, the new test file;
  `daemon.go`, `agent.go` and `daemon_rpc.go` have their recorded hashes;
  `zz_laps_test.go` is not in the tree.

## 8. Needs a ruling, and deviations from the brief

No expectation of H01 to H11 turned out to be wrong about the design, and no
test showed a defect in the handler lines. No production code was changed.

**Needs a ruling.**

1. **H06b: keep or drop.** The point of H06 ("a cached 'nothing to claim'
   verdict must not hide the requeued task") cannot be shown by H06 in this
   suite: `TaskService.EmptyClaim` is nil there. The cache needs Redis and is
   wired only in `cmd/server/router.go:1316`, so there is no verdict to hide
   anything, and H06 as briefed pins the row and the next claim only. I added
   H06b, which installs a real `EmptyClaimCache` over an in-memory stand-in
   for Redis (a go-redis hook answering GET, MGET, SET, INCR and EXPIRE; about
   90 lines of test support in the same file), and showed that it fails when
   the requeue's bump is lost. Without it the invalidation is covered only by
   the service's cache tests, which need `REDIS_TEST_URL` and skip by default.
   It is the only test of the file that stands in for a dependency.
2. **The laps** (section 6): the main session decides whether a delay is
   needed. Besides the count, each lap advances `claim_generation` and
   broadcasts `task:queued`.
3. **The reclaim passes the budget step twice** for one task (section 4,
   under the first table). Whether that is covered belongs to step 5d.
4. **Entry points 7 to 9 of section 4** (work handed out in the heartbeat
   answer, a command on a cloud node, the server's own model calls) fit none
   of the brief's cases. I did not classify them.

**Observation, no test.** `requestHasClientCapability` reads the header with
`r.Header.Get`, so only the first `X-Client-Capabilities` line counts; a
capability sent on a second line of that name is not seen. The helper is older
than this step (in the uncommitted diff it appears only as context;
`claimBudgetProtocol` is the new code), and the socket path stores one string.

**Deviations.**

- The final message: the brief says three lines, the team lead's message ten
  at most with the status line, the whole-package counts, the laps in one line
  and what needs a ruling. I followed the later one.
- Mutant 6 was run as 6a and 6b, and three mutants were added (section 3).
- Laps: six unmeasured warm-up runs and a batch-endpoint row were added after
  series 1; three series instead of one (section 6).
- H07 queues one task of a member issue and one of an issue outside every
  root, not two alike. Both are refused while the lock is held, because the
  try lock comes before the member row is read
  (`issue_budget_finalize.go:239`-`:249`); the test pins that as well.
- Assertions beyond the list: H06, H07 and H08 require the claim generation to
  be one further after the refusal or failure (a claim was really made and
  handed back); H08 requires the 500 to carry `failed to finalize task
  claim`, so it is known to come from the finalize branch; H10 checks its
  premise (a `would_refuse` observation names the attempt).
- H09 is driven at the level of the RPC handler with an identity, not through
  a socket (section 9).
- Two tests swap a process-wide value and restore it: H11 the default `slog`
  logger (and the `log` package's writer and flags, which `slog.SetDefault`
  redirects), H06b `testHandler.TaskService.EmptyClaim`. Neither is parallel.
- The tests carry no skip for a missing database: the package's `TestMain`
  exits with status 2 without a usable `DATABASE_URL`
  (`handler_test.go:44`-`:63`), so in this package a missing database fails
  the run instead of skipping.
- Reading beyond section 2 of the brief, for the two inventories: searches,
  and narrow ranges of `daemon.go`, `collaboration_runtime.go`,
  `issue_budget_finalize.go:160`-`:271`, the router's route groups and the
  column lists of the queue's insert queries. No file was opened whole.
- Brief against design: no disagreement found in what I read (design 10.10
  and 10.11). The design cites `RequeueTaskAfterClaimFailure` at
  `task.go:3872`; in this tree it is at `:3891`.

## 9. Not done

- The capture of the header when the daemon's socket connects
  (`daemon_ws.go:73`) is not driven by a test: the handler package has no
  harness that dials that socket. The path is `daemon_ws.go:66`-`:74` into
  `daemonws.ClientIdentity.Capabilities` (`internal/daemonws/hub.go:38`), then
  `daemon_rpc.go:112`, `ClaimTasksByRuntime`, `claimBudgetProtocol`. H09 and
  the extra mutant cover it from the identity on.
- The wake-up after a requeue is not asserted (the brief says not to).
- Laps with a real daemon and a network between the claims: not possible under
  the rules of this lane, hence the upper bound.
- No `-race` run of the whole package and no run of any other package: not
  asked for. `internal/service` belongs to another lane.
- The callers of the three issue-task insert queries were not walked
  (section 4), and the collaboration knowledge automation files were not
  opened.

## 10. Databases

All clones of `f3_budget_s5v_tmpl`, created and dropped by this lane with
`docker exec multica-postgres-1 psql`:

| Database | Created | Used for | Dropped |
| --- | --- | --- | --- |
| `f3_budget_s5e_a` | 17:10 | writing the tests, the mutants, the three laps series | 21:13 |
| `f3_budget_s5e_b` | 20:08 | the three focused verification runs | 21:13 |
| `f3_budget_s5e_c` | 20:13 | the whole-package run | 21:13 |

No `f3_budget_s5e_*` database is left. The template and every other database
were not connected to, changed or dropped.

## 11. Hooks or reminders that asked for an edit outside this lane's files

None asked for one. What was injected and not acted on: the session-start
workflow text (it routes work to skills and sub-agents; the team lead's
message says to start none), the standing memory instructions (nothing was
written under `~/.claude/**`), and, after a context compaction, a notice
asking for a one-line acknowledgement in the reply, which was given and
touched no file. Nothing under `.trellis/` was written except this report.

STATUS: complete
