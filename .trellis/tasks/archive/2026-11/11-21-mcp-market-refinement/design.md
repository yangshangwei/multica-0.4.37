# MCP market refinement

The primary path is scan templates → review configuration and requirements → save a shared configuration → explicitly assign agents. Search and source selection form the first toolbar. Category selection and the persistent result status form the second. A short standalone introduction explains the path without a hero or onboarding panel.

Preserve the current card grid and 40px icon tiles. Group title and source/category/transport metadata in the card header, keep the two-line use description, provide one concise preparation hint from public metadata and retain the visible action. Required input labels are facts; absent inputs do not prove that setup or credentials are unnecessary. Full requirements remain in the existing dialog.

Layout responds to catalog container size so agent discovery dialogs also work. Below the wide category breakpoint, a native horizontally scrolling category row avoids spending four coarse-pointer rows on English filters; wider containers retain wrapping. Preserve native touch scrolling and sequential keyboard focus without custom pointer handlers. Type-tab touch sizing is scoped to MarketplacePage; the shared Tabs primitive is unchanged. Locale strings remain paired in en/zh-Hans. No new query owner, state store, API, installation or readiness semantics are introduced.

Rejected: replacement page design, a new filter popover, inferred per-tool credential badges and changes to the save/assignment flow. These add scope or information not supported by the current catalog.

Rollback: revert only this task's source hunks. MarketplacePage and its suite already contain unrelated squad-template changes and must never be restored wholesale.
