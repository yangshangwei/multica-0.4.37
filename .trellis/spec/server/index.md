# Server Development Guidelines

> Guidelines for the Go backend (`server/`).

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Security Boundaries](./security-boundaries.md) | Credential minting gates and automation authorization principals | Active |
| [Built-in Template Registries](./builtin-templates.md) | Autopilot template content contracts: `{{date}}` issue titles, server-decided fields | Active |
| [Project execution squads](./project-execution-squad.md) | Project setup, authority, atomic materialization and UI recovery contracts | Active |
| [Local directory snapshots](./local-directory-snapshots.md) | Git index timestamps, private-index fallback and user-work preservation | Active |

## See Also

- `CLAUDE.md` (repo root) — Backend UUID rules, migration rules, testing rules
- `server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md` — the agent-behavior contract table, one row per enforced boundary
