# Quick-create actor discovery

## 1. Scope / Trigger

The shared Web/Desktop `QuickCreateActorPicker` supplies discovery for both issue-creation modes and Chat. Quick create selects the assistant that files an issue; `ManualCreateAssigneePicker` supplies direct assignment, including members and Unassigned. The shared search, categories and responsibility previews do not merge those selection contracts or their preference storage. Ordinary issue/board assignment, sidebar pins and mobile remain separate surfaces.

## 2. Signatures

- Core `QuickCreateActorRef = { type: "agent" | "squad"; id: string }`.
- `captureQuickCreateScope(expectedWorkspaceId?, expectedUserId?)` captures workspace slug/UUID, user and reset generation before asynchronous acceptance.
- `isQuickCreateScopeCurrent(scope)` and `isQuickCreateStoreReady(state, wsId?, userId?)` validate live identity and hydrated identity.
- Store `toggleFavoriteActor(ref, scope?)` and `recordSuccessfulActor(ref, capturedScope)` return false for stale/unready writes.
- Store `recordRecentActor(ref, capturedScope)` records accepted manual agent/squad assignments without changing the default or last quick-create assistant.
- Views `QuickCreateActorPicker` accepts eligible agents/squads, selected identity, favorites/recents, preference readiness, callbacks and separate agent/squad query states.
- `PropertyPicker.navigationResetKey?` and `searchInputRef?` are opt-in generic navigation controls, not actor-specific state.

## 3. Contracts

Persist only last-actor tuple, keepOpen and typed favorite/recent references under the existing workspace-aware `multica_quick_create` key. Never persist names, descriptions, query results, search text, filters or hydration metadata. This is local storage through StorageAdapter, not server synchronization; auth cleanup clears every workspace's preferences.

Build rehydrated preferences from fresh defaults and allowlisted fields. A missing old recent array may be seeded from a valid last-actor tuple; an explicit empty array remains empty. Retain 20 unique recent actors, newest first. Quick creation records only after `api.quickCreateIssue` or `api.createCommentSubIssue` accepts the request, atomically with lastActor; background completion is a different event. Accepted manual creation records only the submitted agent/squad in the same recent list. Selection, failed creation, member assignment and clearing never record actor usage.

Hydrated scope must match live workspace/user/reset generation. Workspace mirrors change before the rehydration microtask, so `persist.hasHydrated()` alone is insufficient. Guard rendering and writes during that gap; stale asynchronous reads cannot publish ready state. Do not eagerly access the auth proxy at module evaluation: workspace rehydration and the mounted panel own hydration, including auth-status changes without a slug change.

The parent retains eligibility: active, runtime-bound, assignable agents and active squads with eligible leaders. Resolve stored references only against that directory. Do not delete preferences based on empty loading defaults or transient failures. Keep explicit actor → unfinished draft → last accepted actor → first eligible agent precedence; wait for preference hydration before the last two fallbacks.

Manual assignment keeps its existing active directory and renders unavailable agents or squads disabled with the canonical permission/runtime reason. Shared shortcuts never bypass those gates. It adds a Members category, preserves Unassigned as an `emptyValue` row so typed search + Enter selects the first real match, and uses the existing pill/controlled-open contract for toolbar overflow. Returning from Members to shortcuts also restores the All filter. Default-assistant actions remain exclusive to quick creation.

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
- Bad: reusing Chat pins for issue creation would impose agent-only semantics and mutate an unrelated surface.
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
- Manual selection and task creation remain available without configured AI or on an older server returning404. The interface says Send to (交给 in Chinese), with a persistent hint below the picker explaining issue creation and default assignment.
- An explicit request shows its three recommendation states the same way the description helper does. While pending, the main button itself reads Cancel and aborts; there is no second cancel control. A failure is `role="alert"` in the destructive tone with an outline Try again that focuses the persistent button and calls `start()`. When a context change discards a shown result or an in-flight request, a `role="status"` notice says so until the next request; discarding only an error, a deliberate cancel and an adoption are silent. With no eligible candidate the button is natively disabled, described by a visible reason, and `start()` returns early.
- Each suggestion's button shows the bare Use verb; its accessible name stays `use_named` ("Use {{name}}"), so the visible text is contained in the name.

Tests: core creator-selection/API/preference suites; views creator-recommendations and cache suites plus phase1 picker regressions; `e2e/quick-create-actor-picker-phase2.spec.ts` covers realAPI defaults/projects, deterministic-provider recommendations, cancellation/failure, actual Electron and Chinese narrow layouts.

## Chat discovery

- Chat's `AgentPicker` adapts the shared picker for new conversations and the
  floating input's agent dropdown. Chat accepts only agent IDs; build leadership
  metadata from active squads, then exclude squad entries and the squad tab.
- Preserve caller-owned access filtering and disable runtime-unbound agents.
  Offline agents with a bound runtime remain selectable. New-chat zero/single
  agent shortcuts retain the same runtime checks.
- Favorites come from `chatPinnedAgentsOptions` and existing pin/unpin mutations,
  with the server's five-pin limit applied to all pins, including hidden actors.
  Recent actors come from active sessions ordered by last-message/update time,
  independently of session pin order. Shared catalog resolution removes duplicate
  and inaccessible references. Never write quick-create preferences from Chat.
- Preference readiness means data is available, not that no mutation is pending.
  Disable pending pin actions separately so favorites/recents stay visible. A
  disabled pin cannot receive restored focus; fall back to a row or the search
  input so keyboard dismissal stays inside the popup.
- Chat provides its own hint and no default-assistant action. `side`/`align`
  preserve bottom placement for new-chat and top placement for the input picker.
- Chat component tests own wiring, runtime shortcuts, pins/cap/rollback and
  pending focus. `e2e/chat-actor-picker.spec.ts` covers actual browser entry points,
  keyboard selection, categories and narrow-screen layout without agent execution.


## Actor hierarchy

- Resolve current leaders through eligible squads' `leader_id`, never names or template provenance. Keep their saved descriptions and direct-selection identities unchanged.
- Category controls appear in order: All, 小阿孚, Agents, AI squads, Planning and coordination. Identify 小阿孚 only by agent `system_key === "mika"`; the planning category uses current squad leadership.
- The dedicated categories open their complete matching directory, preserving typed search. All and Planning and coordination include leaders directly, without a collapsed secondary section. Agents excludes current squad leaders from browsing, search, favorite/recent rows and counts through the shared filter. Preserve defaults, recent history and stored references.
- Explain that selecting a leader individually does not select their squad. No claim that a leader cannot implement is supported by this relationship.
- Row badges: a squad leader carries the Squad leader badge; 小阿孚 is exempt and always carries its own CEO Agent badge (even when it currently leads a squad), so the built-in Chief of Staff is never relabeled as a squad leader or a plain agent. The separate Leads: line still lists its squads. The badge label lives in `mika_badge` and reads "CEO Agent" in both English and Chinese.
- Send to identifies the recipient of the creation request: assignment defaults to the selected actor, but an explicitly named assignee in the request takes precedence. Keep that distinction visible below the picker; the label does not lock backend assignment.

## Creation form accessibility and layout

- Keep the recipient picker and explicit recommendation action in one wrapping group. The picker carries a chevron and visible keyboard focus; an empty description exposes a visible recommendation prerequisite and an accessible description. Recommendations must still require a click and explicit adoption.
- Label the prompt with persistent Task description text and connect it to the actual ContentEditor root through `aria-labelledby`. ContentEditor forwards optional `aria-label`, `aria-labelledby`, and `aria-describedby` and exposes a multiline textbox; updating these attributes must preserve the editor instance and unfinished text.
- Submission failures use an alert region and retain the draft. Warning and accepted-count feedback use status semantics and existing warning/success tokens.
- Keep role-specific examples short and preserve template/system identity resolution. The assignment hint states the default and explicit-request override without implying immediate execution.
- Agent create's Switch to Manual uses `pen-sheen` (owner decision 2026-10-11), the hand-made counterpart to manual create's `border-beam`: a muted chip, a neutral glint every 4.2s behind the label, and on hover/focus a pencil tap plus a drawn underline. Keep the two mode switches visually distinct; reduced motion keeps only the static chip and hover color.
- Narrow and coarse-pointer creation controls have 44px hit targets. Compact agent creation uses the existing 85dvh ceiling with enough height for the labeled editor, refinement and footer. Constrain intermediate-width dialogs to viewport gutters even after the desktop breakpoint; use the shared sizing helper for both manual and agent creation.
- Recommendation lists must leave at least half of the description editor visible and the full Create button reachable at 375×812 in English and Chinese. `CreatorRecommendations` caps the scrollable list at `min(9rem,18dvh)` below `sm`, retaining `min(13rem,25dvh)` above it. Keep every suggestion mounted and reachable through native scrolling; the phase2 E2E tabs through all three buttons, adopts the last, and verifies the draft/default/recent preferences are unchanged. A CSS-class assertion cannot establish this geometry or scroll behavior.
- Actor names truncate only when constrained; normal short names must remain fully visible. Negative margins on an intrinsic-width trigger can consume its text space, so avoid that combination.
- Browser verification can use actual shared UI with isolated Query data and stubbed API acceptance. Distinguish these checks from backend dispatch, physical touch and real screen-reader testing. Hidden editor bubble-menu geometry does not prove visible horizontal overflow.


## Agent create submission and blocked states

- The submit button reads Send to agent (交给智能体); pending, done and count use the same verb (正在交给... / 已交给 / 已交给 N). The recipient trigger's name also starts with Send to / 交给, so browser selectors for the trigger must exclude the submit name, for example `/^Send to (?!agent$)/` and `/^交给(?!智能体$)/`.
- `resolveQuickCreateSubmitState` in core is the single source for the button, the notice and the keyboard path. Its order and its equivalence with "may submit" are pinned in `packages/core/issues/quick-create-submit-state.test.ts`; do not re-run that matrix through a DOM mount. It changes how a blocked state is presented, never whether a submit is allowed.
- Derive `sourceRequired` with `Boolean(anchorCommentId)`, matching the existing composer guard. An empty source ID follows ordinary creation; treating every non-null string as a required source would add a new blocker.
- Native `disabled` is reserved for submitting and uploading. Every other blocker uses `aria-disabled`: the button stays focusable and activatable, carries no `pointer-events-none` and no native `title`. One reason sentence is shown in a Tooltip and is also the button's accessible description through an always-rendered `sr-only` element, so it does not depend on the tooltip being open.
- `submit()` is the only entry for the button, the editor shortcut and the dialog shortcut. It re-evaluates with live editor Markdown, because `hasContent` trails the editor by its debounce. Use the live value alone: `hasContent || liveMarkdown` incorrectly treats a just-cleared description as ready, after which the composer silently refuses it without focusing the editor. Keep regressions for immediate typing, immediate clearing, and attachment-only Markdown. A blocked attempt keeps the draft and moves focus:

  In browser debounce regressions, install the browser clock before navigation,
  pause after the nonempty draft renders, then clear and prove the submit still
  appears enabled before the actual pointer click. Advance only the focus frame
  (32ms, below the 150ms editor debounce), assert editor focus and zero POSTs, and
  resume in `finally`. Host/CDP round-trip timing below 150ms does not prove this
  race and is load-dependent. The unit suite owns the full deferred-editor matrix.

| State | Reason shown | Focus goes to |
| --- | --- | --- |
| `no_recipients` | No agent or squad can take this | First exit of the notice |
| `autonomy_blocked` | Existing autonomy tooltip | The notice's manual hand-off button |
| `version_blocked` | Existing CLI version tooltip | Recipient trigger |
| `source_unavailable` | Source context not ready | Unchanged; the source card owns retry |
| `missing_description` | Write a description first | Description editor |
| `recipients_failed` / `loading_recipients` / `missing_recipient` | Load failed / still loading / choose a recipient | Recipient trigger |

- `no_recipients` (lists and preferences known, nothing passes `canCreateAs`) outranks a missing description and renders a `role="status"` warning notice where the autonomy notice sits, with what, why and next. Exits: an `AppLink` to the agents list that saves the prompt and closes only on an in-place navigation (a modifier click keeps the dialog), and a plain Switch to Manual button calling `switchToManual()`. Only the footer switch carries `pen-sheen`. A list load failure does not show this notice; the picker owns that retry.
- The configured send chord also submits from non-input controls anywhere in the dialog, including the recipient trigger that receives focus after adopting a recommendation. It is a native `keydown` listener on the closest `[role="dialog"]`, not a React handler: React would also deliver keys from portaled popups through the component tree. Exclusions, in order: `defaultPrevented` (the editor handled it, so one press submits once); `repeat` and IME composition; send configured as a bare Enter, which disables the binding so Enter keeps activating the focused control; an editable target; a chord mismatch. Manual create does not have this binding.
- The default recipient trigger is outlined like a property pill, with no negative margin; callers that pass `triggerRender` are unchanged. Keep the persistent hint with the picker inside `CreatorRecommendations`'s child group, so a wrapped recommendation prerequisite cannot separate them. Its gap to the trigger frame must stay smaller than its gap to the Task description label at both default and 375px widths; measuring only a ready desktop row misses the empty/narrow reversal.

## Wording without an umbrella noun

- The recipient position has no collective noun. Actions say 智能体 / agent; drop the noun where context already supplies it (No matches, Loading..., Set current as default); write 智能体或AI小队 / agent or squad only when both must be named. 助手, 对象, assistant and actors do not appear in `create_issue.agent`, `create_issue.actor_picker` or `create_issue.creator_recommendations`. This supersedes the phase 2 "创建助手" naming. Selecting a squad still hands the request to its leader agent, so "sent to agent" holds for squads.
- `actor_picker.no_matches` and `actor_picker.loading` are also rendered by manual assignment and Chat; keep them noun-free so they stay true on all three surfaces.
- A recommendation discarded after a context change exposes the existing stale status, including when the live editor check catches that change at adoption before the debounced draft updates. Explicit cancellation and successful adoption clear results without the stale notice.

## Manual creation refinement

- Manual creation keeps fixed Title (required) and Description (optional) labels. TitleEditor forwards optional `aria-labelledby` / `aria-required` to its actual editable root; the description uses ContentEditor’s labeling contract. Keep title-only submission valid and missing-title keyboard activation focused on the title.
- Window controls have independent accessible names. Manual create's Switch to Agent keeps its `border-beam` highlight, brand tint and hover-rotating icon (owner decision 2026-10-11; do not flatten it in audits) with a visible focus ring, 44px compact/coarse-pointer target and no icon rotation under reduced motion. Draft, upload, and one-time mode-seeding contracts remain unchanged.
- Render configured properties in status → assignee → project → remaining-field order. Preserve all selected display preferences, non-default values, iteration context and custom fields. Named properties retain their visible current value and a field-name description through their trigger title.
- Both creation modes use `manualDialogContentClass` for viewport gutters, expanded width and the compact 640px preferred height under the existing 85dvh cap. Scope 44px compact/coarse-pointer hit areas to creation controls; do not resize global buttons.
- Manual description assistance says Refine description; agent assistance retains its separate localized action name. E2E helpers that exercise both modes must account for that distinction. Only manual-create title selectors use Title (required); inline issue title editing retains its existing accessible name.
