# Design

Replace the automation-specific collapsed catalog with a shared presentational gallery. The empty list and template creation picker supply their existing query results and navigation callbacks. Keep the generic built-in catalog unchanged because other domains use it.

The gallery owns its scene filter and records it with the selected scroll offset through the existing platform view-state adapter. Plain scroll capture alone is insufficient on desktop because the picker and configuration steps share a pathname and leaving configuration replaces that pathname's scroll entries. Restore only after real template content loads. A `return_to=autopilots` marker returns empty-page entries to their originating gallery; standalone picker entries return to the picker. Both native Back and the page's Back action therefore restore the same route-scoped state, without stale filter or scroll query parameters. Unknown templates remain in All, with a generic icon and conservative output text.

Use the existing schedule formatter and UI tokens. Restrict layout width to the gallery; preserve the existing full-width management list. Keep template prompt, schedule and mode server-owned and read-only during creation.

Cleanup plan: replace the two automation template renderers with one, remove the detached empty-state buttons, retain tested creation and permissions behavior, and change no generic catalog behavior.
