# S06 frontend implementation and verification

This is the bounded frontend handoff, not completion evidence for the whole S06 task.

## Implemented

- `packages/core/admin/observability-schema.ts`: explicit overview, health, settings, workspace and audit projections. Unsafe extra response fields are dropped; malformed identities fail closed. Empty success denominators and absent usage stay null. Settings never infer enforced retention from engineering targets.
- `observability-queries.ts`: actor/server/organization-scoped queries, complete normalized filter keys, late-session checks, foreground-only polling (15 seconds for lists, 5 for alert details).
- `alert-schema.ts`: fixed rule/state/action enums, lossless decimal versions, observer write denial, recovery evidence separate from manually selectable dispositions.
- Existing `operation-draft.ts`, `operation-queries.ts`, and shared control/receipt UI extended for alert acknowledgement, assignment and close. No new draft store or alternate recovery mechanism. Same-key lookup handles lost writes and durable failed conflicts; active administrator assignment uses the existing S05 paged/searchable directory.
- New overview, alerts/detail, health, audit, read-only settings and workspaces pages. Existing semantic tokens, typography, navigation adapter and tables retained.
- Existing execution filters retain `time_basis`; `state_scope=current` survives only an explicit nonterminal status with no time range. Pagination accepts effective pinned parameters, so copied cursor links preserve the original window.
- Current-state overview links use `state_scope=current&status=<exact state>&time_basis=created`; finished outcome links use the authoritative returned window and `time_basis=finished`. Workspace links use the same returned window with created-time counts.
- Recovered and closed alert counts are shown separately as history first observed in the overview window, and both links carry that exact returned window and timezone. Open/acknowledged counts retain all-age links.
- Execution-failure alerts display a recorded historical failure even after close; their permanent deduplication fact is not presented as a still-active live condition. Queue/offline alerts retain active/recovered condition display.
- Audit links only the current actor's operation receipts; other operation IDs remain plain metadata. Missing historical actor names are not reconstructed from present-day names.

## Tests executed

Red/green regressions were observed for new response projections, observation windows, alert request persistence/recovery, non-failure close without a failure disposition, observed recovery codes, scoped queries, new representative pages, receipt copy, and execution filter basis preservation.

- `pnpm --filter @multica/core exec vitest run admin`: final bounded-review pass, 116 tests passed (19 files).
- `pnpm --filter @multica/views exec vitest run admin/overview admin/alerts admin/observability admin/operations admin/executions/list-controls.test.tsx locales/parity.test.ts`: 84 passed.
- Core TypeScript and both admin ESLint checks passed after the bounded review corrections. The parent owns merging `alert-history.*.json` before final views typechecking and the two new UI regression tests.
- ESLint passed for both admin directories. `git diff --check` passed.
- Impeccable detector executed once over all new view directories: `[]` (no findings).
- `pnpm exec playwright test e2e/platform-admin-observability.spec.ts --list`: 1 test discovered; syntax/import loading passed without running it.

## Production scenario prepared, not executed here

`e2e/platform-admin-observability.spec.ts` uses only isolated local fixtures and offline synthetic runtime rows. It covers:

1. Finished-time reporting for a task created outside the window, missing usage, and token-only billing semantics.
2. Current queue drilldown and an active alert older than 31 days.
3. Acknowledgement response loss, hidden first lookup, page reload, and recovery of the original single operation without another write.
4. Failure disposition/close, observer UI and HTTP write denial, health/settings/workspaces/audit navigation, private-content exclusion, and narrow-screen overflow.

No agent CLI, model provider, browser production test, or production screenshot was run by this implementation agent. The root must rebuild the stable source, run E2E, collect desktop/mobile screenshots and visual verdict, and review server/client integration.

## Integration notes

- Root owns ApiClient methods, core/view barrels, locales, Web routes and the six-menu admin shell.
- Locale fragments are `observability.*.json`, `receipts.*.json`, `recovery.*.json`, and `alert-history.*.json` in this task directory; merge nested objects without replacing earlier keys.
- Alert list `data_quality` is the common string enum; detector details are a separate `detector_health` object. Incomplete detectors must not hide already-persisted alerts.
- Definitive alert conflict codes are `alert_version_conflict`, `alert_state_conflict`, `alert_assignee_unavailable`, and `alert_resolution_invalid`; all are reconciled through the original operation key.
- Non-failure close omits `resolution_code`. Failure close sends a manual disposition; `retry_succeeded` requires the linked completed retry and is verified server-side.
- Remaining risk is integration/runtime behavior until the production scenario and independent review run. Capacity, native-platform and deployment rollout evidence belongs to the parent/S07 work.
