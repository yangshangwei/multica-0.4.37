# Server Development Guidelines

> Guidelines for the Go backend (`server/`).

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Security Boundaries](./security-boundaries.md) | Credential minting gates and automation authorization principals | Active |
| [Task API capabilities](./task-api-capabilities.md) | Frozen reporter/reviewer scope, current authority, durable non-dispatch and claim limits | M1 increment; host sandbox separate |
| [Parent-task budget observation](./issue-budgets.md) | Lifetime root attribution, atomic usage, unknown holds and human API/CLI recovery | F3-S1 observe only |
| [Deterministic report snapshots](./report-snapshots.md) | Frozen safe facts, source gaps, current Reporter reads, human capture and terminal report withdrawal | F2 minimum slice; deterministic API/CLI validation |
| [Controlled artifact execution](./controlled-artifact-execution.md) | Frozen package approval, one-time claim, confined tar output and durable uncertainty | Local synthetic adapter verified; deployment separate |
| [Transactional completion enforcement](./delivery-completion-enforcement.md) | Explicit human authorization in all real completion transactions, exact source and retained recovery | Implemented; final acceptance tracked in task |
| [Completion evidence hints](./delivery-completion-hints.md) | Human ordinary-issue preflight and shared UI/CLI status-only consumers | Non-authorizing hints; transactional enforcement is separate |
| [Project delivery policy and durable evaluations](./delivery-policy-evaluations.md) | Versioned off/observe policy and immutable current-authority recovery | Observe only; all completion enforcement separate |
| [Delivery evidence observation](./delivery-evidence-observation.md) | Current-authority, version-bound artifact/PR observation | Observe only; full completion enforcement separate |
| [Agent output redaction](./agent-output-redaction.md) | Structured credentials, native token formats and redaction before preview/transport | Active |
| [Daemon process descriptors](./daemon-process-descriptors.md) | Linux background/self-update CLOEXEC isolation and real pipe EOF contract | Active |
| [Database test isolation](./database-test-isolation.md) | Explicit test databases, TestMain guards, compile-only commands and bounded audit | Active |
| [Joint verification environment](./verification-environment.md) | API/test environment separation, Make-safe configuration, exact Go package permits, mixed build/test JSON inventories and reviewed run identity | Active |
| [Collaboration foundation](./collaboration-foundation.md) | Authority, creation, human workflow and pre-v2 execution history | Foundation |
| [Human information recovery](./collaboration-information-recovery.md) | Structured block metadata, atomic repair, retained history and bounded authority cursors | Controlled Web/Desktop and owning-layer acceptance |
| [Collaboration stage authority](./collaboration-stage-approvals.md) | Source-member fences, original signer expiry, protected evidence and migration retry contracts | Scope/code-review boundaries checked; full stage activation separate |
| [Collaboration results and manual policy](./collaboration-delivery.md) | Immutable delivery, exact initiator decisions, self-completion, bounded history and task-owned AI admission | Human Web/Electron journey verified; provider/installed Main evidence separate |
| [Collaboration content](./collaboration-content.md) | Gated atomic creation, encrypted drafts, retained references and current-authority native downloads | Public API implemented; full product gate retained |
| [Run-bound Agent outputs](./collaboration-runtime-output.md) | Original comment/file identity, encrypted Agent provenance, current attempt fences and frozen outcome references | Owning-layer and controlled Web/Desktop R03 accepted; full S3 separate |
| [Collaboration realtime](./collaboration-realtime.md) | Account/issue scopes, final-send authority, retained outbox and Query recovery | Account transport implemented; full product pending |
| [Collaboration records and children](./collaboration-records.md) | Ordinary-child references, retained run telemetry, proven group notifications and shared run links | R51/R52 increment verified; full product pending |
| [Collaboration preview and parent wakeup](./collaboration-preview-wakeup.md) | Read-only eligible batch/continuation facts and committed child-to-ordinary-parent stage wake | Focused increment; full product pending |
| [Collaboration source and editing](./collaboration-source-editing.md) | Mixed source projections, source field/iteration transactions, comment recovery and content-free list invalidation | Current source/editing increment; full product pending |
| [Collaboration command history](./collaboration-command-history.md) | Nullable reason, immutable input/cancellation identity and complete cross-client execution evidence | Focused current Web/Electron and DB race evidence |
| [Collaboration targets](./collaboration-targets.md) | Channel FIFO, input history, holds, configuration invalidation, proof flow and v5 reads | Direct/squad service foundation; full product pending |
| [Collaboration squads](./collaboration-squads.md) | Frozen grants, M08/M11 readers, nonlaunch return and durable leader order | Static Linux CLI/daemon chain verified; full product pending |
| [Collaboration runtime CLI](./collaboration-runtime-cli.md) | Narrow existing commands, shared-context show/update, original write IDs, contained CLI and actual daemon capacity | Static Linux chain verified; general provider/product gates remain |
| [Collaboration runtime recovery](./collaboration-runtime-recovery.md) | Persistent fair scans, actual candidate consumption, deletion fences, original-proof priority and bounded checkpoint/seal recovery | Recovery increments verified; full product pending |
| [Collaboration host execution](./collaboration-host-execution.md) | Ordinary Provider execution, shared-context injection/base freeze, per-launch lifecycle/receipts, limited cleanup, authority and v6 compatibility | Implemented behind default-closed host admission; native/account matrix in progress |
| [Collaboration startup handoff](./collaboration-startup-handoff.md) | Stable startup snapshots, exact application input binding and daemon-side failure handling | Negotiated daemon contract; model behavior remains separately verified |
| [Collaboration knowledge candidates](./collaboration-knowledge.md) | Human/system origins, formal acceptance jobs, project policies, validity, source-safe projections and sealed original receipts | Implemented v1/v2 contracts; integrated acceptance separate |
| [Knowledge preparation and launches](./collaboration-knowledge-execution.md) | Source-free v1/v2 claims, exact validity/exposure fences, per-process authority and local Claude/Codex session policies | Default enabled with explicit disable; Provider acceptance separate |
| [Collaboration context experiments](./collaboration-context-experiments.md) | Gated real-model scenarios, durable private evidence, source/model provenance and behavior scoring | Test-only harness; model results remain sample-scoped |
| [Collaboration preparation](./collaboration-preparation.md) | Frozen invocation/files, bundles/Git, contained adapters, private summaries and Go verification argv/package permits | Linux synthetic execution verified; provider/version rollout evidence separate |
| [Collaboration administration](./collaboration-administration.md) | Disabled-creator lifecycle, atomic successor, scoped reads and exact original-key recovery | Limited shared Web/Desktop workflow; full acceptance tracked separately |
| [Collaboration Switch and skills](./collaboration-switch-and-skills.md) | Staged owner consent, atomic Switch and shared skill content invalidation | Implemented service boundaries; full product pending |
| [Offline delivery](./offline-delivery.md) | Configuration forwarding, preserved overlays and upgrade verification | Active |
| [Triage Admission](./triage.md) | Human review, inert intake, execution fences, CSV replay and retained audit data | Active |
| [Iteration facts](./iterations.md) | Borrowed recorder, W01 lock batching, retry identity and current I1 limits | Active |
| [Platform Observability](./platform-observability.md) | Metric windows, queue clocks, leased alerts, health evidence and audit projections | Active |
| [Managed Installations](./managed-installations.md) | Signed enrollment, scoped runtime authority, recovery and provenance | Active |
| [Creator Recommendations](./creator-recommendations.md) | Human-only, evidence-grounded LLM advice and invocation boundaries | Active |
| [Resource publishing](./resource-publishing.md) | Managed uploads, atomic catalog revisions, authorization and replay | Active |
| [Squad delegation receipts](./squad-delegation-receipts.md) | Immediate CLI/leader receipt consumption, uncertainty and partial admission | Implemented; durable receipts separate |
| [Durable delegation receipts](./durable-delegation-receipts.md) | Opt-in idempotent comment creation, original identity and retained uncertainty | Verified opt-in explicit-target increment |
| [Ordinary comment notification replay](./ordinary-notification-replay.md) | Persisted comment/member delivery IDs, legacy row/state preservation and scoped bus replay | Listener normal/race verified; client/runtime acceptance separate |
| [Personal Inbox snooze](./inbox-snooze.md) | Group transactions, critical-source claims, overdue acknowledgment, writer semantics and shared recovery | Evidence tracked in task |
| [Built-in Template Registries](./builtin-templates.md) | Role/squad/autopilot provenance, security routing, status ownership and server-decided content | Active |
| [Project execution squads](./project-execution-squad.md) | Project setup, authority, atomic materialization and UI recovery contracts | Active |
| [Local directory snapshots](./local-directory-snapshots.md) | Git index timestamps, private-index fallback and user-work preservation | Active |
| [Development fixtures](./development-fixtures.md) | Retained Desktop QA data, consumer contracts, idempotency and scoped cleanup | Active |

## See Also

- `CLAUDE.md` (repo root) — Backend UUID rules, migration rules, testing rules
- `server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md` — the agent-behavior contract table, one row per enforced boundary
