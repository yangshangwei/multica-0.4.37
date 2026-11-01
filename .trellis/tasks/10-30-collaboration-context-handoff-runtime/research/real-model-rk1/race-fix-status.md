# RK1 race investigation (live)

Owner: `/root/rk1_race_fix`; worktree `mc-rk1-framework-20261101`; historical product HEAD `b8fd547dc`. All existing framework files and original failures are preserved. No database command or Provider call has been run by this implementer.

## Original failure is real telemetry incompleteness

The retained `fake-proof-race/wrong_continues/RK1-H3b-t1/public.json` has proof `host_exit_observed`, settled success, 5 checkpoint events and **no recording_seal event**. The HTTP trace likewise contains no `/recording/seal` request. One checkpoint request returned 503 after 1.003143 seconds, then the terminal recovery sent two checkpoints, outcome and proof. No trace/database sampling wait can make an unsent seal appear.

The historical production `recoverHost` shares a 1-second `hostRecordingRecoveryContext` between checkpoint replay and its deferred seal. The two recovered chunks consumed that shared budget; the seal is staged but its expired context prevents HTTP. That is the intended bounded telemetry/proof behavior, with pending recording retained for a later daemon recovery round, and related runtime repairs are not yet in this branch. The current fixture executes only Runner.Run once and does not run the production daemon reconciliation loop. Its paid stop rule correctly marks this sample incomplete. Do not relax lifecycle assertions, add sleep, extend scenario timeout or silently run extra recovery to turn the original attempt green.

## Framework evidence gaps to address

- Preserve the run-owned ledger under the private trial directory instead of only `t.TempDir`, so pending chunk/seal state survives a failed fixture. It contains credentials and must never be copied into public evidence.
- Record sanitized Runner error in the public run summary alongside the existing precise lifecycle reason, rather than hiding the terminal context deadline behind the earlier missing-event classification.
- Trace collection forwards bytes before adding the call: a client may consume a Content-Length response before the handler appends its entry. Investigate a deterministic completion barrier to take an actual finished-handler snapshot; this is separate from the observed unsent seal and cannot repair it.

## Verification needed from coordinator

Request a new explicitly owned, migrated `multica_rk1_*` cmd/server database for fake/trace regression verification after the evidence changes. Proposed command: `env -u MULTICA_RUN_REAL_AGENT_SMOKE -u MULTICA_KNOWLEDGE_CONTEXT_ENABLED -u GIT_CONFIG_COUNT -u GIT_CONFIG_KEY_0 -u GIT_CONFIG_VALUE_0 DATABASE_URL=<coordinator-provided URL> RK1_FAKE_PROOF_OUT=<new private absolute output dir> bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -json -race -tags=agentintegration ./cmd/server -run '^(TestRK1FakeProof|TestRK1ProviderWithoutMarkerCannotEnterTestMain|TestRK1RealProvider|TestRK1Trace.*)$' -count=1 -timeout=30m`.

Do not migrate or create a DB from this child. No final-source binding: `runnable=false`, source_id empty, runtime budget25 consumed0. A repeat on old product can remain incomplete; the final candidate must rerun after actual runtime/knowledge repairs merge.

## Implemented evidence corrections

- A fake HTTP handler that wrote its Content-Length response but had not returned reproduced a snapshot with zero trace calls. The canonical `TestRK1TraceSnapshotWaitsForCompletedHandler` now waits for the already-started handler to finish and captures it exactly once. Race red:1 failure. Race green:`-count=20`,20 passes,0 fail/skip. The barrier is a WaitGroup join after Runner/Provider work stops; it does not sleep, poll for a wanted event, issue recovery calls or alter HTTP payloads.
- Per-run ledgers now persist in the private trial directory (`ledger/<run>/`) with the existing private directory/file primitives; fake assertions require one private run record for every observed run, including incomplete attempts. No ledger bytes enter public JSON or Git.
- Public run summaries retain the scrubbed/size-limited Runner error as `runner_error` independently of lifecycle classification. Missing seal remains incomplete; stop rules and runtime deadlines are unchanged.
- Root allocated explicit cmd/server-only test DB `multica_rk1_trace_root_20261102_0000`, verified/migrated by root. Permission is `.omx/private/rk1-race-fix-root-20261102/db-permission.json`. Full fake/trace race verification is running via the exact permitted test command; child made no DB commands and did not access any other database.

## First allocated rerun: setup error, preserved

The first full race rerun exited1 before any Provider launch: the new retained ledger path used a nested `ledger/<run>` directory, but OpenLedger deliberately creates only its final directory and the intermediate parent did not exist. This is my avoidable fixture-construction error, distinct from the original telemetry failure. Added private parent creation before OpenLedger. The failed cmd/server database and output remain untouched; no migration or cleanup was attempted. The original trace barrier unit still passes20 race repetitions.

Request a fresh cmd/server-only `multica_rk1_*` DB permission for the corrected full race rerun, preserving the same tests and timeout. Do not reuse the failed DB for final full-suite proof.
