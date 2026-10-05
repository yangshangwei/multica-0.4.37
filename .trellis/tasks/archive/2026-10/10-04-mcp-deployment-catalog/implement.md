# Implementation and integration plan

## Ownership

1. Service lane: catalog loader, bounded JSON validation, content version, typed inputs, service tests and sample manifest. Own server/internal/service/mcp_catalog*, mcp_template_dir*, builtin_mcp_inputs*, and the McpServerTemplate struct additions only in builtin_mcp_templates.go; examples/mcp-templates/.
2. API lane: handler injection and request/response flow, migration, SQL/sqlc, source capability negotiation, handler tests. Own server/internal/handler MCP files + handler.go field/wiring only, server/pkg/db/queries/workspace_mcp.sql, related generated files, new migration.
3. Core lane: source/transport schemas/types, source capability query parameters, request identity checks, dynamic query options and tests. Own packages/core/ relevant MCP edits.
4. Views lane: source filters, visibility-aware refresh, typed failure recovery, provenance/icons, localization and component tests. Own packages/views/ relevant MCP files. Depend on core contract below.
5. Leader: task lifecycle, isolated baseline, docs/deployment wiring, E2E evidence, integration and independent reviews. Subagents must not commit, recurse or alter other lanes.

## Shared contracts

- Service: McpCatalog{Directory string, AllowHTTP bool}; List() ([]McpServerTemplate,error); Resolve(source,key,version string,inputs map[string]string) (map[string]any,error).
- Service McpServerTemplate adds Source and Transport. Builtins normalized to source=builtin; current builtin content preserved.
- Export sentinel errors ErrMcpCatalogUnavailable, ErrMcpTemplateChanged, ErrMcpTemplateUnavailable for errors.Is and status mapping. Unknown source/bad input remain ordinary validation errors. Builtin legacy version validation retains current behavior.
- API catalog wire adds source, transport; deployment config omitted (never redacted placeholders). Existing builtin config preserved.
- Template version is opaque nonempty string, deployment sha256:<hex>. Key remains name-safe; identity is (source,key).
- Workspace summary adds template_source; source/key/version all cleared on custom replacement. Missing source on old server with a template key means builtin.
- mcp_source_version=1 on every MCP summary HTTP request. Responses without capability mask deployment source/key/version to null; IDs/names/transports remain available.
- Request adds template_source; core mutation input templateSource. Builtin omission defaults builtin; deployment must explicitly name source.
- Core query helper mcpServerTemplateListOptions(wsId, language, {poll?:boolean}) uses base URL + workspace + language key and visibility-aware 30s refresh; view hook remains thin locale wrapper.
- Do not expose long SHA versions in product copy. No-input deployment is valid without config on client.

## Sequence

Write failing behavioral tests for each owned boundary, observe failure, implement and run focused tests. Service/core contract work can proceed independently. API integrates service; views integrates core. Do not widen scope silently. Review spec compliance then code quality and fix findings. Run merged checks and real Web/Electron acceptance after integration.

## Verification

Use design.md section 9 plus package spec quality requirements. Save results to verification.md. Baseline tests identify pre-existing failures; do not erase unrelated changes to make checks pass. Commit only this task's delta if it can be isolated from pre-existing work; never include other sessions' WIP.
