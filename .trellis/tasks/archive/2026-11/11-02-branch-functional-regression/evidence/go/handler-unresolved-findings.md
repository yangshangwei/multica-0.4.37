# Handler first-run claim failures (unresolved)

The full `internal/handler` race package is still running under corrected v2 environment and its fresh assigned database `multica_func7af9_go_33_20261102`. At the time of this note, five top-level claim tests have failed:

- `TestClaimTask_ProjectGithubReposOverrideWorkspaceRepos`: expected a task, received `task:null`.
- `TestClaimTask_ProjectDescriptionInjected`: expected a task, received `task:null`.
- `TestClaimTask_ProjectWithoutRepos_FallsBackToWorkspaceRepos`: expected a task, received `task:null`.
- `TestClaimTask_AutopilotRunOnly_PopulatesWorkspaceAndProjectContext`: expected a task, received `task:null`.
- `TestClaimTaskByRuntime_TaskWorkspaceMismatch_CancelsAndRejects`: expected HTTP 500, received HTTP 200 with `task:null`.

This is separate from the known global-environment contamination of the first four failing packages. A read-only sample during this package recorded 53 / 100 PostgreSQL connections; no cluster-capacity rejection was observed in these failure outputs. Original JSON events remain in `33-internal_handler/stdout.jsonl`.

Source observation (hypothesis only): all five tests select an agent/runtime from the shared `testWorkspaceID` with `LIMIT 1`, without deterministic ordering or creating their own exact agent fixture. See `server/internal/handler/daemon_test.go:2014` and `daemon_test.go:2354`. A shared-suite fixture/state dependency is possible, but the current evidence does not prove it and does not establish a product regression.

Next diagnostic gate: preserve this complete package result/database, have the coordinator allocate another fresh package database, then replay the five original tests unchanged under an explicit new permit and new run ID. Compare actual claim/runtime/queue facts if the refusal reproduces. Do not increase timeouts or alter assertions, and do not equate an isolated replay with a full-suite pass.
