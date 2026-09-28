# 小阿孚 Implementation Plan

**Goal:** Unify the built-in assistant's product name and safely update unchanged stored defaults.

**Architecture:** Preserve the existing display-name/identity split. Change content at its owning server/shared-view layer and migrate stored defaults separately.

**Tech Stack:** Go, PostgreSQL, TypeScript, React, i18next, Vitest, pnpm.

## Prerequisites

- [x] Create isolated branch/worktree and Trellis task after the user's implementation request following the reviewed scope assessment.
- [x] Persist PRD/design with no unresolved product decision.
- [x] Curate implementation/check context and confirm the existing active task.
- [x] Reuse installed dependencies and verify the isolated worktree database; rerun the onboarding baseline before final review edits.

## Frontend lane

Own only `packages/views/locales/{en,zh-Hans}/{onboarding,runtimes}.json`, `packages/views/onboarding/templates/{mika,install-runtime-issue}.ts`, `packages/core/onboarding/use-bootstrap-mika.ts` and directly affected onboarding/welcome tests.

- [x] Review the existing meaningful frontend assertions and rerun them against both languages. The interrupted implementation had no retained red-phase evidence; no historical red result is claimed.
- [x] Replace fixed product names while preserving locale keys, exported symbols, system identity and custom-name fixtures.
- [x] Verify both locales and shared Web/desktop flows.

```sh
pnpm --filter @multica/core exec vitest run onboarding/mika.test.ts onboarding/use-bootstrap-mika.test.ts
pnpm --filter @multica/views exec vitest run workspace/welcome-after-onboarding.test.tsx onboarding/onboarding-flow-completion.test.tsx onboarding/steps/step-runtime-connect.test.tsx
```

## Backend content lane

Own only `server/internal/service/builtin_agents.go`, `builtin_agents/mika/INSTRUCTIONS.md`, `builtin_skills/multica-onboarding/SKILL.md` and its source map, `server/internal/handler/mika_agent.go`, `mika_onboarding.go` and directly affected Mika handler tests.

- [x] Review default/current-name coverage and extend the creation test to both supported languages; run the final suite.
- [x] Use the exact name-neutral descriptions from design.md; preserve behavior, permissions and identity.
- [x] Run Mika handler and skill-scope tests against the isolated database.

## Migration lane

Own only `server/migrations/457_builtin_agent_display_name.{up,down}.sql` and `server/internal/migrations/builtin_agent_display_name_migration_test.go`.

- [x] Review and run the existing isolated-schema migration behavior tests. Preserve the interrupted implementation without claiming an unrecorded red phase.
- [x] Implement guarded old-default updates, per-row collision reporting and exact default-title cleanup.
- [x] Test custom content, ordinary agents, active/archived collisions, repeated execution, timestamps, history and down-migration preservation.

```sh
cd server
go test ./internal/migrations -run BuiltinAgentDisplayName -count=1 -v
go test ./internal/handler -run 'Mika|ComposeMika' -count=1
```

## Integration and completion

- [x] Review requirements compliance, then code quality, using the Trellis check protocol; fix findings and rerun affected checks.
- [x] Run affected TS tests, `pnpm typecheck`, `pnpm lint`, appropriate Go tests, migration lint and Go vet.
- [x] Update `.trellis/spec/server/builtin-templates.md` with display identity and collision recovery guidance.
- [x] Record exact evidence and rollout limits in `verification.md`.
- [ ] Commit only task-owned files using Lore trailers; hand off integration to the parent agent, preserving unrelated changes.
- [ ] Finish Trellis bookkeeping after verification.

## Final-review repair

- [x] Reproduce missing PostgreSQL notices through the migration CLI pool (red).
- [x] Forward notices with agent/workspace details to the CLI log and verify the focused regression plus concurrent-runner tests (green).
- [x] Keep connection timeout parsing in the existing dbstartup helper and run Go vet.

## Rollout boundary

The migration is delivered through the existing release mechanism. Verify it on the isolated database; production data and packaged desktop releases follow their normal deployment/update cycle. The task does not authorize an external deployment.
