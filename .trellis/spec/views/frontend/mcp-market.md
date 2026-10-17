# MCP market and assignment

## Scope

The shared workspace MCP page and Settings MCP tab manage saved configurations
and link to `/{slug}/marketplace?type=mcp`. `McpMarketPanel` composes member access,
safe saved-instance summaries and `McpTemplateCatalog` in the capability market.
See [Capability market](./capability-market.md) for navigation and lifecycle.
The agent MCP tab opens `McpAgentDiscovery`, which shows existing workspace
instances before the same template catalog. The built-in catalog contains sixteen
reviewed local recipes, including project/database inputs. Deployment recipes
come from the API server's `MULTICA_MCP_TEMPLATE_DIR`, not an Electron scan or an
external registry/plugin store. Source filters are All / Deployment / Built-in;
source counts ignore search/category filters and categories follow the selected
source. Keep search when changing source; reset a category that becomes unavailable.
Microsoft Learn and DeepWiki were withdrawn for intranet deployments; saved
built-in instances keep their key-based icons. Category filters appear only for
categories the loaded catalog uses, including coding, database and documentation.
Show each recipe's requirements before
save; do not describe a recipe's presence in the catalog as runtime
connectivity. New category labels ship in English/Chinese; source-aware icons
remain consistent for market cards and saved instances. Publishing rules and the offline
`make check-mcp-catalog` gate are documented in `docs/mcp-catalog-publishing.md`.
The offline gate rejects input placeholders in commands, arguments and URLs,
including `$NAME` and Windows `%NAME%` as well as braced/angle forms. Keep those
fields on the same test-only detector; checking only `${NAME}` leaves unusable
recipes publishable even when they contain no credential keywords.

## Identity and configuration

- `(source, key)` identifies a recipe, never a user-editable instance name.
  Multiple instances may share that identity and retain distinct names/IDs.
  Built-in and deployment entries may share a key; React keys, related-instance
  matching, availability and icon selection must include source.
- Trusted creation sends name, `template_source`, template key, recipe version and optional
  `template_inputs` values declared by the catalog's public `inputs` metadata. The server
  resolves configuration, rejects mixed custom configuration and recipe identity,
  and persists provenance. Recipe revision is not an upstream package version or
  a claim that a connection has been verified.
- Input destinations belong only to the server. Never substitute values into
  client-controlled config to retain trusted provenance. Missing/invalid inputs
  fail closed on the server, including requests from older clients.
- Input definitions control required-field validation and password masking;
  malformed definitions must fail schema parsing rather than silently vanish.
  Omitted/null definitions from older catalogs normalize to an empty list.
- Input values stay in dialog state, are retained on failure and cleared after
  save/reuse. Reset settled creation mutations and use `gcTime: 0` so secrets
  do not remain in inactive mutation variables. Never persist a database draft.
- Rename preserves source identity. Any full custom configuration replacement
  clears source/key/version. Existing rows are not matched or backfilled by name.
- Summaries remain write-only: no command, args, URL, env, headers or config
  enters the workspace/assignment query caches. Safe source metadata is allowed.
- Deployment responses expose transport and input metadata but never raw config
  or targets. Do not offer a custom-editor bypass for a deployment entry.
- Older catalogs missing source normalize to builtin; missing transport falls
  back to builtin config. Explicit unknown sources, malformed inputs and missing
  deployment versions are unusable. Old built-in catalogs without recipe version
  retain the existing custom editor. Keep advanced custom transport handling intact.
- Send `mcp_source_version=1` through core on every MCP summary request/response
  boundary. Older clients without it receive deployment provenance as null,
  avoiding a false match to a same-key built-in while retaining ordinary management.

## Deployment freshness

- Core owns query options scoped by API base URL, workspace and language. Read
  the base URL through the platform API, never `process.env` in core.
- Poll every 30 seconds only while the market/discovery catalog is visible.
  Refetch on entry, window focus, reconnect and explicit Deployment selection;
  inactive market types must not poll. Desktop mounts only its active tab, so
  switching desktop tabs disposes the query observers.
- Retained inactive panels disable their catalog, member and instance observers,
  so query invalidation and window focus cannot refresh a hidden market. Re-entry
  enables the existing stale query; do not add a second manual refetch effect.
- Deployment versions are opaque content hashes, not display version labels.
  Built-in numeric recipe revisions retain their existing meaning.
- `409 mcp_template_changed` requires refresh and explicit review of the new
  template; accepting reload clears old input values. A removed/invalid entry
  returns `409 mcp_template_unavailable` and cannot be saved from its old draft.
- `503 mcp_catalog_unavailable` is not withdrawal. Preserve cached data and draft
  on directory-level failure; first-load failure is an error, not an empty source.
  Input errors also preserve the draft for correction. A successful save pins its
  instance and assignment step through later catalog changes.
- Updating/removing files never rewrites saved configurations or assignments.
  Saved provenance comes from its stored summary, not a lookup in today's catalog.
  Empty-source copy asks the member to contact their deployment administrator;
  keep environment variables and mount instructions in deployment documentation.

## Flow and permissions

- Empty libraries stay in management and offer explicit market navigation.
  Opening setup retains its saved instance and assignment step through the first
  successful create and cache invalidation. Scope controllers by workspace.
- `McpMarketPanel({ workspaceId, active = true })` passes visibility to the
  catalog and portaled setup/custom dialogs. Type changes hide dialogs and stop
  polling while retaining edited fields and saved instance identity; an actual
  close or workspace unmount disposes the transient state.
- `McpSetupDialog` accepts either a recipe or an existing saved instance. It
  retains the saved ID and separates creation from explicit assignment. Skipping
  or closing after save leaves a reusable workspace instance.
- Assignment checkboxes start empty. Successful rows are retained; retry sends
  only unfinished selected IDs and never creates the configuration again.
- Disable and guard switching to an existing instance while creation is pending;
  otherwise a late create response can replace the intended assignment target.
- Existing disabled assignments stay disabled when reselected. Added, assigned,
  enabled and runtime-connected are different states. Do not display a green
  connection claim based only on a successful save or assignment.
- Human workspace owner/admin can create configurations. An agent owner can
  assign existing configurations without gaining workspace creation rights.
  Server authorization remains authoritative, including actor-type checks.
- The contextual existing-instance button explicitly names its target agent.
  Show already-assigned/disabled states and known name-override conflicts.
  Unsupported runtime context blocks assignment, not catalog browsing.
- Exclude archived and internal/system agents from assignment options. A failed
  or malformed agent list is an error with retry, not an empty successful list.

## Agent selection

- `McpAgentPicker` receives the full eligible directory and parent-controlled
  selection, confirmed successes and failures. Search and selected-only review
  are separate local projections: never replace the directory used to compute
  `pendingIds` with filtered results. Hidden selected IDs still submit; IDs that
  become archived, internal or absent on refresh do not count or submit.
- Category chips are a third local projection over that same eligible directory.
  Resolve each row with `resolveAgentDirectoryCategory`: a saved nonblank
  category wins, recognized `template_key` provenance supplies specialist or
  coordinator fallback, and an unrecognized/template-free row is general
  purpose. The chip inventory and counts are computed before search or
  selected-only filtering, include represented presets in
  `AGENT_CATEGORY_PRESET_ORDER`, and preserve the original custom category
  name. The All chip always counts eligible rows and no squad/system chip is
  added.
- Category, query and selected-only state compose only for rendering. Bulk
  select/deselect acts on visible unfinished rows; assignment still submits
  the parent-controlled selected IDs across hidden categories. A metadata
  refresh error keeps cached classifications and explicit categories usable;
  when template fallback is unresolved, preset counts/results are marked
  incomplete rather than reported as a definitive empty result. Retry must not
  clear category, query or selection state. Reset a category only after a
  complete directory/catalog result proves that category disappeared.
- The saved general picker may enable the existing role/squad template queries
  only when an eligible row has a template key and no explicit category.
  Preserve their locale-scoped query keys, stale time and cached data on
  refresh failure. Contextual single-agent discovery does not enable metadata
  queries or render category chrome.
- Search covers the full name and saved description. Reuse `descriptionPreview`
  only for the inert two-line display snippet; its 300-character cap must not
  truncate search. Use the query-free UI `ActorAvatar` with
  `resolvePublicFileUrl`, without directory/profile navigation or runtime joins.
- Select/Deselect results operate only on visible unfinished eligible IDs.
  Selecting results unions them with hidden choices; Clear selection removes
  unfinished choices without deleting confirmation/failure records. Selected
  counts intersect with eligible unfinished IDs; selected-only review can retain
  confirmed rows, which stay checked and disabled.
- Keep the search/selection toolbar and footer outside the list scroll owner.
  Global failure/result feedback stays visible; filtered-out per-agent failure
  details remain inside the bounded list scroll owner. Busy, permission and
  unsupported-runtime guards cover all selection controls and submission.
- Whole-row `Label` controls keep the checkbox named only by the agent name,
  with description/status wired through `aria-describedby`. Give each row a
  positioned ancestor (`relative`) so its absolute `sr-only` full description
  stays inside the list clip. Without it, long lists inflate the dialog's scroll
  extent and focus can scroll its heading/tools away. The visual Checkbox root
  does not contain its sibling hidden input or the row description.
- Do not combine `block` with `line-clamp-2`: overriding its required display
  makes the long-description preview expand beyond two lines. Verify dialog
  scroll offsets, heading/search/footer bounds and preview height after mouse,
  label and keyboard selection, not only the initial viewport.

## Error and visual contracts

- Preserve available catalog data on refresh failure; expose retry separately.
  Distinguish library error from empty and unknown provenance from no instances.
- Use shared dialog/tab/input primitives and role-based type tokens. Narrow
  dialogs scroll their content while keeping their actions reachable.
- Navigation and transport behavior is shared by Web/Desktop; no platform APIs
  belong in these views. No new global store is needed for an in-progress dialog.
- Keep a persistent result-count status region outside the template list; filtering
  must announce the count without moving focus or reading every card again.
- Template cards use the semantic foreground for their focus outline. The shared
  light `ring` token does not reach 3:1 against the card background; do not restore
  it here without verifying contrast. Preserve distinct icons for browser debugging
  and browser automation, with a visible configuration action on each card.
- State name-format restrictions before submission and keep the helper associated
  with its input when validation errors appear. Required input semantics must not
  bypass the existing localized validation flow.
- Move focus to the assignment heading when a configuration is saved or an existing
  instance is selected. The setup submit button disappears at that transition;
  without an explicit handoff, browser focus falls to `body` outside the dialog.
- The legacy built-in custom-editor handoff carries the catalog's live final-focus
  resolver. Closing returns to the original card, or catalog search if a refresh
  removed it. Inactive sessions suppress this handoff until they are visible again.
  Both the standalone market and `McpAgentDiscovery` must handle
  `onCustom(preset, finalFocus)` explicitly and forward the resolver to
  `McpServerDialog`. Passing `setPreset` directly drops the second argument;
  default dialog focus restoration only works while the opening card survives.
  The contextual regression must withdraw the template while editing and assert
  that closing focuses catalog search without creating a server.
- Existing-instance assignment identifies the pending server, shows progress on
  its button, and announces completion. Disabled controls alone are not progress
  feedback. Normal failure recovery still uses the shared mutation result.
- Coarse-pointer controls have at least 44px targets within each MCP surface,
  including portaled dialogs. Expand checkbox hit areas and labels together;
  retain compact desktop controls. Dialog headings and agent names wrap long words.
- The standalone MCP route uses `CollectionPageHeader` and the full dashboard
  canvas. `McpTab` selects page/settings presentation around the same workspace
  controller; do not duplicate mutations or nest the settings title inside the
  standalone page. The page owns scrolling and 16px/24px content gutters; settings
  and discovery dialogs retain their embedded scrolling owners.
- Use the named catalog container width to choose one, two (`@2xl`) or three
  (`@5xl`) columns, rather than viewport width: the same catalog also appears
  inside agent discovery dialogs. Template cards use skill-card typography and
  40px rounded-square Lucide tiles from the shared registry. Browser tools share
  the existing blue palette with distinct Bug/Workflow shapes; reasoning uses
  Brain with the existing purple palette. This is presentation, not skill-category
  metadata on MCP records.
- Market cards and template-sourced shared configurations use the same
  `common/mcp-template-icon.tsx` presentation, keyed by source + `template_key`, not the
  editable instance name. Renaming retains that icon; replacing the full config
  clears provenance and restores the transport icon. Custom/legacy entries
  without a template key keep their transport icon; deployment entries use a
  generic MCP/category icon, never builtin-specific icons by key. Transport text remains
  visible for every row. Do not fetch secret configuration to resolve an icon.
- `McpTemplateCatalog` owns the query shared by source counts, cards and setup
  freshness. The resource-management page does not mount a catalog observer.
  Independent agent discovery reuses the catalog wrapper. Source names stay stable
  for accessibility; counts exclude unusable templates, remain independent of
  filters, and are omitted while unknown. Show the filtered result count visibly
  in the existing live status region.

## Evidence

- `packages/views/mcp/*.test.tsx`: setup, cache updates, partial failure, reuse,
  member constraints, disabled bindings, malformed lists and pending-save race.
- `packages/core/workspace/mcp-market-mutations.test.tsx`: confirmed assignment
  responses, explicit workspace identity and failed-operation recovery.
- `server/internal/handler/workspace_mcp_template_test.go`: recipe validation,
  provenance lifecycle, write-only summaries and human permission gates.
- `e2e/mcp-market.spec.ts`: real API setup for all sixteen recipes, required/secret inputs, rename, resume,
  contextual reuse, partial failure and a member-owned agent; no real provider
  process or MCP tool execution is started by these tests.
- Category regression coverage lives in `packages/views/mcp/mcp-agent-picker.test.tsx`
  and `mcp-market.test.tsx`; template-query enablement/cache behavior is covered
  by `agents/create/use-role-templates.queries.test.tsx`. The production component
  fixture under `.omx/artifacts/mcp-agent-category-filters/` verifies English and
  Chinese, light and dark themes, wide/narrow/coarse-pointer layouts, long custom
  labels, independent list scrolling, hidden cross-category selections, scoped
  bulk operations, retry/failure recovery and zero browser errors.
