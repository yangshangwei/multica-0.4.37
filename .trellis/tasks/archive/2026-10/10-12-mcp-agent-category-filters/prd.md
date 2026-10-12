# MCP 智能体分类筛选

## Goal

Let members narrow the MCP assignment picker by the agents' real directory categories, especially specialist and coordinating roles, while retaining the previously delivered search, review and explicit assignment behavior.

## Request and authorization

- The user requested category controls similar to the supplied screenshot, distinguishing professional roles and responsible/coordinating agents, and explicitly excluded AI squads from the MCP target picker.
- The user approved: “创建任务，完成分类筛选”. This is an authorized continuation of the completed MCP agent-selection optimization.

## Confirmed behavior

- The shared saved-configuration picker is `packages/views/mcp/mcp-agent-picker.tsx`; `McpSetupDialog` owns eligible IDs, confirmed assignments and retry.
- `resolveAgentDirectoryCategory` is the existing classification authority: explicit saved category wins, then immutable `template_key` role provenance, then general-purpose for agents without a recognized template.
- Existing presets are General-purpose agents / Specialists / Coordinators (通用智能体 / 专业角色 / 统筹角色), followed by custom category names. Template leader metadata identifies coordinating roles; editable names and autonomy levels do not.
- Archived/system agents already remain outside MCP assignment choices. Role/squad template catalogs provide metadata; actual squad instances/membership are not MCP targets.

## Requirements

- R1: Add an All control and category chips with counts, using the existing category/provenance helpers and localized preset names. Show categories represented by eligible choices and preserve actual custom category names.
- R2: Compose one selected category with the existing full-description search and selected-only review. Category counts come from the full eligible directory independently of search/selection filters.
- R3: Switching category preserves explicit selected IDs. Selecting/deselecting results applies only to the currently displayed unfinished eligible IDs; submission still includes hidden selected IDs across categories.
- R4: Template-dependent category loading/failure is truthful and recoverable; it never silently misclassifies responsible agents as general-purpose or blocks All-view assignment. Cached metadata remains usable on background refresh failure; explicit custom categories remain usable independently.
- R5: Preserve stable keyboard-accessible pressed states, recognizable selected/hover states, long custom-name handling and coarse-pointer targets. Keep list-only scrolling, toolbar/footer bounds, existing confirmation/failure handling and contextual singleton behavior.

## Acceptance Criteria

- [ ] Built-in specialists and coordinating/leader roles enter their existing correct categories after renaming; explicit categories override provenance.
- [ ] Category chips/counts reflect only current eligible agent records; no AI squad or system-assistant target/filter is introduced.
- [ ] Search, category and selected-only filters compose without dropping hidden selections; bulk and submitted payloads preserve their existing scopes.
- [ ] Loading/error/retry and unavailable-category refresh behavior retain usable All/explicit-category results and avoid false empty/zero counts.
- [ ] Both languages/themes, wide/narrow layouts, long custom categories, keyboard controls and reachable fixed actions pass focused browser checks.
- [ ] Existing MCP regression baseline (248 tests), meaningful new category regressions, lint/type/static checks and independent review pass.

## Scope

- Shared MCP picker, optional existing template-hook enablement, adjacent category/wiring tests, localized MCP copy and existing MCP spec.
- No real squad/membership query, capability inference by name/autonomy, backend/API/database change, new dependency/store, runtime query, per-agent MCP prefetch, or configuration/assignment rewrite.
