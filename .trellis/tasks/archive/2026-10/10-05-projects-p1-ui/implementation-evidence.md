# Shared P1 implementation evidence

2026-10-05. Scope: core, shared project views, shared Web/Desktop paths. Mobile compatibility is owned by the mobile lane. HG/PG/UG and real browser verification remain parent-owned gates; this document does not declare complete P1 acceptance.

## Implemented

- Strict P1 response DTOs and nested workspace/project identities; malformed snapshots cannot become empty success. Existing project fields remain optional, while complete statistics satisfy N=F+C+U and old done_count=F+C.
- Workspace-scoped capabilities and owned query keys; only dedicated capability 404 followed by confirmed workspace access means unsupported.
- Server-first description/status/delete writes, including the existing deletion cache debt. Explicit completion confirmation is available from detail and list/card status controls.
- Independent overview/statistics/acceptance/health reasons, exact risk list/table and cursor paging independent of saved views, actor filters, hidden categories and sub-issue settings.
- Goal template selection, preview and append; description CAS serialization against the editor-adopted revision, conflict comparison, durable unsaved input, StrictMode-safe lifecycle and unmount recovery.
- Manual progress, acceptance and risk drafts, evidence input, preview, recipient review, publish/retry, corrections and revision history. Durable intent IDs survive failed delivery; later edits cannot be cleared by an older acknowledgement.
- Explicit adoption of a changed description before retrying new acceptance; old acceptance corrections retain their original description.
- Realtime aggregate dependency expansion, planning-day checks, reconnect invalidation, immediate permission-loss removal of protected project caches/drafts and a client visibility guard.
- Shared section/risk paths preserve both platform navigation adapters; English and simplified Chinese strings.

## Tests and checks actually executed

- Preparation RED: existing schema accepted malformed P1 fields; existing deletion removed detail before server response; description optimistically polluted authoritative cache; legacy closure was mislabeled actual completion.
- Additional RED: root React StrictMode effect replay disabled autosave. The regression now passes with a controller scoped to the mounted editor lifecycle.
- `pnpm --filter @multica/core test projects api/project-execution.test.ts api/project-p1-client.test.ts api/project-p1-schema.test.ts issues/cache-coordinator.test.ts realtime/use-realtime-sync.test.ts drafts/cleanup-registry.test.ts`: 14 files / 175 tests passed.
- `pnpm --filter @multica/views test projects/components/project-management.test.tsx projects/components/project-detail.test.tsx projects/components/project-issue-metrics.test.ts projects/components/projects-page.test.tsx`: canonical interactions plus existing project behavior, see final command result in parent evidence.
- `pnpm --filter @multica/core lint`: passed.
- `pnpm --filter @multica/core typecheck`: passed before the last durable-description slice; the subsequent shared views typecheck includes that imported source and passed.
- `pnpm --filter @multica/views typecheck`: passed after all current changes.
- `pnpm --filter @multica/views lint`: no errors, existing repository warnings; direct lint of all new major P1 components passed after final changes.
- `git diff --check`: passed.

## Pending integration evidence

Real API and production Web/Desktop browser validation, final visual-verdict, security/permission removal flows in running applications, and complete acceptance audit remain required. The parent is starting an isolated real backend and production frontend. No production deployment is claimed.
