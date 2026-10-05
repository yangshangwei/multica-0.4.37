# Intranet MCP templates implementation plan

**Goal:** Add GitLab, Atlassian, Grafana, Kubernetes, MongoDB, Redis and ClickHouse as seven built-in MCP recipes, bringing the catalog to sixteen entries.

**Architecture:** Reuse the existing public input metadata, server-owned configuration mappings, write-only workspace configuration and explicit assignment flow. Keep the separate deployment-directory feature out of this change. New recipes launch preinstalled local executables rather than downloading packages at runtime.

**Scope and contracts**

- Preserve the existing nine recipes, including Postgres recipe revision 2.
- Add collaboration and operations categories, bilingual labels/requirements and stable template icons.
- Inputs cover internal service addresses, secrets and runtime-local configuration paths. Backend validation permits HTTP for internal deployments while rejecting URL userinfo/fragments, control characters, unsupported connection schemes and invalid paths.
- Atlassian supports Jira only, Confluence only or both, with matching self-hosted PAT credentials. Do not require unrelated services.
- Use upstream-supported read-only and telemetry/documentation controls where available. Do not claim a read-only mode or offline guarantee without confirming the actual configuration.
- Never embed credentials or shell interpolation. Hostnames and paths refer to the agent machine/network, not the API server.
- No new application dependencies, migrations, credential vault, runtime installer or automatic assignment.

**Execution**

1. Verify current upstream executable names, input environment variables and offline prerequisites.
2. Add failing catalog/resolver tests, implement the seven recipes and necessary validation, then verify service/API persistence and credential-free summaries.
3. Update the shared categories/icons and tests. Reuse the existing masked input form; avoid protocol changes unless required by an actual input shape.
4. Extend Web/Electron market fixtures for sixteen entries and parameterized inputs. Update publishing and setup documentation.
5. Run catalog tests, Go vet and isolated handler tests, relevant TS tests/typecheck/lint, and production UI checks where the environment permits.

**Validation boundary:** User-owned GitLab/Jira/Grafana/clusters/databases and credentials have not been supplied. Saving recipes and safe local protocol fixtures must not be described as successful connections to those real services. Intranet execution requires transferring supported executables and complete dependencies to the agent machines first.
