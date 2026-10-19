# R51/R52 core implementation

## Change boundary

The existing shared issue parser omits ordinary child references, account paths
cannot retain a run anchor, and there is no protected paged source-record reader.
These behaviors belong to core's API schemas, query options, realtime invalidation
and account path helpers. Server remains the relationship, recording and current
authorization owner; views consume only the agreed projections.

Planned changes are limited to `packages/core/collaboration/` (additive children,
records and notification contracts plus protected queries), `api/client.ts`
(the account-scoped records request), `paths/account.ts` (run anchors), and
`realtime/collaboration.ts`/`types/events.ts` (strict child-change invalidation).
Colocated tests cover malformed/private wire data, exact identities, optional
route absence, paging, access-epoch fencing and old account routes. Existing
execution v5 and original-key/access helpers are reused. No dependencies,
ordinary execution changes, persisted source content or release-gate changes.

API contract: [server contract](r51-r52-api-contract-2026-10-19.md).

## Implemented contracts

- `CollaborationOrdinaryChildren` preserves the optional complete minimal list.
  Missing support remains undefined. Child IDs are unique, cannot be the parent,
  and open flags must agree with a valid current source slug. Source editing
  capability is not used to infer membership. Unknown private fields are stripped;
  malformed present blocks make the shared issue unavailable.
- `collaboration:children_changed` accepts exactly `{issue_id}` and refreshes the
  parent projection without advancing access epochs or clearing pending intent.
- `CollaborationRunRecords` / `CollaborationRunRecordItem` parse protocol 1 in a
  pure schema leaf. Run shape is reused from execution v5; v5 wire is unchanged.
  Input order, recording states, resource agent, stable IDs and typed entries are
  checked. Provenance uses an explicit allowlist; usage remains nullable decimal
  strings, preserving signed-int64 counters beyond JavaScript's safe integer range.
- `getCollaborationRunRecords(issueId, runId, params, options)` validates exact
  issue/run identity, page bounds and cursor progress, sends no ambient workspace
  headers, requests no-store and redacts private failures. No ordinary-message
  fallback exists.
- `collaborationRunRecordsOptions(client, scope, issueId, runId)` exposes standard
  infinite-query pages with issue/run-specific keys. Every page uses the existing
  Server/viewer/access-epoch guard. Optional absence, missing runs and malformed
  pages recheck current issue authority; authoritative denial clears the detail
  and all record pages, while transient failures preserve lawful cached data.
- Result notifications retain legacy `source_kind:"issue"`, `source_id:null` and
  add optional `run_id`. Present run IDs require an openable, non-mention AI result
  notification. Denied projections cannot retain a run ID. `accountPath` and
  `parseAccountPath` preserve `#run-UUID`; route mementos still exclude anchors.

The public types/parser/query exports were sent to the shared UI owner, including
the one-anchor-per-destination rule and missing usage/input semantics.

## Files changed

Production: `packages/core/api/client.ts`; collaboration `schema.ts`,
`content-schema.ts`, `execution-schema.ts`, `index.ts`, new
`run-records-schema.ts` and `run-records-queries.ts`; `paths/account.ts`;
`realtime/collaboration.ts`; `types/events.ts`.

Tests: new `api/collaboration-run-records.test.ts`; collaboration
`children-schema.test.ts`, `result-notification.test.ts`,
`run-records-schema.test.ts`, `run-records-queries.test.ts` and
`run-records-test-fixtures.ts`; existing `paths/account.test.ts` and
`realtime/collaboration.test.ts`.

## Verification

Logs and machine-readable command/exit metadata live under
`.omx/reports/cwc-r51-r52-20261019T110745Z/core/`.

| Evidence | Result |
| --- | --- |
| `r51-red` → `r51-green` | Expected 24 failures before child projection/invalidation; 34 tests pass afterward |
| `r52-path-notification-red` → `r52-path-notification-green` | Expected 14 missing-anchor/validation failures; 24 tests pass with existing content coverage |
| `r52-records-red` → `r52-records-green` | 55 tests fail before the new parser/API exist, then all pass |
| `r52-record-queries-red` → `r52-record-queries-green` | 12 tests fail before protected paging exists, then all pass |
| `full-test` | `pnpm --filter @multica/core test`: 298 files, 3,858 tests pass |
| `lint` | `pnpm --filter @multica/core lint`: passes |
| `typecheck` → `typecheck-final` | Two new cache tests initially seeded raw wire fixtures; corrected to parsed query data, then `pnpm --filter @multica/core typecheck` passes |
| `records-queries-final` | 12 paging tests pass after the test-fixture type correction |
| `lint-final` | Targeted ESLint for the corrected test file passes |

`git diff --check -- packages/core` also passes. No dependencies were added and
no commits were made. Server/runtime integration, shared Web/native journeys and
the repository-wide quality gate remain with the main session; this core slice
does not claim full parent-task or 97-criterion completion.
