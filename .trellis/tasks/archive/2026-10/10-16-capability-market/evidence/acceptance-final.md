# Capability market acceptance

Verified on 2026-10-17 UTC against the task-owned detached worktree `/Volumes/artisan/code/2026/multica-capability-market-qa`, base `5049e0f6a4ba5801876c7e821418a23e59262c4c`. The final 76-file task manifest matches the QA checkout byte-for-byte, including application code and final E2E changes. See `qa-snapshot.json` and `final-source-manifest.json`.

## Result

**25 unique E2E tests across 10 spec files passed; no final failures or conditional skips.** There were 41 actual test executions, including the initial failures and their explicit reruns. `acceptance-summary.json` records every execution and exact identity; raw JSON/logs/screenshots are retained under `.omx/artifacts/capability-market-final/acceptance/` in the main checkout.

| Execution | Result | Scope |
| --- | --- | --- |
| `web-legacy-initial.json` | 18 passed, 2 failed | Eight impacted Web specs; original route/draft, Skill, MCP, locale/default/session/Agent cases |
| `web-isolation-recovery.json` | 4 passed | Repaired isolation/Skill expectations; route history; new first-load/cache/real polling regression |
| `password-web-final.json` | 8 passed | All five MCP cases plus all three administrator publishing cases, with native consumer enabled |
| `native-attempt-1.json`–`native-attempt-7.json` | Seven failed attempts | New native journey brought to executable correctness; reasons below |
| `native-final.json` | 1 passed, 49.6 seconds | Full native Electron Skill + MCP workflow; no skip |
| `web-deployment-fixture-final.json` | 1 passed, 34.0 seconds | Final shared deployment fixture readiness assertions against real production Web |

Unique inventory: capability-market 3, skill-market 1, mcp-market 5, skill-template-creation 1, localized-template-defaults 5, workspace-defaults 3, agent-mcp 2, missing-user-session 1, administrator-resource-publishing 3, native mcp-desktop 1.

## Environment and build provenance

Owned environment: `check-20261016144743-87744`, API `18161`, Web `13081`, database `multica_check_20261016144743_87744_api`. Both stages used `make up C=api,web ARGS="--web-mode production --ttl 24"` with the effective registry env file; neither used `next dev`. PostgreSQL tooling was verified at `/opt/homebrew/opt/libpq/bin/psql` and added to this task command's PATH.

The legacy stage used the initial task snapshot and a fresh `next build`/`next start`. After the reviewer's `mcp-agent-discovery.tsx` focus-resolver fix, the final password/admin stage rebuilt Web and Electron; final Web build ID is **`mo_qB94b7-h4UgwnwCwSi`**. Health proved API listener PID `80970`, commit `5049e0f6a`; Web listener PID `81905`. `final-api.running.json`, `final-web.running.json`, and `final-build-provenance.json` preserve source/configuration fingerprints, launch commands and bundle hashes. API mode and administrator availability were checked through the live `/api/config` response.

Electron was rebuilt using `make env-exec ARGS="-- pnpm --filter @multica/desktop exec electron-vite build"`. The existing native fixture launched the actual Electron executable, production renderer, preload, desktop shell, memory router and real local Go API with a temporary userData profile. It isolated daemon/updater/external-network side effects. Actual third-party MCP connections and installed agent execution were outside this navigation/adoption acceptance.

Final test-only repairs occurred after application builds and do not change product bundles. Unchanged Skill/route tests were not repeated wholesale after the contextual MCP focus fix; final MCP Web/native/admin journeys exercised the affected application paths.

## Behavior evidence

- Navigation: Web sidebar, command search, missing/invalid/explicit type, refresh, Back/Forward, unrelated query/fragment preservation and dirty editor hide/restore passed. Native sidebar, manual keyboard activation, history buttons, actual mouse side-button history, background new tab, active session reload/restore and correct management destinations passed.
- Management/copy: empty and populated libraries remain management pages; From template opens the market. Real Skill creation stays in the market, preserves filters, restores opener focus, links related renamed copies, and opens the saved copy on explicit action. Ordinary creation/related navigation retain dirty confirmation and recovery semantics.
- MCP: all sixteen reviewed recipes saved and explicitly assigned through Web and native UI; secret input masking and safe summary payloads passed. Partial assignment retry did not recreate the instance or repeat successful grants; disabled assignments remained disabled. Settings and contextual Agent discovery stayed reachable; a normal member reused an existing instance without gaining creation permission.
- Isolation/preferences: two real workspaces kept distinct resource lists, source/category preferences and transient drafts; switching through the real sidebar switcher restored the correct preferences. An old `libraryView: "market"` payload did not turn the library into a market. Draft text was absent from persisted view preferences.
- Failure/visibility: a synthetic 503 on only the MCP catalog left Skills reachable; cached MCP cards survived refresh failure and retry. Real 30-second request observations verified inactive catalogs stopped requesting: MCP stayed at 2 while Skills polled, and Skills stayed at 2 while MCP polled. Real directory files exercised update/review, cleared old credentials on explicit reload, withdrawal and retained saved configuration/assignment snapshots in both Web and native runs.
- Publishing: actual managed Skill/MCP uploads reached production Web and native markets. One lost publish response recovered through its receipt with exactly one write. Revision conflict returned 409; withdrawal preserved copies; unauthorized publication returned 403; native daemon execution stayed false. Fixture result is `admin-publishing/functional-result.json` in raw artifacts.
- Locale/responsive/focus: English/light and Chinese/dark Web library/catalog/preview captures at 360/375 CSS px were inspected using `view_image`; PNG dimensions were checked. Native captures included 375/420 CSS px, restored desktop sessions and Chinese/light UI. Native PNGs use device scale 2. The visual verdict is 94/pass with no requested changes, retained in `visual-verdict-final.json` and `.omx/state/capability-market-acceptance/ralph-progress.json`.

Existing Agent MCP Apps cases mock the third-party integration responses while using real authentication/workspace setup. The new catalog-failure case injects only the failed MCP responses; successful catalog requests and writes use the real API. These are not represented as live third-party connectivity checks.

## Test repairs and failed attempts

No product source was edited by this acceptance lane.

- `e2e/capability-market.spec.ts`: added two-workspace isolation and real polling/recovery cases. The first isolation attempt used an exact workspace-button name that omitted its avatar initial; suffix matching fixed that confirmed locator error.
- `e2e/skill-template-creation.spec.ts`: changed the stale dirty-navigation URL expectation from `/skills` to `/marketplace?type=skills`, matching the new owner route while the discard confirmation is open.
- `e2e/mcp-desktop.spec.ts`: the new draft test initially sent Cmd+Left from a modal. Native diagnostics proved the modal consumed ArrowLeft before the shell's window listener. It now sends a real Chromium mouse side-button through the existing desktop history entry. Subsequent failures were an ambiguous sidebar/breadcrumb Skills link and a generic copy-name assertion racing route reconciliation; the test now scopes the sidebar link and waits for the persisted desktop route plus canonical resource link.
- One native attempt overlapped a live deployment fixture and correctly observed 17 templates instead of 16. Runs were serialized after directory cleanup; no count assertion was weakened.
- `e2e/fixtures/mcp-deployment.ts`: initial native filling raced the dialog's deferred autofocus. The failure screenshot showed the synthetic token appended to Configuration name and an empty password input. Waiting for the declared initial name focus, then asserting the password value, fixed the fixture. Final native and final Web deployment tests passed with those assertions.

Task-only E2E TypeScript validation passed with strict compiler options. ESLint checked all four changed E2E files directly with the existing base config plus explicit E2E globals/dev dependency/Playwright empty-parameter rules: zero errors/warnings. The regular app lint config excludes root E2Es, so ignored-file output was not treated as verification. Full repository lint/types/units and docs checks are owned by the parent reviewer; their evidence is in the task's `check-results.json` and `.omx/artifacts/capability-market-final/review/`.

## Cleanup

All temporary Electron profiles and native password accounts were removed by their fixtures. Live template directory is empty; both synthetic managed publication keys are withdrawn. The two workspaces left by the first test timeout were deleted through the real authenticated workspace API during a brief task-only legacy API restart; the password env was restored privately afterwards.

`cleanup-resources.json` confirms no owned feature workspaces/native accounts remain. Final registry status reports API, Web, daemon and Desktop **stopped**. The task-only administrator, database/profile, withdrawn-resource audit history and QA worktree are retained for inspection/TTL cleanup. Other checkout listeners and profiles were left untouched. No task env, password, token, private admin session or database credentials were copied into committed evidence.
