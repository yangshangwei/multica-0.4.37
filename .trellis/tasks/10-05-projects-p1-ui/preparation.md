# UI preparation before FG

2026-10-05, worktree `/Volumes/artisan/code/2026/multica-projects-p1`.

## Scope and gate

The reviewed API contract is loaded. Product source remains untouched until the parent records and announces FG. UI owns shared TypeScript/project UI/mobile compatibility; server implementation and canonical server tests belong to other lanes.

## Prepared evidence

- `packages/core/projects/test-fixtures/p1.ts`: shared UUID-scoped wire fixtures for project, statistics, overview, manual draft/preview/revision/write result. The fixtures preserve `done_count=completed+cancelled` and carry no manufactured client snapshots.
- `packages/core/api/project-p1-schema.test.ts`: legacy omission compatibility plus rejection of present malformed revision, unsafe integer, count, and completeness fields.
- `packages/core/projects/mutations.test.tsx`: deletion must await the server and capture workspace; failed deletion preserves project detail/list/view state; versioned description drafts must not enter authoritative caches before acknowledgement.
- `packages/views/projects/components/project-issue-metrics.test.ts`: distinct actual completion, cancelled, open and closure values; legacy unavailable details; empty denominator.

Executed RED checks (assertion failures, not import/runtime failures):

| Command | Result |
| --- | --- |
| `pnpm --filter @multica/core test api/project-p1-schema.test.ts projects/mutations.test.tsx` | 13 tests, 11 expected failures / 2 passed. Existing loose schema accepts malformed P1 values; deletion removes detail before response; description optimistically replaces authoritative body. |
| `pnpm --filter @multica/views test projects/components/project-issue-metrics.test.ts` | 4 tests, 3 expected failures / 1 passed. Existing metrics mislabel the legacy terminal aggregate as actual completion. |

No application implementation, commit or push occurred in preparation. Tests deliberately remain RED until FG enables implementation. The original metrics baseline will be updated when its legacy label changes; the canonical count matrix will remain at the pure-helper layer.

## Implementation order after FG

1. Add strict new DTO schemas/API methods and typed optional legacy fields; implement workspace-safe mutations and owned query prefixes.
2. Build per-project durable intent drafts (request IDs), serialized description CAS, goal template append, and exact server health query result rendering.
3. Compose overview/manual progress/correction/history/completion warning, keeping existing five task views/resources/squads/automation.
4. Complete realtime dependency, day boundary and immediate permission-loss cache/draft clearing.
5. Wire Web/Desktop deep links; mobile independent read/capability/schema/realtime compatibility only.
6. Run package canonical tests, lint/typecheck, then coordinate real-API and visual verification with the parent/verification lane.

## Design boundaries

Use the existing Operate interface system: shared semantic tokens and Base UI primitives, compact factual project metrics, explicit unavailable/error/old-acceptance states. This is an extension of the incumbent design, not a visual replacement. Parent coordinates final visual-verdict and independent review; no recursive delegation from this lane.
