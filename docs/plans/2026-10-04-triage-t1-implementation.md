# Triage T1 Implementation Plan

> Execution: native executor subagents in the authorized current session; leader owns integration and verification.

**Goal:** Implement the reviewed T1 manual triage loop, including CSV intake, safe review, history, notifications and all server execution boundaries.

**Architecture:** Existing Issue content plus server-owned admission, transactional review/import ledgers, shared core API/query logic and shared Web/Desktop views.

**Tech Stack:** Existing Go/PostgreSQL/sqlc and React/TypeScript/TanStack Query/Base UI. No new dependencies.

The authoritative task artifacts are:

- [PRD](../../.trellis/tasks/10-04-triage-t1/prd.md)
- [Technical design](../../.trellis/tasks/10-04-triage-t1/design.md)
- [API contract](../../.trellis/tasks/10-04-triage-t1/api-contract.md)
- [Ordered implementation checklist](../../.trellis/tasks/10-04-triage-t1/implement.md)
- [Full acceptance ledger](../../.trellis/tasks/10-04-triage-t1/verification.md)

Branch: `codex/triage-t1`. Worktree: `/Volumes/artisan/code/2026/multica-triage-t1`.
