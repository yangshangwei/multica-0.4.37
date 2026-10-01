# Publishing the curated MCP catalog

The MCP market is a reviewed list of configuration recipes embedded in the API
binary. Maintain it in `server/internal/service/builtin_mcp_templates.go`.
Members discover recipes through
`GET /api/workspaces/{id}/mcp-servers/templates?language=en|zh`; human workspace
owners/admins save one using `name`, `template_key`, and `template_version`.
The server resolves the configuration. Saving never assigns it to an agent.

## Current catalog

| Key | Transport | Public source | Requirements |
| --- | --- | --- | --- |
| `chrome-devtools` | stdio | [Chrome DevTools MCP](https://github.com/ChromeDevTools/chrome-devtools-mcp) | Node.js/npx and Chrome on the agent runtime |
| `playwright` | stdio | [Playwright MCP](https://github.com/microsoft/playwright-mcp) | Node.js/npx and usable browser binaries |
| `sequential-thinking` | stdio | [MCP reference servers](https://github.com/modelcontextprotocol/servers/tree/main/src/sequentialthinking) | Node.js/npx on the agent runtime |
| `microsoft-learn` | Streamable HTTP | [Microsoft Learn MCP](https://learn.microsoft.com/en-us/training/support/mcp) | Runtime support and network access to `learn.microsoft.com`; public documentation, not training or personal profiles |
| `deepwiki` | Streamable HTTP | [DeepWiki MCP](https://docs.devin.ai/work-with-devin/deepwiki-mcp) | Runtime support and network access to `mcp.deepwiki.com`; indexed public GitHub repositories, not private repositories |

The two remote recipes require no API key and install no local package. They
need outbound internet access; their presence in an offline deployment's market
does not make those endpoints reachable. The existing local recipes may also
need package downloads unless the runtime has a prepared cache.

## Review and add a recipe

1. Read the upstream's official documentation. Record its URL, review date,
   exact command or endpoint, supported transport, authentication requirements,
   public/private content boundary and runtime prerequisites in the task or PR.
   Prefer official maintained services over third-party proxies.
2. Check that the recipe works without credentials or user-supplied parameters.
   Tokens, headers, environment secrets, placeholder keys, filesystem roots,
   database URLs and OAuth flows need a separate input/security design; the
   current catalog cannot collect them. Do not remove the publishing assertions
   just to make such an entry pass.
3. Add a stable name-safe key, recipe `Version: "1"`, supported category,
   HTTPS documentation URL, and explicit `en`/`zh` titles, descriptions and
   requirements. Keys are identity, not display names. Append new entries in
   product order. Browser keys/package arguments are also used by the Windows
   runtime fallback: preserve them unless that code is updated and tested.
4. Supply exactly one target. Current stdio recipes contain `command` and a
   non-empty string argument list; optional `type` can only be `stdio`. Remote
   recipes contain only `type: "http"` and a concrete HTTPS `url`, with no URL
   credentials, query parameters or fragment. No input placeholders are allowed.
5. If adding a category, update its English/Chinese shared UI label and the
   publishing category assertion together. Add any template-specific icon by
   key in `packages/views/common/mcp-template-icon.tsx`, so renamed saved copies
   keep the same icon without exposing their configuration.
6. Verify public endpoints separately when network access is available. Use
   anonymous MCP `initialize` and `tools/list` with the protocol's Accept header;
   an ordinary browser GET can legitimately return 405. Record the result and
   date. Do not run tool calls, submit repositories for indexing, use private
   credentials, or install agent/provider CLIs as part of the default gate.
   Discover live tools dynamically: hosted providers can change tool names.

Microsoft Learn's exact HTTP config is documented in its
[developer reference](https://learn.microsoft.com/en-us/training/support/mcp-developer-reference).
DeepWiki recommends `/mcp`; its legacy `/sse` endpoint is deprecated. Point-in-time
checks do not warrant a “verified connection” badge or an uptime promise.

## Run the publishing checks

From the repository root:

```sh
make check-mcp-catalog
```

This runs the normal service roster tests under the agent CLI guard. It checks
unique/name-safe keys, positive recipe versions, explicit bilingual metadata,
supported categories, concrete HTTPS URLs, transport/configuration shape, no
credentials and no unfilled parameters. It needs the Go dependencies, but no
database, provider network call or user-installed agent CLI. These tests also
run in the existing backend CI job.

Then run the changed frontend tests and checks:

```sh
pnpm --filter @multica/views test mcp/ settings/components/mcp-tab.test.tsx locales/mcp.test.ts
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views lint
```

For API lifecycle evidence, use an isolated, migrated PostgreSQL test database
and run the handler suite with `DATABASE_URL` set to that database:

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/handler -run '^(TestMcpMarket|TestListMcpServerTemplates_)' -count=1 -v
```

Check that the named tests actually run: the handler package's existing
`TestMain` can exit successfully when no database is reachable. The offline
publishing gate deliberately excludes that package. Test trusted creation,
HTTP/stdio preservation, write-only summaries, no implicit assignment, explicit
assignment, rename, custom replacement, stale versions and authorization.
Run the Web and Electron MCP suites against freshly built applications and the
same task-owned API. Browser saves are not live provider execution.

## Release, revise and withdraw

1. Include official-source evidence, check results and changed roster keys in
   the reviewed change. Update the built-in creating-agents skill and its source
   map if documented behavior changes.
2. Ship through the repository's existing backend release/deployment process.
   No database migration or catalog publishing API is involved. Include the
   shared Web/Desktop changes when adding labels/icons. Installed older clients
   can still display/save new recipes using the existing schema, though they
   may show a fallback icon and no new category filter.
3. After deployment, fetch the catalog as a member in both languages and verify
   the current keys, versions and requirements. The template query cache has a
   freshness interval: reload the client or explicitly refetch/invalidate it
   when verifying the newly deployed content. A previous cached response is not
   proof that the new binary contains the old catalog.
4. To change a recipe's command, arguments, endpoint or transport, increment its
   recipe version and update the content assertions/evidence. Keep its stable
   key if it still represents the same service. Display-copy-only corrections
   need no version bump. A recipe version is neither an executable package
   version nor the version of a hosted service; the three existing local
   recipes are not package-pinned.
5. New creates with an old/unknown key or version are rejected. Refresh the
   catalog and reopen setup. Already saved configurations retain their original
   bytes, source version and assignments; this process does not auto-upgrade them.
6. To withdraw a recipe, remove it from the roster, update its explicit content
   assertions and release the backend. Keep icon support for saved instances.
   A stale client's trusted create will be rejected; existing instances and
   bindings are unaffected. Withdrawal is not runtime revocation. If a service
   becomes unsafe, workspace admins must separately disable/remove assignments
   or replace/delete their configurations.
7. Roll back through the normal reviewed code/backend release flow. Keep recipe
   revisions monotonic: restoring previous configuration bytes after a published
   revision should use a new revision number. Do not reuse an old revision for
   different bytes. Existing saved copies still require explicit admin action.

The catalog does not accept public user submissions, synchronize an external
registry, or manage credentials. Those are separate product capabilities.
