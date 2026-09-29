# MCP market and assignment

## Scope

The shared workspace MCP page and Settings MCP tab use `McpLibraryCatalog`.
The agent MCP tab opens `McpAgentDiscovery`, which shows existing workspace
instances before the same template catalog. The first catalog contains the
three existing keyless templates; it is not an external registry or plugin store.

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

## Evidence

- `packages/views/mcp/*.test.tsx`: setup, cache updates, partial failure, reuse,
  member constraints, disabled bindings, malformed lists and pending-save race.
- `packages/core/workspace/mcp-market-mutations.test.tsx`: confirmed assignment
  responses, explicit workspace identity and failed-operation recovery.
- `server/internal/handler/workspace_mcp_template_test.go`: recipe validation,
  provenance lifecycle, write-only summaries and human permission gates.
- `e2e/mcp-market.spec.ts`: real API setup for all three recipes, rename, resume,
  contextual reuse, partial failure and a member-owned agent; no real provider
  process or MCP tool execution is started by these tests.
