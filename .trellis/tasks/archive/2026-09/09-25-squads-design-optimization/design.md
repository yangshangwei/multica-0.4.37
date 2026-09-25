# Approved design

Use existing page-header, Base UI tabs, dropdown menus, buttons and AppLink.
Replace the in-page collapsible template catalog with a dedicated template tab. Keep the workspace list the default and show explicit empty-state creation actions. The template tab owns a single scrolling content area, a compact responsive two-column catalog, concise use-case copy, role summaries, and links to existing instances identified by template_key.

The page creation menu exposes existing template/custom flows. The list remains a dense table with a bounded identity track, metadata immediately following, and row background extending across the workspace. Creator/date stay user-toggleable; only defaults change. Preserve custom names/descriptions, creator-based scope semantics, archive permission gates, and query error handling.

Keyboard access uses native links/buttons plus Base UI keyboard handling. View tab state is carried in the navigation query parameters where supported by the shared adapter. No new server state or data duplication.

Alternatives: Keeping two expanded sections preserves the screenshot's duplicate scanning cost; a modal-only catalog hides the template/project workflow. Tabs provide explicit separation without replacing either workflow.
