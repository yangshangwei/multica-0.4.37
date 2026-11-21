# MCP welcome design

## Direction and authority

This is a scoped extension of the existing Operate interface. `SkillWelcome` and `AgentWelcome` are the visual authority: restrained semantic surfaces, existing type scale, a left introduction and a right manually controlled scenario carousel. The user approved this direction in the preceding conversational summary and requested development. Keep all product behavior within that scope.

Use the already-loaded impeccable onboarding guidance and craft floor. Keep the new surface local, omit a redundant eyebrow, make the examples recognizably illustrative, and do not redesign global tokens or existing welcome pages. The three scenarios demonstrate real supported concepts rather than a synthetic success dashboard: choose a service, supply its configuration, explicitly choose agents. No executable tutorial, fabricated live connection status, account credentials or timers.

## Architecture and state

- Add a presentational `McpWelcome` under `packages/views/mcp/` with a sibling CSS module and explicit module typing, following the existing welcome components. Local state contains only the selected scenario.
- The canonical gate lives in `McpWorkspaceTab`, which already owns the query snapshots and permissions. Require the standalone page, current member, defined successful inventory and no active member/inventory failure before checking the raw saved array length.
- Preserve `SettingsQueryFeedback` as a stable sibling. A recoverable error with cached empty data shows its retry message and compact empty content rather than a welcome. Cached populated content retains existing refresh/recovery behavior.
- Select only the central content presentation. Keep mutation hooks, rename state and every existing dialog mounted under the existing owner; do not return early from a welcome branch and accidentally unmount creation/assignment forms.
- The welcome uses the existing market navigation callback and `openCustomEditor`. Gate creation from existing `canManage`; provide a live button ref if needed for the existing dialog focus lifecycle. Keep one action per purpose during the welcome; populated/header behavior stays unchanged.
- Keep the local carousel scoped by workspace (the existing keyed `McpWorkspaceTab` already remounts). Inactive panels are `aria-hidden` and `inert`, position controls expose `aria-pressed`/`aria-controls`, and status announces the selected scenario. All authored transitions respect reduced motion.

## Alternatives

1. **Chosen: local presentational component with existing query owner.** Matches references and isolates the behavior with a small reversible diff.
2. Route-level gate in `McpPage`: rejected because it duplicates membership/server queries and splits permission/recovery ownership.
3. Generic shared welcome/carousel framework: deferred because it expands scope across already-shipped pages without being necessary for this feature.

## Compatibility and rollback

No new route, public API, backend field, storage key or dependency. Both Web and Desktop consume the same view and navigation adapter. Settings presentation retains existing structure. Reverting the welcome component, locale keys and central presentation selection restores the original UI; existing persisted MCP records are untouched.

## Verification

Canonical state/permission/query transitions belong beside `mcp-tab.test.tsx`; local scene interaction/content belongs beside `mcp-welcome.test.tsx`. Existing configuration and assignment suites remain canonical for those forms. Use the existing MCP/Skill browser harness for actual compiled CSS, keyboard/focus, locale/theme and narrow/desktop captures. Inspect the captures against current Skill/Agent references, persist a visual verdict, run impeccable's detector once after UI completion, and fix concrete findings in one bounded pass.
