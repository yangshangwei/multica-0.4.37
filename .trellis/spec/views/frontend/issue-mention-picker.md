# Issue-detail mention discovery

## Scope / trigger

The categorized picker is shared by Web/Desktop issue comments, replies and both comment-edit surfaces. It is an explicit ContentEditor presentation choice. Other editors and Chat's context mentions keep their existing contracts. Use this spec when changing mention categories, discovery, popup controls or candidate acceptance.

## Signatures and owners

- `ContentEditor.mentionPicker?: "default" | "categorized"` flows through extension options to `createMentionSuggestion`.
- `mentionMode: "context"` takes precedence over categorized presentation; `disableMentions` still disables suggestions.
- `CategorizedMentionList` receives the editor, explicit QueryClient, PluginKey and existing permission-aware synchronous item builder.
- `common/actor-discovery.ts` owns shared saved-description metadata and ranking; quick-create shortcuts/default selection stay in the creation model.
- `mention-ranking.ts` owns unchanged issue conversion and cancelled/direct-hit ordering for both presentations.

## Contracts

Use the current workspace member/agent/squad queries as server-state owners. An open popup must observe successful data changes, including members: reading query status alone does not subscribe to member `.data` changes under TanStack Query tracked notifications. Member role changes affect agent eligibility even if the agent directory did not change. Filter permissions/archive state before metadata, counts, shortcuts and search; revalidate current identity and non-issue eligibility at acceptance.

Categories are All, Agents, AI squads, Members and Issues. Agents includes squad leaders; the secondary leaders filter narrows it. The quick-create ordinary-agent predicate excludes leaders, so do not silently substitute it for the mention predicate. Leadership comes from saved squad relationships and responsibilities from the selected entity's own saved description.

Search the full eligible directory before paging. Rank exact name, name substring, pinyin, then full saved-description substring; previews never define the search domain. Expand local directory rows in batches of 50. All members is a separate target outside the local page budget, available under All/Members. Server issue results retain their distinct bounded search; remaining counts describe local candidates, not all workspace issues.

With an empty query, read up to 3 scoped creation favorites and 5 recent mentions as deduplicated shortcuts. Creation favorites are read-only and hidden until correct user/workspace hydration is ready. Mention selection records mention recency only; it never changes the creation default, last actor, recent actors or favorites.

The editor remains the search input. Pointer filter/page/retry actions preserve its ProseMirror selection; restore DOM focus without an unconditional selection-only transaction, which can disarm the typed @ trigger. Alt/Option+Down enters the filter toolbar, Left/Right moves among categories, and the auxiliary Tab sequence includes Return to typing. Control activation must not submit the comment. Non-composition Escape from a control explicitly exits the correct suggestion PluginKey and contains the event, not just hides the DOM.

IME-owned keys stay with composition. A guard only in the list callback is insufficient for Escape because upstream suggestion handling may exit afterward; the opt-in popup guard runs before that handler. Keep the guard scoped to categorized mentions. Preserve default/context/slash behavior.

Use one rendered order and typed `type:id` identity for highlight and acceptance. Clear a selected identity when it disappears or becomes disabled; it must not revive on returning to an earlier filter or late cache restoration. Reset stored paging scope on each query/category/subset transition; comparing with the last expanded tuple without replacing it can resurrect old page sizes. Preserve valid selection on append/reorder. Link the focused editor to stable option IDs through active-descendant semantics and clear those attributes on exit.

## Validation and error matrix

| Event | Required behavior |
| --- | --- |
| Target beyond first 20/50 | Full search and Show more can find/select it |
| Member cache changes, status still success | Refresh roster and member-derived authority without a keystroke |
| Filter away and back | First valid fallback; no abandoned selection or expanded-page resurrection |
| Directory retry with successful remote issue matches | Retain those matches and active identity; retry only failed search |
| Account/workspace changes with same query | Close stale popup and reject acceptance before cleanup completes |
| Filter -> continued typing | Original @ range stays active, correct mention inserted, no submit |
| Focused-control Escape | Exit plugin and popup only; typed new @ can reopen |
| IME Escape/Enter/arrows/229 | No picker action or host submission |
| Initial fetch error or pending data | Truthful recovery/loading, not proven no-results |

## Good / base / bad cases

- Good: a saved responsibility past the displayed preview still finds an agent on a later page.
- Base: no favorites/history still exposes a browsable directory.
- Bad: reusing the creation popover controller steals focus into another search field and conflates mention acceptance with assignment/default actions.

## Required tests

- Categorized component/QueryClient tests: large catalog, inclusive leaders, identity/paging round trips, member/agent arrival, scope rejection, favorites readiness, successful issue retention and active-descendant target.
- Real Tiptap tests: trigger arming, pointer filtering then typing, auxiliary focus/acceptance, IME Escape and Safari composition fallback; existing serialization and default/context tests remain authoritative.
- Host tests: new comments, replies and both edit opt-ins, preserving submit/draft/upload behavior.
- `e2e/issue-mention-picker.spec.ts`: production Web and isolated Electron, synthetic directory, visible chip plus persisted Markdown identity, no server mutations or creation-preference writes from picker actions, keyboard controls, viewport screenshots. Local draft and mention-recency writes are intentional. Native OS IME/screen-reader checks must be reported separately from synthetic composition events.

## Wrong vs correct

Wrong: `items.slice(0, 20).filter(matchesCategory)` hides matching candidates before searching. Correct: authorize, search/filter, rank/deduplicate, then take the display page.

Wrong: restoring `editor.commands.setTextSelection(savedRange)` after every toolbar action. Correct: keep the existing ProseMirror selection and restore editor DOM focus, then verify continued input uses the original trigger.
