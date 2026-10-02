# MCP market and assignment

## Scope

The shared workspace MCP page and Settings MCP tab use `McpLibraryCatalog`.
The agent MCP tab opens `McpAgentDiscovery`, which shows existing workspace
instances before the same template catalog. The first catalog contains the
five reviewed keyless templates; it is not an external registry or plugin store.
The documentation category contains Microsoft Learn and DeepWiki remote HTTP
recipes. Show their outbound-network and public-content requirements before save;
do not describe their presence in the catalog as runtime connectivity. New
category labels ship in English/Chinese; key-based icons remain consistent for
market cards and saved instances. Publishing rules and the offline
`make check-mcp-catalog` gate are documented in `docs/mcp-catalog-publishing.md`.
The offline gate rejects input placeholders in commands, arguments and URLs,
including `$NAME` and Windows `%NAME%` as well as braced/angle forms. Keep those
fields on the same test-only detector; checking only `${NAME}` leaves unusable
recipes publishable even when they contain no credential keywords.

## Identity and configuration

- A template key identifies a recipe, never a user-editable instance name.
  Multiple instances may share `template_key` and retain distinct names/IDs.
- Trusted creation sends only name, template key and recipe version. The server
  resolves configuration, rejects mixed custom configuration and recipe identity,
  and persists provenance. Recipe revision is not an upstream package version or
  a claim that a connection has been verified.
- Rename preserves source identity. Any full custom configuration replacement
  clears both source fields. Existing rows are not matched or backfilled by name.
- Summaries remain write-only: no command, args, URL, env, headers or config
  enters the workspace/assignment query caches. Safe source metadata is allowed.
- Old catalogs without recipe version use the existing custom editor and do not
  acquire trusted source identity. Keep advanced custom transport handling intact.

## Flow and permissions

- A successfully loaded empty library can initialize the Market view. Explicit
  choice wins. Opening setup pins that view so the first successful create and
  cache invalidation cannot unmount the assignment step. Scope it by workspace.
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
  `common/mcp-template-icon.tsx` presentation, keyed by `template_key`, not the
  editable instance name. Renaming retains that icon; replacing the full config
  clears provenance and restores the transport icon. Custom/legacy entries
  without a template key keep their transport icon, and transport text remains
  visible for every row. Do not fetch secret configuration to resolve an icon.
- Keep the query owner above the library tabs so market counts and cards share
  one query result. Independent discovery mounts its own catalog query wrapper.
  Tab names stay stable for accessibility; inventory counts exclude unusable
  templates, remain independent of filters, and are omitted while unknown.
  Show the filtered result count visibly in the existing live status region.

## Evidence

- `packages/views/mcp/*.test.tsx`: setup, cache updates, partial failure, reuse,
  member constraints, disabled bindings, malformed lists and pending-save race.
- `packages/core/workspace/mcp-market-mutations.test.tsx`: confirmed assignment
  responses, explicit workspace identity and failed-operation recovery.
- `server/internal/handler/workspace_mcp_template_test.go`: recipe validation,
  provenance lifecycle, write-only summaries and human permission gates.
- `e2e/mcp-market.spec.ts`: real API setup for all five recipes, rename, resume,
  contextual reuse, partial failure and a member-owned agent; no real provider
  process or MCP tool execution is started by these tests.
