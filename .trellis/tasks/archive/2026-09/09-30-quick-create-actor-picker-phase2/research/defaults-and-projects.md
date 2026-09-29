# Defaults and project candidates

Planning evidence, 2026-09-30. The original phase1 implementation is present in the working directory.

- `packages/core/projects/execution-squad.ts:getProjectExecutionSquads` gives ordered plural `execution_squads` precedence, including intentional `[]`, then legacy singular fallback.
- `packages/core/types/project.ts:ProjectExecutionSquad` distinguishes configured/needs_runtime/failed/none. Project response conversion in `server/internal/handler/project.go:projectToResponse` includes both singular and plural fields in list and detail responses. Existing `projectListOptions(wsId)` is sufficient for the selected project's configuration; no eager per-project detail requests.
- Project execution squad is an eventual issue assignee default, not a quick-create author. Show configured squad IDs only after intersecting current eligible squads; do not infer agent members or auto-assign the project squad as creator.
- Phase1 core `quick-create-store.ts` has typed refs, scoped hydration, reset generation, normalized persistence and cleanup. Extend it with nullable `defaultActor` and a guarded setter; use the same local/device/logout contract.
- Seed priority becomes caller > unfinished draft > explicit local default > last accepted > first eligible agent. Do not introduce automatic Mika selection or project fallback. A default/caller/draft squad waiting for its query is unknown, not invalid; do not lock a lower-priority agent before the query resolves.
- `QuickCreateActorPicker` has persistent search, 3 favorite/5 recent home candidates, typed filters, full search and50-item pages. Keep action focus inside the popup before view changes; phase1 nested-dialog Escape regressions are mandatory.
- Add default controls once in picker footer rather than another icon on every row. Setting/clearing default does not modify current actor or submit anything. Inaccessible default refs remain stored but are never dereferenced into unauthorized names; allow explicit clearing.

Canonical tests: core preference normalization/isolation, pure seed precedence with pending vs settled datasets, pure project plural/legacy/permission projection, parent project switch retaining explicit actor, real picker footer focus and first-phase keyboard regressions.
