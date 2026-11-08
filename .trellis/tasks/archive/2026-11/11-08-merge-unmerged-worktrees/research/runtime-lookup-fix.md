# Runtime capability lookup instrumentation repair

## Diagnosis and boundary

The first full gate failed `TestNoDirectGetAgentRuntimeCalls` on `auth.CurrentTaskCapability`. Its direct sqlc runtime read bypassed `RuntimeLookup`, leaving capability authorization reads uncounted. The production helper and scanner are byte-identical at integration base e541cfb43 and candidate 18e43dc93; archived `baseline-blobs.json` records both object IDs. `guard-before.log` reproduces the exact failure on the candidate. This is a pre-existing omission, not an exemption from the gate.

`auth` cannot import `service` because service already imports auth. Existing three callers are the claim response builder, request authorization, and final claim validation. Each already has access to `service.RuntimeLookup`. Supply its bound `Get` function to the auth helper, preserving lazy lookup after preliminary validation and the same `*db.Queries` snapshot. Handler validation uses task attribution; claim response construction uses daemon_api attribution. Make final validation a TaskService method to bind the transaction queries and the service metrics collector. No new package, query, fallback, source label, scanner allowance, permission rule or dependency is introduced.

Expected product files: auth/task_capability.go and test; handler/task_capability.go, daemon.go and new task_capability_runtime_lookup_test.go; service/task_capability.go and its task.go caller. Tests prove the capability read is counted during finalization, a repeatable-read request still consumes the old runtime owner after a concurrent owner change, current authority correctly rejects the changed owner, and missing/database-error lookups propagate original errors. Existing capability permission/claim/comment regressions remain canonical.

## Evidence

All logs are retained under `/Volumes/artisan/code/2026/mc-merge-takeover-archives-20261108/verification/runtime-lookup-fix/`. Tests use recorded `router.env`, assert actual database `multica_merge_takeover_20261108_router_1`, run through the repository agent CLI guard and disable real-agent smoke. The private runner is `/tmp/multica-runtime-lookup-tests.py`.

- `guard-before.log`: guard fails at the original direct runtime read.
- `behavior-before.log`: initial regression exposed snapshot counter 0; first finalization oracle incorrectly counted the entire claim (8), retained as exploratory evidence.
- `behavior-before-02.log`: narrowed finalization call; two pre-existing managed-runtime reads meant the initial expected2 passed. Retained as test-oracle refinement, not final red evidence.
- `behavior-before-03.log`: final assertions fail on the original production code: finalization2 rather than3; snapshot/current reads0 rather than2.
- `behavior-after.log`: complete handler `^TestTaskCapability` suite passes after repair.

- `auth-metrics-service-after.log`: focused normal auth/metrics/service checks pass, including the unchanged full-source guard (10 top-level /20 total test cases;0 skipped/failed).
- `behavior-after.log`:17 top-level /40 total handler capability cases;0 skipped/failed.
- `focused-race.log`: auth/metrics/service/handler focused race pass (see `results.json` for exact inventory); no race reports, skips or failures. The extra handler runtime source regression matched this broader filter too.
- `vet.log`: `go vet ./internal/auth ./internal/metrics ./internal/service ./internal/handler`, exit0.
- `git diff --check`: pass. Modified Go files formatted with gofmt.

Main session owns independent review, commit and fresh full gate; no merge authorization follows from these targeted results. Root product files were not changed. The parent independently adds the runtime snapshot/metric contract to the spec. No active test process remains from this implementation slice.
