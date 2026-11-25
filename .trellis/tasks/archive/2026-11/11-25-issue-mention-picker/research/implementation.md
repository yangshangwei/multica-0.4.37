# Implementation evidence

Implemented the opt-in categorized mention picker under `packages/views` only. Runtime sources are ready for final Web/Desktop builds. No branches, commits, environment changes, E2E changes, or spec changes were made by the implementer.

## Changed files

- `common/actor-discovery.ts`: extracted existing pure catalog, ranking, filter and paging functions; quick-create shortcut/default lifecycle stays in its original model. Updated direct imports in quick-create picker, model tests, creator recommendations and quick-create issue.
- `editor/extensions/categorized-mention-list.tsx`: explicit QueryClient observers, scoped read-only favorites, mention recency, full-description discovery, inclusive agent/leader filters, directory paging, separate All members/issue budgets, category controls, control focus/IME handling, guarded acceptance, active-descendant/listbox semantics.
- `editor/extensions/mention-ranking.ts`: neutral issue conversion and cancelled/direct-hit ranking shared by both lists without a runtime import cycle.
- `editor/content-editor.tsx`, `editor/extensions/index.ts`, `editor/extensions/mention-suggestion.tsx`: default-off `mentionPicker` option; context mode wins.
- `editor/extensions/suggestion-popup.tsx`: opt-in composition capture guard before upstream suggestion Escape handling.
- `issues/components/comment-input.tsx`, `reply-input.tsx`, both editors in `comment-card.tsx`: explicit opt-in.
- English/Chinese `locales/*/editor.json`: categories, descriptions, leadership, shortcuts, controls and hints.
- Tests: new categorized suite; real-plugin composition regression; host opt-in assertions in composer/edit suites; direct-import updates for discovery extraction.

## Verification (final sources)

- Focused command from approved plan, plus categorized and comment-card-edit-gate suites: **10 files, 185 tests passed**, 14.70s.
- `pnpm --filter @multica/views typecheck`: exit 0.
- `pnpm --filter @multica/views lint`: exit 0; 27 pre-existing warnings, no warnings in new picker/discovery/ranking files.
- `git diff --check`: clean.
- Regression tests cover full 550-row discovery, responsibility text beyond preview, inclusive leaders, stable identity on append, removal/reintroduction and filter round trips, pagination reset, account-change rejection, live query updates, read-only favorite readiness, separate All-members budget, retained server issue on directory retry, and active descendant.
- Real Editor integration types through `handleTextInput`, clicks a filter, continues typing, enters controls via Alt+Down, returns to typing, and accepts the original range. Separate real suggestion plugin regression verifies composition Escape and Safari keyCode 229 leave popup active, then ordinary Escape closes it.

## Review fixes

Addressed all four findings in `research/check-review.md`: abandoned selection reconciliation, pagination scope reconciliation, successful issue search preserved during unrelated directory retry, and editor-owned active candidate semantics.

## Remaining verification scope

Native/browser geometry, real Chinese IME, platform shortcut formatting, full keyboard Tab sequence through every auxiliary action, and production Web/Electron host journeys remain with the leader. Unit tests mock scoped preference readiness and directory eligibility; existing factory/store suites retain authority/runtime contracts, but these mocks alone do not establish real persisted hydration races. The categorized presentation explicitly sets `profileLink={false}` on actor avatars. Root fixture is `data-mention-picker="categorized"`; candidates expose `data-mention-key="type:id"` and role=option.


## Final member-observer follow-up

- Reproduced the member observer defect with a real QueryClient and the actual `createMentionSuggestion` directory factory: a settled-to-settled member update did not refresh roster rows or revoke the workspace-granted agent after current membership disappeared.
- The member observer now explicitly notifies on `data`, `isPending` and `isError`, preserving the existing canonical directory/permission projection without an unused data read.
- Categorized avatars explicitly disable profile links. Both locale hints interpolate Core's platform shortcut formatter (macOS `⌥↓`, Windows `Alt+↓`).
- Focused categorized suite: **14 tests passed**, including the member/permission update regression and both platform hint cases. Avatar mocks assert the explicit non-link contract for every rendered actor.
- Follow-up verification: Views typecheck exit 0; ESLint on the two affected TypeScript files exit 0 with no findings; `git diff --check` clean. Unaffected broad suites were not repeated.
