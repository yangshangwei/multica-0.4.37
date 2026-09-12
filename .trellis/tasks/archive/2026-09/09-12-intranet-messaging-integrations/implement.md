# Isolated deployment messaging policy implementation plan

**Goal:** Disable external messaging providers by deployment policy while preserving intranet Git.

**Architecture:** A server startup setting controls provider wiring and HTTP availability and is exposed through `/api/config`. Shared core loads it into the existing config store; shared views gate settings sections and agent-detail integration UI.

**Tech stack:** Go/Chi, TypeScript, React, Zustand configuration, React Query, Vitest, existing deployment Compose files.

## 1. Server policy and regression coverage

- Inspect `server/cmd/server/router.go`, `server/internal/handler/handler.go`, `config.go`, and current provider routes/workers.
- Add failing tests for runtime config, disabled provider entry points with credentials present, and independent VCS availability.
- Add the startup policy, gate the five connector lifecycle paths and HTTP operations, and expose the public boolean.
- Run focused Go tests and `go vet` for changed packages; use isolated DB fixtures when required.

## 2. Shared runtime configuration

- Extend `packages/core/api/schemas.ts`, `packages/core/config/index.ts`, and `packages/core/platform/auth-initializer.tsx`.
- Write schema/malformed-response and initializer tests first, including explicit false, true, omission, and unrelated response drift.
- Verify focused core tests before UI integration.

## 3. Shared view behavior

- Add failing Settings tests for messaging hidden and VCS retained.
- Add agent detail tab/status tests and stale-tab recovery coverage.
- Gate Settings sections, agent integration content, and inspector shortcuts using the shared config. Do not touch unrelated agent catalog files.
- Run focused views tests, lint, typecheck, and a bounded visual verification of the affected surfaces.

## 4. Deployment documentation and integration verification

- Pass the switch through self-host Compose and document `false` for isolated networks alongside independent VCS requirements.
- Update the Chinese self-host documentation and regenerate its in-app docs bundle if edited.
- Validate Compose interpolation without exposing secrets, run applicable static checks, and review the full diff.
- Report changed file groups, verification evidence, restart requirements, and any untested live-provider behavior.

## Rollback

Set the switch back to `true` and restart the API service. Saved credentials/installations remain intact. Removing these code changes restores historical behavior without a data migration.
