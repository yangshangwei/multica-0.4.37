# Server Development Guidelines

> Guidelines for the Go backend (`server/`).

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Security Boundaries](./security-boundaries.md) | Credential minting gates and automation authorization principals | Active |
| [Collaboration foundation](./collaboration-foundation.md) | Authority, creation, human workflow and pre-v2 execution history | Foundation |
| [Collaboration content](./collaboration-content.md) | Gated atomic creation, encrypted drafts, retained references and current-authority native downloads | Public API implemented; full product gate retained |
| [Collaboration realtime](./collaboration-realtime.md) | Account/issue scopes, final-send authority, retained outbox and Query recovery | Account transport implemented; full product pending |
| [Collaboration records and children](./collaboration-records.md) | Ordinary-child references, retained run telemetry, proven group notifications and shared run links | R51/R52 increment verified; full product pending |
| [Collaboration source and editing](./collaboration-source-editing.md) | Mixed source projections, source field/iteration transactions, comment recovery and content-free list invalidation | Current source/editing increment; full product pending |
| [Collaboration targets](./collaboration-targets.md) | Channel FIFO, input history, holds, configuration invalidation, proof flow and v5 reads | Direct/squad service foundation; full product pending |
| [Collaboration squads](./collaboration-squads.md) | Frozen grants, M08/M11 readers, nonlaunch return and durable leader order | Static Linux CLI/daemon chain verified; full product pending |
| [Collaboration runtime CLI](./collaboration-runtime-cli.md) | Narrow existing commands, original write IDs, contained CLI and actual daemon capacity | Static Linux chain verified; general provider/product gates remain |
| [Collaboration Switch and skills](./collaboration-switch-and-skills.md) | Staged owner consent, atomic Switch and shared skill content invalidation | Implemented service boundaries; full product pending |
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
| [Development fixtures](./development-fixtures.md) | Retained Desktop QA data, consumer contracts, idempotency and scoped cleanup | Active |

## See Also

- `CLAUDE.md` (repo root) — Backend UUID rules, migration rules, testing rules
- `server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md` — the agent-behavior contract table, one row per enforced boundary
