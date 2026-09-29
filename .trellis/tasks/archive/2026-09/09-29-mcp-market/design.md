# Design

Preserve current semantic dark/light tokens, skill-market tab/card/search idioms, accessible shared dialogs. Operate mode: clear task flow over decorative UI. No global CSS or design-system changes.

## Backend/core contract
- Extend catalog with version (recipe revision, initially "1"), category (browser/reasoning), requirements (localized string[]), documentation_url. Existing key/title/description/config stay compatible.
- Add nullable template_key/template_version to workspace_mcp_server, no FK or index. Server resolves recipe via existing POST /workspaces/{id}/mcp-servers accepting name+template_key+template_version without config. Reject simultaneous arbitrary config and template fields; unknown/stale key/version rejected. Existing custom POST stays compatible.
- Summary exposes template_key/template_version only; rename retains and any config replacement clears both. Agent-scoped summary carries identity too.
- TS McpServerTemplate uses optional version/category/requirements/documentationUrl; schema maps documentation_url. WorkspaceMcpServer uses optional template_key/template_version matching existing snake-case type. Add api.createWorkspaceMcpServerFromTemplate(wsId,name,templateKey,templateVersion), hook useCreateWorkspaceMcpServerFromTemplate(wsId) with {name,templateKey,templateVersion}. Existing custom creation untouched.
- Reuse existing assignment endpoints; frontend compose useMutation/useQuery in headless core helpers if needed, no direct server mirrors in stores.

## UI
Market/workspace tabs with explicit selection; empty successfully loaded library defaults market. Cards/details for three templates, category filters/browser vs reasoning. New guided flow uses trustworthy recipe creation for current catalog, custom JSON uses existing dialog and ordinary create. Old catalog missing recipe support offers ordinary custom flow, no false attribution.
Saved instance ID retained during assignment; explicit checkboxes, per-agent success/error, retry only unfinished choices. Assignment POST idempotent but disabled existing binding must not silently enable. Closing/skipping never deletes saved instance. After save, no back action that creates a duplicate.
C uses the same browser/setup inside agent MCP view, existing instances first. Agent owner permission differs from workspace create; current member/user evidence controls actions with service as authority. Only visible/eligible agents and authorized summaries exposed. Runtime unsupported/unknown states disclosed; do not equate provider support with tested availability.

## Verification
Use failing tests for contract and flow changes. Source tests cover provenance/permission/version/write-only boundary; component tests cover no duplicate creation after partial failure, skip, agent context, member permissions and API errors. Existing MCP dialog/agent suites protect manual paths. Browser use isolated local environment with actual API and disposable fixture agents, no real agent execution.
