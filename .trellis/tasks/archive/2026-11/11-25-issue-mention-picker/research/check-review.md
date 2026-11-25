# Final source/spec disposition — 2026-11-25

**No remaining source-review blocker found.** All five reported correctness findings are now closed by source inspection. This disposition covers the completed source and spec; it does not substitute for main's final build/browser/Electron evidence or claim native OS IME/screen-reader verification.

## Findings (fixed)

- The four original findings remain fixed as detailed in the prior review below.
- **Member observation:** `packages/views/editor/extensions/categorized-mention-list.tsx:79` explicitly requests notifications for `data`, `isPending`, and `isError`. This addresses the actual TanStack tracked-property failure without changing the canonical permission projection.
- **Regression quality:** `categorized-mention-list.test.tsx:421` uses a real QueryClient and the actual `createMentionSuggestion` synchronous item builder. It starts from successful cached member/agent data, then changes members while status remains success and asserts the new member appears, the removed member disappears, and the workspace-granted agent disappears when current membership is removed. Unlike the earlier agent-only fixture, this exercises member-derived permission filtering. API/auth identity and preference readiness are still mocked; the test does not claim real hydration or server access enforcement.
- Avatar options explicitly pass `profileLink={false}` and the avatar mock asserts it; source inspection of ActorAvatar confirms that prop overrides link defaults. Localized hint text interpolates `formatShortcut(createShortcutChord("Down", { alt: true }))`; tests assert macOS and Windows output and reset platform configuration afterward.

## Findings (not fixed)

None requiring a product-source change in this bounded final pass.

## Spec accuracy

Reviewed `.trellis/spec/views/frontend/issue-mention-picker.md` and its frontend index entry. Scope, opt-in/context precedence, query ownership, member notification requirement, inclusive leader category, read-only scoped favorites, mention recency, local versus issue budgets, selection/pagination reconciliation, active-descendant lifecycle and scoped IME guard agree with the reviewed implementation. Required native/browser checks are requirements rather than a claim that synthetic events establish native OS behavior. The document correctly separates native IME/screen-reader verification.

One wording clarification for main's documentation handoff, not a source blocker: the required-E2E bullet's “zero UI mutation” should mean **no comment submission, execution, assignment, or creation-preference mutation**. Mention insertion intentionally changes the editor, draft storage and mention recency; those allowed writes are already stated elsewhere in the spec.

## Verification

- Lint/typecheck: prior implementer report passes; the latest member/avatar/hint follow-up checks were still being finalized at dispatch. Require their final results before overall completion.
- Tests: implementer reports the updated categorized suite at **14 passed**. The earlier **185 tests / 10 files** report predates the follow-up. No reviewer reruns were requested or performed.
- Inspected E2E source uses visible React NodeView chip and persisted Markdown identity rather than absent serializer attributes; execution evidence remains owned by main.
- Remaining evidence limitations are unchanged: mocked readiness does not prove hydration races; mocked `exitSuggestion` does not prove actual account/control teardown; synthetic composition does not prove native OS IME. Main should retain those distinctions in final verification.
- Only this review report changed in this pass; product source and spec remained read-only.

---

# Historical completed-patch re-review — 2026-11-25

This section supersedes the interim findings below. Product files remained read-only; no checks were rerun, as requested. Reviewed `research/implementation.md`, the completed component, new regressions, actual mention factory and installed TanStack Query subscription implementation.

## Findings (fixed by implementer; source reviewed)

1. **Removed selection resurrection:** `categorized-mention-list.tsx:266` now reconciles a missing/disabled selected identity to the active fallback. The new filter-round-trip and asynchronous remove/reintroduce tests assert accepted identity. Still-valid selections remain stable on append.
2. **Pagination resurrection:** `categorized-mention-list.tsx:92` now updates the stored scope to a fresh 50-row page on query/category/leader-subset changes. The round-trip regression asserts return to 51 total rows (one favorite plus 50 directory rows). Query and subset changes use the same branch, though they do not have separate round-trip regressions.
3. **Directory retry discarding issues:** `categorized-mention-list.tsx:534` retries issue search only when `search.failed`; an unrelated directory retry preserves the successful result. The new test uses a real QueryClient, injects a directory error and verifies the remote row remains selected and the search call count stays one beyond the debounce. This directly covers the reported defect. It does not establish real network/access-error recovery: directory API methods are absent from the API mock, and the initial error is injected with `Query.setState`.
4. **Accessible active identity:** `categorized-mention-list.tsx:282` now sets editor `aria-controls`/`aria-activedescendant` and removes them on cleanup; `:423` and `:442` supply matching listbox/option IDs and active option semantics. The test asserts the active-descendant ID equals the selected row's ID. This closes the missing-wiring finding; screen-reader announcement and cleanup through an actual popup exit remain acceptance checks, not proven by that assertion alone.

## Findings (not fixed)

### P2 — Member data updates do not notify the open picker

- File: `packages/views/editor/extensions/categorized-mention-list.tsx:78`, with reads at `:170`, `:327` and `:335`.
- Verified code path: the component subscribes using `useQuery(memberListOptions(scope.ws), qc)` but never accesses `members.data`. It reads only `members.isPending`, `members.isError` and `members.refetch`. The directory itself is read imperatively by `buildItems`, through `qc.getQueryData` in `mention-suggestion.tsx:639`. Installed `@tanstack/react-query/src/useBaseQuery.ts:167` returns `observer.trackResult(result)` by default; no `notifyOnChangeProps` override was found in Core/Web/Desktop source. Consequently, a successful-to-successful member cache update does not change any tracked member-query property and does not itself rerender this component.
- Concrete scenario: open the picker with settled member/agent/squad queries and empty query. Add/remove/rename a member with `qc.setQueryData(workspaceKeys.members(ws), nextMembers)` while leaving query status successful and other data unchanged. The member rows remain stale until an unrelated render or keystroke. A role update also leaves displayed agent eligibility stale because `buildSyncItems` derives `myRole` from members (`mention-suggestion.tsx:652`) before permission filtering (`:675`). Acceptance's fresh `buildItems` check prevents insertion of a newly forbidden non-issue target, but it does not refresh the displayed directory/counts on arrival or removal.
- Recommendation: ensure the actual member data is part of this observer's tracked render dependencies (prefer deriving rows/authority from observed data, or explicitly configure the required notifications). Add a real QueryClient regression for a successful-to-successful member update with no editor query change. Cover both roster and current-user role changes through the actual mention factory where practical.
- Test gap: the existing directory-arrival regression updates **agents**, whose `.data` is already read. Its simplified `buildItems` only returns agents/squad and cannot exercise member-derived permission behavior. Cold member arrival changes `isPending` and would also miss this defect. This is a concrete remaining correctness blocker for R6/AC7, not a speculative concern about mocks.

## Verification and evidence limits

- **Reported implementer evidence:** 185 tests across 10 files passed; Views typecheck and lint exited 0 (27 reported pre-existing lint warnings); diff check clean. Evidence source: `research/implementation.md`. These checks were not independently rerun by this reviewer.
- **Actual boundaries inspected:** presentation remains explicit/default-off, context mode wins, disableMentions wins, shared composition guard is opt-in, and local/issue budgets remain distinct. No new product defect was found in those paths during this bounded re-review.
- **Mocks versus proof:** `captureQuickCreateScope` always returns null and readiness is a boolean mock in the categorized suite, so its readiness test proves conditional display only, not actual hydration. `exitSuggestion` remains mocked even in that file's real-editor integration, so the account/toolbar tests prove calls and rejection, not actual teardown/reopen. The integration substitutes insertion text for the production mention command, so Markdown/node identity remains the main session's E2E/serialization responsibility. These limits are accurately acknowledged in implementation.md; they are not additional reproduced product defects.
- **Pending acceptance:** production Web/Electron builds and E2E, native composition/focus/geometry, persisted Markdown plus visible-chip identity, full auxiliary keyboard sequence, and the already-planned Views spec update remain with main. Existing report of first English journey success is preliminary evidence, not final acceptance.

**Disposition:** all four original findings closed by source review; one new member-observer correctness finding remains. No product changes made. Re-review that fix and final evidence before final sign-off.

---

# Historical interim bounded review — 2026-11-25

Read-only product review while implementation is in flight. Reviewed the approved PRD/design, check.jsonl references, saved hook context, current Views diff and new categorized component/tests. Source anchors refer to the snapshot inspected during this review. This is not final sign-off or browser/native acceptance. Only this report was written; no product/test files, branches or commits were changed.

## Findings (fixed)

None. This review was explicitly restricted to reporting.

## Findings (not fixed)

### P2 — Removed active identities are never cleared

- File: `packages/views/editor/extensions/categorized-mention-list.tsx:247` (state at `:91`; setters only at `:280` and `:420`).
- Evidence: `active` falls back to the first selectable row when `selected` disappears, but the stored identity remains unchanged. Select an agent other than the first, switch to AI squads, then return to All: the old agent becomes active again without any new navigation. Removing a selected candidate through an asynchronous directory update and later restoring it has the same effect.
- Impact: violates the approved focus contract that filtering clears a removed selection to the first selectable match. The visibly active fallback does not become the durable selection, so a later data/filter update can unexpectedly revive the abandoned identity. Highlight and acceptance use the same derived row; this is **not** a claim that they disagree in one render.
- Recommendation: reconcile a removed/disabled active identity to the current fallback (or an explicit cleared state) while preserving still-valid identities on append/reorder. Add filter-away/filter-back and remove/reintroduce regressions, asserting both highlight and accepted ID.

### P2 — Returning to a prior query/filter restores its expanded page

- File: `packages/views/editor/extensions/categorized-mention-list.tsx:84`–`:90`, update at `:501`.
- Evidence: the page tuple is only written by Show more. Expand All to 100 rows, choose Agents, then return to All without expanding Agents. The old tuple matches again and restores 100. Expanding an empty query, typing a query, then clearing it has the same result.
- Impact: the approved requirement to reset the limit on query/category changes is not fulfilled. A large previously expanded directory can reappear after a narrow search/filter, with an unexpected remaining count and rendering cost.
- Recommendation: reset the stored pagination scope on each actual query/category/subset transition, retaining the expanded limit only within the current scope. Add round-trip transitions; the existing one-way search test cannot detect this.

### P2 — Directory retry drops successful issue-search results

- File: `packages/views/editor/extensions/categorized-mention-list.tsx:132`, `:152`, retry at `:484`–`:489`.
- Evidence: the shared retry button increments `retry` even when only a directory query failed. That reruns issue search and immediately replaces its previously successful results with `items: []`; a retry failure also leaves `items: []`. Thus a server-only issue matching the unchanged query disappears when retrying an unrelated failed directory source, including when it was the selected candidate.
- Impact: conflicts with R6's requirement that retry retain usable cached candidates. Cached issue-list rows can survive through `buildItems`, but successful server-only matches cannot.
- Recommendation: retain the last successful issue result for the same query/scope while retrying and on transient failure, or retry only the failed sources. Still clear obsolete query/scope data. Cover successful server-only match + directory failure + retry pending/failure, with selection assertions.

### P2 — Editor-owned keyboard candidate has no accessible active identity

- File: `packages/views/editor/extensions/categorized-mention-list.tsx:399`–`:423`; editor key handling at `:263`–`:285`.
- Evidence: rows expose labels but are `tabIndex={-1}` buttons; active state is only a background class. There is no stable option ID or `aria-activedescendant` relationship on the focused editor, and no alternate active-candidate announcement. Repository search found no editor-side wiring supplying that relationship.
- Impact: arrow navigation changes which mention Enter accepts while assistive technology has no semantic indication of the current candidate. Full row labels alone do not establish the `core-ui-state-contracts.md` keyboard-candidate contract.
- Recommendation: provide scoped active-candidate semantics between the editor and categorized results, and clean them up on exit without altering default/context/slash hosts. Verify that the referenced candidate changes with keyboard navigation and equals the accepted identity.

## Verified source observations

- `mention-suggestion.tsx:746` explicitly selects the categorized renderer only when requested and when mode is not context. `extensions/index.ts` keeps disableMentions precedence. The shared popup composition extension defaults off, preserving other callers' existing path.
- All four requested comment/reply/edit hosts opt in. No task-description opt-in was found.
- Auxiliary actions call `editor.view.focus()`; no new selection-only transaction was found. Focused-control Escape explicitly exits the supplied suggestion PluginKey. The capture guard handles composition Escape before the upstream editor suggestion handler.
- Directory observers receive the supplied QueryClient. Local actors are filtered through existing mention authority before metadata/search; acceptance rechecks current scope and live non-issue eligibility. Favorites are gated through scoped readiness and this component contains no creation-preference mutation.
- Local paging excludes issues (`:225`–`:229`), and Show more counts the remaining local directory only. No claim of a complete workspace issue count was found. Search ranks full saved descriptions before paging and Agents includes leaders.

## Test coverage and follow-up

The new component suite meaningfully covers 550-row initial expansion, responsibility matches beyond the preview, inclusive leader filtering, selected identity across page append, Alt+Down entry, account-change acceptance rejection, and directory arrival. The newly added real-editor test uses trigger arming and checks pointer filter → continued typing → acceptance. The shared popup test checks IME Escape/keyCode 229 with a real suggestion plugin. These are inspected tests, not test runs from this reviewer.

Before final sign-off, add the named regressions above. The current component setup supplies a simplified `buildItems` and mocked readiness/exit; those cases do not establish real permission/runtime changes, actual preference hydration races, or actual scope-change teardown. Add targeted factory/real-editor integration coverage for those boundaries, identical-query late issue responses after a scope change, and keyboard Tab/Enter/Space through Retry/Show more/Return to typing. The current integration test clicks Return to typing, so it does not prove the auxiliary keyboard sequence. Keep native IME, geometry, host submit/draft behavior and Electron verification with the main session as assigned.

## Verification

- Lint: not run, per explicit dispatch instruction to avoid duplicating in-flight checks.
- TypeCheck: not run, same reason.
- Tests: inspected only; no pass/fail claim.
- Browser/Electron: not run; owned by main session.
- Spec sync: final verified behavior still needs the planned Views spec update; this interim report does not document proposed behavior as shipped.

Remaining risk: implementation was changing during inspection (a real-editor test was added during this pass). Recheck findings against the completed patch and fresh verification evidence before final approval.
