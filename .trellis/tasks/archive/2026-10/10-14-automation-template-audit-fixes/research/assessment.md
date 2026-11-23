# Research: Automation catalog layout and patrol output semantics

- Query: Assess the approved A01–A07 fixes against existing visual evidence, verify all five patrols' actual follow-up rules, and identify reusable focus-restoration patterns.
- Scope: Internal; supplied screenshots, current shared views, server template registry/prompts, and project specs. Research only; no product edits or detector run.
- Date: 2026-10-14

## Findings

### 1. Operate-mode layout assessment

**Spatial thesis:** Keep the existing compact, category-colored catalog. The task path should read: title and setup facts → category filters → template identity/recommendation → description → default schedule and output → configuration. Put the secondary blank action beside the introduction, with natural wrapping on narrow containers. This matches the populated `design.md`; no alternate page design is needed.

Evidence is `.impeccable/audit/2026-10-14-automation/desktop.png` (1771 × 1080) and `narrow-light.png` (390 × 844), visually inspected for this assessment. The desktop capture shows the recommended label separated from its title and substantial whitespace above the other first-row footers. The narrow capture shows the same recommendation as an extra line and blank creation reduced to an isolated plus. Existing measurements in `report.md` record first-row/other-row heights of 222/194px and prerequisite guidance at y=1144px; those measurements were not repeated in this research pass.

| Dimension | Evidence and minimal recommendation |
| --- | --- |
| Reading order | `packages/views/autopilots/components/autopilot-template-catalog.tsx:122` already owns the title/description/action group; its prerequisite hint is currently after all cards at `:238`. Move that fact above the filters and add the default-plan fact there: schedules are defaults, timezone is selected next, and edits happen after creation. Replace overlapping introductory wording instead of stacking another onboarding panel. |
| Grouping | Keep the two short setup facts near the title, then a larger gap before the filters. The existing `mt-2`, `mt-6`, `mb-5`, and `gap-4` rhythm at catalog `:127`, `:155`, and `:179` already distinguishes the introduction, filters, and card list. Use proximity and existing spacing rather than another border/divider. |
| Rhythm | The current 40px icon at catalog `:208` defines an existing header band. Put “推荐起点” / “Recommended starting point” in the adjacent title block as a compact second line, with readable semantic foreground/medium weight or a restrained neutral marker. Remove the independent footer row at `:218`. A single-line title plus a 16px caption can fit the existing 40px band; a padded badge or forced side-by-side title/long English badge may exceed it. Verify the actual English rendering. |
| Structure | Cards represent equivalent template choices and are native whole-card buttons (`:179`, `:192`). Preserve backend order and the `@xl` two-column / `@4xl` three-column grid. Keep the recommendation inside the first card's identity group; a featured full-width card would change both density and order without helping this task. |
| Density | Keep description clamping, schedule, and short output labels. Use one shared patrol explanation outside individual card footers if clarification is needed; do not repeat a paragraph on all five patrol cards. At narrow sizes, allow the footer's existing `flex-wrap` (`:225`) to accommodate honest English labels without clipping. Ordinary recommendation placement should remove the old 28px row penalty; content extremes may legitimately grow. |
| Adaptation | The catalog already has an optional labelled blank-action slot at `:131`, with a 44px compact target. Wire the empty-list dialog opener into that slot: `packages/views/autopilots/components/autopilots-page.tsx:837` currently mounts `EmptyAutopilots` without supplying it. Remove only the competing empty-state header action (`:675`); retain the populated-list action. Keep the label visible, let the header wrap naturally, and retain ≥44px height for coarse input even above the compact breakpoint. The generic header intentionally becomes a 32px icon at `packages/views/layout/collection-page.tsx:143` and hides its label at `:148`, so a generic-header redesign is unnecessary. |
| Extremes | Keep DOM order equal to visual order. Let long titles, raw-cron fallback, translated labels, and enlarged text wrap; use `min-w-0` and avoid fixed card heights or absolutely positioned badges. Preserve a blank path in loading/error/empty-catalog states, because the existing action slot is outside those branches (`catalog :131`, `:139`). Retain the active-filter fallback when a backend removes a category (`:113`). Fresh entry must not focus a stale card, and delayed data must not steal focus from a dialog or a deliberate user selection. |

**Concrete reuse for A01/A02:**

- `packages/views/settings/components/integrations-tab.tsx:59` stores the prior provider and a map of card refs; `:63` focuses the detail Back link and returns focus to the prior card; `:107` registers/removes each card ref. Its local solid semantic focus treatment at `:77` and `:151` (`outline-foreground`, 2px outline/offset) is an incumbent pattern for avoiding a global `--ring` change. `integrations-tab.test.tsx:93` tests the keyboard round-trip.
- Integrations stays mounted while its provider changes, so its `useRef` alone is insufficient for this gallery, which unmounts across configuration. Reuse the existing route/tab-scoped view-state channel: `packages/views/platform/scroll-restoration.tsx:74` documents explicit writes and `:163` / `:173` expose the read/write hooks, including clearing with `undefined`.
- The catalog already restores only after content is ready (`autopilot-template-catalog.tsx:91`) and saves group/scroll immediately before `onPick` (`:194`). Save a pending template key there as well, resolve it to a live card ref after scroll restoration, call `focus({ preventScroll: true })`, and consume it once. If missing, focus the currently selected filter. Do not replay this on refetch or override a newer focus owner. Preserve explicit `all` and `0`; do not add a URL default that supersedes the saved state.
- `packages/views/skills/components/template-skill-create-panel.tsx:123` records the clicked row. Its narrow Back handler at `:248` waits until the panel is visible, focuses that row only if `isConnected`, and otherwise focuses search (`:251`). This is a concrete connected-target/fallback pattern; it is not a reason to add an unguarded animation-frame focus loop.
- `packages/views/autopilots/components/autopilot-template-catalog.test.tsx:16` already provides the useful regression harness: a view-state map, no plain scroll entry, unmount/remount, and delayed data. Extend that behavioral path for focus and consumption; the existing fixture does not prove focus restoration. `template-create-autopilot-page.tsx:91` / `:118` already select the correct originating gallery through `return_to=autopilots`.

### 2. Verified patrol semantics and bilingual output copy

`PATROL_KEYS` at `packages/views/autopilots/components/autopilot-template-catalog.tsx:61` exactly matches the five v1 `run_only` entries in `server/internal/service/builtin_autopilot_templates_roster.go`. All five follow the same write rule: search this automation's existing open workspace issues; add evidence by comment when an issue covers the substantive finding; create a new issue only when none covers it; create neither issues nor comments when there is no substantive finding. This is prompt-directed conditional follow-up, not automatic issue creation on every run and not a prohibition on creating issues.

| Patrol / registry source | Default schedule | Verified threshold and follow-up source |
| --- | --- | --- |
| `workday-repo-audit` — roster `:37` | `0 9 * * 1-5`; v1, `run_only` | Dependency/test/stalled-change findings need real actionable impact; sensitive code alone is insufficient. `server/internal/service/builtin_autopilot_templates/workday-repo-audit/PROMPT.md:9`, `:14`, `:22`, `:25`, `:27`. Related findings are merged. |
| `hourly-queue-check` — roster `:102` | `0 * * * *`; v1, `run_only` | Checks stalled work, generated-file drift, and sustained quick-check failures. A failure that passes on retry or an unavailable check alone is not substantive. `server/internal/service/builtin_autopilot_templates/hourly-queue-check/PROMPT.md:9`, `:21`, `:24`, `:26`, `:39`. |
| `stale-pr-reminder` — roster `:123` | `0 10 * * 1-5`; v1, `run_only` | Only open, non-draft PRs created more than 24 hours ago with no review records qualify. Reuse one issue for the queue; no new issue per PR/day. `server/internal/service/builtin_autopilot_templates/stale-pr-reminder/PROMPT.md:9`, `:17`, `:20`, `:22`, `:34`. |
| `dependency-audit` — roster `:210` | `0 8 * * 1`; v1, `run_only` | Requires concrete project impact, such as a reachable known vulnerability or version lag warranting planned work. Related/transitive findings are combined. `server/internal/service/builtin_autopilot_templates/dependency-audit/PROMPT.md:9`, `:20`, `:23`, `:25`, `:37`. |
| `documentation-check` — roster `:231` | `0 14 * * 1`; v1, `run_only` | Reviews the previous seven days, verifies missing documentation by searching/reading, and reports gaps that affect readers. Internal-only changes are excluded; weekly related gaps share one issue. `server/internal/service/builtin_autopilot_templates/documentation-check/PROMPT.md:9`, `:18`, `:21`, `:23`, `:34`. |

Recommended copy, shared only by those five known patrol keys:

| Placement | zh-Hans | en |
| --- | --- | --- |
| Patrol footer | 按需创建或补充任务 | Creates or updates issues as needed |
| `create_issue` footer | 每次运行创建任务 | Creates an issue each run |
| Shared patrol explanation | 巡检发现实质问题时，优先补充已有任务；没有任务覆盖时才新建，无实质问题则不新增任务或评论。 | Patrols add substantive findings to an existing issue, or create one if none covers them. With no substantive findings, they create no issues or comments. |

The explanation makes “updates” concrete: it means adding evidence to a covering issue, chiefly through comments. Keep it visible without a hover requirement; a single concise line/paragraph associated with the gallery is enough. The footer is the compact comparison; the explanatory sentence need not repeat per card. A shorter acceptable English footer is “Follow-up issues as needed” only when the visible explanation remains present.

Keep unknown `run_only` templates on the conservative existing generic-mode fallback (`catalog :183`); do not infer the five patrols' no-finding silence contract for an arbitrary server template. Unknown execution modes must not receive either guaranteed outcome. `create_issue` is pre-dispatch issue creation, not a promise that a run will successfully finish or deliver a report; avoid “creates an issue after each run.” The roster's explicit mode distinction is at `builtin_autopilot_templates_roster.go:28`, with dated issue-title examples at `:58` and `:166`.

Related specs and documentation consulted:

- `.trellis/spec/views/frontend/automation-template-discovery.md` — shared catalog, conditional `run_only` semantics, server-owned adoption, route/group/scroll restoration.
- `.trellis/spec/views/frontend/component-guidelines.md:72` — integration catalog navigation and return-focus contract.
- `.trellis/spec/views/frontend/skill-market-discovery.md:62` — preserve opener identity and use a connected opener or persistent fallback.
- `.trellis/spec/views/frontend/desktop-core-accessibility.md:36` — a newer deliberate focus owner supersedes stale restoration.
- `.trellis/spec/server/builtin-templates.md:370` — canonical Chinese prompt bodies, five patrol/five summary distinction, and template adoption boundaries; `:427` documents pre-dispatch issue creation.
- `CLAUDE.md` and `apps/docs/content/docs/developers/conventions.mdx` / `conventions.zh.mdx` — shared-view boundaries, semantic tokens, and `issue` → 任务 while execution `task` remains distinct.
- External guidance: local Impeccable v4.5.1 `reference/layout.md` and `reference/operate.md` under `/Users/artisan/.agents/skills/impeccable/`. No external web research was needed. The supplied runtime audit records Electron 39.8.7 / Chromium 142.0.7444.265; this assessment did not launch them.

## Caveats / Not Found

- No blocker. Layout recommendations preserve the current style and fit the parent-owned design/implementation scope.
- Impeccable context found no PRODUCT.md, DESIGN.md, or matching surface brief. Its scoped-fix directive explicitly allows using incumbent code; the supplied screenshots provide visual evidence. No context artifacts were created.
- No individual manifest files exist beside the five PROMPT.md files; `builtin_autopilot_templates_roster.go` is the authoritative metadata roster. Prompts describe intended agent behavior; no live automation or real agent was executed, so this research does not prove model compliance.
- Responsive, zoom, contrast, restored focus, and new English text sizing still require the parent's planned post-change runtime verification. Audit measurements above are baseline evidence only. No detector scan or product tests ran in this research role.
- Source references record the inspected snapshot and may move during concurrent implementation. Only this research file was written.

## Follow-up research: Pending-focus lifetime after creation

- Query: Find an existing API that distinguishes a genuine gallery return from fresh entry, or clears the originating gallery's pending-focus entry after confirmed creation, including the empty-list origin.
- Scope: Internal; shared navigation/restoration contracts and Web/Desktop providers. No product edits.
- Date: 2026-10-14

### Findings

**There is no supported shared API for either operation today.** A complete fix for the successful-create leak across both origins needs a small restoration-capability extension or a broader change to ownership of the flow. Clearing the current entry is a valid partial fix for the creation-picker origin only. This is a concrete boundary limitation, not missing research.

| File / existing API | Verified behavior and implication |
| --- | --- |
| `packages/views/navigation/types.ts:1` — `NavigationAdapter` | Exposes `push`, `replace`, `back`, current pathname/search/hash, optional `canGoBack`, and forward. It exposes no location key, previous URL, transition kind, or route-state accessor. `canGoBack` (`:40`) answers whether history is available, not how the current page was entered. |
| `packages/views/navigation/context.tsx:45` / `:87` — `useIsNavigating()` | Reports a React transition or in-page pending swap. It is not a committed navigation-cause signal and cannot reliably classify native Back versus fresh entry. |
| `packages/views/platform/scroll-restoration.tsx:84` / `:173` — `getViewState`, `setViewState`, `useViewStateWriter` | Read/write only the current route. `undefined` clears one entry. No arbitrary-route argument or scope binder exists. Adding a pathname into `entryKey` does not escape the provider's current-route prefix. |
| `apps/web/platform/scroll-restoration.tsx:25` / `:53` / `:105` | Explicitly restores both Back and fresh-link visits, keyed by `pathname::entryKey`, not a history entry. The memoized adapter closes over Next's rendered pathname. `savedViewState` (`:40`) is module-private; there is no exported clearing function. |
| `apps/desktop/src/renderer/src/platform/tab-coordinator.ts:129` / `:148` | Every write resolves the live active tab pathname at call time, then calls `commitViewState`. Capturing a writer while the source gallery is mounted does **not** bind it to that gallery. This differs from Web, so retaining the old callback is not a portable workaround. |
| `apps/desktop/src/renderer/src/platform/tab-coordinator.ts:31` / `:181` | Desktop native Back is the tab store's virtual-history operation. The coordinator always projects it into React Router with `replace`. Browser `popstate`, `history.length`, or React Router POP detection would not identify Desktop Back. |
| `apps/desktop/src/renderer/src/stores/tab-store.ts:232` / `:817` — `commitViewState(tabId, routeKey, entryKey, value)` | The underlying Desktop store already supports arbitrary-route clearing and locates the tab by ID across workspace groups. This capability is hidden by the shared adapter. Shared views must not import this app-owned store. |
| `packages/views/autopilots/components/template-create-autopilot-page.tsx:91` / `:273` / `:400` | Existing `initialReturnTo === "autopilots"` distinguishes the two source routes. Confirmation is the resolved creation response with a nonempty `created.autopilot.id`; a resolved schema-fallback response is still failure. The correct invalidation point is after that guard and before the detail navigation at `:405`. |

The two indistinguishable cases are: (a) gallery → configuration → native Back → gallery, and (b) gallery → configuration → successful create → another page → fresh gallery. At the final mount, the shared code may observe the same pathname, query, pending key, history availability, and body focus. For the empty-list origin, even clearing when a later list is nonempty cannot close the gap: the created automation can be removed elsewhere before this tab returns to the list. The list then mounts directly as empty with the stale token. Group/scroll persistence is intentionally valid on both paths; the focus token has a different lifetime.

### Smallest complete change for the confirmed-create leak

If the parent relaxes the **no interface changes** constraint, extend the existing setter/writer with an optional target pathname; do not add a store, URL parameter, navigation-history abstraction, or new adapter family. For example, retain existing calls and add a third optional `routePathname` argument to `setViewState(entryKey, value, routePathname?)` and `useViewStateWriter()`.

1. Web can use `mementoKey(routePathname ?? pathname, entryKey)` against its existing map.
2. Desktop can use the supplied route with its existing adapter-bound `tabId` and `commitViewState(tabId, routePathname, entryKey, value)`. Preserve the existing active-route behavior for calls without the argument; do not guess the originating tab from whichever tab is currently active after an async response.
3. In `TemplateConfigureStep`, derive the source from the already parsed allowlisted origin: `initialDefaults.initialReturnTo === "autopilots" ? paths.autopilots() : paths.newAutopilotTemplate()`. After the nonempty-ID guard, clear `autopilot-template-focus` on that pathname, then navigate. Keep group and scroll entries intact. Failed or unreadable responses retain the pending return target.
4. Keep the catalog's existing consume-on-ready-return behavior. Add meaningful coverage for both origins, failed/unreadable creation, and a later fresh gallery mount after success; provider tests must prove explicit-path clearing leaves another route and another Desktop tab untouched.

This optional parameter is an **extension of the shared interface** and falls outside the parent's original view-only plan. It must be treated explicitly as a scope decision; disguising route selection inside a magic entry-key string would introduce an undocumented interface instead. Under the current strict no-interface-change constraint, there is no complete existing-API solution to recommend. The parent can apply the same-path fix while tracking the cross-origin limitation, but should not mark the whole lifecycle resolved on that basis.

Strictly restoring focus **only on Back**, including after an abandoned configuration navigates elsewhere and later re-enters, is a larger requirement than the confirmed-create leak. Even explicit-path success clearing does not by itself classify that case. It requires a platform-owned navigation-cause/history-entry contract or a flow owner that survives both routes. Neither is currently exposed. Do not claim a `canGoBack`, pending-transition, timeout, browser-history, nonempty-list, or captured-writer heuristic solves it.

### Related specs, references, and caveats

- `.trellis/spec/views/frontend/automation-template-discovery.md` requires route-owned gallery restoration and preserves explicit All/zero values; it contains no cross-route focus lifecycle API.
- `CLAUDE.md` keeps Next/React Router APIs and Desktop stores in app platform layers; shared views cannot reach through the adapter to clear the native memento directly.
- `.trellis/spec/web/frontend/state-management.md`, `component-guidelines.md`, and `.trellis/spec/desktop/frontend/state-management.md` add no API for this operation; their relevant general sections are placeholders.
- No external references, runtime checks, detector scans, tests, or implementation changes were needed for this source-level API inventory. Exact line numbers are from the inspected concurrent-work snapshot.

### Approved implementation handoff: optional origin pathname

The parent subsequently authorized this concrete extension to complete successful-create cleanup for both origins, and will revise the technical scope. That decision supersedes the earlier no-interface-change constraint for this capability. No remaining product decision or permission is required.

Narrowest signature, on the existing adapter and the callable returned by the existing hook:

```ts
setViewState?(
  entryKey: string,
  value: string | undefined,
  routePathname?: string,
): void;

// useViewStateWriter() returns:
(entryKey: string, value: string | undefined, routePathname?: string) => void
```

Omitting the argument retains current-route behavior. Providing it selects a pathname in the writer's existing browser-tab/Desktop-tab scope. Use path helpers; do not pass query strings or fragments. Reads and ordinary scroll restoration need no extension.

| Product file | Narrow change |
| --- | --- |
| `packages/views/platform/scroll-restoration.tsx:84` and `:173` | Add/document the optional setter parameter and forward it through `useViewStateWriter`. Preserve optional-adapter no-op behavior. |
| `apps/web/platform/scroll-restoration.tsx:110` | Construct the existing map key from `routePathname ?? pathname`. Keep the rendered-pathname default and existing maps. |
| `apps/desktop/src/renderer/src/platform/tab-coordinator.ts:148` | Choose `routePathname ?? activeRouteKey()`, then call existing `commitViewState(tabId, routeKey, entryKey, value)`. An explicit route must retain the adapter's `tabId`, even if another tab has become active; do not apply the live-active-tab guard before checking the supplied route. Default calls retain their existing guard. No change to `tab-store.ts` is required. |
| `packages/views/autopilots/components/template-create-autopilot-page.tsx:273` and `:400` | Use the existing writer. After confirmed nonempty creation ID, clear the pending-focus key at the origin derived from `initialDefaults.initialReturnTo` and `paths`, then perform the existing detail navigation. Leave group/scroll and failed-response return behavior intact. |

| Canonical test file | Necessary evidence |
| --- | --- |
| `packages/views/platform/scroll-restoration.test.tsx:15` | Small wiring check for the optional target argument through the public hook, plus continued no-op with a provider lacking the optional setter. Keep route behavior matrices in platform tests. |
| `apps/web/platform/scroll-restoration.test.tsx:27` | Extend the existing rendered-pathname probe: seed source and current-route entries; clear the source explicitly from configuration; navigate the mocked rendered route back and verify only the source focus key was removed. Confirm normal current-route writes and other keys survive. |
| `apps/desktop/src/renderer/src/platform/tab-coordinator.test.ts:18` / `:133` | Reuse seeded tabs and real tab-store mementos. Explicit-path clearing removes the origin focus entry in the adapter's tab; preserves current-route data and another tab's same key; works when the owner tab is inactive. Existing default reads/writes keep active-tab semantics. |
| `packages/views/autopilots/components/template-create-autopilot-page.test.tsx:197` | Make the restoration fixture route-scoped: its current `Map<string, string>` keyed only by `entryKey` at `:199` masks cross-route bugs. Test confirmed creation from ordinary picker and `initialReturnTo: "autopilots"`, then fresh origin mount with no restored focus; failed and unreadable responses retain the pending key. Preserve group/scroll and ordinary Back behavior. |

No changes are needed in `NavigationAdapter`, navigation providers, `tab-store.ts`, route builders, URL parameters, or exported index barrels. This bounded change closes the reported **successful-create** lifecycle for both origins without global state. Broader classification of abandoned flows as Back versus fresh entry remains outside this handoff.
