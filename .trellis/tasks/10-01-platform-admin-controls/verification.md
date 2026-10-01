# S04 integration verification

Worktree `/Volumes/artisan/code/2026/multica-platform-admin`; baseline commit `a38158def`. Updated 2026-10-02. Implementation, bounded review/checks and fresh production browser acceptance are complete. S06/S07 remain outstanding.

## Implemented

- Administrative cancellation reuses the existing cancellation/failure transactions and post-commit effects. Each actor/key keeps a receipt and audit while concurrent requests share a cancellation root. Durable root/follower reconciliation preserves unknown/late evidence without reversing the task's terminal state.
- Admission changes take the current actor lock then installation lock, compare the version, and commit policy, operation and all phase audits together. Claim/reclaim SQL and service transactions enforce the persisted policy independently of UI/configuration flags. Resume invalidates empty/reclaim hints and emits a bounded wakeup after commit.
- Migration494 adds the task state-version trigger, admission proof snapshot and operation reconciliation fields;495–498 are separate concurrent indexes. Migration499 adds a delivery generation independent of immutable execution identity. Startup refuses missing/invalid control indexes or a disabled state-version trigger; destructive rollback refuses retained control/delivery history.
- Managed reclaim preserves the original execution fence. A separate claim_generation guards token finalization, comment receipts, failed-response requeue and claim-specific cancel/fail paths, so delayed delivery A cannot overwrite reclaimed B. Finalization retains the original payload's trigger provenance. Pre-S04 managed deliveries without admission proof can recover only while accepting, without fabricating historical proof.
- HTTP/WS claim responses carry the managed execution fence. The daemon persists exact scoped cancellation receipts before delivery; stopped requires real runner exit evidence. Legacy cleanup, missing provider evidence, lost processes and not_observed never mean confirmed stopping. Explicit management ACK JSON is strict.
- Core/views share operation parsing and use original-key reconciliation. Independent per-request StorageAdapter keys survive reload/navigation and concurrent tabs, stay scoped to server/actor/organization/target, and clear only on session teardown. Only a proven admission conflict or durable failed receipt releases an obsolete intent. Controls preserve original private-content authorization.

## Verification evidence

All Go runs use the agent-CLI guard and task database `multica_platform_admin_s01_test`. No real provider CLI, user profile or model account was used. Handler tests sharing the fixed TestMain workspace are serialized; one overlapping run was discarded and rerun alone.

- Admission first red: all4 existing agent/runtime/batch/raw-SQL claim primitives dispatched a stopped task. Corrected service race suite passed12.462s, including actor/version/replay/audit rollback, stop-vs-claim lock ordering, preservation of previously admitted fences, stale delivery rollback/token/comment guards, and unknown admission rejection. Added upgrade compatibility control passed4.344s.
- Actual authenticated HTTP single/batch/fallback plus real WebSocket tasks.claim on a stopped installation passed3.348s. The row remained queued throughout.
- Control startup schema test passed1.340s with valid indexes/trigger, rejected disabled trigger and missing coordinator index using a private fixture schema. Migrations package checks passed.
- Broader cancellation service race76.654s; final isolated cancellation/claim/pin/ACK/projection handler race11.436s. Existing trigger-deletion provenance regression initially exposed replacement of the original payload snapshot; repaired and verified. Installation/execution read-model race regressions passed8.312s.
- Daemon race72.115s; agent fake-process race254.216s; CLI2.612s/internal-cli cached; affected vet passed. Exact lost-ACK-response retry passed1.921s. Provider/native limits are explicit in the client handoff.
- Core19 files/109 tests and views12 files/86 tests passed; core/views/Web typechecks and scoped lint passed. Independent UI review reran31 core and7 views tests and closed the three recovery findings. Root API tests cover bigint version strings, original idempotency keys and nanosecond timestamps without conversion.
- Knip reports the same11 original-main findings, with no added findings. `git diff --check` and affected Go vet pass.

## Independent review corrections

Backend review closed: two-field in-flight fences rejected; status-only progress incorrectly invalidating execution identity; SKIP LOCKED followers permanently unscheduled; malformed explicit ACKs confirming; stale delivery rollback/finalization and claim-time rejection mutations; upgrade recovery of missing admission snapshots. Definitive fence conflicts now retain failed receipts and audit under the original key.

UI review closed: workspace deletion clearing global control drafts; concurrent tabs overwriting a shared array; obsolete requests trapped after conflicts. Failed cancellation receipts now remain reachable even on the initial409 response. Unproven generic conflicts retain their original key.

Lane evidence: [cancellation-verification.md](cancellation-verification.md), [frontend-verification.md](frontend-verification.md), `.omx/reports/platform-admin/client-s04-handoff.md`. Root logs are saved under `.omx/reports/platform-admin/s04-integration/`.

## Production browser closure

- API PID59743 and production Web PID59991 both report source `fd7249e5636f6d1d02585e4a73bd5cb76c5bc594043c1ba655367042f118c7fb`; Web build `8dkUrAthY4pcskYIkU7Nu`, baseline commit `a38158def`. `/health` confirmed the API listener, and startup exercised the control-schema checks after migrations494–499.
- `e2e/platform-admin-controls.spec.ts --workers=1 --retries=0 --reporter=json` passed1/1 in3.328s, no retries/skips/failures. It covers admission stop/resume, in-flight preservation, lost committed cancellation response, temporarily empty lookup, reload recovery using the original key with one mutation, standalone receipt navigation and legacy confirmation-unavailable state.
- The first attempt failed during setup because its queued fixture omitted the already-required runtime_id. Only that fixture insert was corrected; no application code changed. Both attempts cleaned up, and the browser database contains zero s04-prefixed fixture accounts.
- Desktop and390px screenshots were inspected against the established S05 administration shell/receipt reference. Visual verdict93/pass: no overflow or overlapping fields, and server application/process confirmation/uncertainty remain distinct. The mobile view uses the existing internal scroll container. Verdict/state and provenance are saved in `.omx/reports/platform-admin/s04-production/` and `.omx/state/platform-admin-controls/ralph-progress.json`.

## Remaining project acceptance

Full native installed-app Windows/macOS, additional provider exit evidence, capacity and rollout verification remain S07. Local Windows execution is unavailable and an asynchronous environment question is pending; this does not block S06 implementation. No deployment or full-project completion is claimed.
