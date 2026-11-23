# 优化能力市场 MCP 浏览与配置引导

## Goal

落实已审计并获用户授权的 MCP 市场界面优化：紧凑筛选、精简卡片、配置流程和准备条件提示，以及类型标签触屏适配；保留现有创建分配及导航行为。

## Requirements

- Refine the existing MCP marketplace using the five findings already presented in the Impeccable audit. The user approved direct implementation with “直接优化吧”.
- Reduce the vertical space spent on filters and repeated card metadata while retaining search, independent source counts, category composition, visible result counts and explicit card actions.
- Explain that a template creates a shared configuration and agents must be assigned explicitly. Keep save, assignment and runtime connectivity distinct.
- Surface concise preparation requirements derived only from existing public template input/requirement metadata.
- Keep the established neutral surfaces, semantic tokens, icon tiles and container-based grid. Support English and Simplified Chinese, light/dark themes and narrow/coarse-pointer layouts.
- Preserve existing setup, permissions, errors, retries, focus restoration, inactive-panel polling/draft retention and all current marketplace types.
- Respect all pre-existing working-tree changes; add no dependencies and change no backend/API or global UI primitives.

## Acceptance Criteria

- [x] Wide catalogs group search and source controls into one toolbar; categories and the live result count share the following row. Narrow toolbars wrap and the native category strip scrolls horizontally without document overflow.
- [x] Source and category groups have clear localized visible and accessible labels.
- [x] Cards use compact metadata and concise preparation hints without claiming runtime readiness or exposing secret/configuration values.
- [x] The standalone market explains shared save and explicit assignment; existing agent discovery remains coherent.
- [x] Marketplace type controls have at least 44px targets for coarse pointers and remain keyboard navigable, including the pre-existing squad template category.
- [x] Existing MCP and marketplace behavior tests pass; focused new tests cover preparation hints and localized guidance only where useful.
- [x] Targeted lint/typecheck, locale parity, a compiled real-component visual/interaction matrix and final detector checks provide recorded evidence.

## Notes

- Baseline: 4 test files / 127 tests passed before edits. Existing marketplace keyboard tests emit React act warnings.
- Existing local API/web environment fails checkout/build ownership checks; use a task-owned compiled component fixture with real shared views/styles and synthetic catalog/cache data for bounded visual verification.
