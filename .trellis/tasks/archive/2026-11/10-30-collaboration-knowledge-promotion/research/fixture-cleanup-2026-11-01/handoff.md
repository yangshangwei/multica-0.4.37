# Retained-database fixture cleanup

Only `server/internal/service/task_knowledge_preparation_boundary_test.go` and `server/internal/service/task_knowledge_logging_test.go` changed: nine cleanup lines per file. Each successful service-created source now registers cleanup for its exact issue ID before parent teardown, using the existing `Fixture.Cleanup` pattern. The cleanup includes events, notifications, participants, session, handoff, memory, issue/collaboration rows, exact operation scopes, and the fixture-owned account fences. Knowledge candidates and invalidations remain retained.

The external preparation source is outside the base fixture's workspace, so the inherited cleanup never reached it. The logging probe creates its collaboration source without the shared collaboration fixture. In both cases, legacy workspace cleanup deleted the ordinary issue while leaving a pending event and `issue_collaboration` row. The old realtime drainer repeatedly selected that event and failed its issue lookup. The schema owner independently identified the same two families on main's retained database; this correction used only C2's database.

The ordered reproduction and checks used `multica_knowledge_c2_20261101_7d95f46f` and its private environment file. Commands, UTC timestamps, process results, complete test inventories, and log hashes are in `*.command.json`.

| Sequence | Fixture result | State immediately after fixture cleanup | Existing drainer result |
| --- | --- | --- | --- |
| Before correction | 11 leaf cases pass | 42 pending events for nine deleted source issues | All four tests fail with `no rows in result set` |
| After correction | 11 leaf cases pass | Zero pending events; zero orphan pending events | All four tests pass |
| After correction, race detector | 11 leaf cases pass | Zero pending events; zero orphan pending events | All four tests pass |

The existing drainers are `TestCollaborationMemoryRealtimePublishesOnlyIssueScope`, `TestCollaborationRealtimeOutboxRetriesWithoutPrivatePayloads`, `TestCollaborationRealtimeRetainsFailedSourceInvalidation`, and `TestCollaborationRealtimeRemovalReachesRevokedAccount`. No test assertion or dispatcher logic was weakened. All green runs have zero skips. Normal/race repetitions are not additional independent cases.

For a clean starting point, `prior-owned-orphans.json` records the 30 exact C2 source IDs from these two test families and the 132 old pending events. `red-owned-orphans.json` records the nine newly reproduced source IDs and 42 events. Only those positively identified orphan fixture rows were cleaned between red and green. Both archives prove the protected candidate and invalidation table counts/full-row hashes stayed unchanged during manual cleanup. No manual cleanup ran between the corrected fixture executions and their following drainer tests.

`final-state.json` proves no pending event head remains. All 57 retained source candidate histories from the prior, red, green, and race runs are still present; their transient source rows, sessions, participants, handoffs, notifications, memory, and receipts are gone. `go vet ./internal/service`, formatting, and scoped diff checks passed.

The original acceptance artifacts remain historical evidence. `source-before.json` and `source-after.json` identify the cleanup delta. `production-hashes.json` records the final frozen Go/SQL source state, measured after focused verification; it is not represented as a subprocess compile-start timestamp. This lane changed no production source, sqlc output, or protected history and made no Provider calls or commits. Main owns full-suite acceptance; no broad Go rerun was performed solely for this cleanup.

All source edits are frozen.
