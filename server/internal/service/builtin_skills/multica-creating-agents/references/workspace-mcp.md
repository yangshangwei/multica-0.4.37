# Workspace MCP servers

A workspace keeps a LIBRARY of MCP servers (workspace Settings → MCP, or
`multica workspace mcp list|add|update|remove`). Adding one there gives it to
NO agent — same shape as a workspace skill. It reaches an agent only when
someone assigns it:

```bash
multica workspace mcp list --output table        # find the server id
multica agent mcp add <agent-id> <server-id>     # give it to one agent
multica agent mcp disable <agent-id> <server-id> # stop sending it, keep the assignment
multica agent mcp remove <agent-id> <server-id>  # take it away
```

At claim time the effective set is:

| Layer | Reaches the agent when |
| --- | --- |
| runtime-local servers | always (the daemon merges the runtime's own file) |
| workspace servers | assigned to THIS agent and left enabled |
| the agent's own `mcp_config` | always; it WINS on a name collision |

Two consequences worth knowing before writing an agent's config: assigning a
shared server does not require re-listing it in `mcp_config` (they merge), and
`mcp_config` is now only about servers private to that agent — a
managed-but-empty `{}` no longer means anything about the workspace layer,
because nothing is inherited in the first place.

The stored entry is **write-only** — reads return the server's identity, name,
transport and optional template source, never urls, commands, headers, or env,
for any role. Human owners/admins can create a built-in or deployment recipe through
`POST /api/workspaces/{id}/mcp-servers` with `name`, `template_source`, `template_key` and
`template_version`, omitting `config`. Recipes declare public `inputs` metadata;
submit the required values in `template_inputs` (Serena: `project_path`;
DBHub/Postgres MCP Pro: `database_url`). The server validates these inputs and
maps them to fixed arguments/environment variables or HTTP headers; values are never included
in list/create summaries or validation errors. The server resolves the current recipe;
unknown or outdated versions and simultaneous custom config are rejected.
Identity is source + key: builtin and deployment recipes with the same key are
distinct. Omitting `template_source` selects builtin only. Renaming preserves
source/key/version; replacing config clears all three. Built-in revisions and
deployment content hashes identify the recipe, not a pinned executable release.
Creation assigns no agents. A human agent owner or workspace owner/admin must
explicitly assign the entry; agent actors cannot make either write.

The default catalog includes Chrome DevTools, Playwright, Sequential Thinking,
Serena, Codebase Memory MCP, Repomix, MarkItDown, DBHub, Postgres MCP Pro,
GitLab MCP, Atlassian MCP, Grafana MCP, Kubernetes MCP, MongoDB MCP, Redis MCP,
and ClickHouse MCP. The seven intranet recipes launch preinstalled executables,
without downloading packages. Read their declared inputs before saving; Atlassian
needs at least one complete Jira/Confluence URL/PAT pair, Redis credentials are
separate from its address, and ClickHouse expects an HTTP(S) endpoint. Redis
permissions rely on the configured ACL account; it has no MCP read-only switch.
These are local launch recipes: their runtime dependencies must be available on
the agent's machine. Postgres uses restricted/read-only access by default.
Microsoft Learn and DeepWiki are excluded for intranet use; fetch current recipe versions from the catalog.
Removing recipes blocks new template creation but retains saved configurations and explicit assignments.
Saving or assigning a recipe does not verify connectivity.

Deployment administrators can publish `<key>/mcp.json` in the API server's
`MULTICA_MCP_TEMPLATE_DIR`. After the initial server/database and client upgrade,
file changes appear in the market's Deployment source on refresh without a
restart. This is not an upload to the agent machine: commands, dependencies,
paths and network access must be prepared on its runtime separately. Deployment
catalog responses expose metadata/transport/inputs but no raw config or input
targets; never reconstruct a template in the client or fall back to custom
config to bypass its checks. See `docs/mcp-catalog-publishing.md` for the format.

Fetch the current version before creation. `409 mcp_template_changed` means
reload and review the changed template; `409 mcp_template_unavailable` means it
was removed or became invalid. `503 mcp_catalog_unavailable` is a directory
failure: retain the draft and retry, rather than concluding the recipe was
withdrawn. Already saved configurations and assignments remain available.
Clients requesting full provenance send `mcp_source_version=1` on MCP summary
endpoints; without it deployment source/key/version are null, so older clients
manage the saved entry as an ordinary configuration without confusing it with
a same-key builtin.

