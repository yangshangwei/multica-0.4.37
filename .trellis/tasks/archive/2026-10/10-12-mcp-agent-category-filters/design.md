# MCP category filters

## Boundary and reuse

The new category is view state in the shared MCP picker; selected IDs and mutation results remain in `McpSetupDialog`. Reuse core `buildAgentRoleMetadata`, `resolveAgentRole`, `resolveAgentDirectoryCategory`, and `AGENT_CATEGORY_PRESET_ORDER` rather than copying a name-based or autonomy-based classifier. The screenshot supplies the category-chip presentation, not its actor inventory.

Extend `McpAgentChoice` with optional `category`/`template_key` fields so real Agent rows retain their metadata and ID/name-only contextual/test rows still work. Use existing agent-template and squad-template hooks for immutable template metadata, not actual squads/membership. If needed, add a small optional `enabled` argument to those existing hooks with default true: MCP requests metadata only in the general post-save picker when eligible template-backed rows lack explicit category. Preserve existing locale keys/stale times and callers; no new query-key family or API client method.

## Data flow and uncertainty

Resolve an effective category for every eligible choice. Explicit categories remain authoritative regardless of template readiness. With missing initial template catalogs, template-dependent fallback stays unknown; untemplated agents can still resolve to general-purpose. Cached catalogs can serve classification even if a background refresh errors. Retry reloads metadata without clearing search/category/selected IDs.

Derive category options/counts from the complete eligible choices, order represented presets using the existing order, then sort custom names by locale. All always remains available. Do not show an empty squad/system chip. Unknown template fallback keeps preset counts/results visibly incomplete and must not produce a successful zero-match message for a category that might still gain rows.

Category, query and selected-only state form the visible projection. Bulk controls use this projection; unfinished counts and submission use the full eligible choices as before. Counts for category inventory ignore the visible projection. Reset a vanished category only when complete classification proves it no longer exists; a temporary metadata failure is not disappearance. Clearing category selects All while retaining query and selection.

## Presentation

Use a horizontal compact chip row in the fixed picker toolbar, visually consistent with the supplied reference and existing AgentDiscoveryToolbar. Use real Button controls with one pressed category, counts and an accessible category-group label. The row can scroll horizontally for many or long custom names without creating page overflow or consuming several additional toolbar rows. Selected state remains identifiable on hover; coarse targets are at least 44px.

Do not replace the previously verified avatar/description list, search, selected review, row-relative accessible descriptions or footer. Contextual single-agent discovery does not need category/catalog chrome.

## Validation

Tests keep the existing core helper classification matrix canonical. Picker integration covers renamed specialist/leader, explicit custom override, count independence, cross-category hidden selection payloads and category-scoped bulk operations, cached/initial-error metadata, retry, category disappearance and no squad choices. Existing 248-test baseline passes before edits.

Reuse the previous production component browser fixture with synthetic category/template metadata and actual shared CSS. Inspect wide/narrow, themes/locales, long category labels and category + search/selection composition in one batch, then one confirmation pass only if a defect requires it. Existing human desktop stays running for HMR; do not use its account or dataset as synthetic test fixtures.

No rollout or migration is needed; reverting the scoped view/hook/localization changes restores the previous picker.
