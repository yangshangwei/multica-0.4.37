# Publishing the MCP catalog

The MCP market combines platform built-in recipes and deployment-provided files.
Maintain built-ins in `server/internal/service/builtin_mcp_templates.go`; publish
deployment recipes as `<key>/mcp.json` in the API's configured directory.
Members discover recipes through
`GET /api/workspaces/{id}/mcp-servers/templates?language=en|zh`; human workspace
owners/admins save one using `name`, `template_source`, `template_key`, `template_version`, and
`template_inputs` when the recipe declares required fields.
The server resolves the configuration. Saving never assigns it to an agent.

## Publish deployment recipes

The first rollout requires the backend update, its database migration, and an
updated Web/Desktop client. Once deployed, adding, changing or removing files
needs no rebuild, restart, migration or new offline bundle. Changing a mount or
environment variable still requires recreating the API container.

Compose mounts `${MCP_TEMPLATE_DIRECTORY:-./mcp-templates}` read-only at
`/app/data/mcp-templates` and sets `MULTICA_MCP_TEMPLATE_DIR` to that container
path. Leave the container path at its default; set `MCP_TEMPLATE_DIRECTORY` in
`.env` to choose another host directory. For a non-container API, set
`MULTICA_MCP_TEMPLATE_DIR` directly to its local directory. The catalog is shared
by the deployment's workspaces; it is not a per-workspace upload area.

```text
mcp-templates/
  company-search/
    mcp.json
```

The directory belongs to the **API server**. A stdio command runs later on the
**agent machine**, which can be a different computer. Paths and dependencies
must exist there; publishing does not install packages, copy executables, probe
URLs, or start MCP programs. Remote services must be reachable from that runtime.

Example `company-search/mcp.json` (no real credentials):

```json
{
  "schema_version": 1,
  "titles": { "zh": "公司知识检索", "en": "Company Search" },
  "descriptions": { "zh": "检索内部知识库。", "en": "Search the internal knowledge base." },
  "category": "documentation",
  "documentation_url": "https://docs.example.internal/mcp",
  "requirements": {
    "zh": ["智能体所在机器能访问公司内网"],
    "en": ["The agent machine must have access to the internal network"]
  },
  "config": { "type": "http", "url": "https://search.example.internal/mcp" },
  "inputs": [
    {
      "key": "access_token",
      "labels": { "zh": "访问令牌", "en": "Access token" },
      "required": true,
      "secret": true,
      "target": { "kind": "header", "name": "Authorization", "prefix": "Bearer " }
    }
  ]
}
```

### File contract and inputs

- One first-level folder per template; its name is the key and must match
  `^[A-Za-z0-9_-]+$`. Do not add a `key` field to the manifest.
- `schema_version` must be `1`. Supply at least one nonempty `en`/`zh` title;
  display text falls back from the requested language to English, then Chinese.
  Descriptions and requirements are optional. Category is `browser`, `reasoning`,
  `documentation`, `coding`, `database` or `other` (default). An optional
  documentation URL must use HTTPS.
- Config supports stdio (`type: "stdio"`, a nonempty `command`, string `args`)
  or Streamable HTTP (`type: "http"`, a concrete `url`). URLs cannot contain
  userinfo, query or fragment. HTTPS can address internal hosts or IPs. Plain
  HTTP is rejected unless `MULTICA_MCP_TEMPLATE_ALLOW_HTTP=true` on the API;
  the default is false and the market shows this transport requirement.
- Declare credentials as user inputs. Static `env` and `headers` are forbidden.
  Never put credentials in titles, descriptions, requirements, commands, args,
  documentation links or service URLs; a parser cannot recognize every secret.
- Each input has a unique key, localized `labels`, optional `descriptions`,
  `required` and `secret` flags, an optional validator (`string`,
  `absolute_path`, `database_url`), and one fixed target. `env` uses `name` and
  applies only to stdio. `arg` optionally uses `flag` and appends a non-secret
  value to stdio args. `header` uses `name` and optional `prefix`, applies only
  to HTTP and defaults to secret. Credentials cannot be put in argv.
- Target names must be valid and unique. Unknown targets/inputs, incompatible
  transports, invalid values, NUL/newlines and oversized input values are
  rejected. The limits are 8192 bytes per value and 65536 bytes per input map.
  There is no shell expansion, dynamic command/URL, JSONPath or interpolation.
- Files are limited to 64 KiB each, 256 top-level entries and 4 MiB total reads
  per scan. Unknown fields/schema versions, duplicate JSON keys, trailing JSON,
  excessive nesting, symlinks, path escapes and non-regular files are rejected.
  Scripts, executable attachments and image assets are not distributed.

Deployment list responses contain display metadata, `source`, `version`,
`transport` and public input definitions. They never contain raw `config`, input
`target`, filesystem paths or credentials. The API resolves the manifest itself;
clients must not reconstruct it or fall back to custom config for these entries.

### Publish, refresh and recover

Write a temporary file in the template directory, then atomically rename it to
`mcp.json`. Mount the containing directory, not a single file or Kubernetes
`subPath`, so atomic replacement becomes visible. The API scans on every catalog
request. The visible MCP market/discovery catalog refreshes every 30 seconds;
reopening, focusing, reconnecting or selecting **Deployment** also refreshes it.
Web and Desktop share this behavior; no Electron directory scan is involved.

An empty/unconfigured/missing directory returns built-ins successfully. One
invalid manifest is skipped with a key/error-category log warning, while valid
entries remain available. Directory-level IO/permission or aggregate-limit
failures return `503 mcp_catalog_unavailable`: clients retain the last list and
draft, and offer retry. They must not treat this as a withdrawn template. Logs
do not print manifests or input values. Built-in creation and saved configuration
management do not depend on directory health. Multiple API replicas need the
same synchronized files; inconsistent replicas can reject a save as stale.

Identity is `(source, key)`, with source `builtin` or `deployment`. The same key
may exist in both sources without overriding either. Versions for deployment
recipes are opaque `sha256:<digest>` values computed from the validated,
normalized manifest, including display metadata. Whitespace/object-key order
and requested language do not change the version; array order and actual content
do. `schema_version` is the file format, not this content revision. Restoring
identical content restores its hash. It is not an upstream executable version.

Save the source, key and exact version returned by the catalog along with the
user's `template_inputs`; omit `config`. The API compares the current version
and resolves from that same validated snapshot. `409 mcp_template_changed`
means reload and review the new template (clear old inputs when accepting it).
`409 mcp_template_unavailable` means the file was removed or became invalid.
Neither permits saving the stale draft. A 503 or an ordinary input error retains
the draft for retry/correction. Updating or withdrawing a file leaves all saved
workspace configurations and assignments unchanged. Rename retains provenance;
replacing a saved config clears source/key/version. Disable or delete existing
assignments explicitly if runtime access must be revoked.

### Client compatibility

New clients send `mcp_source_version=1` on workspace MCP and agent-assignment
summary requests, including create/update responses. Without that capability,
deployment summaries keep their ordinary id/name/transport but return null
source/key/version, preventing old clients from matching them to a same-key
built-in. Older clients filter deployment catalog entries without `config`;
they can still manage saved entries as ordinary configurations. Omitting
`template_source` on creation selects built-ins only. New clients treat an
absent source on an older backend as built-in, but reject explicit unknown
sources, malformed inputs, or deployment entries without a version.

### Backend rollback boundary

Migration `512_workspace_mcp_template_source` deliberately retains the source
column on rollback to avoid losing saved provenance. Retaining that column alone
does not make an older API binary compatible with deployment instances: an old
backend does not implement source masking or `mcp_source_version` negotiation
and can expose a deployment key as if it identified a same-key built-in.

Once deployment instances have been saved, do not roll back to a backend without
the source-isolation changes. Keep those changes in the rollback build, or first
handle the affected instances through an explicitly reviewed rollback procedure.
Never silently relabel deployment instances as builtin to make a downgrade work.

## Current built-in catalog

| Key | Transport | Public source | Requirements |
| --- | --- | --- | --- |
| `chrome-devtools` | stdio | [Chrome DevTools MCP](https://github.com/ChromeDevTools/chrome-devtools-mcp) | Node.js/npx and Chrome on the agent runtime |
| `playwright` | stdio | [Playwright MCP](https://github.com/microsoft/playwright-mcp) | Node.js/npx and usable browser binaries |
| `sequential-thinking` | stdio | [MCP reference servers](https://github.com/modelcontextprotocol/servers/tree/main/src/sequentialthinking) | Node.js/npx on the agent runtime |
| `serena` | stdio | [Serena](https://github.com/oraios/serena) | uv/Python 3.13, language servers as needed, absolute project directory on the agent runtime |
| `codebase-memory` | stdio | [Codebase Memory MCP](https://github.com/DeusData/codebase-memory-mcp) | Native `codebase-memory-mcp` executable installed on the agent runtime's PATH |
| `repomix` | stdio | [Repomix](https://github.com/yamadashy/repomix) | Node.js 22+ and npx; local repository access |
| `markitdown` | stdio | [MarkItDown MCP](https://github.com/microsoft/markitdown/tree/main/packages/markitdown-mcp) | uv/Python and access to local documents |
| `dbhub` | stdio | [DBHub](https://github.com/bytebase/dbhub) | Node.js 22.5+ and npx; database URL supplied as `env.DSN` |
| `postgres-mcp` | stdio | [Postgres MCP Pro](https://github.com/crystaldba/postgres-mcp) | uv/Python 3.12+; database URL supplied as `env.DATABASE_URI`; restricted/read-only access |

Microsoft Learn and DeepWiki are excluded from the default catalog for intranet
use. Existing saved configurations and assignments are retained. The local
recipes may still need package downloads unless the runtime has a prepared cache.
These are launch recipes, not bundled third-party executables. Local means the
agent runtime's machine, which may differ from the browser or API host. The six
development recipes were reviewed against upstream documentation on 2026-10-04.

## Live smoke verification

On 2026-10-04 all six development servers were started and exercised through
MCP initialize, tools/list and tools/call against synthetic local fixtures.
Serena symbol/reference search, Codebase Memory indexing/call tracing, Repomix
packing/search, MarkItDown HTML/DOCX conversion and both database SQL queries
returned the expected fixture values. Postgres rejected the isolated write probe.

Postgres recipe revision 2 adds `uvx --with "mcp<2"`: the upstream package still
imports `mcp.server.fastmcp`, which is absent in MCP Python SDK 2.x. Do not remove
this runtime dependency constraint without repeating a real launch and query.
Revision-1 saved configurations are not silently rewritten; replace their config
or create a new copy from revision 2.

## Parameterized recipes

Catalog `inputs` contains only localized `key`, `label`, `description`, `required`
and `secret` metadata. Built-ins keep their public base `config` for client
compatibility; deployment entries do not expose it. Neither includes credentials.
Serena requires `project_path`; DBHub and Postgres require `database_url`.
The server validates supplied strings and injects them only into destinations
fixed by the recipe. Unknown, missing, malformed and oversized values are
rejected without echoing the submitted value. No shell interpolation is used.
Database URLs go into environment variables, not process command-line arguments.

The setup form masks secrets and retains a failed draft only in component state.
After a successful save or reuse it clears that draft; completed creation
mutations are reset and have zero garbage-collection retention. Responses and
query caches continue to contain only write-only summaries. This does not add a
credential vault or change the existing storage model for MCP configurations.

## Review and add a built-in recipe

1. Read the upstream's official documentation. Record its URL, review date,
   exact command or endpoint, supported transport, authentication requirements,
   public/private content boundary and runtime prerequisites in the task or PR.
   Prefer official maintained services over third-party proxies.
2. Declare any required values through reviewed input definitions with fixed
   server-side argument/environment destinations. Never put real credentials,
   example credentials or interpolation placeholders in public config. Add
   validation/resolution tests and mark sensitive input metadata `secret: true`.
   New authorization mechanisms such as OAuth need their own implementation.
3. Add a stable name-safe key, recipe `Version: "1"`, supported category,
   HTTPS documentation URL, and explicit `en`/`zh` titles, descriptions and
   requirements. Keys are identity, not display names. Append new entries in
   product order. Browser keys/package arguments are also used by the Windows
   runtime fallback: preserve them unless that code is updated and tested.
4. Supply exactly one target. Current stdio recipes contain `command` and a
   explicit string argument list (empty for a native program that needs none);
   optional `type` can only be `stdio`. Remote
   recipes contain only `type: "http"` and a concrete HTTPS `url`, with no URL
   credentials, query parameters or fragment. No input placeholders are allowed
   in commands, arguments or URLs, including `$NAME`, `${NAME}`, `%NAME%`,
   `{{NAME}}` and `<name>`.
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

## Release, revise and withdraw built-ins

1. Include official-source evidence, check results and changed roster keys in
   the reviewed change. Update the built-in creating-agents skill and its source
   map if documented behavior changes.
2. Ship through the repository's existing backend release/deployment process.
   Changing a built-in roster alone requires no database migration or catalog
   publishing API. Include the
   shared Web/Desktop changes when adding labels/icons. Installed older clients
   can display new recipes with fallback icons, but cannot collect new required
   inputs; the API rejects those incomplete saves. Ship the shared client update
   with parameterized recipes. Existing no-input recipes remain compatible.
3. After deployment, fetch the catalog as a member in both languages and verify
   the current keys, versions and requirements. The template query cache has a
   visible 30-second refresh interval: reload the client or explicitly refetch/invalidate it
   when verifying the newly deployed content. A previous cached response is not
   proof that the new binary contains the old catalog.
4. To change a recipe's command, arguments, endpoint or transport, increment its
   recipe version and update the content assertions/evidence. Keep its stable
   key if it still represents the same service. Display-copy-only corrections
   need no version bump. A recipe version is neither an executable package
   version nor the version of a hosted service; launcher packages are not pinned
   by the recipe version.
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
registry, or rotate credentials. Those are separate product capabilities.
