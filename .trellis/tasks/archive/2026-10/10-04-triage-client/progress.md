# Triage client lane progress

2026-10-05, latest full core validation 00:38 local. Assigned API/cache/realtime implementation and core checks pass. Independent review follow-up is in progress. Five real backend DTO classes have also passed client parsing. Source is stable for parent integration. No commits created by this lane.

## Implemented

- `packages/core/types/triage.ts`: all frozen Settings, Item, Action, mixed HistoryEntry, batch/import DTOs and request shapes. Public types exported from core types and triage subpaths.
- `packages/core/api/triage-schemas.ts`, `api/client.ts`: every T1 endpoint parsed with Zod + parseWithFallback. Missing/malformed success rejects; settings alone treats HTTP404 as unsupported. CSV download checks text/csv and endpoint generation. IDs encoded in paths and workspace header explicitly overrides ambient slug.
- Response checks bind all available workspace/issue/action/batch identities to the request, accommodate normalized UUIDs and scoped human issue identifiers, and reject unrequested/duplicate batch or import result rows. Batch preview preserves original expected revisions. Decision names and known success counts must match the requested action and actual receipts.
- `types/issue.ts`, `api/schemas.ts`: legacy absent admission defaults to not_required; unknown/malformed states fail closed through shared isFormalAdmission.
- `triage/queries.ts`: explicit workspace keys and parameters, global counts independent of list filters, 30-second ready/count polling, separate immutable history and import detail queries.
- `triage/mutations.ts`: nonoptimistic decisions/imports with per-invocation `{workspaceId,input}` envelopes consumed by transport and cache callbacks. This survives TanStack replacing observer options while an offline request is paused. Public hook inputs, variables and per-call callbacks remain unchanged. Caller-owned request IDs survive transport retries.
- `triage/cache.ts`: preserve newer realtime revision; only successful batch receipts update detail. Invalidate uncertain outcomes and issue/project/dashboard projections. Preview only stores preview state.
- `types/events.ts`, `realtime/use-realtime-sync.ts`: parsed triage:updated and reconnect invalidation. Generic issue updates/deletes, labels, attachments, metadata, properties, comment create/update/delete and issue reactions refresh the affected triage detail and workspace queue lists. Unrelated formal content, other details/workspaces, immutable history and import previews are not refreshed by this targeted path. No Zustand server-state writes.
- `types/api.ts`, `api/schemas.ts`: additive IssueTriggerPreview.blocked entries are typed and parsed; valid triage/future reason codes survive while malformed entries are dropped individually. Two failing-first schema regressions added.
- `diagnostics/diagnostic-context.ts`: triage path registered after parent expanded this lane; existing builder parity regression passes.

## Review fixes and failing-first evidence

The independent review `.trellis/tasks/10-04-triage-t1/research/core-review.md` identified C1–C4. All four were corrected in this lane and sent back for independent re-review.

- `identity-red.log`: 17 failing regressions before fixes, including the actual offline hook race sending a workspace-A settings intention to workspace B.
- `realtime-red.log`: failing existing-event wiring and targeted helper tests before integration.
- `counts-red.log`: three failures for contradictory preview/batch/import success counts before receipt consistency checks.
- Earlier API/cache tests failed first for absent modules/methods, then passed after implementation; no optimistic decision data is inserted into formal lists.

## Current verification evidence

- Focused API/cache/realtime/diagnostic checks: **5 suites / 133 tests passed**. This includes incumbent suites, not 133 distinct T1 acceptance scenarios.
- `pnpm --filter @multica/core test`: **205 suites / 2,504 tests passed**, 00:38. Output `full-core-test.log`.
- `pnpm --filter @multica/core typecheck`: passed. Output `typecheck.log`.
- Scoped ESLint on every changed lane source/test (including diagnostics): passed. Output `lint.log`.
- `git diff --check` on lane files: passed.
- The earlier full run had two new route registration failures; both are now resolved (diagnostics by this lane, paths consistency by UI owner).

## Actual backend parsing

- Backend owner exported actual Go handler responses from TestTriageWireFixtures. Captured artifact: `backend-wire-fixtures.json`.
- Strict client schema checks passed **5/5** for settings, item, actionResult, items and history, including native +08:00 timestamps, UUIDs and nullable fields. Output `backend-wire-validation.log`.
- Reproducible check source retained as `wire-fixture-check.test.ts.txt`; temporarily copy to `packages/core/api/triage-wire-fixture.check.test.ts`, run the named Vitest file, then move it back. It reads the captured task artifact. No transient fixture test was left in package source.
- Added IssueTriggerPreview.blocked boundary checks after the 2,504-test full run: `api/schemas.test.ts` **156 tests passed** (two new). Parent is running full-root verification including this final additive change.

## Remaining integration proof

- Actual import/batch endpoint fixtures have not yet been exported. Their strict schema/request-correlation coverage currently uses representative frozen-contract responses. Current Go import commit implementation was inspected and returns only the requested rows, matching the client selected-only result checks.
- UI owner must keep failed request drafts/request IDs scoped to their original workspace, handle unknown server enum states with default branches, and derive capabilities from supported/enabled. The core envelope protects a submitted invocation, including offline pause/resume.
- Full T1 end-to-end, database concurrency, notification rendering and visual QA remain parent/UI-owned. Client tests support but do not independently prove entire TRI-AC scenarios.


## Final evidence location

Raw logs and exported fixture files mentioned above are retained under `.omx/triage-t1/evidence/lanes/10-04-triage-client` (ignored runtime evidence). The parent verification ledger is authoritative for final whole-feature results.
