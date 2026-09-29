# Quick-create actor discovery

## 1. Scope / Trigger

The shared Web/Desktop `QuickCreateActorPicker` selects the actor that files an issue. It does not set the issue's eventual assignee. Keep this surface local to quick create; manual assignee, Chat pins, sidebar pins and mobile have different contracts.

## 2. Signatures

- Core `QuickCreateActorRef = { type: "agent" | "squad"; id: string }`.
- `captureQuickCreateScope(expectedWorkspaceId?, expectedUserId?)` captures workspace slug/UUID, user and reset generation before asynchronous acceptance.
- `isQuickCreateScopeCurrent(scope)` and `isQuickCreateStoreReady(state, wsId?, userId?)` validate live identity and hydrated identity.
- Store `toggleFavoriteActor(ref, scope?)` and `recordSuccessfulActor(ref, capturedScope)` return false for stale/unready writes.
- Views `QuickCreateActorPicker` accepts eligible agents/squads, selected identity, favorites/recents, preference readiness, callbacks and separate agent/squad query states.
- `PropertyPicker.navigationResetKey?` and `searchInputRef?` are opt-in generic navigation controls, not actor-specific state.

## 3. Contracts

Persist only last-actor tuple, keepOpen and typed favorite/recent references under the existing workspace-aware `multica_quick_create` key. Never persist names, descriptions, query results, search text, filters or hydration metadata. This is local storage through StorageAdapter, not server synchronization; auth cleanup clears every workspace's preferences.

Build rehydrated preferences from fresh defaults and allowlisted fields. A missing old recent array may be seeded from a valid last-actor tuple; an explicit empty array remains empty. Retain 20 unique recent actors, newest first. Record only after `api.quickCreateIssue` or `api.createCommentSubIssue` accepts the request, atomically with lastActor; background completion is a different event.

Hydrated scope must match live workspace/user/reset generation. Workspace mirrors change before the rehydration microtask, so `persist.hasHydrated()` alone is insufficient. Guard rendering and writes during that gap; stale asynchronous reads cannot publish ready state. Do not eagerly access the auth proxy at module evaluation: workspace rehydration and the mounted panel own hydration, including auth-status changes without a slug change.

The parent retains eligibility: active, runtime-bound, assignable agents and active squads with eligible leaders. Resolve stored references only against that directory. Do not delete preferences based on empty loading defaults or transient failures. Keep explicit actor → unfinished draft → last accepted actor → first eligible agent precedence; wait for preference hydration before the last two fallbacks.

Home shows up to 3 favorites and 5 non-favorite recents. All favorites are excluded from recents, including favorite overflow. Full favorites and the complete directory stay reachable. Search always examines the whole eligible directory, even from favorites; type filter intersects that result. Matching priority is exact name, name substring, existing name-pinyin match, then full saved-description substring. Stable name/type/ID ordering breaks ties. Render in batches of 50 without limiting the search domain.

Use the saved actor's own description and `descriptionPreview` for inert display text. Search the full description, not its 300-character preview. Missing/malformed descriptions are empty; never infer capabilities from editable names, template provenance, avatar or the squad leader's description.

## 4. Validation & Error Matrix

| Condition | Required behavior |
| --- | --- |
| Missing/corrupt preference storage | Fresh defaults, no inherited previous-workspace data and no destructive write before read |
| Workspace/account/reset generation changes before acceptance | Keep successful request outcome, skip stale preference write |
| Query pending without data | Loading state, not no-results |
| Query fails with cached data | Keep usable data and expose retry |
| Only squad query unavailable | Known agents remain available; do not claim complete all-type results |
| Favorite becomes inaccessible | Omit from rows/counts; stored ref never grants access |
| Description match after preview limit | Actor still appears in full search |
| Typed search changes results | Highlight first real match; Enter selects it; IME keeps its own Enter/arrows |
| External filter/reorder changes results | Invalidate stale numeric highlight and unique-result fallback until new input/navigation |
| Append-only page expansion | Preserve existing highlighted actor |
| Auxiliary action removes/reparents its button | Focus persistent search before transition; first Escape dismisses only picker |

## 5. Good / Base / Bad Cases

- Good: user pins both an agent and its squad; they remain distinct typed identities.
- Base: no history means a browsable full directory, not an empty shortcuts screen.
- Bad: reusing Chat pins would impose agent-only semantics and mutate an unrelated surface.
- Bad: disabling the focused Browse all button or removing View all favorites, Back, Show more or Retry can blur Chromium to body. Both nested dismissal handlers then observe Escape. Preserve a valid in-popup focus target instead of changing global dialog dismissal.

## 6. Tests Required

- Core quick-create store tests own normalization, legacy tuple, MRU, auth cleanup, empty/corrupt workspaces, pre-microtask and stale-read races.
- Model tests own filtering, ranking, typed deduplication, full-description search and paging.
- Real-primitive actor picker/PropertyPicker tests own nested-dialog Escape, focus after pin movement, IME, typed-search versus external reset and unchanged non-opted-in callers. A div mock cannot establish these properties.
- Parent tests own both accepted-submit branches, captured actor identity, hydration seed priority and unchanged drafts/version/permission behavior.
- `e2e/quick-create-actor-picker.spec.ts` verifies real API acceptance/persistence, scoped synthetic 550-actor search/paging/performance, narrow layout and an isolated Electron renderer with no real daemon execution.

## 7. Wrong vs Correct

```ts
// Wrong: the display preview omits searchable responsibilities.
const matches = descriptionPreview(actor.description).includes(query);
// Correct: normalize the saved field, keep full text for matching.
const description = typeof actor.description === "string" ? actor.description : "";
const matches = description.toLowerCase().includes(query.trim().toLowerCase());
```

```ts
// Wrong: active storage namespace can change during await.
await api.quickCreateIssue(payload);
setLastActor(actor.type, actor.id);
// Correct: accepted submission records its original scoped identity.
const scope = captureQuickCreateScope(wsId, userId);
const submittedActor = actor;
await api.quickCreateIssue(payload);
if (scope && isQuickCreateScopeCurrent(scope)) {
  recordSuccessfulActor(submittedActor, scope);
}
```


## Phase 2: defaults, projects and explicit recommendations

- `defaultActor` is a nullable typed local preference. `setDefaultActor(ref, capturedScope)` uses the same workspace/login/readiness guard as favorites. It never changes recent history or the current draft.
- `resolveQuickCreateCreator` owns caller → draft → explicit default → last accepted → first eligible agent precedence. Unknown high-priority actor data waits instead of falling through; malformed actor kinds/blank IDs are skipped. Caller/draft resolution precedes preference hydration, saved fallbacks follow it.
- Project candidates come from `getProjectExecutionSquads` (plural including [] wins) and only configured, currently eligible squad IDs. They are recommendations for the creator, not a change to the project's eventual-assignee contract. The first3 project candidates, first3 non-project favorites and remaining recents share an8-item home budget; full project browsing and full-catalog text search remain available.
- `CreatorRecommendations` calls `useRecommendIssueCreators(wsId)` only on the explicit Help me choose button. Flush and read the current editor, keep at most3 locally validated suggestions, adopt only through the existing onPick path. No default/history/issue writes occur on recommendation or adoption.
- A request captures text, project, actor, workspace/login/reset identity and description snapshots. Abort on context changes; an ignored abort still cannot publish stale data. Revalidate live editor text, identity, candidate eligibility and unchanged description before adopting.
- Mutation `gcTime:0` alone does not clear an observed mutation. Reset the mutation observer on current-request settlement, clear/cancel and unmount, guarded by request identity. Retain only the visible local result/context; do not leave task text in the global mutation cache. `creator-recommendations-cache.test.tsx` exercises the real QueryClient.
- New endpoint responses use Zod business-field validation while allowing harmless added fields. Missing/invalid/duplicate/over-limit results are errors, not authoritative no-match responses. Requests pin workspace UUID and forward AbortSignal; the mutation has no automatic retries.
- Manual selection and task creation remain available without configured AI or on an older server returning404. The interface says Creation assistant to distinguish it from the final issue assignee.

Tests: core creator-selection/API/preference suites; views creator-recommendations and cache suites plus phase1 picker regressions; `e2e/quick-create-actor-picker-phase2.spec.ts` covers realAPI defaults/projects, deterministic-provider recommendations, cancellation/failure, actual Electron and Chinese narrow layouts.


## Actor hierarchy

- Resolve current leaders through eligible squads' `leader_id`, never names or template provenance. Keep their saved descriptions and direct-selection identities unchanged.
- Primary directory and shortcut rows show other agents and squads. Put leaders in the default-collapsed Planning and coordination section; search exposes matching leaders automatically.
- Favorite navigation/counts include leaders even though their rows remain in the secondary section. Preserve defaults, recent history and stored references.
- Explain that selecting a leader individually does not select their squad. No claim that a leader cannot implement is supported by this relationship.
- Keep Creation assistant: assignment defaults to the selected actor, but an explicitly named assignee in the request takes precedence. This change does not lock backend assignment.
