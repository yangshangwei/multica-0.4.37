# Implementation verification

Date: 2026-10-12. Role: Trellis implementer. Main session owns browser acceptance, independent review, specs, commit and task lifecycle.

## Files changed

- `packages/views/mcp/mcp-agent-picker.tsx`: local search/selected-review state, controlled ID-based selection, derived eligible counts, result-scoped bulk controls, query-free avatars, inert description previews, full accessible descriptions and a bounded list scroll owner.
- `packages/views/mcp/mcp-setup-dialog.tsx`: preserves canonical eligibility, saved identity, mutation/results and retry ownership; guards selection changes/clear; separates fixed saved-step content from list scrolling; keeps Assign selected visible while disabled without fresh unfinished IDs.
- `packages/views/mcp/mcp-market.test.tsx`: meaningful selection/payload regressions and a narrow API base-URL test mock for public avatar resolution.
- `packages/views/locales/en/settings.json` and `zh-Hans/settings.json`: matching control, count, empty/search and filtered-failure copy.

No dependency, store, API/backend change, runtime join, assignment prefetch, platform import or commit was added. Existing unrelated work remains untouched.

## Verification run

- Red before implementation: `pnpm --filter @multica/views exec vitest run mcp/mcp-market.test.tsx --reporter=dot` — 6 expected failures from missing search/bulk controls and missing disabled initial Assign selected; 30 passed.
- `pnpm --filter @multica/views exec vitest run mcp locales/mcp.test.ts locales/parity.test.ts --reporter=dot` — 247 tests passed across 12 files.
- `pnpm --filter @multica/views typecheck` — passed.
- `pnpm --filter @multica/views lint` — exit 0; 27 existing warnings, all outside task-owned files.
- After the browser correction batch: `pnpm --filter @multica/views exec vitest run mcp/mcp-market.test.tsx mcp/mcp-agent-discovery.test.tsx locales/mcp.test.ts locales/parity.test.ts --reporter=dot` — 111 tests passed across 4 files.
- `pnpm --filter @multica/views exec eslint mcp/mcp-agent-picker.tsx mcp/mcp-setup-dialog.tsx mcp/mcp-market.test.tsx` — passed with no task-file warnings.
- `git diff --check` — passed.

Tests cover full-description search beyond the preview cap, duplicate names with distinct IDs, no-match recovery/focus, relative avatar URL resolution, hidden selected IDs in submission, scoped select/deselect, selected review/clear, busy/permission guards, contextual singleton/unsupported runtime, and confirmed-success retention plus filtered failure retry without recreating the saved server.

## Browser correction and handoff

The main session's first production fixture capture found focus-induced ancestor scrolling and a preview display conflict. Base UI CheckboxRoot renders its hidden input beside the visual span; the installed browser version gives that input fixed positioning. The row's absolute `sr-only` full description had no positioned row ancestor and inflated the dialog scroll height beyond the list. Keep each picker row label `relative` so the description belongs to the clipped list row; this is not a timer/reset workaround. Remove `block` from a `line-clamp-2` preview so its `-webkit-box` display works. Browser computed-style evidence takes precedence over the initial source shorthand about the hidden input.

Filtered-out failure details now render inside the same list scroll owner, while aggregate failure feedback and result/selection/confirmation counts stay outside it. This prevents an unbounded failure block from consuming fixed toolbar/footer space.

Main session must confirm actual post-interaction ancestor scroll offsets/header-toolbar-footer bounds, two-line previews, both locales/themes and coarse targets in its final production fixture capture. Unit tests do not prove layout. Independent review and broader/static integration checks remain main-session responsibilities.
