# R22 original capabilities-read RED boundary check

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Verdict — NOT ACCEPTED

The frozen original RED boundary has one concrete blocker: its original registration writer control cannot reach the advertised account/runtime barrier. No Go/test/DB/API/build/process command has run in this review, and no observed RED result is claimed.

Original test SHA256 `d7965cbca8da0f034afa6527bde5cd2ec4b2594b01184fa8397876d482abaa11`; original5047-file manifest SHA256 `f42869d3cde43ac80f091ab984c786161017eb3ebd3adefc29df59b57bb4884b`; original inventory SHA256 `30160de581d174a2a87a9cd33e089d0ed4119c22dd0073eb189c93380f5f4837`; draft SHA256 `4b7ed6fb5722448627e2648e45776880f4ca5f0cae581a4c29a7b2638673f1f1`. Preserve all four original inputs unexecuted.

## Blocking finding R22-RED-W01

At new test line175, `TestCollaborationRuntimeCapabilitiesReadOriginalWriterFence` increments `f.registration.Capabilities.Limits.FileCount`. Its existing `preparationRegistrationRequest` initializes FileCount32 (`collaboration_runtime_preparation_capabilities_test.go:28`). The protocol's `preparationLimits` requires FileCount≤32 (`collaboration_preparation.go:264`). Incrementing produces33.

`RegisterRuntimeCapabilities` validates the entire summary at `collaboration_runtime_preparation_capabilities.go:88–92`, before `collaborationRuntimeMutation` or any managed account/runtime locks. This control returns400 `invalid_runtime_capabilities`; it cannot reach its `LockAgentRuntime` barrier, assert real writer exclusion or observe the declared epoch/summary/invalidation commit. Therefore the intended two failing coexistence leaves plus five passing control leaves are not a supported original-RED expectation.

The minimal correction is a **separate frozen test-only supplement** changing that one increment to a valid changed summary, such as FileCount31 via `--`. Keep original inputs, two coexistence assertions, writer refusal/final commit assertions, all four authority controls and deadlines unchanged. The checker made no test or product fix.

## Remaining static boundary — supported

The existing managed fixture writes real password credentials, active installation/binding/runtime rows and sets password authentication. `newStageRuntimeFixture` creates separate real QA/reviewer/release users with password version1 and source participation. The new fixture promotes only the actual QA's source membership to admin, uses `ownerSwitchNamespace` for a distinct real QA-owned namespace/binding, registers its capabilities and asserts matching session/owner/binding principals.

The artifact prerequisite uses existing actual packager bytes/canonical spec and retained fixture execution data, then performs actual QA R4 and checks the exact candidate with a nonnull artifact baseline. No collaboration run is created. This is a synthetic read-authorization prerequisite, not native Host execution or real external acceptance.

The actual GET path is `RuntimeCapabilities -> collaborationRuntimeMutation -> lockPreparationRuntime -> authorizeCollaborationDaemon -> LockManagedRuntime -> LockPasswordSession/managed namespace/current account/admission checks -> LockAgentRuntime`. The same next-query needle is reachable on a **valid** registration writer. `LockPasswordUser` is real `FOR UPDATE`; R4's artifact-user fence is real `FOR SHARE NOWAIT`. The passthrough barrier observes actual PostgreSQL PIDs and forwards original Query/QueryRow results.

The reverse order copies the stage fixture solely to select QA's context and reuses the real R4 post-authority barrier. In both read orders the second request's five-second context starts before the held request's; the second must finish before release, so the held request's expiry cannot release its lock and produce an apparent success. Distinct actual backend PIDs, exact responses/baselines, buffered results, `sync.Once` release, deferred channel close and cleanup release/cancel/drain remain in place.

Password change, account disable, binding revoke and the other real namespace credential all reach existing current-authority checks that map refusal to `managed_runtime_required` and return zero capability data. Field/types used by the fixture match their existing definitions. This is source inspection; no compilation/typecheck or observed authority test is claimed.

## Source, inventory and execution inputs

Independently hashed all5047 current frozen files and modes. The path set exactly matches the manifest; all5046 baseline entries are identical; the new concurrency test is the only added path. No missing/extra file or hash/mode drift was found.

The declared static inventory is exactly4 top-level/8 named/7 leaf: two intended coexistence failures, one intended writer pass and four intended authority passes. The enclosing authority parent would also pass, giving intended named2FAIL/6PASS. The writer blocker prevents accepting that expectation until a corrected separate boundary is checked.

Draft4b7ed6fb has11 matching references, the exact frozen cwd/source manifest, anchored four-test selector, full eight-name inventory, red profile `multica_pay241_20261116_s2_capread_red_r22`, one original `go test -json ./internal/service ... -count=1 -timeout=5m` command and fresh absent output. Its arguments/input/source/profile guards are supported, but the draft is **not accepted for execution** while the original writer control is invalid.

Allocator and runner adaptations preserve their original task/package/source/audit/parser/cleanup mechanics; changes are registered R22 paths/names/output labels. Python AST parsing succeeded without importing or executing them. The runner rechecks all required hashes and full source paths/bytes/modes before and after, uses an explicit allowlisted offline environment and registered database, records real argv/raw/exit, requires current DB identity/empty fixture/zero other connections, and checks named/package terminals, duplicates, parse/build failures and after cleanup. It reports actual failed controls; “observation complete” does not imply expected-RED acceptance.

Prepared red/green/race receipts and environment hashes match. Each0600 environment records the correct task database, package `./internal/service`, offline Go settings and matching trusted0700 HOME/temp chains; all TMP keys use that trusted temp. The original donor and three clone snapshots have zero other connections,853 ledger entries,663 indexes and211 table counts with identical schema/counts. These are recorded allocator observations, not new/current DB checks. Leader owns resource budgets and actual execution.

## Ownership and next gate

Only this report and its JSON companion were written. No source/test/parent/task/plan/spec or old report was changed. No Go/tests/DB/API/build/process/Git action was executed. The original R20 holder remains unproved and business acceptance increment remains0.

Freeze/check the valid one-line test-only supplement and a new exact command binding separately before running an original-product RED. This original NOT_ACCEPTED report and all original inputs must remain unchanged.
