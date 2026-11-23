# SQL-cost diagnostic batch

Active task: .trellis/tasks/11-02-branch-functional-regression.

Continue the approved acceptance and diagnostic scope. Candidate remains 7af9ab9905c98c77bb091223ebe687c639475bb4. Do not create a task, rerun the frozen P95 gate, change a product file/dependency/index, or dispatch remote work or commercial calls.

1. Preserve existing task-file hashes and all working-tree changes. Reattach the existing in_progress task and load its approved context.
2. Add a task-private Go diagnostic through a new-file-only overlay in the isolated Go checkout. Use a fresh explicitly owned migrated database, verify the empty template, actual database identity and shared connection budget, and keep Agent CLI guard/account isolation enabled.
3. Reproduce the canonical 1,000-issue closure population and transaction/lock prerequisites. Capture canonical source-leave, issue-CAS and target-join SQL, actual schema/index definitions, EXPLAIN ANALYZE/BUFFERS/WAL where supported and rollback boundaries. Do not replace SQL or drop/create indexes for a favorable result. Label instrumented execution and cache/order limitations explicitly.
4. If needed, compare serial reads with pipelined transport of identical canonical reads on rollback-only fixtures. Preserve order, values, locks, complete results and error checks. This is a transport experiment, not a production implementation or acceptance pass.
5. Recheck only local platform prerequisites allowed by remaining-platform-handoff-1.md; preserve completed DEB/macOS subgates. Do not access accounts, keys or remote runners.
6. Independently review the saved diagnostic evidence, reconcile report/progress/defects/matrix/session prompt, and verify historical fingerprints, unchanged source/frozen test, owned processes/ports/protocol registrations/mounts and database connections. Keep in_progress and original QA-PERF-001 FAIL while required gates remain open.

Write scope: new task evidence/research and private diagnostic files only; the coordinator also maintains the existing task coordination documents. Expected outcome is measured SQL/index/transport attribution with limits and a concrete next step, not a product fix.
