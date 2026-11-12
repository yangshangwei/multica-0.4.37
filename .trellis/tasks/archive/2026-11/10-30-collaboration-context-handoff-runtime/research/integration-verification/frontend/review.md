# Initial frontend integration review

Scope: read-only review of `mc-runtime-integration-20261031`, based at `e3620721e` plus the captured runtime WIP. No product source edits, DB tests, real Provider calls, browser runs or commits were performed. All 21 reviewed source file hashes stayed unchanged during these checks (`source-before.json`, `verification-summary.json`).

## Findings not fixed

1. **P2 — recording retention is not bound across a page chain.** `packages/core/collaboration/run-records-queries.ts:14-18` compares the preparation snapshot but not the newly introduced `recording.retention`. Each page passes `run-records-schema.ts` independently, so a later page with a different stream ID and its own valid ordinal can be appended. `RunRecords` combines all items but displays the last page's retention summary, producing inconsistent history/coverage. The read-only reproduction in `retention-chain-repro.test.ts` uses two individually schema-valid pages and fails the expected rejection (`retention-chain-repro.log`). Extend the existing chain check to the retention identity/watermarks, test absent/introduced/changed metadata within a chain, and retain coherent full-refresh behavior. Recommended owner: Core recorded-run query owner. This was intentionally left unchanged under the assigned read-only ownership.

2. **Missing focused coverage — new human handoff query fences.** `run-handoff-schema.test.ts` verifies the human URL, identity mismatch and optional-route behavior at ApiClient level. `run-handoff.test.tsx` mocks `useQuery`, so it does not execute `collaborationRunHandoffOptions`. That helper borrows the established issue guard, but its absent/malformed-route parent recheck, live access loss, late scope response and cache invalidation still need a small canonical query suite. Recommended owner: Core handoff owner; preserve existing guard behavior rather than duplicate its entire matrix in DOM tests.

3. **Freshness decision to resolve for live handoff.** `run-handoff-queries.ts:7-8` has mount/focus/reconnect refresh but no polling, while adjacent issue, comments, memory and run-record reads poll every 15 seconds. A missed realtime hint can therefore leave an open live handoff's handler/result/reason projection stale while the surrounding record page advances. Confirm the intended recovery bound and reuse the existing unavailable-aware polling policy if the live view must recover without a focus/reconnect event. This is a design/consistency recommendation, not a demonstrated failed product test.

## Paths that are wired in this snapshot

- Human handoff reads use `/api/collaboration/issues/{id}/runs/{runId}/handoff` with execution version negotiation and the ordinary human fetch path; API parsing checks issue/run identity and uses a pure schema leaf. Matching server registration exists in `server/cmd/server/router.go:1677`.
- The protected query lives under the issue key, and `RunRecords` mounts the shared handoff view. Existing issue access cleanup and realtime issue-prefix invalidation therefore cover this cache. The broader parent E2E must still exercise the real chain.
- Comment query keys include limit and byte budget. New budgeted pages have strict schema/identity checks, while legacy pages preserve unknown coverage instead of inventing totals.
- Placeholder comments use the human single-comment route; editing stays disabled until a matching revision's full content is loaded. The focused tests cover revision mismatch and do not use runtime capability routes.
- Per-page retention validates ordinal/stream bounds and unknown tails. UI distinguishes known, unknown and unavailable coverage; the cross-page omission above remains.
- The new `recall` translation keys have identical English/Chinese key sets.

## Verification

Exact commands, start/end timestamps, exit codes and duration are saved in each `<name>.json`; console output is in `<name>.log`. Vitest JSON reports retain test identities.

| Check | Result |
| --- | --- |
| Core targeted Vitest: content transport/schema, recall query key, handoff schema/API, records schema/query, comment actions | 7 files, 117 passed, 0 failed/skipped |
| Views targeted Vitest: comment list, handoff view, workflow | 3 files, 17 passed, 0 failed/skipped |
| Core package typecheck | Passed |
| Views package typecheck | Passed |
| Scoped Core ESLint | Passed |
| Scoped Views ESLint | Passed |
| Scoped `git diff --check` | Passed |
| Cross-page retention regression reproduction | 1 failed as expected: missing rejection; source not changed |

These results validate the fixed frontend snapshot only. They do not establish final runtime integration, production Web/Electron acceptance, full repository quality, or real-model behavior. Before final acceptance, rerun affected checks if imports from the original owners change source hashes, then add API→query→UI browser coverage for source context, retention, and comment-budget restoration.
