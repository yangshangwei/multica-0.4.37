# Skill Template Refresh Implementation Plan

**Goal:** Discover mounted skill template changes without restarting the client.

**Architecture:** The backend already scans disk per request. Override the global
infinite Query freshness only for the template catalog. Use Query's observer
lifecycle and foreground timer, opt into polling only in the picker, and reuse
the existing refresh status and retry UI. Keep adopted drafts independent.

**Tech Stack:** React, TanStack Query v5, Vitest, existing Go template API.

## Scope and rationale

- `packages/core/workspace/queries.ts`: `staleTime: 0`, mount/focus/reconnect refresh,
  optional picker polling every 30 seconds, no polling in background.
- `packages/core/workspace/skill-template-live-queries.test.ts`: production-client
  observer regression suite; jsdom is necessary for Query timers/focus behavior.
- `packages/views/skills/components/template-skill-create-panel.tsx`: enable the
  timer only for preview; selecting deployment source requests fresh data.
- `packages/views/skills/components/template-skill-create-panel.test.tsx`: real
  dialog wiring regression, retaining search/selection and edited copies.
- `.trellis/spec/views/frontend/builtin-skill-localization.md`: document why the
  directory-backed catalog must opt out of global infinite freshness.

Rejected: backend filesystem watcher + WS event, because per-request scanning
already exists and the client needs a reliable recovery path regardless.
Rejected: changing global query defaults, because WS-managed server data depends
on them. Reuse the existing picker without adding layout or translation changes.

## Execution

1. Add focused failing tests and record their failure against unchanged production code.
2. Apply the narrow Query policy and picker wiring changes.
3. Run core catalog and shared skill-creation tests, then package lint/typecheck.
4. Verify the local desktop and backend remain healthy, and exercise a temporary
   mounted template if the current browser/debug configuration permits it.
5. Review the final diff and record evidence. Preserve pre-existing dirty files.

Commands: `pnpm --filter @multica/core exec vitest run workspace/skill-template-queries.test.ts workspace/skill-template-live-queries.test.ts`,
`pnpm --filter @multica/views exec vitest run skills`,
`pnpm --filter @multica/core --filter @multica/views lint`,
`pnpm --filter @multica/core --filter @multica/views typecheck`.
