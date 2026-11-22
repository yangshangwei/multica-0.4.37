# Sidebar start-work design

## Approved interaction

The fixed sidebar header contains the product name, existing scoped search when applicable, and a named Start work group. Its visible order is New issue, Start collaboration, AI Chat. Ordinary issue creation is the primary action. The other controls retain the existing sidebar chrome, icons, typography, hover, focus, and active tokens.

Ordinary creation and AI Chat display the authoritative sidebar-selected workspace name. Collaboration displays Cross-workspace and its neutral Beta marker. The existing Collaboration directory stays above the Workspace directory. Chat is removed from the latter's personal links, and its native destination/selected state/unread feedback move to the header.

## Behavior owners and change boundary

The current gap lives in `packages/views/layout/app-sidebar.tsx`: a dropdown hides creation, personalNav contains Chat, and ordinary creation accepts only the actual route workspace. Replace the dropdown with a small shared Start work composition. Keep the existing creation forms and the account/workspace navigation adapter.

`packages/core/modals/store.ts` is the owner for the transient ordinary-creation handoff. An account shell cannot open workspace forms directly: its workspace is a preview and only its feedback host is mounted. A target-bound, nonpersisted intent recorded before navigation survives the Web account-to-dashboard remount. Consume it once after the actual target workspace route, viewer and API owner are current; call the existing preference-backed creation helper only then. Ordinary active-workspace clicks continue to open directly.

Do not await `NavigationAdapter.push`, install the preview as workspace context, introduce timeout-based scheduling, add a public creation URL flag, or maintain another store. Navigation away, owner/reset changes and a newer interaction must cancel or supersede the original intent. Consumption must be atomic to survive StrictMode and repeated renders. Use current Core identity/reset facilities discovered in research.

AI Chat uses a native `AppLink` to the selected workspace's existing chat route, so Web modifier-click and Desktop cross-workspace tab wiring remain supported. Scope-specific unread queries and the draft dot remain beneath the actual workspace boundary. Account preview rows may navigate but must not run workspace reads. Existing unread-message counting, foreground/open-session exclusions and unknown-versus-zero semantics remain unchanged.

Collaboration opens its existing controlled entry with the selected workspace UUID, preserving original recovery/source locks. Its `returnFocus` points at the visible Start collaboration button, rather than the removed New trigger.

## Files

- `packages/views/layout/app-sidebar.tsx` and its canonical component tests: composition, action ownership, visibility, scope labels and unread wiring.
- `packages/core/modals/store.ts` and focused store tests, `packages/views/modals/registry.tsx` and focused host tests, and `packages/core/platform/session-cleanup.ts`/its existing tests: one-shot route handoff, actual workspace admission and explicit session teardown. Research confirmed these existing owners; no platform adapter change is required.
- `packages/views/locales/{en,zh-Hans}/{layout,chat}.json`: Start work/scope/unavailable copy and consistent Chat label. Collaboration form titles remain existing product terminology.
- `packages/views/collaboration/collaboration-detail.tsx` and its existing regression suite: the production acceptance uncovered an existing anchored-history return omission. Both platform adapters publish Back/Forward as `isHistoryTraversal: true` with `isScopeResume: false`; the detail must treat that explicit return as restoration rather than replaying the old anchor. Limit this correction to the existing restore/focus guards and their sibling wiring.
- `e2e/fixtures/sidebar-navigation.ts`, `e2e/global-scope-navigation.spec.ts` and selector-only affected E2E callers: direct visible actions and browser scope assertions.
- `apps/docs/content/docs/collaboration.zh.mdx` and `.trellis/spec/core/frontend/global-scope-navigation.md`: accurate documented navigation and the new handoff contract.

If research identifies necessary cleanup wiring beyond this list, identify the existing owner and justify it before editing. Platform code is extended only if shared/core ownership cannot correctly observe actual route admission.

The finalized research is `research/navigation.md`. Capture the API instance, `getSessionScope()` epochs, authenticated viewer, target UUID/slug, original complete path and exact target path. The actual `ModalRegistry` validates the real provider plus platform workspace mirror and path before atomic consumption. A sidebar-side committed-path/explicit-navigation cancellation retires superseded waiting intents, and `clearClientSessionData()` clears the pending slot on session teardown.

## Verification and risks

Meaningful unit regressions cover direct entry, delayed route admission, one-shot consumption, owner changes/cancellation, no-preview queries, collaboration source/focus, and moved unread/active behavior. Existing tests own creation mode/draft semantics; do not duplicate them.

Production Web and actual Electron renderer journeys verify one click from account to target creation, target API workspace ownership, direct collaboration opening/focus, AI Chat navigation, compact sheet dismissal, both locales/themes and preserved scope restoration. Use the existing isolated verification-environment helpers and existing native fixture. No live model/daemon execution is required.

The material risk is a creation intent outliving the navigation that owns it; cover cancellation and identity fences before visual review. Layout risk is long workspace names and 320px compact controls; clamp scope captions while exposing full names, retain 44px coarse targets, and keep primary actions outside the scrollable directory.

The actual source-bound production Web/Electron run established the original
anchored-return failure (saved 800px returns to the description's 325px offset)
after all new start-work operations and ordinary/query-error restoration passed.
Preserve the original 800px E2E assertion; add a failing history-traversal detail
regression before correcting its guard. This is a bounded extension required to
preserve the approved navigation contract, with no platform/storage redesign.

## Rollback

Revert this task's atomic product commit and its synchronized spec/documentation changes. The handoff is ephemeral and introduces no persisted-state migration or backend/data change.
