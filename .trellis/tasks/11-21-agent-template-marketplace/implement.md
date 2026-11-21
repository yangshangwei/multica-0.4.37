# Implementation plan

Status: implementation and acceptance completed on 2026-11-21. See `verification.md` for the final119 unit tests,9 production Web/native journeys, visual evidence and static-analysis baseline.

## Ordered work

1. **Re-establish exact write ownership.** Re-read the active task and current market/creation files. Reconcile any changes from `11-21-squad-marketplace`; preserve unrelated staged/unstaged work. Load required package specs. Capture the existing market and role-picker appearance before UI edits.
2. **Lock existing behavior before extraction.** Run the canonical template page/submit/query suites. Add regression coverage only for currently unprotected behavior affected by extraction, plus expected failing cases for the new category and return source. Do not weaken existing instance, no-mutation, scroll, squad or portal assertions.
3. **Extract shared role cards.** Move only shared card/grid/known-autonomy presentation into agents-domain components. Keep the picker controller, instance Query, exact matching, actions and memento ownership where they are. Re-run the original picker suite immediately.
4. **Add the market category and panel.** Extend Core path type/test; append `agent-templates` to URL selection, visited map, Tabs and hidden/inert panel lifecycle. Add `AgentTemplatesMarketPanel` with active-gated role template Query and explicit loading/error/retry/empty states. Do not query agents or mount full creation/management pages. Keep MCP/Skills/squad ordering and default.
5. **Connect configuration and return.** Market action links to the existing selected-template route with the fixed `source=marketplace`. Reuse/extend the existing local URL helper only as needed. Add the source-aware history/fallback branch, preserving original `squad` precedence and the unmarked picker-back path. Do not change submit hook/request or success destination.
6. **Complete bilingual copy and states.** Add en/zh-Hans market category, tagline and market-only role description/action. Verify existing picker copy/instance actions, unknown autonomy, invalid key/source, empty/error, narrow labels and a single content scroll root.
7. **Update canonical tests and specifications.** Extend market and role-template journeys without replacing the original creation journey. Update capability-market and agent-discovery ownership/contracts; curate only task-owned evidence. Follow project Trellis implement/check dispatch at execution time, with clear file scopes and native context injection when available.
8. **Verify and review.** Run the commands below, inspect actual outputs, then review the focused diff. Record counts, baseline failures and runtime provenance truthfully; fix task-introduced failures. Perform bounded production Web/native Desktop and visual acceptance before claiming feature completion.

## Canonical test ownership

| Contract | Owning suite |
| --- | --- |
| Fourth type URL builder | `packages/core/paths/paths.test.ts` |
| URL source, query/hash, lazy/retained mounts, active flags, workspace reset | `packages/views/marketplace/marketplace-page.test.tsx` |
| Real role query gating, no agent-list dependency, states and action target | New `agent-templates-market-panel.test.tsx` |
| Card semantics, known/unknown autonomy, original instance navigation and gallery restoration | Existing `template-create-agent-page.test.tsx`, plus a small domain-card suite only for independently owned behavior |
| Source-aware Back/fallback, squad priority, unknown source/key | Existing template page suite; a local pure helper test only if a helper is actually introduced |
| Request body, awaited creation, destination, no-runtime and join-failure semantics | Existing `use-create-template-agent-submit.test.tsx`; do not replay its whole matrix through market DOM |
| Language-scoped role query behavior | Existing `use-role-templates` suites |
| Browser market route/history and original creation | `e2e/capability-market.spec.ts`, `e2e/agent-role-template.spec.ts` |

## Expected write scope

- `packages/core/paths/paths.ts` and corresponding path tests.
- `packages/views/marketplace/marketplace-page.tsx`, its tests, and a new agent-template market panel/test.
- `packages/views/agents/create/template-create-agent-page.tsx`, its tests, extracted role card presentation, and only necessary changes to `squad-param.ts` or a focused link helper.
- `packages/views/locales/{en,zh-Hans}/{layout,agents}.json`.
- Focused E2E and `.trellis/spec/views/frontend/{capability-market,agent-discovery}.md`.
- This task directory's research, planning, manifests and verification evidence.

No submit hook, backend, database, mobile, global store, platform router, welcome page or other market-domain change is planned. If inspection shows one is necessary, record the concrete reason and revise the plan before widening ownership.

## Validation commands

Run existing tests before edits, then include new suites after they exist:

```bash
pnpm --filter @multica/core exec vitest run paths/paths.test.ts paths/consistency.test.ts paths/tab-presentation.test.ts
pnpm --filter @multica/views exec vitest run agents/create/template-create-agent-page.test.tsx agents/create/use-create-template-agent-submit.test.tsx agents/create/use-role-templates.test.ts agents/create/use-role-templates.queries.test.tsx agents/create/create-agent-ui.test.ts marketplace/marketplace-page.test.tsx
pnpm --filter @multica/views exec vitest run marketplace/agent-templates-market-panel.test.tsx agents/agents-i18n-parity.test.ts
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/core lint
pnpm --filter @multica/views lint
pnpm knip
git diff --check -- packages/core/paths packages/views/agents/create packages/views/marketplace packages/views/locales/en packages/views/locales/zh-Hans e2e/agent-role-template.spec.ts e2e/capability-market.spec.ts .trellis/spec/views/frontend/agent-discovery.md .trellis/spec/views/frontend/capability-market.md
```

Static analysis may report pre-existing checkout problems. Keep baseline evidence separate; do not expand this task into general cleanup. Backend tests need not run solely for a Views/Core path change; run broader `make check` only if expanded risk or final integration requirements justify it.

Production Web acceptance uses task-owned services and the documented registry allocator:

```bash
make up C=api,web ARGS="--web-mode production"
make status ARGS="--json"
make env-exec ARGS="-- pnpm exec playwright test e2e/capability-market.spec.ts e2e/agent-role-template.spec.ts --workers=1 --retries=0"
```

Before trusting those results, verify listener ownership/build identity and actual auth configuration; do not change the checkout's source auth settings or kill unrelated listeners. Record any prerequisite failure before page assertions separately from feature failures.

## Runtime and visual checklist

- [ ] Market direct URL, keyboard switch, reload, browser Back/Forward and unrelated query/hash preservation.
- [ ] Market action → correct configuration; app Back with safe history and cold/no-history fallback.
- [ ] Original chooser → picker → configuration → picker, including `squad` and existing-member open without mutation.
- [ ] No market agent-list dependency; template-only error/retry/empty, unknown role policy and invalid link states.
- [ ] Existing market dirty editors survive category round-trips; hidden portals remain closed; cross-route disposal remains the existing contract.
- [ ] Native Desktop same-tab and modifier/new-tab links, source fallback, workspace switch and actual tab restoration.
- [ ] English/Simplified Chinese, desktop and 768/390/320 widths: no document overflow, clear selected tab, readable cards, accessible main action and single content scrollbar.
- [ ] For each UI edit iteration use the required visual-verdict workflow, persist its result under task scope, and follow Impeccable's bounded inspect/fix/confirm limit. No visual verdict is claimed for this planning-only turn.

## Phase-transition and rollback controls

The user approved the final summary before `task.py start`. Implementation/check sub-agents completed their assigned reviews. All product acceptance criteria are verified; remaining steps are scoped work commit, current-task archive and session recording. `commit-plan.md` defines ownership and preservation of existing WIP.

Before `task.py start`: latest final summary explicitly approved; PRD convergence complete; design/execution files readable; both JSONL manifests contain real spec/research rows. Capture approval in task notes without fabricating a user decision.

Keep route/category integration and domain-card extraction reviewable as separate conceptual changes. If a regression appears, preserve existing creation URLs and members; revert the new market panel/source branch before considering any larger rerouting. No data rollback is needed.
