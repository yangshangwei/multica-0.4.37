# Capability market

## Scope

The workspace sidebar exposes one Capability market destination while Skills and
MCP remain management pages. Web and Desktop mount the same `MarketplacePage`.
The market has MCP and Skills tabs; neither an external registry nor a combined
resource/permission model is introduced.

## Signatures and routes

- `paths.workspace(slug).marketplace()` → `/{slug}/marketplace`.
- `paths.workspace(slug).marketplaceTab(type: "mcp" | "skills")` adds `?type=…`.
- `McpMarketPanel({ workspaceId, active? })` and
  `SkillMarketPanel({ workspaceId, active? })` own their domain catalogs/dialogs.
- Missing or unrecognized `type` selects MCP. Navigation uses the shared
  adapter, retaining unrelated query parameters and the fragment.

The parameterless page builder, `WORKSPACE_PAGES`, sidebar, route icon mapping,
command-search keywords and editor workspace-link recognition must agree.
Register the route in diagnostics bucketing too, retaining slug/query redaction.
Desktop uses a workspace session route, not a pre-workspace overlay. This is
not a new root route and does not reserve a workspace slug.

## Lifecycle contracts

The URL owns the selected type. Once visited, a type panel stays mounted but
hidden/inert when inactive. Pass `active=false` into its queries and dialogs;
CSS on the surrounding panel alone cannot hide a portal. Returning to a type
restores its draft and source/filter context. Never eagerly mount the complete
SkillsPage or MCP management controller just to reuse its catalog.

Controllers remount on workspace identity changes. Desktop's TabContent mounts
only one active tab host; it does not keep inactive React page trees alive.
There is no new global navigation guard or cross-route draft persistence.
Leaving a workspace/page or switching Desktop tabs retains existing transient
dialog disposal behavior, while caches/preferences remain workspace-scoped.

Market copy creation and ordinary skill creation keep their different success
navigation. MCP save and assignment remain separate. Domain permission,
provenance, input and retry contracts live in the corresponding market specs.

## Validation and errors

| State | Required outcome |
| --- | --- |
| Missing/invalid type | MCP catalog; never an invalid request |
| Type navigation / browser Back | URL and selected tab agree |
| Inactive visited type | No visible portals or catalog polling; retain draft |
| Workspace changes | Fresh scoped controller; no old assignment/draft shown |
| One catalog fails | Other type remains reachable; error is not an empty catalog |
| Empty resource library | Keep management page and offer explicit market link |

## Good, base and bad cases

- Good: a dirty Skill editor survives a type round-trip with the same DOM values.
- Base: the menu opens MCP and Skills opens from its library's From template link.
- Bad: selecting a type unmounts a dirty editor, or hiding the panel leaves its
  portaled dialog over the other type.

## Required tests

Test route generation/defaults, URL history, sidebar/resource destinations,
command search, desktop tab presentation and editor workspace prefixes. Market
page tests cover lazy first mount, retained visited panels and workspace reset.
Domain suites own real creation, dirty state, portal hiding and permission tests.
Production Web and native Desktop verify the routed user journeys.

## Wrong vs correct

Do not render an inactive domain as `selected && <SkillMarketPanel … />` after
the user has begun editing. Keep the visited panel and pass its visibility:

```tsx
<SkillMarketPanel workspaceId={workspaceId} active={type === "skills"} />
```

The surrounding shell must also hide/inert the inactive panel.
