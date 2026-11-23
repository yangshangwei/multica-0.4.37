# Final sidebar review

## Scope and code review

Reviewed all 22 task-owned paths from `task.json`, including the new Core modal-store and shared registry suites and the three approved display-label test updates. Loaded the saved native check hook output, task requirements/design/plan, implementation/browser evidence, the relevant package indexes and the authoritative Chinese conventions. Preserved the unrelated dirty files and the concurrent `62d73b0c7` concise Settings-label change; no files were staged or committed.

The approved visible Start work composition, workspace captions, direct collaboration source/focus handling, single AI Chat destination and actual-workspace-only unread observer match the task. The Core request stays nonpersisted and target-bound. Reference-identity reconciliation protects a newer intent from an outgoing effect; API/session/viewer and exact-path checks retire obsolete requests. The actual registry gates consumption on the real provider and API mirror, consumes before opening the existing preference-backed form, and reads live auth ownership. Session teardown explicitly clears the pending slot. No product flaw was found in that handoff or package boundary.

## Findings fixed

1. **Unread announcement lost behind a stable link name.** `NavRow(scopeCaption)` intentionally gives the Chat link the stable accessible name AI Chat / AI 对话, which overrides name-from-content. Its description originally contained only the workspace, so focusing the link no longer announced the known positive message count. Added an optional accessible description to the existing row and supplied the same positive count used by its visible badge. The visible workspace caption and stable action name are unchanged. Zero/unknown counts retain workspace-only guidance. Existing localized FAB count copy counts *chats*, so the tiny bilingual `sidebar.chat_unread` message-count keys were added with English singular/plural and Chinese plural conventions.
2. **Compact text geometry skipped nested clipping spans.** The E2E primary-label Range check started its clipping walk at the control and missed the label's own `truncate` ancestors. It now walks from each matched text node's parent through the actual ancestors, retaining the painted target and viewport assertions.
3. **Selected E2E journeys retained the prior Settings name.** `global-scope-navigation.spec.ts`'s Chinese footer assertion and the two Settings links in `settings-preferences.spec.ts` still expected the long name removed by parallel commit `62d73b0c7`. Updated only those owned selectors to 设置 / Settings; the product label and remaining journeys are preserved.

Review changed only these existing task paths:

- `packages/views/layout/app-sidebar.tsx`
- `packages/views/layout/app-sidebar.test.tsx`
- `packages/views/locales/en/layout.json`
- `packages/views/locales/zh-Hans/layout.json`
- `e2e/global-scope-navigation.spec.ts`
- `e2e/settings-preferences.spec.ts`

## RED/GREEN accessibility evidence

Before the product fix, the English summed-message description, English singular description and Chinese description regressions all failed for the expected omission: the actual description was only Acme. The existing all-read case remained passing. The extended existing English test uses the package's production i18n wrapper so its accessible-name assertion exercises the real resource map.

- RED: `pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx -t 'summed unread_count|announces.*unread|every session is read' --maxWorkers=2` — three expected failures, one passed; `/tmp/sidebar-start-work-check-unread-red.log`.
- GREEN: `pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx locales/parity.test.ts --maxWorkers=2` — **2 files / 124 tests passed**, exit 0; `/tmp/sidebar-start-work-check-unread-green.log`.

## Verification

- Forced full-scope static check: `pnpm exec turbo run lint typecheck --filter=@multica/core --filter=@multica/views --filter=@multica/web --filter=@multica/desktop --concurrency=1 --force` — **11 tasks successful, zero cached**, exit 0. Includes dependent packages. Existing warnings are 27 outside this task in Views and one existing Desktop hook warning; no lint errors. Log: `/tmp/sidebar-start-work-final-static.log`.
- After the accessible-description fix, fresh `pnpm --filter @multica/views typecheck` and explicit eslint over `layout/app-sidebar.tsx` / `layout/app-sidebar.test.tsx` both exited 0. Logs: `/tmp/sidebar-start-work-check-views-final-types.log`, `/tmp/sidebar-start-work-check-changed-lint.log`.
- Additional handoff-adjacent verification: `pnpm --filter @multica/core exec vitest run navigation platform/workspace-storage.test.ts auth/store.test.ts platform/auth-initializer.test.tsx --maxWorkers=2` — **6 files / 83 tests passed**, exit 0. Log: `/tmp/sidebar-start-work-check-boundaries.log`.
- Scoped E2E TypeScript check after the final test corrections: `pnpm exec tsc --noEmit --module esnext --moduleResolution bundler --target es2022 --esModuleInterop --skipLibCheck --types node --jsx react-jsx e2e/fixtures/sidebar-navigation.ts e2e/global-scope-navigation.spec.ts e2e/settings-preferences.spec.ts e2e/collaboration-creation-default.spec.ts` — exit 0. Log: `/tmp/sidebar-start-work-check-e2e-final-types.log`.
- Playwright discovery after the final corrections: the three planned files still list **six tests**, exit 0. Log: `/tmp/sidebar-start-work-check-e2e-final-list.log`.
- Task-scoped `git diff --check` — exit 0. Existing source metadata, including the relevant HEAD executable modes, is unchanged.
- Confirmed the implementer's evidence for the initial **186 Views / 22 Core** focused tests and its separate live-auth RED/GREEN regression. The reviewer runs above supplement that evidence; their counts overlap and should not be summed as unique tests.

## Findings outside the task

Broader search found other exact Workspace settings sidebar selectors made stale by the concurrent Settings-label commit. They were not modified because the parent assigned no ownership of those journeys: `e2e/navigation.spec.ts:32`, `e2e/iterations-audit-desktop.spec.ts:299`, `e2e/mcp-desktop.spec.ts:333`, `e2e/fixtures/collaboration-knowledge.ts:218`, and `e2e/fixtures/iterations-i1.ts:35` / `:186` / `:203`. This is existing parallel-change E2E drift, not a remaining sidebar product defect or a claim that those wider journeys passed. `Workspace settings saved` in `settings.spec.ts` is unchanged toast copy and must not be mechanically renamed.

## Main-session acceptance

Product files were frozen and the parent notified immediately after the small accessibility fix. Main owns the final detector, source-bound production Web/Desktop build, actual browser/native run, screenshots, authoritative spec synchronization and commit/archive. Update the navigation spec to explicitly preserve the positive localized unread-message announcement alongside the stable Chat action name and visible workspace caption. This check does not claim real-browser, physical-input or spoken-assistive-technology acceptance.

## Actual-browser test repair follow-up

The parent dispatched two failures from its actual production run for diagnosis. This follow-up uses systematic debugging and changes the owned test assertions only; it does not change product modal semantics, navigation adapters, shared native fixtures or API behavior.

### Confirmed causes

- **Web background-role assertion while the modal is open:** the ordinary creation popup passed both count-one and visible assertions, then the test tried to locate background Application navigation / Workspace issues through the accessibility tree. The provided `error-context.md` exposes only the out-of-modal notification region, and the production creation shell uses the existing modal Base UI Dialog. Modal isolation makes the background role unavailable. The active-destination assertion now runs after Escape and popup removal, retaining the actual modal and one-popup identity proof.
- **Cached legacy GET assumption:** `IssueDetail` already calls `useQuery(issueListOptions(wsId))`, whose seven category catalog reads populate the cache. `createQueryClient()` defaults to `staleTime: Infinity`. Moreover, the current IssueSurface repository explicitly serves list, board, table and swimlane through the read-only Table query channel. Requiring a new GET `/api/issues` during the account-to-list handoff was therefore invalid. After closing and disposing the popup observer, the test reloads the admitted list once, observes the actual read-only POST `/api/issues/table/rows`, verifies status 200 and finished transport plus the selected workspace request header, and asserts that the transient modal did not replay. These checks occur before the original account filter/history/scroll mementos are constructed.
- **Native shareable anchor format:** `AppLink` deliberately renders `getShareableUrl(href)` when the native tab adapter provides `openInNewTab`, preserving copy-link-address behavior for packaged renderers. The failed native link was an absolute URL with the correct selected workspace pathname. `expectStartWork()` now compares the parsed URL pathname for either platform rather than requiring a route-relative raw attribute. The exact intended workspace is still asserted.

### Repairs and ownership

The browser repair changes only `e2e/global-scope-navigation.spec.ts`: native/Web pathname comparison, post-dismissal background assertion, and a cold read-only rows request plus reload-without-replay proof. No force clicks, added timeouts, visibility overrides, API mutations or AI execution were introduced. The original route/history/scroll/identity assertions remain in place.

The parent separately reported newer no-workspace visible-caption copy in the root product file. This reviewer did not author that change: the only reported product repair was the prior unread description/keys, and this follow-up edited no product files. The root now uses the existing `open_workspace_to_create` / `open_workspace_to_chat` text for the corresponding disabled captions. Two text assertions were added to the existing no-workspace component regression to verify that visible explanation alongside its existing disabled/ARIA/query-isolation checks.

The parent is rebuilding an isolated source snapshot with corrected internal workspace-package links before rerunning all six journeys. Actual rerun acceptance remains parent-owned.

### Final follow-up freeze

- `e2e/global-scope-navigation.spec.ts`: `fe9daa1a2a579495b5f482d5a98a6331f27f5d8ad79cd5fa5163e75b89223cc2`
- `packages/views/layout/app-sidebar.test.tsx`: `0ffd1d4863ac4c117efdfaae76453905b7699aa6d2896850753e321c8959b9c0`
- Current unchanged-by-this-review product `app-sidebar.tsx`: `91f2a45e2ae575ece04bb502f3a8f2100467bdb5eac56a7564e7e23cef9409bb`

The parent received source freeze before the read-only verification commands. Follow-up command outputs are retained under `/tmp/sidebar-start-work-check-browser-repair-{types,list}.log` and `/tmp/sidebar-start-work-check-no-space-captions.log`.

Follow-up verification passed:

- Scoped `tsc --noEmit --module esnext --moduleResolution bundler --target es2022 --esModuleInterop --skipLibCheck --types node --jsx react-jsx e2e/global-scope-navigation.spec.ts`: exit 0.
- `pnpm exec playwright test e2e/global-scope-navigation.spec.ts --list`: exit 0, both existing Web/Desktop journeys discovered.
- `pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx -t 'without a workspace or workspace providers' --maxWorkers=2`: one relevant no-workspace regression passed, 57 unrelated cases skipped, exit 0. The new visible-caption assertions and existing disabled/ARIA/no-preview-query checks all remain satisfied.
- Task-owned follow-up `git diff --check`: exit 0; the frozen hashes above remained identical after verification.

The repaired production browser rerun is still main-owned; static discovery is not recorded as a browser pass.

## Anchored Back/Forward restoration repair

The next source-bound production run verified the new task/collaboration/chat paths, the cold Table request header, one-popup behavior, the first saved 620px offset and query-error recovery in both clients. Both existing global journeys then failed their original anchored return assertion: expected 800px, received 325px with `#description` focused.

The parent authorized a bounded extension to the existing shared collaboration detail and its canonical tests after the code and diagnostic evidence agreed:

- Both navigation adapters mark Back/Forward with `isHistoryTraversal: true` and `isScopeResume: false`.
- The detail's normal restoration and explicit-anchor effects previously inspected only `isScopeResume`. An anchored history return therefore skipped its saved offset and explicitly moved to the description anchor.
- A temporary Web-only timeline probe retained the original 800px return assertion and added a pre-departure assertion. The actual trace showed user/manual scroll 800, account scroll 800, departure 800 and workspace click 800; after return, a newly mounted anchor node 3 was explicitly focused by the same detail callback at offset 325. This ruled out an early manual-scroll race and offscreen sidebar click. The parent retained the trace under the task's acceptance artifacts.
- The deterministic existing-detail regression sets `isHistoryTraversal: true` with scope resume false and a description anchor. Its pre-fix run failed with **zero restoreScroll calls**. The test also asserts saved 800 and no anchor focus, while the existing explicit repeated-open case remains required.

The minimal product repair derives `restoringPosition = navigation.isScopeResume || navigation.isHistoryTraversal` in `CollaborationDetail`. Normal restoration, explicit-anchor suppression, comment reveal, auxiliary/delivery restore flags and the submission-ready focus guard use this condition. The notifications-specific `returnRestorationKey` branch is unchanged. No platform adapter, API, store or sibling implementation was changed.

The temporary E2E focus/prototype monkeypatch and timing listeners were removed after the trace proved the cause. The original explicit-anchor flow and 800px return assertion remain, with a substantive 800px assertion immediately before departure. No sleeps, force clicks or changed expected offsets were introduced.

Verification and freeze:

- RED: `pnpm --filter @multica/views exec vitest run collaboration/collaboration-detail.test.tsx -t 'history traversal without replaying' --maxWorkers=2` — one expected failure, 15 skipped; `/tmp/sidebar-start-work-check-history-anchor-red.log`.
- GREEN: `pnpm --filter @multica/views exec vitest run collaboration/collaboration-detail.test.tsx collaboration/detail-auxiliary.test.tsx collaboration/delivery-panel.test.tsx collaboration/run-navigation.test.tsx --maxWorkers=2` — **4 files / 30 tests passed**, exit 0; `/tmp/sidebar-start-work-check-history-anchor-green.log`.
- Product source frozen: `collaboration-detail.tsx` SHA256 `b7f2e297d77eebd9473fe0d1677cdffcbb09c44facdc7d984c45380d83e07f4b`.
- Canonical detail test SHA256 `3a767936ef4a2c1b3c33f3684e858865f7234d17b0a1bb799dc3e4302e66b34d`.
- Final E2E without diagnostic instrumentation SHA256 `286141068ce11991979345b76fe630ed298654b6025fe9e12d4a7d49523cb0aa`.

Main was notified immediately after the full local suite passed, allowing its final source-bound rebuild/browser run while fresh Views typecheck, changed-file eslint and scoped E2E checks finished independently. The static/actual-browser acceptance boundary remains explicit.

Final history-repair static verification also passed after freeze:

- `pnpm --filter @multica/views typecheck`: exit 0; `/tmp/sidebar-start-work-check-history-anchor-types.log`.
- Explicit eslint over `collaboration/collaboration-detail.tsx` and its canonical test: exit 0 with no findings; `/tmp/sidebar-start-work-check-history-anchor-lint.log`.
- Scoped E2E TypeScript check and discovery: exit 0, the same two original global-scope journeys remain; `/tmp/sidebar-start-work-check-history-anchor-e2e-{types,list}.log`.
- Task-owned diff whitespace check: exit 0. All three frozen source hashes remained identical after verification; no further product edits were made.
