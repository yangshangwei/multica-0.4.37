# Settings remediation continuation review

Reviewed on 2026-10-14 by the dispatched `trellis-check` agents. This report consolidates the prior review and its final continuation after the previous agent's provider-resource interruption. The original pass loaded the complete saved native hook context, task artifacts, A/B/C reports, original audit, and related source/regressions. The final pass reloaded the saved context and existing report, inspected the settled DingTalk queries/component/callers and updater change, and ran the final focused checks below. Prior verified fixes were preserved. Concurrent automation, documentation, collaboration, tab/platform and PAT/desktop implementation edits remain intact. No reset, checkout, stash, commit, browser interaction or real-service write was performed by the continuation reviewer.

## Findings (fixed)

### F02/F08: cancellation restored a denied Query snapshot

- Files: `packages/core/query-access.ts`, `packages/core/query-access.test.ts`.
- Evidence: with a real QueryObserver and the production retry shape (`retry: 1`), the first 403 cleared visible data, but unsubscribing during the retry delay restored `{ name: "Denied settings" }`. The original `query.setState({ data: undefined, dataUpdatedAt: 0 })` did not replace TanStack Query's cancellation rollback snapshot. The new regression failed at the assertion immediately after unsubscribe.
- Fix: use the public `query.setData(undefined, { manual: true, updatedAt: 0 })` API on the already captured Query. A manual success dispatch updates TanStack's rollback snapshot as well as visible data. Keep the aborted-signal and live stale-credential guards; do not change global retry options or access private fields.
- Verification: 9 canonical tests pass. They cover 401/403 followed by 500/remount, unmount during automatic retry, successful automatic retry, cancelled older denials, stale credentials, and preservation after non-access errors.

### F02/F08: subordinate lists and invitation confirmation outlived denial

- Files: `packages/core/workspace/queries.ts`, `packages/core/github/queries.ts`, `packages/views/settings/components/members-tab.tsx`, `members-tab.test.tsx`, `repositories-tab.recovery.test.tsx`.
- Evidence: invitation and share-link reads used the new view feedback but lacked the core boundary. Real QueryClient regressions showed their old rows returning after 403 -> Retry -> 500. The GitHub repository picker likewise restored the selected denied repository. An already open invitation-revocation confirmation remained visible after its own list was denied.
- Fix: apply the existing boundary to `invitationListOptions`, `shareLinkListOptions` and `githubInstallationRepositoriesOptions`. Preserve keys, page parameters and the query context; the pagination adapter captures the correctly inferred outer context instead of changing the shared wrapper interface. Require a loaded authorized subquery for invitation/share-link management. Give the existing private confirmation state a member/invitation discriminator, then gate and clear only the invitation confirmation when that access is lost.
- Verification: the three denied-list wiring regressions and the confirmation regression fail before and pass after. Existing invitation, member-action and repository persistence cases remain green.

### F02/F08: triage did not expose or honor its membership query state

- Files: `packages/views/triage/triage-settings-tab.tsx`, `triage-settings-tab.test.tsx`, `triage-settings-recovery.test.tsx`.
- Evidence: the view consumed only `useCurrentMember().role`; member read errors had no Retry, and cached triage content remained after membership denial. Two real QueryClient regressions failed before the fix.
- Fix: read the existing member query directly, derive the current member from the auth identity and authorized query snapshot, and reuse `SettingsQueryFeedback`. Show the form/count query only with known membership plus loaded triage settings. Preserve the same input and draft on a cached 500; clear the local draft when the protected snapshot is unavailable. Save/patch entry points also require current edit permission. No store or public interface was added.
- Verification: both triage files pass all 9 tests, including initial permission recovery, focused-node preservation, 403 -> 500 protection and clean recovery after access returns.

### F02/F08: nested DingTalk groups discarded retained rows and revived denied data

- Files: `packages/core/dingtalk/queries.ts`, `queries.test.ts`, the `agentListOptions` hunk in `packages/core/workspace/queries.ts`, `packages/views/settings/components/dingtalk-tab.tsx`, `dingtalk-tab.test.tsx`, `provider-settings-recovery.test.tsx`, and the sole shared caller `packages/views/agents/components/tabs/integrations-tab.tsx` with its test.
- Evidence: the existing group panel replaced active and expanded inactive rows on any refresh error; its inline inactive query bypassed the shared access boundary. The regular-member installation filter also depended on an agent-list snapshot that could return after denial and a transient retry. Real QueryClient regressions cover each path, rather than inferring behavior from mocked `isError` flags.
- Fix: pass the existing full group Query result into `DingTalkBotGroups` from settings and both render branches of its one agent-detail caller. Reuse `SettingsQueryFeedback` as a stable sibling. Reuse the existing workspace/agent inactive query options and apply the exact-query boundary to their page reads, the agent-scoped active read, and the agent-list permission join. Keep runtime keys, workspace/agent isolation, page offsets, `next_offset`, polling, group sorting, installation filtering and binding paths unchanged. Load More cannot cancel an in-flight refresh.
- Verification: the final focused views run passes 80 tests across the settings, provider-recovery and agent-integration suites. Named real-query cases retain the exact focused active/inactive nodes across 500 and retry, keep denied groups and denied agent-filtered installations absent after a later failure, and exercise the actual production options. The final core run passes 20 tests, including workspace/agent inactive offset and endpoint isolation, the agent-scoped denial boundary, and the canonical access/cancellation matrix.
- Resolution: the previous report's DingTalk deferral is superseded. These nested flows are included in F02/F08 and are now covered.

### F08/F11: an inactive-group denial blocked an authorized active-group confirmation

- Files: `packages/views/settings/components/dingtalk-tab.tsx`, `provider-settings-recovery.test.tsx`.
- Evidence: the final continuation found that `handleForget` rejected every target when the independent inactive query was denied. Active rows still exposed Forget and a confirmation, but confirming made no request. A new real-query regression failed at the expected `forgetDingTalkGroup` call before the correction.
- Fix: retain the selected group's active/inactive source in the existing private confirmation state. The confirmation and handler require management permission, the authorized parent group read, and the inactive read only for an inactive target. A denied inactive target is cleared, while an authorized active target remains usable. The existing request arguments, pending guard, invalidation and failure/retry behavior are unchanged.
- Verification: the same regression now passes and is included in the 80-test final focused views run. It verifies that an open inactive confirmation closes on denial and that the remaining active group can still be forgotten once.

## Independent review across passes

- A: status category labels no longer subscribe to the catalog in child rows/dialogs; cold errors cannot create the former observer loop. The catalog helpers keep feedback outside retained content, distinguish successful emptiness and use the existing exact query boundary. Content-container layout changes and property dialog bounds are confined to their affected views. Shortcut values use accessible descriptions and warning text uses semantic foregrounds.
- B: profile/workspace avatar callbacks rethrow failed persistence. Invalid required names retain other fields and expose associated errors plus unsaved status. `useAutoSave.saveNow` now settles its own serialized candidate, retains the latest queued candidate, and reports failure/supersession as false. Repository removal awaits that result, including a last row and an unsaved blank row. Invitation/prefix/member handlers share pending guards; timezone locks the accepted write.
- C: provider installation/configuration reads distinguish unavailable from unconfigured. MCP, VCS, Telegram and Composio regressions verify actual retained nodes/focus. GitHub's switches, Connect and Disconnect share one synchronous pending guard. Catalog Retry controls remain outside links; navigation and feature gates are preserved.
- D: the preceding pass independently read the PAT schemas/client hunks, auth hooks/tests, TokensTab/tests, and all three desktop component/test diffs. Creation delivers the one-time secret through a callback and returns `void`; mutation variables contain only the request and both hooks disable retries. Cache-observation tests cover transient cache secrecy; offline writes settle rather than queue. Account identity, malformed responses and all acknowledgement-gated dismissal paths are covered. The final continuation separately reviewed the latest updater fix after native testing exposed Chromium's blur when the focused button becomes disabled. Its pending state now uses `aria-disabled` and a synchronous ref guard, releases the guard in `finally`, retains one live region and never calls `focus()` on settlement. Seven fresh updater tests and its scoped lint pass, including repeated activation, failure recovery and deliberate focus movement. Native focus acceptance and all D source edits remain with the coordinator/implementer.

## Findings (not fixed)

- No remaining confirmed F01–F17 source defect was found in the reviewed scope after the fixes above. The shared DingTalk caller is part of the completed source review, not a deferred flow.
- The agreed existing workspace-leave pre-navigation/realtime ordering remains unchanged. This is documented lifecycle debt, not a pattern for the newly fixed destructive confirmations.
- Geometry, AX, contrast, native zoom, source fingerprints and final combined acceptance remain coordinator-owned. The reviewer did not independently rerun native Electron probes, real backend writes, Windows/Linux or spoken screen-reader testing.
- The coordinator owns `.trellis/spec/` edits. `settings-recovery.md` matches the settled rollback, retry, inference and PAT ownership contracts. The continuation sent two additions: explicitly include grouped/paginated subordinate reads and permission joins such as `agentListOptions`, and record `aria-disabled` plus a synchronous handler guard for the updater because native `disabled` loses Chromium focus. No spec files were edited by this reviewer.

## Verification

The first table preserves actual commands/results from the preceding reviewer. The final continuation's fresh commands are listed separately below; counts from overlapping runs must not be added together. The coordinator owns final combined tests, consumer checks and native acceptance.

| Command / scope | Result |
| --- | --- |
| `pnpm -C packages/views test settings triage common/avatar-upload-control.test.tsx --exclude '**/tokens-tab.test.tsx'` | Baseline: 55 files, 668 tests passed. |
| `pnpm -C packages/core test query-access.test.ts labels properties quick-actions issue-statuses triage/queries.test.ts workspace/queries.test.ts workspace/mutations.test.tsx workspace/mcp-template-queries.test.ts dingtalk/queries.test.ts` | Baseline: 10 files, 73 tests passed. |
| `pnpm -C packages/core test query-access.test.ts` | Final: 1 file, 9 tests passed. |
| `pnpm -C packages/views test settings/components/members-tab.test.tsx settings/components/repositories-tab.recovery.test.tsx triage/triage-settings-recovery.test.tsx triage/triage-settings-tab.test.tsx` | Final: 4 files, 28 tests passed. |
| Members/repositories recovery plus `seat-invite-purchase.test.ts` | 3 files, 25 tests passed after the subordinate-list fix. |
| `pnpm -C packages/views typecheck` | Passed after the shared locale merge and review fixes. The earlier transient failures named missing D locale keys; no workaround was added. |
| `pnpm -C packages/core typecheck` | Passed after the boundary and query changes. |
| ESLint on 40 changed A/B/C settings/triage source/test files | Passed. |
| ESLint on `query-access.ts`, its test, and 14 changed catalog/workspace/provider query modules | Passed; boundary lint rerun after the final added retry-success test also passed. |
| `git diff --check` on reviewed settings/triage and changed core paths | Passed. |

Logs are `/tmp/settings-continuation-review-*.log`. Reproductions: `access-red`, `wiring-red`, `picker-red` and `triage-red`; passing runs: `access-green`, `final-wiring`, `triage-green`, `wiring-green`, `views-tests`, `core-tests`, `views-typecheck`, `core-typecheck`, `views-lint`, `core-lint` and `access-lint`. The first picker attempt used an incorrect button label; `picker-red` uses the real `Add repositories` label and demonstrates the product regression. Routine pnpm configuration-location warnings are pre-existing and did not fail checks.

### Final continuation verification

| Command / scope | Result |
| --- | --- |
| `pnpm -C packages/views test settings/components/provider-settings-recovery.test.tsx -t 'closes a denied inactive'` before the local correction | Expected failure: confirming the authorized active target made zero requests. Log: `/tmp/settings-final-review-forget-red.log`. |
| `pnpm -C packages/views test settings/components/dingtalk-tab.test.tsx settings/components/provider-settings-recovery.test.tsx agents/components/tabs/integrations-tab.test.tsx` | Passed after the correction: 3 files, 80 tests. Log: `/tmp/settings-final-review-views-tests-green.log`. |
| `pnpm -C packages/core test query-access.test.ts dingtalk/queries.test.ts workspace/queries.test.ts` | Passed: 3 files, 20 tests. Log: `/tmp/settings-final-review-core-tests.log`. No subsequent core source changes. |
| `pnpm -C packages/views typecheck` | Passed after the final correction. Log: `/tmp/settings-final-review-views-typecheck-green.log`. |
| `pnpm -C packages/core typecheck` | Passed. Log: `/tmp/settings-final-review-core-typecheck.log`. |
| `pnpm -C packages/views exec eslint settings/components/dingtalk-tab.tsx settings/components/dingtalk-tab.test.tsx settings/components/provider-settings-recovery.test.tsx agents/components/tabs/integrations-tab.tsx agents/components/tabs/integrations-tab.test.tsx` | Passed after the final correction. Log: `/tmp/settings-final-review-views-lint-green.log`. |
| `pnpm -C packages/core exec eslint query-access.ts query-access.test.ts dingtalk/queries.ts dingtalk/queries.test.ts workspace/queries.ts workspace/queries.test.ts` | Passed. Log: `/tmp/settings-final-review-core-lint.log`. |
| `pnpm -C apps/desktop test src/renderer/src/components/updates-settings-tab.test.tsx` | Passed: 1 file, 7 tests. Log: `/tmp/settings-final-review-updater-tests.log`. |
| `pnpm -C apps/desktop exec eslint src/renderer/src/components/updates-settings-tab.tsx src/renderer/src/components/updates-settings-tab.test.tsx` | Passed. Log: `/tmp/settings-final-review-updater-lint.log`. |
| `git diff --check` over the reviewed core files, shared settings/triage, agent integration caller/test and updater/test | Passed. |

The desktop implementer's full node/renderer typecheck is recorded in `research/token-desktop-result.md`; the continuation did not substitute a views check for desktop verification. No new dependency, endpoint or platform dependency was introduced. Checks above were real package commands, not mocked verification results.

## Reviewer edit inventory and simplification

Across both review passes, implementation/test edits are limited to these paths under `packages/`:

- Core: `query-access.ts`, `query-access.test.ts`, `workspace/queries.ts` (task-owned boundary hunks), `github/queries.ts`, `dingtalk/queries.ts`, and `dingtalk/queries.test.ts`.
- Views settings: `members-tab.tsx`, `members-tab.test.tsx`, `repositories-tab.recovery.test.tsx`, `dingtalk-tab.tsx`, `dingtalk-tab.test.tsx`, and `provider-settings-recovery.test.tsx`, under `views/settings/components/`.
- Views triage: `triage-settings-tab.tsx`, `triage-settings-tab.test.tsx`, and `triage-settings-recovery.test.tsx`, under `views/triage/`.
- Shared DingTalk caller: `views/agents/components/tabs/integrations-tab.tsx` and `integrations-tab.test.tsx`.

The final continuation itself changed only `dingtalk-tab.tsx`, `provider-settings-recovery.test.tsx` and this report. The earlier review's `research/locale-review.json` fragment was merged by the coordinator.

The fixes reuse one access boundary and one feedback helper, remove triage's opaque permission-only consumption in favor of its existing Query, and replace the duplicated inline inactive-group query with its existing core options. Only the existing shared component's two calling contexts were adapted. No dependency, store, endpoint, route, global retry change or generic abstraction was added. PAT/desktop sources, locale JSON, unrelated edits and task lifecycle/spec ownership were preserved.
