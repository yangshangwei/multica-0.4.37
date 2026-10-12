# Independent MCP agent-selection check

- Date: 2026-10-12
- Role: dispatched `trellis-check`; no further agents dispatched.
- Reviewed task: `10-12-mcp-agent-selection`, its PRD/design/implementation plan,
  complete saved native hook context, check context and applicable MCP specs.
- Product scope: `mcp-agent-picker.tsx`, `mcp-setup-dialog.tsx`,
  `mcp-market.test.tsx`, English and Simplified Chinese `settings.json`.
- The reviewer stayed read-only until the main session handed off stable source.
  Concurrent desktop/core recovery changes were left untouched.

## Findings (fixed)

1. **Missing directory-refresh regression** —
   `packages/views/mcp/mcp-market.test.tsx:650`.
   Existing tests covered hidden selections and bulk filtering, but did not
   verify selected IDs after the underlying agent directory changed. Added one
   canonical component regression that selects three agents, refreshes the
   query cache with an archived agent, an internal agent and a new eligible
   agent while removing the third original agent, then verifies selected count
   zero, disabled submission and a subsequent payload containing only the new
   eligible ID. This exercises cache-to-view eligibility and payload wiring
   without adding a duplicate model matrix or changing product logic.

2. **Inaccurate positioning comment** —
   `packages/views/mcp/mcp-agent-picker.tsx:154`.
   Corrected the comment to identify the absolute `sr-only` full description
   as the element anchored by the relative row. Base UI's sibling hidden input
   is fixed to the dialog containing block in the actual browser fixture; the
   row's relative positioning does not anchor that input. This correction is
   documentation only and changes no layout or runtime behavior.

## Findings (not fixed)

No new task-scope functional defect or blocking judgment issue was found.

One pre-existing API limitation remains outside this task's ownership:
`packages/core/api/client.ts:2163` parses the agent response with an empty-list
fallback, and `packages/core/api/schema.ts:40` returns that fallback on schema
failure. Consequently some malformed real API responses become a successful
empty directory before the dialog's non-array guard can inspect them. This
limitation was already recorded in `selection-evidence.md`; changing the shared
API parsing/error contract requires a separate core task and was not attempted.

## Verified contracts

- Submission and unfinished counts intersect selected IDs with the full
  eligible directory, independently of search and selected-only review.
- Current-result selection unions visible unfinished IDs with hidden choices;
  deselection and clear preserve confirmed successes and failure records.
- Successful rows remain checked and disabled, including after clear.
- Filtered-out failure details remain rendered inside the list scroll owner;
  the overall failure message stays outside filtering.
- Parent operation, permission and unsupported-runtime guards cover callbacks
  and disabled controls. Selection starts empty and contextual ID/name-only
  input retains its singleton flow.
- Full-description search, exact checkbox names, full accessible descriptions,
  relative public avatar URLs and whole-row click selection have behavioral
  assertions in the canonical market suite.
- No new runtime join, per-agent assignment prefetch, global store, dependency,
  API change or platform import was introduced. Existing assignment mutation
  and saved-instance identity stay authoritative.
- The main session's updated MCP spec records these filtering/count/scrolling
  and accessibility contracts, including the positioning and clamp lessons.

## Verification

Commands executed independently by the reviewer:

```bash
pnpm --filter @multica/views exec vitest run mcp/mcp-market.test.tsx mcp/mcp-agent-discovery.test.tsx locales/mcp.test.ts locales/parity.test.ts
pnpm --filter @multica/views exec eslint mcp/mcp-agent-picker.tsx mcp/mcp-setup-dialog.tsx mcp/mcp-market.test.tsx
pnpm --filter @multica/views typecheck
git diff --check -- packages/views/mcp/mcp-agent-picker.tsx packages/views/mcp/mcp-setup-dialog.tsx packages/views/mcp/mcp-market.test.tsx packages/views/locales/en/settings.json packages/views/locales/zh-Hans/settings.json
```

- Tests: **pass**, 112 tests across four files after the new refresh regression
  (Vitest 4.1.0, 5.93 seconds). The initial independent run passed 111 tests.
- Lint: **pass**, scoped ESLint exit 0; repeated after the comment correction.
- TypeCheck: **pass**, views `tsc --noEmit` exit 0 after the test addition.
- Whitespace: **pass**, scoped `git diff --check` exit 0.
- No product logic or visual edits were required by this review.

Browser and repository-wide validation belong to the main session and are
recorded in `../verification.md`, not claimed as independent reviewer runs.
The main reported five passing production fixture captures, reachable controls,
bounded dialog/list scrolling, two-line previews, coarse targets of at least
44px, keyboard Space, hidden payloads, selected-only/clear and partial retry;
root typecheck passed 9/9 and lint 6/6, with Knip unchanged from baseline.
The browser fixture uses synthetic cached agents and mutation fixtures: it is
shared-component browser validation, not real API or native Electron acceptance.

## Handoff

Reviewer changes are limited to the refresh regression, accurate source comment
and this report. No commit, archive or task lifecycle mutation was performed.
The main session owns final evidence aggregation and finish-work.
