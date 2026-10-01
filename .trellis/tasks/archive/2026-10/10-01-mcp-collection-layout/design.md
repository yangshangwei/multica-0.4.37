# Design

Use the Agents/Skills full flex canvas and CollectionPageHeader. Keep PageHeader's shared gutter unchanged. Remove the nested SettingsTab presentation only for the standalone page, while preserving its stateful MCP workspace controller for both entries. Prefer a small explicit presentation variant/slot over duplicated query/mutation logic or a new state layer.

Use a named container for the market grid. Skills supplies the card typography, spacing, surface and 40px icon-tile proportions; retain MCP's higher-contrast focus outline rather than copying the weaker ring token. Browser templates remain visually distinct (Bug vs Workflow), sharing a blue tone; reasoning uses Brain with purple. Reuse the shared icon registry and existing semantic palette without assigning MCP records skill business categories.

Counts must come from the existing query cache; ensure a single active catalog polling owner. Preserve explicit tab selection and the saved-configuration assignment step on refresh. Header actions must share the existing custom editor handler, disabled and permission state. No hook-bearing component remounts caused by inline component definitions.

The grid is selected by container size rather than viewport so embedded discovery remains one/two columns. The standalone body fills remaining height and scrolls, while shared settings and dialog owners retain their current scrolling contracts.
