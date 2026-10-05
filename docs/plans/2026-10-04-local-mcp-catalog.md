# Local development MCP catalog implementation plan

**Goal:** Add Serena, Codebase Memory MCP, Repomix, MarkItDown, DBHub and Postgres MCP Pro to the shared Web/Desktop MCP market.

**Architecture:** Extend the compiled catalog and the existing trusted key/version creation flow. Public input metadata describes project-directory and database-connection fields; only the server maps submitted values to fixed argv/env destinations. Existing write-only summaries, provenance, permissions and explicit agent assignment remain authoritative. No migrations or application dependencies are required.

**Tech stack:** Go, React, TypeScript, Zod, TanStack Query, existing shared UI components.

## Contract

- Keep the existing three local recipes and the withdrawal of remote documentation recipes.
- Add `coding` and `database` categories with English/Chinese labels and stable key-based icons.
- Catalog `inputs`: `{key, label, description, required, secret}[]`; absent/null means no inputs for older servers.
- Trusted POST adds optional `template_inputs: Record<string, string>`. Serena requires `project_path`; DBHub and Postgres require secret `database_url`.
- Reject unknown, missing, oversized, malformed inputs and mixed custom config/recipe requests. Never interpolate a shell string or expose values in errors/list responses.
- Collect values only in the setup dialog, mask secrets, validate required fields, preserve failed drafts, clear values after save/reuse, and remove completed mutation variables.
- Native Codebase Memory requires a prepared executable on the agent runtime. Package launchers may download on first use; intranet runtimes need prepared installations/caches.

## Execution and verification

1. Add failing service/API tests for the roster, inputs, credential-free catalog, runtime config, provenance, permissions and write-only summaries. Implement the backend in its isolated file ownership lane.
2. Add failing core parsing/request tests and shared dialog tests. Extend types, schema, client/mutation, fields, categories, and icons without platform-specific code.
3. Update publishing guidance, MCP surface contract and built-in agent API documentation, preserving pre-existing edits.
4. Run focused Go/TS regressions, catalog publishing check, lint/typecheck and static checks. Verify the actual market and input form in the browser when the local environment permits.
5. Review the complete diff against the starting dirty state. Do not stage or commit unrelated user work.

## Acceptance

- Nine local recipes appear in the market and six new entries are searchable/filterable.
- Database/project inputs are required before saving and become only the server-designated configuration values.
- Saved entries retain their template icons and source identity; save does not silently assign an agent.
- Runtime prerequisites are visible; neither saving nor a catalog listing claims a successful MCP connection.
