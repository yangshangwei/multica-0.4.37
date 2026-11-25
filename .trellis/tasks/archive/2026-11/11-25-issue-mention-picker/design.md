# Issue-detail mention discovery design

Status: approved for implementation on 2026-11-25. See `prd.md` for user-visible requirements.

## Interaction

The caret remains the search input. Opening @ does not focus a second search field or open a nested popover. Use the familiar actor-row treatment from task creation with a fixed category toolbar and one scrolling result region.

```text
All   Agents   AI squads   Members   Issues
  [All agents | Squad leaders]  (Agents only)
Type after @ to search · Alt/Option+Down: filters
------------------------------------------------
My frequent choices
  avatar  Agent name               Agent
          Saved responsibility preview
Recently mentioned
  avatar  Coordinator              Squad leader
          Saved responsibility preview
          Leads: Incident response
Agents / AI squads / Members / Issues
  ...remaining matching rows...
------------------------------------------------
Show more (N remaining)
```

This is a structural sketch, not final pixels. Use existing semantic tokens, focus rings and text roles. Start from the reference picker's 24rem width, clamped to viewport gutters, and the existing popup height budget. Header and footer stay fixed; only the result body scrolls with `min-height: 0`. Avoid nested scroll regions and a minimum height that overrides the available viewport space.

Empty query: All members remains a separate row when applicable; favorites (up to 3) and recent mentions (up to 5) precede remaining type groups, with typed-identity deduplication. Favorites not displayed as shortcuts stay reachable in the directory. Applying a category filters shortcuts too. Query present: suppress shortcut promotion and rank matching results, preserving exact issue hits and cancelled-result ordering.

Agents is inclusive of squad leaders. The secondary Squad leaders filter is a subset. This differs intentionally from the existing quick-create model's ordinary-agent filter, which excludes leaders; do not change that caller's semantics. No special Mika category is added; its name and normal agent identity remain searchable.

## Boundaries and opt-in

Add an explicit presentation option through `ContentEditor` -> extension options -> `createMentionSuggestion`, independent of the existing `mentionMode: default | context`. A possible name is `mentionPicker: default | categorized`; finalize names under the conventions document.

Enable it only in:

- `packages/views/issues/components/comment-input.tsx:220`.
- `packages/views/issues/components/reply-input.tsx:242`.
- Both edit editors in `packages/views/issues/components/comment-card.tsx:742` and `:1080`.

Keep `mentionMode=context`, other default hosts, slash commands and `MentionExtension` serialization unchanged. If context mode and categorized presentation are supplied together, context mode wins; `disableMentions` still disables suggestions entirely. The task-description editor in `issue-detail.tsx` stays on the existing default. Do not infer opt-in from `currentIssueId`, which also serves other editor features.

Use a categorized list component at the same suggestion-popup boundary rather than embedding `QuickCreateActorPicker`/`PropertyPicker`, whose popover, search input and acceptance lifecycle are different. Retain the common popup positioning and outside-dismiss controller.

## Discovery and data flow

1. Capture the current workspace/user identity for the popup. Use the explicit QueryClient already supplied to the suggestion factory rather than assuming an unverified provider exists around the Tiptap portal.
2. Observe existing member/agent/squad query options with that QueryClient while the categorized popup is open. Existing keys/cache remain the server-state owner; no copied directory store or new API is introduced. Avoid a fetch for every typed character. Do not count an initially empty cache as a successful empty directory.
3. Apply the current mention permissions/archive/runtime rules before building metadata, counts, favorites or search results. Keep unknown squad leader state distinct from a known unbound leader, matching the existing mention tests. Recheck identity and eligibility when accepting a row.
4. Reuse the pure actor catalog/name-description ranking logic. If both picker consumers need it, move only the shared pure discovery functions and types from `modals/quick-create-actor-picker-model.ts` into a neutral Views common module; update direct imports instead of adding compatibility wrappers. Leave quick-create shortcut/default/controller behavior with its original owner. Maintain its existing model tests before the move.
5. For the mention adapter, call shared ranking over all actor types then apply the mention's inclusive Agents/leader subset filter. Do not reuse quick-create's exclusive ordinary-agent category predicate for the new Agents tab.
6. Derive metadata from each saved agent/squad's own description and `Squad.leader_id`. Use `descriptionPreview` for inert rendering only; keep full saved description for search. Keep member and issue matching behavior.
7. Read scoped `favoriteActors` through `isQuickCreateStoreReady` and the existing guarded hydration pattern (`manual-create-assignee-picker.tsx:46`). They are read-only in this UI. Keep current `mention-recency.ts` usage/ranking for recently mentioned rows. Never call `recordRecentActor`, `recordSuccessfulActor`, `setDefaultActor` or `toggleFavoriteActor` when mentioning.
8. Merge eligible actor/member matches and existing cached/server issue results, deduplicate by `type:id`, apply query/category and ranking, then paginate. A batch is 50 local directory rows; shortcuts do not remove access to overflow rows. Show remaining local matches truthfully. Issue results keep their separate bounded server lookup and exact-hit/cancelled guarantees.

Use popup-local state for category, leader subset and current visible limit. Reset the limit on query/category changes; preserve it on append and preserve the selected identity where still valid. End the categorized popup on scope changes; do not reuse old actor refs in a new workspace/account.

## Focus and acceptance

- Editor retains the @ range and text selection. Pointer activation of category, page and retry controls must not blur the editor or replace the range; return DOM focus without resetting the selection when needed. Do not issue an unconditional `setTextSelection` or other selection-only transaction: it can disarm the typed @ trigger. Verify pointer filter -> continued typing -> acceptance at the original range.
- **Deterministic keyboard route:** while this popup is active and not composing, Alt/Option+ArrowDown focuses the selected category button without inserting a mention. Process it before generic row ArrowDown handling. Expose a localized, platform-formatted hint. Focused category buttons use roving Left/Right; Tab/Shift+Tab reaches the leader-subset control, Retry, Show more and an explicit Return to typing control. Candidate rows stay in editor-managed arrow navigation, not the auxiliary Tab sequence. Do not trap focus: leaving the popup uses normal outside-focus dismissal. Activating a category/expansion/retry/return control restores DOM focus to the unchanged editor selection, so ordinary typing can immediately continue. Preserve modified-Tab behavior when the editor itself owns focus.
- Controls are named real buttons with pressed state and visible focus. Keyboard activation from a focused control performs its action and contains the event, rather than leaking Enter/Space to mention acceptance or comment submission. The new shortcut is scoped only to an open categorized popup; ordinary editor caret keys keep their meaning.
- If auxiliary controls take keyboard focus, Escape must still close only the suggestion popup and restore DOM focus to the editor. Explicitly dispatch `exitSuggestion` with the correct PluginKey as well as cleaning up the DOM; the old editor-only Escape handler relies on upstream Tiptap to perform plugin exit. Verify the decoration/state is gone and a newly typed @ reopens correctly.
- IME-owned events do not accept, page, change categories or dismiss. Guard `isComposing`, Safari keyCode 229 and editor composition state before the new shortcut/control/dismiss paths. Restrict any needed popup-controller extension to the opt-in branch; preserve default/context/slash behavior.
- Arrow navigation, hover/click, highlighted row, scroll-to-view and acceptance all use one flattened rendered order keyed by `type:id`. Filter changes clear a removed selection to the first selectable match; append-only paging retains the identity. Async arrival cannot silently change an explicitly highlighted item.
- Reject stale or disabled items at acceptance and keep missing-runtime explanations visible. Full labels are accessible without introducing interactive nested links inside candidate rows.
- Selection still calls the existing mention insertion command and recency recorder. It must not execute the composer submit callback or create any network write.

## Loading and failure behavior

| State | Categorized popup |
| --- | --- |
| Directory pending without data | Localized loading feedback; no zero-count/no-results claim |
| One directory failed | Named retry, partial usable groups retained; no claim that All is complete |
| Cached refresh failed | Retain eligible cached rows and expose retry |
| No known matches after successful relevant reads | No matches plus category controls for recovery |
| Query changes during issue search | Abort/ignore stale results using the existing request guard |
| Auth/workspace identity changes | Close/discard old popup; no stale insertion or preference write |
| Favorites not hydrated | Directory usable, favorite shortcut withheld until correct scope is ready |

## Compatibility and rollout

No API/schema migration, new dependency, package-boundary exception or persisted data format is required. Existing mention nodes remain `type/id/label`; discovery metadata is not serialized. Web and Electron get the same shared Views implementation.

Rollback is the four explicit host opt-ins plus the categorized implementation. Keep the legacy default/context paths independently tested. Any shared pure-helper move is separately behavior-preserving and can be reviewed independently.

## Risks and validation

- The main risk is focus loss or accidental submit while interacting with toolbar/footer controls; verify with a real Tiptap editor and native Electron, not only mocked row tests.
- Shared catalog extraction can change quick-create's category/rank behavior; run its existing model and picker suites unchanged.
- Query loading and identity guards must keep forbidden/stale actors out of counts as well as rows. Use a real QueryClient for recovery/scope tests.
- A larger row layout can clip the popup near the viewport edge. Inspect 375px/desktop, both themes and both languages, and run `visual-verdict` before each UI iteration.
- Preserve existing issue direct-hit and cancelled-result ordering under the new actor pagination budget; test both All and Issues categories.
