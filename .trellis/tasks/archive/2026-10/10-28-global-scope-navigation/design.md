# 固定全局切换与工作位置恢复 — Design

## Interaction and layout

Operate surface; retain the current semantic colors, typography and component conventions. Add one persistent 48px shell row with `[工作空间] [协作]` on the left. It sits outside page canvases and sidebar collapse. On Desktop the row reserves the native traffic-light inset and marks interactive controls no-drag; the existing history/tab row and sidebar start below it. Web uses the same switch with its regular left inset. Workspace navigation remains contextual below the global row; collaboration retains Current / Participating within its page.

```text
┌ [工作空间] [协作] ─────────────────────────┐
│ 空间选择 / 任务 / 项目 │ 原任务、项目内容   │
└───────────────────────────────────────────┘
┌ [工作空间] [协作] ─────────────────────────┐
│ 协作 · 当前负责 / 我参与的                 │
│ 搜索与排序、任务列表 │ 所选任务详情         │
└───────────────────────────────────────────┘
```

Selected scope uses persistent weight/background/foreground and `aria-current`. Both items stay reachable at compact width; coarse pointers retain 44px targets. Current-scope activation is a no-op. Loading, empty and failed workspace discovery must not remove the shell or block account access. Keep create/retry available as secondary workspace actions, not duplicated global navigation.

## Behavior ownership and change boundary

- Shared Views owns a small `GlobalScopeNavigation` component with `activeScope`, `onWorkspace`, `onCollaboration` and optional secondary actions. It contains no platform imports or store. Remove AppSidebar's footer Collaboration link.
- Desktop platform/shell owns placement and resume dispatch; extend the existing tab store only with the most recent workspace pointer and any narrowly necessary navigation intent. No new desktop state store. `switchAccount()` and `switchWorkspace(slug)` omit target paths for global switching; explicit paths retain the existing direct-open contract. Clear the workspace mirror before account publication.
- Web platform owns a persistent authenticated shell, route observation, Next navigation and capture. Add small client navigation state under Core for Server/viewer-scoped last workspace/account paths. Retain path, query and fragment. Validate internal paths and existing account destinations; auth/onboarding/admin/one-shot routes do not overwrite workspace history. Keep Next APIs in `apps/web/platform`.
- Web restoration reuses the existing provider. Account mementos use `accountRouteKey(fullPath)`; workspace routes preserve their existing semantics. Include Server/viewer identity in ownership and clear old maps on identity changes. Capture against the rendered route so outgoing scroll cannot be assigned to the incoming scope.
- Shared collaboration details must differentiate resume from explicit focus. Use the existing restoration/view-state channel or a narrow platform navigation-intent signal. A consumed anchor must not steal scroll on resume; a repeated explicit link must still refocus. Preserve run/comment/shared-context-version meaning in URLs. Lock this contract with tests in both platform wiring and shared detail behavior.

## Data flow

1. Observe committed workspace/account location and bind it to the current Server/viewer.
2. Existing views record filters and scroll through restoration adapters.
3. Global activation resolves the last valid destination for the other scope, marking it as resume; current activation returns immediately.
4. Desktop changes active group without a target path; Web navigates to the recorded full path.
5. The new host reads saved view state and offsets, retrying detail restoration after data readiness as the current views already do.
6. An explicit link supplies a destination and new focus intent; it never consults the resume destination.

## First use and recovery

Prefer last valid workspace; otherwise use the current authoritative workspace list's first accessible space and the existing default route. A confirmed empty list reaches existing creation flow. A failed/pending list is not proof of membership loss: retain remembered state and offer retry; Collaboration remains available. Deleted/left spaces invalidate the remembered pointer only after authoritative evidence. Unavailable issues continue through current permission/error UI; no private server payload is persisted.

## Compatibility and alternatives

No route/API/schema migration and no dependency additions. Existing desktop profiles lack true historical workspace MRU; derive only a valid available fallback on first use and record future activations. Keep the old workspace-active-null-in-account contract. Do not infer recency from object key order.

Rejected: move only the footer button (does not restore either scope); merge tasks and collaboration (changes account/workspace semantics); keep both page trees alive (adds hidden subscriptions and a second state owner); rewrite the tab/scroll systems (existing mechanisms cover the requirement).

## Validation and rollback

Canonical store tests cover destination/pointer validation and identity transitions. Platform tests cover resume/direct intent and shell placement. Shared tests cover the switch and filter/anchor behavior. Actual browser/native acceptance verifies one-click roundtrip, scroll and geometry. Record runtime/build provenance; report unrun native coverage explicitly. Roll back the new shell/controller and pointer changes as one feature; no server or stored-data rollback is required.

## Research

- [Desktop](research/desktop.md)
- [Web](research/web.md)
