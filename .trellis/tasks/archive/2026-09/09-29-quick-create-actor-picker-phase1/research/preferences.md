# Actor picker preferences and data contracts

Research only, 2026-09-29. No product code or global task state changed.

## Recommendation

Extend `packages/core/issues/stores/quick-create-store.ts` with **picker-local favorite actor references and recent accepted-submit references**. Keep both agent and squad identity as `{ type: "agent" | "squad", id: string }`; never persist names, descriptions, runtime state, permissions, or query results. The proposed 20 retained / 5 displayed recent entries is a new product limit, not an existing backend constraint. Keep search and the All / Agents / AI squads filter ephemeral.

Reuse the existing quick-create store's workspace persistence and cleanup registration, but fix this store's hydration defaults and guard its asynchronous success writes. Do not broaden this task into generic storage or pin infrastructure changes. Preserve valid existing `lastActorType` / `lastActorId` as the default-selection fallback; old preferences need not become fabricated timestamped recents.

### Why not existing pins?

| Existing mechanism | Actual contract | Consequence |
| --- | --- | --- |
| Sidebar pins | `PinnedItemType` is only issue/project/view; server rejects any other type (`packages/core/types/pin.ts:1`, `server/internal/handler/pin.go:99`). Query keys include workspace and user (`packages/core/pins/queries.ts:5`). | Actor support requires an API/schema/server change and sidebar semantics. Installed-client compatibility already requires opt-in for view pins (`server/internal/handler/pin.go:69`). This is outside phase 1. |
| Chat quick-agent pins | Agent ID + position only (`packages/core/types/chat.ts:3`). Server is user/workspace scoped, filters access, and caps pins at five (`server/internal/handler/chat_pinned_agent.go:11`, `:37`, `:51`, `:118`). UI explicitly starts chats from these pins (`packages/views/chat/components/quick-agent-bar.tsx:27`). | Cannot represent squads. Sharing agent pins would silently change the Chat bar when a user favorites an issue creator and import an unrelated five-item limit. |
| Picker-local preferences | Existing store already remembers an agent-or-squad creator (`packages/core/issues/stores/quick-create-store.ts:9`, `:36`). | Covers both types without backend work or unrelated UI changes. Tradeoff: preferences remain device/profile local and clear on logout/expiry; they do not sync between web and desktop/devices. |

No existing squad-favorite contract was found in the core/views/server pin surfaces inspected.

## Current preference and selection behavior

- The store persists `lastActorType`, `lastActorId`, and `keepOpen` under `multica_quick_create:<workspace slug>`, with no `partialize`, custom `merge`, migration, or validation (`packages/core/issues/stores/quick-create-store.ts:44`). Its cleanup resets all three (`:62`).
- The comment claiming per-user isolation comes “for free” from browser profiles (`:14`) is imprecise: the key contains **no user ID**. Same-profile account isolation depends on the session cleanup path below. Do not claim account-keyed or cross-device persistence.
- Existing actor resolution checks the current visible dataset. Seed precedence is caller agent → caller squad → unfinished draft actor → last successful actor → first visible agent (`packages/views/modals/quick-create-issue.tsx:203`, `:224`). Favorites and recents should reorder discovery, not replace this precedence or select an actor merely when starred.
- Unfinished choices live in the unified issue draft and do not overwrite the last-successful preference (`packages/views/modals/quick-create-issue.tsx:271`). Legacy `lastAgentId` is intentionally not migrated (`packages/core/issues/stores/quick-create-store.ts:27`). Legacy `lastProjectId` must remain ignored; project is not a standing preference (`:19`).

## Acceptance timing and race boundaries

- Both ordinary quick-create and comment-source sub-issue creation await their API call, then call `setLastActor(actor.type, actor.id)` (`packages/views/modals/quick-create-issue.tsx:440`, `:469`). Record a recent entry at this same successful boundary, atomically with `lastActor`; do not record row selection, pinning, opening, close, rejected request, or blocked submit.
- “Successful” currently means **server accepted/queued the request**, not completed issue creation or daemon execution. Ordinary quick-create returns HTTP 202 plus `task_id` (`server/internal/handler/issue.go:2460`, `:2666`), and E2E explicitly verifies queuing without executing a daemon (`e2e/issue-assist-flow.spec.ts:166`). PRD/UI language should not imply eventual agent success. Waiting for actual task success would require a different cross-session tracking design.
- The shared composer blocks empty, duplicate and upload-blocked submissions before invoking the API (`packages/views/editor/use-composer-submit.ts:149`). Existing late-success protection only guards consuming a replaced draft / closing an unmounted panel; it does **not** protect preference writes (`packages/views/modals/quick-create-issue.tsx:408`, `:469`, `:544`).
- The storage adapter reads the active slug at **write time** (`packages/core/platform/workspace-storage.ts:106`). Therefore a request submitted in A and accepted after switching to B currently writes A's actor into B's key. A late success after logout/relogin can also repopulate cleaned preferences.
- Capture the submitted actor and workspace/user/session identity before awaiting. Only update actor preferences when the original identity is still current and authenticated. With the bounded policy, skip that preference update after switching away instead of writing into an inactive namespace. The submission itself still succeeded; do not treat a skipped preference write as request failure.
- Available identity readers: `getCurrentSlug()` / `getCurrentWsId()` (`packages/core/platform/workspace-storage.ts:67`, `:72`) and `useAuthStore.getState().user?.id` / `.status` (`packages/core/auth/store.ts:28`). API request headers use the slug mirror (`packages/core/api/client.ts:801`). `ApiClient.authEpoch` and `bumpAuthEpoch()` are **private**, with no public epoch getter (`packages/core/api/client.ts:768`, `:789`); the existing epoch only rejects stale 401 side effects (`:815`). Do not plan against a nonexistent getter.
- A small non-persisted quick-create reset-generation token, incremented in its registered `resetInMemory`, is a bounded way to reject A → logout → A late callbacks, combined with workspace/user/status checks. If an actual credential epoch is chosen instead, exposing it is an explicit additional API-client change and needs corresponding tests. User ID alone does not catch same-user logout/relogin.

## Hydration and auth cleanup

- `setCurrentWorkspace` updates paired slug/ID mirrors synchronously and triggers registered rehydrations in a microtask on slug change (`packages/core/platform/workspace-storage.ts:34`). No active workspace means reads return null and writes are dropped (`:94`).
- The store's default Zustand merge overlays persisted state onto the **current** in-memory state. Missing destination storage therefore retains the previous workspace's values, including new arrays unless explicitly defaulted. Malformed JSON throws before merge; Zustand catches hydration failure and leaves previous state in memory. A custom merge alone cannot recover malformed JSON.
- Verified with a read-only Node probe using installed `zustand/vanilla` + the exact current persist options and a Map adapter: A had agent `a-agent`; rehydrating missing B retained A with `hasHydrated=true`; corrupt C (`{bad-json`) retained A with `hasHydrated=false`; valid D restored D; a late A setter then overwrote D's persisted actor. This reproduces the middleware/storage behavior, not a mounted product test. Middleware evidence: `packages/core/node_modules/zustand/middleware.js:292`, `:335`, `:417`, `:433`.
- Normalize allowlisted fields against fresh defaults rather than spreading arbitrary persisted state. Validate actor type and nonempty string ID; arrays must be arrays; deduplicate by **type + ID**, cap recent retention, and reject malformed entries/timestamps. Keep an intentional empty favorites list. Preserve valid legacy last-actor tuples; default newly absent arrays to empty. Malformed JSON must reach safe defaults through a store-local read/error strategy, not remain the previous workspace's memory. A reset via ordinary persisted `setState` before reading risks overwriting the destination's stored data; avoid that order.
- Quick-create already self-registers for cleanup (`packages/core/issues/stores/quick-create-store.ts:62`); add every new preference field to that reset. The all-drafts import guarantees registration before cleanup (`packages/core/platform/storage-cleanup.ts:6`).
- `AuthInitializer` calls `clearClientSessionData` whenever auth is unauthenticated, including cold-start rejection (`packages/core/platform/auth-initializer.tsx:487`). It resets memory before deleting storage, enumerates all workspace-prefixed keys (including unknown workspaces), then clears Query cache (`packages/core/platform/session-cleanup.ts:32`, `:51`, `:71`; `packages/core/platform/storage-cleanup.ts:61`). Offline/5xx recovery is not logout and intentionally keeps preferences (`packages/core/platform/auth-initializer.tsx:481`).

## Available actor data and phase-1 search

- The panel already loads agent and squad lists using workspace-scoped Query keys (`packages/views/modals/quick-create-issue.tsx:144`; `packages/core/workspace/queries.ts:12`, `:51`, `:103`). No new endpoint or eager per-actor detail fetch is needed.
- Agents have saved `name` and `description`; squads have their own saved `name` and `description` (`packages/core/types/agent.ts:494`; `packages/core/types/squad.ts:11`). Display/search these descriptions, not generated role claims, instructions, or the squad leader's description. Template provenance is not current capability (`packages/core/agents/discovery.ts:50`; `.trellis/spec/views/frontend/agent-discovery.md`).
- Squad list responses pass `SquadListSchema`; missing description defaults to empty (`packages/core/api/client.ts:4460`; `packages/core/api/schemas.ts:2134`). Agent list is still a typed raw fetch with no response schema (`packages/core/api/client.ts:1633`, `:897`). New description handling should guard with `typeof value === "string"` (or equivalent safe normalization), rather than assuming every installed/server combination supplies a valid string. Broad agent API normalization is separate work unless explicitly included.
- Eligibility remains: active, runtime-bound, assignable agent; active squad whose leader is among those agents (`packages/views/modals/quick-create-issue.tsx:160`). Favorites/recents are projected onto this eligible set every render, so a stored reference never grants visibility or access. No actor should appear twice across favorite/recent/remaining sections.
- Current picker searches case-insensitive names plus `matchesPinyin(name, query)`, and clears text on close (`packages/views/modals/quick-create-issue.tsx:993`). Extend with saved-description text matching while retaining name/pinyin behavior. Apply type filter and text query consistently to every section; preserve selected actor even if temporarily filtered out. Unknown/blank descriptions should be harmless and should not be filled from a guessed role.
- Do not destructively prune preferences from transient empty query defaults or search/type-filter results. The panel's agent/squad query calls currently do not expose loading/error status (`packages/views/modals/quick-create-issue.tsx:145`), and malformed squad-list fallback is an empty array. An unavailable dataset does not establish deletion. Intersect for display without erasing saved choices.

## Existing tests to retain and extend

| File and anchors | Existing evidence |
| --- | --- |
| `packages/core/issues/stores/quick-create-store.test.ts:19`, `:24` | No project memory; agent/squad tuple setter/clear. No persistence/hydration, recents, favorites, or identity-race tests. |
| `packages/core/platform/workspace-storage.test.ts:23`, `:38`, `:50`, `:73`, `:112` | No-workspace drops, slug namespacing, dynamic switching, microtask rehydration, logout/re-entry. Tests adapter callbacks, not quick-create store isolation. |
| `packages/core/platform/session-cleanup.test.ts:44`, `:83`, `:104`, `:127` | Shared-account cleanup, every workspace, reset-before-delete, cold-start cleanup without workspace list. |
| `packages/core/platform/storage-cleanup.test.ts:38`, `:82`; `packages/core/drafts/cleanup-registry.test.ts:27` | Registered key clearing, unknown-workspace sweep, in-memory resets. |
| `packages/views/modals/quick-create-issue.test.tsx:645`, `:687`, `:807`, `:837`, `:863`, `:917`, `:949`, `:965`, `:1137`, `:1270` | Unfinished selection restore; successful agent preference; mounted/unmounted late-draft guards; squad request and preference; inaccessible leader exclusion; ignored old project; comment-source branch; single-flight shortcut. Store is mocked, so these do not establish real persistence isolation. |
| `e2e/issue-assist-flow.spec.ts:166`; `e2e/upstream-selected-fixes.spec.ts:296` | Real HTTP 202 queue acceptance without daemon execution; private runtime version handling. No phase-1 favorite/recent/type-search coverage yet. |

Suggested focused commands after implementation (existing package scripts run Vitest):

```sh
pnpm --filter @multica/core test issues/stores/quick-create-store.test.ts platform/workspace-storage.test.ts platform/session-cleanup.test.ts platform/storage-cleanup.test.ts drafts/cleanup-registry.test.ts
pnpm --filter @multica/views test modals/quick-create-issue.test.tsx
```

Canonical new core tests should cover favorite toggle/dedupe/type collision; recent MRU order/cap/repeat promotion; absent/null/wrong-shaped persisted state; corrupt JSON; old last-actor tuple with new arrays absent; unknown legacy fields; A → B (empty, valid, corrupt storage) → A; auth reset including both arrays; captured context rejection after workspace switch, user switch and same-user logout/relogin. Keep presentation tests for star action without selection/closure, type+description/pinyin filtering, source-context success, failed/blocked submit not recording, visible-reference projection, loading/error not erasing preferences, and unchanged seed precedence. Run the cross-workspace race with deferred API acceptance, not only a synchronous store unit test.

Repository tests were not executed during this research-only pass. The read-only middleware probe above was executed and its results inspected.
