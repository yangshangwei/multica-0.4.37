# MCP catalog publishing: recovery notes

Inspected on 2026-10-02 at `aa0d3ed83`. This records repository evidence,
not approval of a new publishing model.

## Task state

The task was created on 2026-10-01, but `prd.md` contains only TBD placeholders,
`task.json` is still `planning`, and both context manifests contain only a seed
row. The user's request to continue authorizes advancing this task; it does not
identify which of the materially different publishing models is intended.

A scope question was sent on 2026-10-02: curated official catalog expansion,
workspace-user publication to a public market, or external registry discovery.
The user subsequently chose curated official catalog expansion; see the updated PRD.

## Existing behavior

| Concern | Evidence |
| --- | --- |
| Source | `server/internal/service/builtin_mcp_templates.go`: product-ordered, embedded recipes, shipped with the server binary |
| Current roster | `chrome-devtools`, `playwright`, `sequential-thinking`; all keyless stdio recipes |
| Metadata | Stable key, recipe version, category, documentation URL, runtime requirements, English/Chinese titles and descriptions |
| Discovery | `server/internal/handler/mcp_template.go`: member-visible `GET /api/workspaces/{id}/mcp-servers/templates?language=` |
| Trusted creation | `server/internal/handler/workspace_mcp_api.go`: accepts name/key/version, resolves config on the server, rejects mixed custom config and recipe identity |
| Configuration validation | `server/internal/handler/workspace_mcp.go`: name and individual MCP config validation |
| Provenance | Rename preserves key/version; full configuration replacement clears both; list responses never expose instance configuration |
| API parsing | `packages/core/api/schemas.ts`, `packages/core/types/mcp-template.ts`, `packages/core/api/client.ts` |
| Shared UI | `packages/views/mcp/mcp-market.tsx`, `mcp-setup-dialog.tsx`, `mcp-agent-discovery.tsx` |
| Assignment | Explicit selection after save; no automatic assignment; partial retries do not recreate the configuration |
| Runtime | `server/internal/daemon/runtime_mcp.go`; Windows browser handling in `server/pkg/agent/browser_mcp_config.go` depends on browser keys/package tokens |

The existing marketplace is functional, not a static mockup. Adding another
usable keyless recipe does not inherently require a new database table or API.

## Preserved product decisions

- `.trellis/tasks/archive/2026-09/09-29-mcp-market/prd.md` approved the existing three recipes.
  OAuth, connection probes, upgrade UI, package pinning/verified badges, external
  registries, plugin authorization changes, and mobile parity were deferred.
- `.trellis/workspace/artisan/journal-1.md`, Session 23, records that Context7
  was removed at the user's request. Do not quietly add it back.
- `.trellis/tasks/archive/2026-09/09-19-builtin-mcp-presets/prd.md` excludes credential and
  required-parameter recipes, including filesystem roots, until their input
  flow is designed. Its older no-provenance/four-language decisions were
  superseded by the market task and current conventions.
- `.trellis/spec/views/frontend/mcp-market.md` distinguishes saved, assigned,
  enabled, and connected; recipe version is not upstream package version.
- Current recipes use two `@latest` package references and one unversioned
  package reference. Do not describe these as pinned or runtime-verified.
- Root `CLAUDE.md` requires current English/Chinese copy, API compatibility,
  write-only configuration boundaries, and no new dependencies without request.

## Implementation choices to resolve

| Direction | Concrete work | Tradeoff |
| --- | --- | --- |
| Curated official catalog, recommended starting scope | Verify candidate upstream configuration and requirements; add eligible recipes and metadata; strengthen roster validation; document contribution, recipe changes, release and withdrawal; update existing tests | Reuses the working flow; new content still follows backend deployment |
| User publication to a public market | Define publisher permissions, visibility, moderation, public metadata versus private credentials, versioning and withdrawal before designing storage/API | Enables contributions but adds a new cross-workspace trust boundary |
| External registry discovery | Choose registry/protocol, normalization, supported package/transports, cache/sync and error behavior, plus explicit configuration/assignment boundaries | Wider discovery, with external availability and compatibility dependencies |

If curated expansion is selected, determine the candidate roster from verified
official sources and the existing no-credential/no-required-input boundary.
Do not invent tokens, filesystem paths, or success claims to make a recipe fit.

## Verification map

- `server/internal/service/builtin_mcp_templates_test.go`: unique/valid keys,
  configuration target, localization fallback and no credentials.
- `server/internal/handler/mcp_template_test.go`: catalog response and language.
- `server/internal/handler/workspace_mcp_template_test.go`: metadata, trusted
  creation, provenance lifecycle, malformed recipes and human permissions.
  Metadata tests currently hardcode three entries, recipe `1`, and two categories;
  adjust intentionally if the approved roster changes.
- `packages/core/api/mcp-template-schemas.test.ts` and
  `mcp-market-client.test.ts`: malformed payloads and API calls.
- `packages/views/mcp/*.test.tsx`: discovery, setup, permissions and retries.
- `e2e/mcp-market.spec.ts` and `e2e/mcp-desktop.spec.ts`: existing Web/Desktop
  integration flows. They do not execute real provider processes or tools.

Focused Go baseline command, from `server/`:

```sh
bash ../scripts/go-test-with-agent-cli-guard.sh -- go test ./internal/service ./internal/handler -run '^(TestMcpServerTemplates_|TestListMcpServerTemplates_|TestMcpMarketCatalogMetadata$)' -count=1 -v
```

Database-backed lifecycle and live provider checks are separate from this
catalog baseline. Report skips and external limitations explicitly.

Baseline executed on 2026-10-02: all seven selected Go tests passed (three
service roster tests and four handler response/metadata tests); no selected
test skipped. This validates the existing embedded catalog only. No new recipe,
database lifecycle, browser, or real MCP provider was exercised in this recovery.

## Documentation drift to repair with implementation

`server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md`
still describes catalog creation as prefilling a form without provenance.
Update it alongside the final catalog implementation so it describes the
existing trusted key/version flow accurately.

## Verification caveat discovered during implementation

The handler package TestMain connects to PostgreSQL and can exit 0 before m.Run
when it is unreachable. The original catalog tests are individually DB-free,
but their package process is not. The new make check-mcp-catalog gate therefore
runs service tests only. Handler lifecycle checks use an isolated migrated DB
and inspect named JSON test events to ensure execution and reject skips.
