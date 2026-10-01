# S04 frontend verification

Status: implementation and package checks complete; independent review and a fresh S04 production browser run are pending.

## Scope

- Shared core control schemas preserve positive decimal CAS versions and exact nanosecond dispatch fences. Legacy safe integer admission versions normalize at the API boundary; missing action/fence data does not authorize controls.
- Account and control operation responses share one schema. The UI consumes the server-resolved effective root outcome, while retaining this actor’s receipt identity and timestamps.
- Admission and cancellation forms freeze the target snapshot. Network/5xx/malformed outcomes query the original key; zero receipts remain uncertain and retries retain the same key and body.
- Non-secret request drafts use the existing StorageAdapter with per-request keys and dedicated session cleanup. Workspace leave/delete does not clear them. Stable server, actor, organization and target scope isolate drafts; concurrently saved requests never share a read-modify-write value; remount reconciles the original key before another write. A storage failure prevents a new command.
- Receipts keep server application, process confirmation and reconciliation separate. Polling is foreground-only and bounded to 30 seconds, with manual refresh and a standalone receipt route.
- Installation/execution detail pages expose controls only for current super administrators and matching backend allowed actions. No prompt, raw log, private path or content bypass was added.

## Evidence collected

- `pnpm exec vitest run admin platform/session-cleanup.test.ts platform/storage-cleanup.test.ts platform/core-provider.test.tsx` in `packages/core`: 19 files / 109 tests passed.
- `pnpm exec vitest run admin locales/parity.test.ts` in `packages/views`: 12 files / 86 tests passed.
- `pnpm exec tsc --noEmit` in both `packages/core` and `packages/views`: passed.
- Scoped ESLint across new operation/control code and modified detail/schema files: passed, no warnings.
- `git diff --check`: passed.
- `pnpm exec playwright test e2e/platform-admin-controls.spec.ts --list`: the new acceptance test is discovered. This is not a browser pass.
- Tests were added and run red before schema, request, draft and form implementation. Covered lost responses, empty lookups, original-key replay, actor/organization/server isolation, permissions, stale versions, unknown enums, queued fences, exact timestamp preservation, storage failure, restored-form lookup, role revocation and keyboard focus.

## Browser acceptance prepared, not yet run

`e2e/platform-admin-controls.spec.ts` creates a task-owned account/workspace, enrolls an installation using the real signing protocol, and seeds only fake executions on an offline runtime. It exercises admission stop/resume, unchanged in-flight status, a committed cancellation with a lost HTTP response and hidden first lookup, reload recovery with one write, standalone receipt navigation, narrow-screen overflow and legacy confirmation-unavailable presentation.

The test does not start native agents or invoke a provider CLI. The production Web/API source identity must be recorded before running it. Prior S02/S03/S05 snapshots do not establish S04 acceptance.

## Remaining limits

- Independent review and fresh production browser/visual evidence remain open.
- Full server concurrency, managed daemon confirmation/reconciliation, native Electron→daemon and capacity acceptance belong to the corresponding S04 backend/protocol checks and S07. This frontend report does not claim those results.

## Independent review corrections

- Reproduced then fixed workspace cleanup deleting unresolved administration requests. Drafts now have session-only cleanup, covered through real workspace and session cleanup functions, including cold-session teardown without a workspace list.
- Reproduced then fixed interleaved tab saves overwriting a shared array. Each request has its own scoped StorageAdapter key; both same-target and different-target interleavings retain every original key. An adapter without key enumeration fails closed before sending. Backend actor/target CAS arbitrates simultaneous explicit submissions.
- Reproduced then fixed obsolete admission intent reopening forever after a retry conflict. Only `admission_version_conflict` plus a successful empty original-key lookup can retire that intent, because same-actor key lookup precedes the monotonic admission CAS. The form waits for a successful target refresh before closing. Arbitrary conflict/empty responses do not establish this proof.
- Cancellation `execution_fence_conflict` resolves through the backend’s durable failed operation receipt, on both first rejection and an uncertain retry. Failed receipt recovery clears the original intent only once the receipt is known. Generic conflicts continue querying/retrying the same original key.
- Corrective regressions were run red before the fixes. No backend implementation was edited by this frontend slice.
