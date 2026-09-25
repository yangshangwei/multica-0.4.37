# Verification — agent-discovery

Commit under test: `6d43b7b71` (feat(agents): find agents by role and squad, distinguish templates from members). Branch `feat/agent-discovery`.

## Code-level checks (independent trellis-check pass)
- `pnpm --filter @multica/core typecheck` / `@multica/views typecheck` — PASS (EXIT=0).
- `pnpm --filter @multica/core lint` / `@multica/views lint` — 0 errors (pre-existing warnings only, unrelated files).
- core vitest (`agents/discovery`, `agents/stores/view-store`, `api/squad-members`, `workspace/squad-members-query`) — 35/35.
- `pnpm --filter @multica/views test agents/` — 481/481 (52 files).
- `locales/parity.test.ts` + `agents-i18n-parity.test.ts` — 183/183.
- `go vet ./internal/handler` — clean.
- 5/5 design contracts source-verified: squad `agent_member_ids` schema/parse/wsId-key/gating; template picker matches by `template_key` + `!archived_at`, `<AppLink>` (no mutation); `resolveAgentRole` derives kind purely from `template_key`; `hiddenColumns` preserved exactly; `setScope`/`toggleFilter` compose.

### Self-fixed during check (3 dead-locale-key classes, all inside the 4 `agents.json`)
1. Orphaned `catalog` block (8 keys × 4 locales) left dead by the BuiltinAgentCatalog deletion.
2. `role_templates.instances_count_one` in ja/ko/zh-Hans — CJK locales have no CLDR "one" category; failed parity's dead-plural guard.
3. 4 unreferenced `discovery` keys (`membership_loading`, `no_grouping`, `grouping`, `group_count`) × 4 locales.

## Go handler tests — real migrated clone DB (not a silent skip)
Cloned the migrated template DB, applied migrations to head (…455/456), ran, dropped. Run twice independently (main session + check agent), both green:
- `TestSquadMembershipSummaryIncludesAgentsBeyondPreview` — PASS.
- `TestListSquadsCompleteAgentMembership` — PASS (count=6 / preview=3 / agent_member_ids=5; shared specialist across squads; empty squad encodes `[]`; archived + foreign-workspace excluded).
- `go test ./internal/handler -run Squad -count=1 -v` — `ok`, all ~110 Squad tests pass (3.561s).

## AC6 browser evidence — production web, isolated worktree
- Env: git worktree at `6d43b7b71`, isolated env `multica_agentdisc_e2e-467`, DB `multica_multica_agentdisc_e2e_467`, api `:18547`, web `:13467`. Web served **production** build `UKi05kZejVsSX_nbz06pc`, api `/health` reported commit `6d43b7b71`. The human env (`:18572`/`:13492`/`multica_multica_0_4_37_492`) was never touched.
- `make env-exec -- pnpm exec playwright test e2e/agent-role-template.spec.ts e2e/agents-discovery.spec.ts --workers=1 --retries=0` — **4/4 passed** (12.5s, EXIT=0).
- Non-mutation asserted by the discovery spec (`mutationRequests` empty, `pageErrors` empty, `scrollWidth <= innerWidth` at each width).
- Screenshots at 1440/768/390 (preserved `/tmp/agentdisc-shots/`): `agents-22-grouped-1440-zh`, `agents-grouped-{1440,768,390}-zh`, `agents-shared-{1440,768,390}-zh`, `role-templates-{1440,768,390}`. Visual verdict: template picker distinguishes 0/1/multiple instances; directory grouped 统筹6/专业13/其他2 with role provenance + squad/leader chips + labeled 30-day window; no overflow at 390.

## Teardown
Isolated DB dropped, worktree removed, registry entry removed. Human env verified healthy and unchanged after teardown.

## Limitations
- Scoped to task-relevant suites — not full `pnpm test` / full Go suite / full e2e.
- Screenshots are the spec-generated responsive set (Chinese locale); no separate manual visual pass.
