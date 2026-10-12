# MCP 智能体选择页优化

## Goal

Improve the agent-selection step after saving or reusing an MCP configuration so workspace members can find and explicitly choose the intended agents, while preserving the existing assignment and retry behavior.

## Request and consent

- User request: “继续推进 MCP 智能体选择页优化”.
- On 2026-10-12 the user approved creating this task and continuing the work.
- Recovered source request, session `01a1300a-44c3-7d72-9d17-fc79a9ace823`: after clicking MCP Market → Save and continue, too many agents make the selection page inconvenient; the user explicitly requested an Impeccable audit and optimization, followed by “创建任务并完成优化”. This continuation retains that implementation authorization.

## Confirmed background

- `packages/views/mcp/mcp-setup-dialog.tsx` owns the saved-configuration assignment step shared by the workspace catalog and agent-context discovery.
- The current picker renders names and checkboxes without search or a selection summary. Selections start empty; successful assignments remain visible; failed selected IDs can be retried without creating the configuration again.
- Archived and internal/system agents are excluded. Loading, list errors, malformed responses, and empty lists have distinct states.
- The existing market/collection work is complete. Its acceptance criteria require shared Web/Desktop behavior, semantic tokens, accessible controls, responsive scrolling, and no new dependencies or platform imports.

## Requirements

- R1: Search eligible agents by name and complete description, with distinct no-match and empty-library states and a clear-search recovery action.
- R2: Show result and selection counts, a way to inspect only selected agents, and explicit bulk selection limited to currently visible eligible results. Clearing/deselecting must not obscure or reverse successful assignments.
- R3: Keep selected IDs across search and selected-only filtering. Submission still includes every unfinished selected eligible ID, including those hidden by a filter. Never preselect or automatically assign agents.
- R4: Show compact agent identity rows with the existing avatar primitive, wrapping names and concise descriptions. Maintain recognizable selected/hover/focus states and coarse-pointer targets.
- R5: Keep the search/selection controls and dialog actions reachable while the agent list scrolls. Make the primary action discoverable when no agents are selected, with a clear disabled state.
- R6: Preserve permission checks, unsupported-runtime handling, saved configuration identity, loading/error/malformed-list states, and partial-failure retry without recreating the configuration. Search must not hide the overall failure/result feedback.

## Acceptance Criteria

- [x] Search matches agent names and full descriptions; clearing a no-match search restores the list without changing selected IDs.
- [x] Bulk selection affects only currently visible unfinished eligible agents; selected-only review and clear-selection behave predictably.
- [x] An assignment includes hidden selected agents, preserves successful rows, and retries only failed unfinished selected IDs.
- [x] No archived/system agent becomes selectable; contextual unsupported runtimes stay blocked; selection starts empty.
- [x] At narrow and desktop widths the search and footer remain usable with a long agent list, long names, both languages, and light/dark themes; keyboard labels/focus and at least 44px coarse-pointer targets remain intact.
- [x] Focused regression tests, locale parity, typecheck, lint and static checks pass; browser fixture evidence records the exact source/components exercised.

## Scope boundaries

- This task targets the existing shared MCP setup/assignment surface.
- No new dependency, global store, backend/API/database change, third-party MCP execution, or deployment.
- No runtime/provider joins, connectivity claims, or extra per-agent assignment queries. Existing assignments remain authoritative through the existing mutation; “Assigned” describes confirmed success in the current operation.

## Validation baseline

- Before implementation: the market, agent discovery, and MCP locale suites passed, 41 tests across 3 files.
- Existing human API/Web/Desktop environment reports ownership mismatch. Use a task-owned browser fixture rather than interpreting that environment as this change.
