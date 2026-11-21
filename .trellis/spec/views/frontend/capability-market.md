# Capability market

## Scope

The workspace sidebar exposes one Capability market destination while Skills and
MCP remain management pages. Web and Desktop mount the same `MarketplacePage`.
The market has MCP, Skills, built-in AI squad templates, and agent role template
tabs. Agent templates are a discovery catalog; the existing agent creation flow
remains a direct entrance to the same role cards. Neither an external registry
nor a combined resource/permission model is introduced.

## Signatures and routes

- `paths.workspace(slug).marketplace()` → `/{slug}/marketplace`.
- `paths.workspace(slug).marketplaceTab(type: "mcp" | "skills" | "squad-templates" | "agent-templates")` adds `?type=…`.
- `McpMarketPanel({ workspaceId, active? })` and
  `SkillMarketPanel({ workspaceId, active? })` own their domain catalogs/dialogs.
- `SquadTemplatesMarketPanel({ workspaceId, active })` adapts the shared squad
  template catalog to the market, loading the saved squad inventory only while
  active so template cards can link existing instances accurately.
- `AgentTemplatesMarketPanel({ workspaceId, active })` uses the locale-scoped
  role-template query with `enabled: active` and shared role cards. It does not
  load agents or show existing-instance counts/links. A broken agent-list API
  cannot block this catalog.
- Missing or unrecognized `type` selects MCP. Navigation uses the shared
  adapter, retaining unrelated query parameters and the fragment.
- The old `/{slug}/squads?view=templates` bookmark replaces its location with
  `/{slug}/marketplace?type=squad-templates`, preserving unrelated query
  parameters and the fragment.

The parameterless page builder, `WORKSPACE_PAGES`, sidebar, route icon mapping,
command-search keywords and editor workspace-link recognition must agree.
Register the route in diagnostics bucketing too, retaining slug/query redaction.
Desktop uses a workspace session route, not a pre-workspace overlay. This is
not a new root route and does not reserve a workspace slug.

## Lifecycle contracts

The URL owns the selected type. Once visited, a type panel stays mounted but
hidden/inert when inactive. Pass `active=false` into its queries and dialogs;
CSS on the surrounding panel alone cannot hide a portal. Returning to a type
restores its draft and source/filter context. Squad template queries are disabled
while inactive and their project setup dialog is closed with the panel. Never
eagerly mount the complete SkillsPage or MCP management controller just to reuse
its catalog.

Controllers remount on workspace identity changes. Desktop's TabContent mounts
only one active tab host; it does not keep inactive React page trees alive.
There is no new global navigation guard or cross-route draft persistence.
Leaving a workspace/page or switching Desktop tabs retains existing transient
dialog disposal behavior, while caches/preferences remain workspace-scoped.

## Welcome and case directory

`MarketplaceWelcome` stays inside the shared market and is driven by the same
URL-selected category as the retained catalog panels. Keep all four directory
entries visible; Base UI tab semantics provide arrow-key focus movement and
Enter/Space activation, while navigation still commits through the existing
adapter. The selected entry, case detail, and documentation destination must
agree after direct entry and browser history changes. Invalid or missing types
continue to select MCP.

The release sign-up check is clearly marked as an example. Its MCP, Skills,
squad-template, and agent-template details explain what the user would
configure, reuse, assemble, or create, and what that enables. Selecting a
directory entry or opening its compact disclosure is presentation-only: it
does not install, assign, create, or execute a resource. Keep the documentation
link category-specific and use the existing documentation/navigation adapter.

The welcome directory uses a two-column layout on narrow surfaces and sits
beside the example on wide surfaces. On compact layouts, use a native closed
`details` disclosure inside the active catalog's existing scroll root so the
example can open without trapping the catalog below a second scroller. Preserve
the agent-template panel's original scroll root and restoration key by passing
the disclosure through its leading slot. Do not bring back a horizontal type
strip or a width observer. Active styling must stay identifiable on hover;
keyboard focus stays visible, coarse-pointer targets remain at least 44px, and
the case transition is disabled for reduced-motion users. Keep the welcome
presentation-only and workspace-scoped; it must not add preferences, queries,
or a new route.

Market copy creation and ordinary skill creation keep their different success
navigation. MCP save and assignment remain separate. Domain permission,
provenance, input and retry contracts live in the corresponding market specs.

## Agent-template creation handoff

- Keep the existing category order and default; append agent templates.
- Keep the selected type fully visible in the horizontal strip on direct entry,
  history/type changes and container resize. Scrolling the strip must not cause
  document-level horizontal overflow. Type buttons retain their intrinsic label
  width (`flex-none`): coarse-pointer minimum widths must not allow labels to
  shrink and overlap. Disconnect the width observer on type change/unmount.
- Market cards link to the existing agent template configuration route with a
  selected `template` and exact `source=marketplace`. Selection itself does not
  create an agent, materialize skills, or add a squad member.
- The source is a navigation hint, not a return URL or API field. For a market
  source without `squad`, the creation page uses `useBackOrReplace` with the
  fixed `marketplaceTab("agent-templates")` fallback. An application-proven
  history entry takes precedence; a cold/new tab or unsupported adapter uses
  the fallback. Unknown sources use the original creation flow.
- An explicit `squad` retains the original picker/configuration/back and submit
  semantics even if a URL also contains the market marker. The market never
  infers squad membership from its own query parameters.
- Ordinary successful creation still lands on the new agent; squad creation
  still attempts membership and lands on the squad. Never retry an already
  committed agent merely because joining a squad failed.
- Keep the market panel's single content scroll root separate from the original
  picker's same-path scroll memento. Retain visited category DOM; restore through
  the existing platform adapter only after template content can hold the saved
  offset. Do not add storage or promise configuration drafts across routes.
- Do not render the complete creation page inside the market. Its creation
  shell, agent-list query and existing-member controls belong to its own flow.

## Validation and errors

| State | Required outcome |
| --- | --- |
| Missing/invalid type | MCP catalog; never an invalid request |
| Type navigation / browser Back | URL and selected tab agree |
| Inactive visited type | No visible portals or catalog polling; retain draft |
| Workspace changes | Fresh scoped controller; no old assignment/draft shown |
| One catalog fails | Other type remains reachable; error is not an empty catalog |
| Squad inventory fails | Show retry feedback; do not imply no saved instances |
| Agent inventory fails | Agent-template catalog remains usable; no instance request is needed |
| Agent template load fails / succeeds empty | Error with retry / explicit empty catalog; never conflate them |
| Market configuration opened cold | Source-aware Back replaces with the agent-template category |
| Market marker plus explicit squad | Preserve the squad creation/back semantics |
| Legacy squad template URL | Replace with the market squad-template type and preserve other URL context |
| Empty resource library | Keep management page and offer explicit market link |

## Good, base and bad cases

- Good: a dirty Skill editor survives a type round-trip with the same DOM values.
- Base: the menu opens MCP; Skills opens from its library's From template link;
  the squad welcome opens the squad-template category.
- Bad: selecting a type unmounts a dirty editor, or hiding the panel leaves its
  portaled dialog over the other type.

## Required tests

Test route generation/defaults, URL history, sidebar/resource destinations,
command search, desktop tab presentation and editor workspace prefixes. Market
page tests cover lazy first mount, retained visited panels and workspace reset.
Domain suites own real creation, dirty state, portal hiding and permission tests.
Production Web and native Desktop verify the routed user journeys.
Agent-template suites additionally cover no instance Query, template loading and
retry, known/unknown policy display, exact source-marker handoff, safe-history
versus cold fallback, squad priority, original picker links and scroll recovery.
Production coarse-pointer checks measure role actions at least 44px high at
tablet and phone widths; retain fine-pointer desktop geometry. Narrow captures
must show the entire selected tab, not merely a clipped active underline.

## Wrong vs correct

Do not render an inactive domain as `selected && <SkillMarketPanel … />` after
the user has begun editing. Keep the visited panel and pass its visibility:

```tsx
<SkillMarketPanel workspaceId={workspaceId} active={type === "skills"} />
```

The squad catalog follows the same retained panel contract:

```tsx
<SquadTemplatesMarketPanel workspaceId={workspaceId} active={type === "squad-templates"} />
```

The surrounding shell must also hide/inert the inactive panel.
