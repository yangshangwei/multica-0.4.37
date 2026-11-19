# Handler TestMain dependent-target cleanup — 2026-11-18

## Reproduced failure

The current S3 field tests are functionally green, but handler-normal is **NOT_ACCEPTED**: its exact 1 top / 3 named / 2 leaf tests passed, exit 0, source/configuration and gate unchanged, connections 0; `agent_invocation_target` alone remained at count 1 after package teardown. All 30 declared relations were empty before execution and every other final count is 0. The original database, source-verification-r03, raw stream and result remain unchanged.

Private result: `/Volumes/artisan/.multica-acceptance/pay241-s3-cas-20261118-hxjmd77_/verification/handler-normal/result.json`.

A separate fresh database intentionally executed the already compiled handler binary with `-test.run '^$'`, so TestMain ran and zero test bodies matched. It again ended at invocation-target count 1, all other counts 0, exit 0 and no remaining connection. This reproduces the cleanup failure independently of both S3 child CAS bodies.

Private reproduction: `testmain-diagnostic-red-r01/{command,before,after,result}.json` under the same evidence root. The diagnostic is intentional TestMain execution, not a compile-only probe. Both failed databases are retained.

## Root cause

`setupHandlerTestFixture` in `server/internal/handler/handler_test.go` inserts a workspace invocation target for the seeded agent. `cleanupHandlerTestFixture` removes workspace/user rows but does not explicitly remove the new target relation. That relation has no database foreign key/cascade; existing parent cleanup therefore leaves it orphaned. No product priority/CAS defect is demonstrated.

The relevant existing ROOT file matches the frozen R03 bytes. The independent checker verified the same seed/cleanup path. Removing the global seed in an individual child test would withdraw authority while later package tests still need it, so the repair belongs in TestMain teardown.

## Cleanup plan and ownership

1. Widen only the handler implement owner's private write scope to `source-r01/server/internal/handler/handler_test.go`, in addition to its already finished new child test. Preserve the existing file's mode and all other bytes.
2. In `cleanupHandlerTestFixture`, before deleting its workspace, delete invocation-target rows whose agents belong to the selected handler-test workspace. Reuse the current parameterized `pool.Exec`/checked-error pattern and the existing workspace-slug fixture identity. Propagate errors; do not add an FK/cascade, new abstraction, dependency or production change.
3. Keep the package seed available throughout the tests. Preserve unrelated workspaces and their agents/targets by selecting only the actual handler fixture's agents.
4. Keep the earlier R01/R02 unexecuted snapshots, R03 handler cleanup failure and TestMain-only RED unchanged. Build a new R04 candidate that declares the single changed existing fixture file plus the two new S3 test files.

## Verification

- Format, compile-only and vet the exact corrected private handler package; no implicit database fallback.
- Run a fresh TestMain-only GREEN diagnostic against a new recorded database: all 30 scoped relations return to their original empty counts, connection/gate state restored. This is teardown proof, not a named business test.
- Run the unchanged 3-name child CAS inventory in normal and race modes on two fresh recorded handler databases; exact names/terminals, no skips/failure/duplicate/unfinished identities, all 30 final counts 0 and source/config/gate/connection checks pass.
- Reuse accepted service R03 normal/race evidence only after checking that the new delta is exclusively a handler `_test.go` file and is not consumed by the service package. Do not manufacture a new service execution or whole-ROOT pass.
- Independent checker verifies the repair scope, original failure preservation, static/input bindings, raw inventories and full fixture audit before S3 first-increment acceptance.

Full S3, current API/Web/Desktop recovery, real S4/S5 and parent business signatures remain open. This scope addition repairs test infrastructure; it does not alter a product or database migration contract.
