# Notification page quality review

Date: 2026-11-22
Baseline: `f789a86af58c49ab67aa58b4be0ade4ef1bfb8ed`
Reviewer: native `trellis-check` (`notifications_quality_check`)

## Scope checked

Read the saved native hook context, CLAUDE.md, task PRD/design/implementation plan, check manifest, API/routes/E2E research, and applicable Core/Views/Web/Desktop/Server contracts. Review excluded the three tracked baseline changes recorded in `research/baseline.json` and the concurrently edited E2E files. No production behavior, E2E, shared spec, journal, or planning artifact was changed by this reviewer.

- Backend: notification handler/service and tests; canonical/generated notification SQL. Confirmed recipient filtering and state predicates precede `(created_at, id)` pagination, repeatable-read count/projection, additive explicit-view markers, legacy mixed history, recipient-only mutations, archive/read independence, and denied title/source/run redaction.
- Core: notification view leaf, API/schema/query/mutation paths and canonical tests; account destination/parser/builder/route-key changes and scope-history tests. Confirmed strict marker/membership checks, legacy default/All acceptance, account headers, Query ownership, session/access fences, exact target serialization, and notification-specific parameters disappearing on canonical participating content links.
- Shared Views: notification page/link/labels, workbench dispatch, sidebar/icon changes, activity links, locale changes and canonical tests; existing detail/run-records/focus/scroll helpers. Confirmed known unavailable/mismatched selections do not mount/subscribe to private detail, same-issue drafts/list DOM survive selection, filtered-out previews survive confirmed writes, and query retry does not replay a write. No obsolete popover production exports/imports remain.
- Platforms: Web/native account route/title changes, native tab title/Bell/popout checks and account memento tests; existing navigation, scroll provider, tab coordinator/host/store and issue-window navigation. Notification paths fail issue-window admission and fall through to `openAccount` in a content popout, including the standalone workbench entry. AppLink retains shareable native hrefs and modifier/middle-click routing.

## Findings fixed

1. `apps/desktop/src/renderer/src/components/tab-bar.test.tsx:249,565`: the two new role queries passed unsupported `exact` properties to `ByRoleOptions`, causing Desktop typecheck to fail. Removed only those properties; literal accessible-name matching remains the default. The full tab-bar suite passes.
2. `apps/desktop/src/renderer/src/stores/account-tab.test.ts:23,25,43`: the new memento roundtrip wrote/asserted `notifications-list`, while production uses `account-notifications-list`. Updated the two writes and one assertion to the real container key. The complete account-tab suite passes.

## Findings not fixed

### P2 — Same-filter, same-issue history returns do not restore the list memento (R6/AC6)

`packages/views/collaboration/notifications-page.tsx:107–113` restores the list once per mounted scope/filter, guarded by `restored.current`. Its key at line 83 intentionally remains unchanged between notifications for the same filter. Native `ActiveTabHost` also stays mounted on account query changes (`tab-content.tsx:59–71`), and its restoration adapter is stable. The coordinator saves separate offsets for each account route, but Back/Forward cannot make the retained list read its destination's offset.

Concrete sequence: open notification A for issue X, scroll the list to 240, select B for the same issue/filter, scroll the list to 480, then Back to A. The coordinator has A's 240 memento; the retained list skips restoration and stays at 480. Existing page tests verify first-mount restoration and no reset on explicit selection, and the store test verifies memento storage, but none verifies this history-return case.

Detail return semantics need checking in the same regression. The notification detail ref is stable for a retained issue (`notifications-page.tsx:45–49,86`), canonical detail has a once-only restoration guard (`collaboration-detail.tsx:41–48`), and both native `stepHistory` (`tab-store.ts:1261`) and Web Back/popstate mark the navigation as `isScopeResume:false`. An old comment/run/body anchor is consequently eligible for fresh focus instead of the user's saved detail offset.

Recommendation: add a same-issue A/B Back/Forward regression with deliberately different list/detail offsets, plus an explicit-link control proving that selection still preserves list/draft lifetime and fresh anchor focus. Fix return intent/restoration through the existing platform mementos, preserving the stable list and same-issue detail. This is a substantive navigation behavior decision across existing owners, so it was reported to the coordinator before any behavior edit and left unchanged by this reviewer.

### Remaining coordinator acceptance and spec sync

- Production Web and actual Electron wide/compact painting, inert/hidden focus behavior, long-title overflow, exact comment/run/body focus, read/archive transitions, and Back/Forward/scope-resume offsets still require the planned live acceptance. JSDOM assignments and class assertions do not prove those outcomes.
- The navigation/workbench/record specs still describe the former popover. Update those contracts during coordinator integration, including the new destination/view-marker semantics and the final restoration decision. Shared spec writes were expressly outside this reviewer's ownership.

## Verification

Reviewer-executed checks (all exit 0):

- `pnpm --filter @multica/desktop exec vitest run src/renderer/src/components/tab-bar.test.tsx`: 24 tests pass.
- `pnpm --filter @multica/desktop exec vitest run src/renderer/src/stores/account-tab.test.ts src/renderer/src/platform/issue-window-navigation.test.tsx src/shared/issue-window.test.ts`: 37 tests pass.
- `pnpm --filter @multica/views exec vitest run collaboration/notifications-page.test.tsx collaboration/run-navigation.test.tsx navigation/app-link.test.tsx`: 49 tests pass.
- `pnpm --filter @multica/core exec vitest run collaboration/notification-queries.test.ts collaboration/access.test.ts`: 12 tests pass.
- Scoped ESLint on the two reviewer-edited Desktop tests: pass, zero file warnings.
- Task production/test `git diff --check`: pass. Task Go handler/service `gofmt -l`: no output.

Total reviewer-executed tests: **122 pass**. The Views run includes the existing JSDOM diagnostic `Not implemented: navigation to another Document`; the suites still pass. No listeners, DB suites, or agent subprocesses were started.

Coordinator-reported prior checks: all package lint commands passed with 31 existing unrelated warnings; Core/UI/Views/docs/plugin typechecks passed; original Desktop typecheck found the now-fixed role-query options. Final Desktop/Web typecheck rerun belongs to the coordinator and was pending when this note was first written. Prior owner evidence also covers Core notification boundary tests, Go handler/service isolated DB cases, sqlc generation and focused Go vet; these were not rerun by this reviewer.

This review is complete as a review; the R6/AC6 finding and live acceptance prevent a full task quality-gate claim.
