# Quick-create actor picker: interaction research

Planning-only research, 2026-09-29. No product files or runtime state changed. This note owns interaction boundaries; persisted preference migration and workspace/account scoping belong to the separate state research.

## Existing surface and invariants

- `packages/views/modals/quick-create-issue.tsx:976` defines the private `ActorPicker`. Its own comment at line 971 deliberately confines the UI to quick create, rather than promoting it into the generic issue pickers.
- The parent already supplies eligible entities. At `quick-create-issue.tsx:160`, agents must be active, runtime-bound and assignable; squads must be active with a leader in that eligible-agent set. Favorites/recent/all must derive from these exact lists. Do not recover excluded entities from persisted IDs or widen visibility in the picker.
- Current search is `trim().toLowerCase()` and name substring/pinyin only (`quick-create-issue.tsx:993`). Agents and squads render in separate groups; each pick updates actor/draft and clears the panel error (`quick-create-issue.tsx:661`). Row selection closes the popover (`quick-create-issue.tsx:1056`).
- Selection is `{type, id}`, not an ID alone (`quick-create-issue.tsx:91`). Preserve this identity in preference keys, React keys, focus targets, deduplication and selected state.
- Existing seed priority is explicit caller actor → unfinished draft → last successful actor → first visible agent (`quick-create-issue.tsx:224`). A new Favorites/Recent display order must not silently change this fallback order or pick an actor while merely browsing.
- A squad's `selectedAgent` is its execution leader (`quick-create-issue.tsx:258`); `selectedSquad` is the display identity (`quick-create-issue.tsx:266`). Do not display/search the leader's description as the squad's responsibility.
- The API success branch writes `setLastActor` only after the awaited quick-create/source-context operation (`quick-create-issue.tsx:457`, `quick-create-issue.tsx:469`). The existing comment at line 271 distinguishes unfinished picks from successful use. “Recent” means accepted creation, not hover, selection, pin, or background execution completion.

## PropertyPicker/PickerItem contract

| Concern | Existing evidence | Phase-1 implication |
| --- | --- | --- |
| Navigation candidates | `packages/views/issues/components/pickers/property-picker.tsx:19`: `button[data-picker-item]:not(:disabled)` | Only the primary actor selection button gets this marker. Pin/filter/browse/retry controls must never carry it. |
| Highlight | `property-picker.tsx:89`, `property-picker.tsx:120`: numeric index; DOM-added `bg-accent` | Highlight is keyboard position, not selected actor. Keep selected check/weight visible under hover/highlight. |
| Search changes | `property-picker.tsx:105`: after filtered children render, highlight first non-empty candidate | Preserve typing → first match → Enter. It is an intentional regression fix. |
| Non-search result changes | `property-picker.tsx:120`: reapplies existing index on children changes | Type filters, all/compact changes, pin insertion/removal and async data arrival can leave the same index pointing at a different actor. Handle explicitly. |
| Key scope | `property-picker.tsx:146`, `property-picker.tsx:213`: handler attached only to search input | A focused sibling pin button already receives native Enter/Space independently. Do not move handler onto the entire popup without excluding interactive controls. |
| Arrow/Enter | `property-picker.tsx:154`: arrows wrap, scroll highlighted row; Enter clicks highlight or sole real result | Keep arrows on the input for fast selection and Tab for secondary controls. No-result Enter stays inert. |
| IME | `property-picker.tsx:150` calls `isImeComposing` | Keep guard for Enter and arrows. `packages/core/utils.ts:41` explains Safari's `keyCode === 229` fallback in addition to native `isComposing`. |
| Close reset | `property-picker.tsx:131`: effect observes open → closed, clears internal and external query | This covers direct `setOpen(false)` after selection. Do not regress to handler-only reset. |
| Fixed controls | `property-picker.tsx:68`, `property-picker.tsx:79`, `property-picker.tsx:220`: sticky header and footer outside scroll list | Place All/Agents/Squads filters in header; compact/all browse action can be footer. They stay out of result arrows. |
| Rows | `property-picker.tsx:262`: real `<button type="button">`, text wrapper and reserved selected-check slot | Never put pin `<button>` inside `PickerItem` children. Wrap primary PickerItem and pin as siblings in a non-interactive row container. |
| Semantics | `property-picker.tsx:221`: ordinary div list, despite comments saying “listbox” | Do not add `role="option"` around primary + pin buttons or claim combobox/listbox semantics without a complete corresponding keyboard/ARIA design. Native button semantics are the narrow path. |
| Popup | `packages/ui/components/ui/popover.tsx:35`: Base UI portal/positioner/popup | Real focus/dismissal behavior must be tested with the real primitive, not div mocks. |

### Required versus optional shared changes

Separate keyboard-accessible pin controls do **not** inherently require a shared `PropertyPicker` API change: a local `<div>` containing a flexing `PickerItem` and a sibling labeled `<button>` works with its current selector and native Tab/Enter/Space behavior.

The new result-order changes do require deliberate highlight bookkeeping. The smallest reusable extension is an optional `navigationResetKey`/equivalent prop on `PropertyPicker`, derived locally from the ordered visible actor keys plus browse/filter mode. When that signature changes, invalidate the previous numeric highlight before Enter can act; preserve the existing typed-query first-real-result behavior. Choose and test one explicit policy: clear highlight on external result change, or preserve the highlighted actor by identity if it still exists. Do not reset on arbitrary `children` identity because inline JSX changes on unrelated renders. If external changes should select the first result by default, make that policy explicit in tests rather than inheriting the old numeric index.

A search-input ref callback is optional, useful for local focus recovery when the last favorite disappears. Avoid a wholesale generic combobox rewrite or business-specific favorites state in the shell. If implementation instead keeps navigation state wholly local to the extracted picker, keep its behavior compatible with the existing IME, close-reset and no-result contracts; do not duplicate a second uncontrolled search value.

### Local focus and selection policy

- Pin is a separate native button, `type="button"`, accessible name containing the actor name and pin/unpin action, with `aria-pressed` reflecting persisted membership. Icons are decorative. It remains visible while focused, not hover-only.
- Pinning/unpinning keeps the picker open and leaves the selected actor/draft/prompt unchanged. Only primary selection runs `onPick`.
- Pin mutation may move a row between sections even when its actor key is unchanged; React keys do not preserve a node across different parents. After an explicit focused pin action, restore focus to that actor's surviving pin/primary control if present; otherwise the next eligible row, then previous row, then search. Do not let focus fall to document body or dismiss the create dialog. Do not steal focus for background data changes.
- Use explicit refs in the local row component/container for focus recovery. A generic optional search input ref only needs adding if the fallback cannot be implemented locally without querying an unrelated popup.
- Filter/browse changes do not select an actor or clear the draft. Preserve typed query across All/Agents/Squads; close resets query and ephemeral browse/type state to the documented entry state. Do not remount the whole popover on every filter change: this risks losing input/caret/focus.
- Keep selected actor styling separate from hover and search highlight. `PickerItem` currently retains its checkmark (`property-picker.tsx:277`); a richer row should additionally expose selection accessibly rather than relying on color alone.

## Existing sibling-action and filter patterns

- `packages/views/skills/components/skill-card.tsx:53` uses a non-button container with a separate selection button; lines 86–103 provide `aria-pressed`, an accessible name, and `focus-visible:opacity-100`. This is a useful visual/a11y pattern, not a reason to make the picker wrapper clickable.
- `packages/views/agents/components/agents-page.tsx:388` has a separate selection control; `agents-page.tsx:422` shows the established two-line actor identity with avatar, truncating name and description. The row's template badge explicitly says provenance (`agents-page.tsx:447`).
- `packages/views/agents/components/agent-discovery-toolbar.tsx:50` uses real filter buttons with `aria-pressed`, `font-semibold` and selected text/background tokens. A three-button All/Agents/Squads group can reuse this lightweight pattern without a tabs implementation or global preferences.
- Clickable table rows are plain divs with nested interactive controls. `packages/views/navigation/use-row-link.ts:47` stops click and auxclick. This is a navigation pattern, not a picker keyboard abstraction: it has no keyboard handler and does not solve selected-option or pin focus behavior. Prefer a non-clickable actor row wrapper so no propagation workaround is necessary.

## Truthful summaries and search

### Available metadata

- `packages/core/types/agent.ts:494` exposes saved `name`, `description`, and `instructions`; `system_key` at line 501 is product identity independent of editable name; `template_key` at line 508 is explicitly provenance only and never guarantees current instructions.
- `packages/core/types/squad.ts:14` exposes the squad's own name/description/instructions and leader ID. `template_key` at line 29 is likewise provenance, and `member_preview` at line 25 is incomplete membership.
- `.trellis/spec/views/frontend/agent-discovery.md:43` forbids inferring capability from editable name, emoji or autonomy. Unknown/custom actors must not be relabeled with invented specializations. Mika-specific copy needs `system_key === "mika"` (line 53).
- `packages/views/modals/quick-create-scenario.ts:40` deliberately uses squad identity before leader identity; line 54 uses the custom actor's saved description. Scenario examples are not reusable responsibility summaries: their built-in template mapping would overstate modified template instances.

### Reusable text utility

Use `descriptionPreview` from `packages/views/issues/components/description-preview.ts:19` for a compact inert-text summary. It removes common Markdown link/image/file/emphasis syntax, flattens whitespace, and caps DOM text at 300 UTF-16 code units with surrogate-pair-safe ellipsis (line 47). Its existing canonical tests cover media markers, escaped links, nested URL parentheses, punctuation, 300-character cap and emoji boundary (`description-preview.test.ts:14`, `description-preview.test.ts:33`, `description-preview.test.ts:76`). No new Markdown dependency is needed.

Limitations: this is a preview flattener, not a full Markdown parser or sanitizer. Arbitrary raw HTML/comments are not generally stripped; render the result as a React text node, never rich HTML. Empty/whitespace/image-only description should produce a neutral localized “No description” fallback. Prefer the actor's saved description regardless of template provenance; do not synthesize responsibilities from name/template/leader/instructions. A CSS one/two-line clamp controls height; the utility's cap controls hidden DOM weight.

For phase 1, search should match normalized name substring OR `matchesPinyin(name, query)` OR full saved-description substring. Do not search only the 300-character preview or only rendered/clamped text. This preserves discoverability of a responsibility mentioned late in a long description. No capability taxonomy, role-catalog fetch, roster fetch or backend change is necessary.

`packages/views/editor/extensions/pinyin-match.ts:10` handles full, initial and hybrid pinyin **prefix** matching, only for strings containing Chinese. Existing tests are `pinyin-match.test.ts:6`; reuse it without changing semantics. `packages/views/agents/components/agents-page.tsx:181` and `packages/views/editor/extensions/mention-suggestion.tsx:679` demonstrate name/description search composition. They additionally support description pinyin, but that is not required for this phase's explicit name-pinyin/full-description-substring contract.

## Compact favorites/recent/all composition

Recommended pure pipeline: eligible typed actors → full-text/type filtering → preference resolution/deduplication → compact or all projection. Search must always reach all eligible actors for the active type, even when the initial view displays only Favorites/Recent. Do not apply compact truncation before search.

- Empty query compact view: bounded Favorites then bounded Recent, deduplicated by `{type,id}`; omit empty headings. Exact caps/order are product decisions in the PRD, not implicit render constants scattered through components.
- Browse All remains reachable when compact sections are empty or nonempty, and expands the eligible catalog without changing selection or draft.
- Search results are a single deterministic projected set (can remain grouped by actor type). No duplicate favorite/recent/all rows and no duplicate tab stops.
- Invalid/deleted/unassignable IDs do not render; transient loading/error must not trigger destructive preference pruning.
- All/Agents/Squads is a type filter, not squad membership filtering. It needs neither `agent_member_ids` nor legacy member endpoint fallback.

## Loading/error versus no results

The current parent destructures agents/squads to `[]` and drops query status (`quick-create-issue.tsx:145`). Current empty rendering at line 1043 therefore cannot distinguish “no eligible actors” from pending/failed data. Carry status to the extracted picker alongside eligible arrays.

Show definitive “No matching actors” only once every data source required by the active scope is resolved. Retain usable cached/loaded actors during a background failure; show a small retry/error affordance for the unavailable source. Agents-only need not wait for squads; squads require squads plus the eligible-leader/permission inputs. Do not turn a role metadata outage into no matches: phase 1 can avoid those new dependencies completely. Existing precedent is `.trellis/spec/views/frontend/agent-discovery.md:58` and `agents-page.test.tsx:357`/`:388`.

## Exact implementation/test boundaries

| File | Responsibility |
| --- | --- |
| `packages/views/modals/quick-create-issue.tsx` | Keep eligibility, draft selection, seed precedence, query state and submission success ownership. Wire extracted picker; update recent history only in existing accepted-success branch. |
| `packages/views/modals/quick-create-actor-picker.tsx` (new) | Local quick-create trigger/popup, sibling actor/pin row, filters, sections, browse mode, focus recovery, error/loading wiring. No store definitions or backend calls. |
| `packages/views/modals/quick-create-actor-picker-model.ts` (new, if projection merits extraction) | Pure typed identity/search/summary/projection helpers. Keep business-specific shaping local to modals; reuse preview/pinyin helpers. |
| `packages/views/issues/components/pickers/property-picker.tsx` (conditional narrow edit) | Optional generic navigation reset/input-ref surface only; no favorites, actors or preference logic. |
| `packages/views/locales/en/modals.json`, `packages/views/locales/zh-Hans/modals.json` | Picker labels, actions, neutral summary and loading/error copy. Read repo conventions before editing Chinese copy. |
| `packages/views/modals/quick-create-actor-picker-model.test.ts` (new) | Canonical pure projection/search matrix with `// @vitest-environment node`. |
| `packages/views/modals/quick-create-actor-picker.test.tsx` (new) | Real PropertyPicker/Base UI keyboard, accessible controls, focus and popup lifecycle tests. Stub data/avatar boundaries only. |
| `packages/views/modals/quick-create-issue.test.tsx` | Parent wiring/success-only recents/seed and draft regressions; do not put the entire pure projection matrix here. |
| `packages/views/issues/components/pickers/property-picker.test.tsx` (new if shell changed) | Generic external-result-reset behavior, single-result/empty-row defaults, close reset and IME contract. |
| `e2e/quick-create-actor-picker.spec.ts` (new) | One full popup flow with TestApiClient setup, real selection/submission, persistence reopening, narrow viewport and keyboard pin focus. |

## Existing test evidence and proposed acceptance tests

Existing evidence (inspected, not executed during planning):

- `quick-create-issue.test.tsx:433` mocks the entire `PropertyPicker` and `PickerItem`, bypassing open state, focus, real DOM marker and key handling. It is appropriate for parent wiring only. It already proves draft restoration (line 645), agent submission/history (line 687), squad payload/history (line 917), and hidden-leader squad exclusion (line 949).
- `assignee-picker.keyboard.test.tsx:1` intentionally uses real primitives. At lines 67 and 82 it protects first-search-match Enter and no-match Enter inertness. Run it if the shared shell changes.
- `quick-create-scenario.test.ts:44` protects system identity; line 51 protects actual custom descriptions; line 58 protects squad-versus-leader responsibility separation.
- `e2e/issue-assist-flow.spec.ts:166` selects a real actor via `button[data-picker-item]` and verifies accepted quick create without running a daemon. It is gated by `E2E_ASSIST_PROVIDER_URL` (line 55), so it is not a standalone default picker gate.
- `e2e/upstream-selected-fixes.spec.ts:296`/`:321` selects through the real popup in a hidden-runtime-version regression. Preserve the primary button marker to avoid unnecessary E2E churn.

Add these canonical cases:

1. Pure projection: favorites/recent order, typed-ID dedupe, caps, no duplicate actor in compact view, unavailable reference omission, all browsing and search across actors omitted from compact sections.
2. Search: case/whitespace normalization, Chinese literal/name pinyin, full saved description match past character 300, independent agent/squad type filters, custom actor description, same ID across actor kinds. Keep pinyin internals and preview syntax matrices in their existing canonical suites.
3. Summary: saved description takes precedence over provenance; empty/media-only descriptions get neutral copy; custom actor named after a specialty gains no fabricated ability; squad text never uses leader text. Rendering remains inert.
4. Keyboard with real primitives: open and type → Enter chooses match exactly once; arrows skip pins/filter/footer and wrap selectable rows; no-result Enter does nothing; IME Enter/arrows with native `isComposing` and Safari `229` do not select.
5. Pin action: Tab reaches pin; Enter/Space toggle only preference, retain popup, selection and prompt; no nested `button button`; pin visible on focus. Pinning or removing the currently focused row restores deliberate focus, including last favorite removal and relocation between sections.
6. Result changes: typing/filter/browse/pin/async eligibility changes cannot make stale numeric highlight select a different actor. Type controls preserve query, closing by selection/Escape/outside click resets query, reopening is predictable.
7. Data lifecycle: initial pending, empty success, error with retry, one source unavailable, cached background failure, retry recovery; false “no results” never replaces unknown data; unavailable favorites cannot bypass eligibility.
8. Parent success wiring: accepted agent/squad/source-context create records submitted actor; failed/rejected create, pin, hover or selection records no recent. During an in-flight request, changing the current actor must not cause success to record the replacement actor.
9. Browser visual/interaction gate: full create dialog, long names/descriptions, selected+hover state, all-mode scroll, 390px width and short viewport, keyboard focus after pin mutation, popup dismissal restoring trigger focus. Existing create-layout contract is `.trellis/spec/views/frontend/issue-description-assist.md`.

Suggested narrow execution commands once implementation exists: `pnpm --filter @multica/views test modals/quick-create-actor-picker-model.test.ts modals/quick-create-actor-picker.test.tsx modals/quick-create-issue.test.tsx issues/components/pickers/assignee-picker.keyboard.test.tsx`, then views lint/typecheck and locale parity; run the standalone E2E in the checkout's isolated environment. Planning did not run tests or claim a passing baseline.
