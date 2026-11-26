# Boundary and evidence design

## Ownership
Smallest gap: acceptance fixtures address retired shell labels instead of today's same business surfaces; old CLI spends its budget before the tenth runtime starts. The behavior under repair lives in e2e fixtures, not product permissions or timers.

Expected writes: e2e/fixtures/collaboration-detail-journey.ts, collaboration-pay-creation.ts, collaboration-resources.ts, collaboration-recovery-extras.ts, collaboration-installed-main.mjs, collaboration-installed-clients.ts; e2e/inbox-snooze.spec.ts; narrowly necessary fresh helper/tests; this task and campaign clients evidence/private paths. Shared controller/navigation helpers may need a coordinated narrow edit after confirming ownership. No host-runtime/cross-client execution fixture edits; runtime agent owns those.

No product component/backend edits unless runtime evidence proves a product gap and parent agrees to expand ownership. No changing timeouts, retries, skips, authorization assertions, gates or pinned CLI identity.

## Original evidence → hypotheses → discriminating checks
- D06 packaged Main login passes, old Collaboration heading absent: compare current account header/sidebar contract and original trace, then run the entire original scenario after semantic assertion repair. Never replace real Main by renderer evidence.
- D07 manual-detail/PAY/resource publishing: inspect exact failed locator and current heading/disclosure/creation dialog, retain all API/state/business assertions and run whole cases on both platforms where specified.
- D08 snooze: compare workspace title resume button with adjacent Select workspace menu button; verify selected workspace and complete wakeup/unread scenario.
- D09 Desktop delayed-read: six identical denial strings; scope to detail denial region while preserving absence/protected-read/source-loss assertions. Native natural response hold requires a fresh actual probe, not old acceptance clearance.
- D11 CLI: original copied UID501 mode0700 failed under cap-drop ALL; byte-identical0755 passed entry. Preserve cap-drop, network-none, readonly, no-new-privileges and pinned v0.6.7 image. Inspect original command timestamps and resource ledger to separate Docker wrapper overhead, copy/start latency and relay/API latency. Optimize only measured fixture overhead, retain ten CLI executions and full cleanup inventory. Cancellation must report cleanup failures.

## Runtime interface
Parent supplies migrated task DB, production Web/API/renderer builds and owned listener proofs; clients supplies fresh per-journey allocation/admission adapters, semantic fixture changes and launch requirements. Source identity is base SHA plus dirty diff/file hashes; old SHA/clearance substitutions are forbidden. Each test gets unique actors and private profiles; bounded sequential groups avoid auth-rate collisions without changing rate limits.

## Rollback
Small uncommitted diffs only; preserve failures and owned DBs for inspection. Stop only recorded owned processes/resources; keep cleanup receipts including fallback use.

## macOS protocol boundary
Capture the complete URL-handler set and default before package preparation, after preparation and around Main launch; restore the exact original set/default and retain receipts. No package prep or launch before that baseline. Parent owns package preparation and must supply that proof.

## Confirmed current navigation contract
`packages/views/layout/app-sidebar.tsx:817` renders CollaborationNavigation as a CollapsibleTrigger and explicit AppLink accountPath({view}) children. `apps/web/platform/scope-shell.tsx` exposes only onWorkspaceSelect; there is no account-resume UI callback. Therefore the retired collaboration scope button cannot be selected without fabricating UI. The repaired journey uses the actual workspace-name resume button, then Participating and the same issue row; retains selected issue, exact saved scroll and one-account-tab assertions. This is app navigation, not browser history. If the same saved scroll fails, retain that failure for review rather than substitute history. Main's synthetic login actor is onboarded with zero workspaces and a fresh profile: account bootstrap defaults to current, so require exact Current handling (no alternative heading).

## Independent review C1 (unresolved original behavior)
Explicit issue reselection has been removed: it would hide loss of remembered selection. Current UI has no Web account-scope resume operation: WebScopeShell only wires onWorkspaceSelect; CollaborationNavigation's disclosure toggles without navigation and children push list-only account paths. The test will require original issue selection and saved scroll immediately after actual Participating navigation, with no issue click. If the directory clears selection as its source implies, retain FAIL for this original business condition. Product changes to introduce a resume affordance require parent scope decision; no history substitution and no success based on explicit issue reselection.

## Frozen native snooze observer regression
Final snooze01 had strict source integrity PASS, five cases PASS, native FAIL at observer settle with inbox/snoozed body_resource_unavailable. Retained evidence shows failures just before new /api/me and socket after explicit locale reload; observer only awaited responses already received, excluding pending requests whose headers had not arrived. Track matching original requests through requestfinished/requestfailed, then drain actual response readers before locale reload/teardown. Keep body/schema/error assertions, transport and deadlines unchanged; rerun all six original cases on separate fresh DB. This extends the fixture write set to e2e/fixtures/inbox-snooze.ts.

## Approved C1 amendment (2026-11-25)
User explicitly selected restore explicit return affordance and complete regression. Add AppSidebar onAccountResume callback, displayed once as Return to previous collaboration / 返回上次协作 in expanded, collapsed-disclosure and icon-only sidebar (native button with accessible label/tooltip). Keep disclosure title toggle and explicit list links unchanged. Platform owns remembered destination: Web resolves current identity account path then useWebScopeResume; Desktop calls resumeDesktopScope(account) with no path. No extra persisted state, no explicit issue reselection/history navigation, no mobile changes. Callback does nothing when already in account scope to preserve focused draft/view.
Expected writes: packages/views/layout/app-sidebar.tsx and test; en/zh-Hans collaboration locale; apps/web/platform/scope-shell.tsx and test (existing shells spread props); Desktop desktop-layout.tsx and workspace-gate test, existing navigation regressions; canonical manual fixture; global-scope-navigation.md. Tests prove disclosure remains independent, return stays available in icon/collapsed modes and one callback, identity-scoped full query/fragment on Web, no-path native resume and original single-tab/scroll selected issue E2E.
Fresh builds/run under new external snapshot c1-final01; never overwrite prior native/client frozen trees. Parent provides clients_manual_final01. Capture exact URL protocol full set/default baseline before preparation/launch, restore after all native runs through parent-authorized helper. Earlier13finalPASS retained with old frontend-source scope.

C1 review follow-up: account resume invalidates any earlier pending workspace lookup immediately, before an account no-op or route commit. A deferred lookup regression fails before the change and proves the stale resolution cannot navigate after resume intent. No scope state duplication. Web build must be refreshed after this two-line change; prior C1 build remains preserved.

## C2 approved strict observation closure
Retain snooze-final02's real200 client_closed late-reader fact and authentic6PASS terminal; withdraw error-free acceptance claim. Seal future request admission explicitly and record that boundary before shutdown; all requests admitted earlier retain response/body/schema/error observation, including responses arriving during the drain. New post-boundary requests are counted as excluded, never relabeled as read. Drain admitted requests+readers, then close client and assert the complete observer error set after close. A small fixture-owned lifecycle helper plus deterministic deferred request/reader regressions proves a reader arriving after seal is drained and its failure propagated. Full6-case replay uses new parentDB clients_snooze_final03 with latestC1UI.
