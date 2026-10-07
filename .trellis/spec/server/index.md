# Server Development Guidelines

> Guidelines for the Go backend (`server/`).

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Security Boundaries](./security-boundaries.md) | Credential minting gates and automation authorization principals | Active |
| [Offline delivery](./offline-delivery.md) | Configuration forwarding, preserved overlays and upgrade verification | Active |
| [Triage Admission](./triage.md) | Human review, inert intake, execution fences, CSV replay and retained audit data | Active |
| [Iteration facts](./iterations.md) | Borrowed recorder, W01 lock batching, retry identity and current I1 limits | Active |
| [Platform Observability](./platform-observability.md) | Metric windows, queue clocks, leased alerts, health evidence and audit projections | Active |
| [Managed Installations](./managed-installations.md) | Signed enrollment, scoped runtime authority, recovery and provenance | Active |
| [Creator Recommendations](./creator-recommendations.md) | Human-only, evidence-grounded LLM advice and invocation boundaries | Active |
| [Resource publishing](./resource-publishing.md) | Managed uploads, atomic catalog revisions, authorization and replay | Active |
| [Built-in Template Registries](./builtin-templates.md) | Autopilot template content contracts: `{{date}}` issue titles, server-decided fields | Active |
| [Project execution squads](./project-execution-squad.md) | Project setup, authority, atomic materialization and UI recovery contracts | Active |
| [Local directory snapshots](./local-directory-snapshots.md) | Git index timestamps, private-index fallback and user-work preservation | Active |

## See Also

- `CLAUDE.md` (repo root) — Backend UUID rules, migration rules, testing rules
- `server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md` — the agent-behavior contract table, one row per enforced boundary
