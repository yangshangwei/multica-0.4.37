# Iteration running-task count performance review

Independent read-only review of the final four-file count change. No API/database/test execution by this reviewer. The initial implementation redeclared issueIDs in the same function; author corrected it to resolvedIssueIDs and retains the failed compilation evidence. No remaining source-level blocker found.

## Semantics and isolation

- The service gathers IDs from p.issues, the complete authorized issue set already selected by workspace and lifecycle scope. The new count runs after unchanged iteration, issue, reference and coordinator locks. It uses the same borrowed transaction and changes no Begin/commit/retry/authorization owner.
- Preview retains REPEATABLE READ; Apply retains its existing READ COMMITTED owner. On Apply, this single SQL is still after the issue lock wait, so it sees newly committed task state at the proper fresh statement boundary. Replacing N separate count snapshots with one snapshot provides a consistent instant across the selected set; no user-visible count rule or write hash input changes. Running counts are presentation data and the existing preview-comparison path deliberately zeros them.
- SQL joins agent_task_queue.issue_id to issue.id and filters issue.workspace_id plus the exact selected UUID array. agent_task_queue has no workspace_id column. A foreign workspace requested with these IDs returns no rows; an empty selection returns no rows. A task with a foreign agent reference on an authorized issue preserves the old task-count semantics; agent ownership is not reinterpreted here.
- Only status='running' contributes. Queued/dispatched/waiting-local-directory/completed/failed/cancelled do not contribute, matching the former active-task fetch followed by status filter. GROUP BY returns one int64 count per issue; absent entries default to0. Duplicate input IDs cannot multiply counts through ANY. SQL does not mutate or lock tasks.
- Generated params/row fields and SQL argument order match the source query. Row Close/Err handling is unchanged generated style. Author reports sqlc regeneration idempotence passed. No migration/index/dependency added.

## Test adequacy

New real-service preview regression seeds 2/1/0 running-task counts, all relevant non-running statuses and an unrelated issue, then asserts exact counts and one agent_task_queue query. Author's valid RED retained taskreads3/want1; earlier fixture uniqueness failures are setup evidence, not the product red. Separate direct-query test checks foreign-workspace and empty-set refusal. Existing iteration lifecycle tests assert running task/session/project/issue status are unchanged through start and membership moves; coordinator/settings-lock and stale revision/rollback cases remain in the selected ^TestIteration suite.

Fresh all-iteration service race run is pending. Final performance must separately show all3 original gates without weakening thresholds. A query-count test proves the N+1 removal but is not itself latency acceptance. Root must rebuild the API and current package source/CLI bindings after this production change before final runtime claims.

Exact reviewed hashes are in private backend/iteration-count-independent-review.json.


Verified current `counts-green3/result.json`: all selected ^TestIteration service tests passed under race,65top-level/93named,0FAIL/SKIP,exit0. LogSHA25635adf58278cd21e54afae76bdfc74f25b38c76660ca8e7460c4460ed05588b9c. Original performance2 showed disable-apply improvement4083ms→969ms and triagePASS, but closureP952041.992ms still exceeded unchanged2000ms; performance completion remains open and a separately reviewed preparation batch follows.
