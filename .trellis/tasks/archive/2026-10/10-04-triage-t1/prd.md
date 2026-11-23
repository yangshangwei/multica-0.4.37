# 分拣台 T1 人工审核闭环

## Objective

Implement every T1 requirement in the reviewed v0.2 triage PRD, including CSV import, shared Web/Desktop UI, atomic admission, history, responsibility notifications, and enforcement at all existing server execution and ordinary-query boundaries. Establish technical design and an ordered development plan before implementation.

## Source of truth

- `docs/plans/2026-10-04-work-management-prds/triage-prd.md`: TRI-FR-01 through TRI-FR-20 and TRI-FR-26; every T1 acceptance row (TRI-AC-01–32 and TRI-AC-37–41).
- `docs/plans/2026-10-04-work-management-prds/README.md`: common permissions, project scope, compatibility, retries and cross-module acceptance.
- The user's instruction explicitly authorizes task creation, a new branch, technical design, task decomposition and implementation. Routine reversible steps proceed autonomously.

## Scope and invariants

1. Workspace opt-in, default off. Owners/admins configure; human members review. Existing formal work retains its behavior. No pending inputs may be stranded by disabling.
2. Reuse Issue identity, content, comments and attachments. Admission is separate from the seven execution categories. Pending/rejected/duplicate issues are absent from ordinary views and project totals and cannot execute through any client or background path.
3. Queue with filters, stable ordering, all/actionable/snoozed counts, explicit empty states, keyboard navigation and responsive detail. Manual intake, four actions, complete acceptance field editing, optional explicit accept-and-execute, responsibility reassignment, append-only history and reasoned reopen.
4. UTF-8 CSV upload, mapping, validation-only preview, per-row warning/error/duplicate feedback, confirmation, atomic per-row writes, partial results, failed CSV download and durable retry identity. No row-triggered execution or notification storm.
5. Concurrency is server-owned. A committed action/result is not repeated after an uncertain response. Invalid acceptance attributes roll back together. Accept-and-execute reports successful admission independently of failed execution and retries only the unstarted execution.
6. New review/result/responsibility/import notifications remain workspace-authorized and deduplicated. Snoozed tasks become actionable from stored time even after restart.
7. Shared Web/Desktop UI and routing; server-side protection for old desktop/mobile/CLI/agent clients. No T1 rule engine, AI suggestions, automatic acceptance, connectors, iterations module or new Project entity.
8. No new dependencies, database foreign keys or cascades. Every index uses a separate concurrent migration. Preserve existing code/package conventions.

## Acceptance evidence

`verification.md` tracks every T1 scenario, proof source and execution result. All rows must be verified before completion; an existing capability or planned test is not proof. Final validation includes migration safety, real PostgreSQL tests, schema-malformation and cache/UI regressions, Web/Desktop routing, lint/typecheck/static analysis and browser flow evidence. No live agent account is needed: execution tests use fixtures/fake runtimes.
