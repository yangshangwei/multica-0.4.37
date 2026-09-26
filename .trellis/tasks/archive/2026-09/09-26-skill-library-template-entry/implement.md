# Implementation plan

Status: iteration 2, future work only. This task has not been activated and application code must not be changed during planning. Use [prd.md](prd.md), [design.md](design.md), and [test-spec.md](test-spec.md) together; review-adopted corrections are recorded in design.

## Constraints and preflight

Read `CLAUDE.md`, `.trellis/workflow.md`, `.trellis/spec/views/frontend/builtin-skill-localization.md`, `skill-presentation.md`, and conventions before implementation. Browser work additionally follows `.trellis/spec/web/frontend/e2e-run-environment.md`. Preserve unrelated `.impeccable` artifacts and other contributors' work. Recheck the branch/diff and current task before activating any task.

Scope cleanup is bounded: remove the skills-only duplicate catalog, replace its callsites, and remove only keys/imports proven unused. Existing component tests protect management and creation behavior; extend them before each behavioral change. Do not refactor common catalog consumers, card layout, origin facets, core sessions, API contracts, or mutation semantics. If a contract appears to need expansion, return to design review first.

## Ordered work slices

| Slice | Owned files / changes | Evidence and dependency |
| --- | --- | --- |
| 1. Pure discovery and localized summaries | Add `packages/views/skills/lib/skill-template-discovery.ts` and `.test.ts`; add template-only summary resolver in `skill-presentation.ts`; add `summary` and entry/phase/status keys to all four `packages/views/locales/{en,zh-Hans,ja,ko}/skills.json`. Use exact existing default recognition; keep full descriptions and search text. | Write pure relation/source/summary tests first; existing presentation/source-sync and parity suites remain canonical. No component work depends on new server fields. |
| 2. Direct entry and stable picker host | Replace `builtin-skill-catalog.tsx` with `skill-template-entry.tsx`; update `skills-page.tsx` to pass `workspaceId` and open `initialEntry: {kind: "templates"}`. Replace duplicated normal/error dialog mounts with one stable sibling outside the conditional page body, keyed only by workspace identity if needed. Update `create-skill-dialog.tsx` to the discriminated entry prop; keep New skill/manual-category defaults. Update picker summaries, headers, source/visible-selection consistency, and query states. | Depends on slice 1. Extend `skills-page.test.tsx`, `create-skill-dialog.test.tsx`, `template-skill-create-panel.test.tsx`, and the direct-entry happy path in `create-skill-template-flow.test.tsx`. Add `skills/components/skills-page-template-session.test.tsx` with real QueryClient/page/dialog to prove same-workspace cached failure/retry preserves draft, unconfirmed submission and pending discard. |
| 3. Related links and guarded intent snapshots | In `template-skill-create-panel.tsx`, derive all related skills and render query-aware state/links; emit the immutable `RelatedSkillNavigationRequest` from the original gesture. In `create-skill-dialog.tsx`, guard every Desktop adapter action and Web in-place push with existing busy/dirty/unconfirmed handling. On acceptance reset/close and execute original path/title/intent once; keep recovery callbacks separate. Reject stale source UUID/slug snapshots and clear pending actions on workspace change. | Depends on slices 1–2. Guard/snapshot matrix belongs to `create-skill-template-flow.test.tsx`, including cancel/confirm/busy, original title/intent and existing-tab background activation; picker tests cover link wiring. AppLink tests remain canonical for gesture classification. Do not change Desktop tab-store behavior. |
| 4. Integrate and verify | Update `e2e/skill-template-creation.spec.ts` to cover compact entry and retain method-chooser path. Reroute `e2e/localized-template-defaults.spec.ts` test `specialist skills localize` away from deleted inline catalog, preserving identity/no-write assertions. Update `.trellis/spec/views/frontend/builtin-skill-localization.md` for display-only summaries and current source-tab/empty-source behavior; its section-only guidance predates the inspected tabs. | Run focused checks, consumer typechecks, and production browser verification below. Review all source/callsite deletions and Web/Desktop parity; collect visual evidence before another visual edit. |

## Exact commands for future execution

Run from repository root after tests/implementation exist. These commands have not been run for this plan.

```bash
pnpm --filter @multica/views exec vitest run skills/lib/skill-template-discovery.test.ts skills/lib/skill-presentation.test.ts locales/parity.test.ts --maxWorkers=2
pnpm --filter @multica/views exec vitest run skills/components/skills-page.test.tsx skills/components/skills-page-template-session.test.tsx skills/components/create-skill-dialog.test.tsx skills/components/create-skill-template-flow.test.tsx skills/components/template-skill-create-panel.test.tsx navigation/app-link.test.tsx --maxWorkers=2
pnpm --filter @multica/views lint
pnpm --filter @multica/views typecheck
pnpm exec turbo run typecheck --filter=@multica/web --filter=@multica/desktop --concurrency=1
rg -n 'BuiltinSkillCatalog|initialTemplateName' packages apps e2e
git diff --check
```

The reference scan should leave no removed API/component callsites; retained test descriptions must be intentionally updated. Inspect catalog locale key references before deletion. Use `pnpm knip` if deletion/export analysis exposes broader dead-code uncertainty; do not delete unrelated findings. Source summary changes still require the existing presentation suite's source synchronization check.

For a task-owned environment already configured with classic test authentication, matching API/DB URLs, and an embed-only catalog (`MULTICA_SKILL_TEMPLATE_DIR` empty/unset):

```bash
make up C=api,web ARGS="--web-mode production"
make status ARGS="--json"
make env-exec ARGS="-- pnpm exec playwright test e2e/skill-template-creation.spec.ts --project=chromium --workers=1 --retries=0"
make env-exec ARGS="-- pnpm exec playwright test e2e/localized-template-defaults.spec.ts --grep 'specialist skills localize' --project=chromium --workers=1 --retries=0"
make env-exec ARGS="-- pnpm exec playwright test e2e/skill-category-taxonomy.spec.ts --project=chromium --workers=1 --retries=0"
```

The category spec is included because the page shell above the toolbar changes; its fixed-card/filter contract stays untouched. Record build/service provenance and visual verdicts, then run `make down` only for the environment this task started and owns. Do not destroy environment data or stop unrelated listeners. Do not build concurrently in the same checkout.

The alternative below creates a fresh isolated environment, first runs broad static/TS/Go checks and a production Web build, then runs all three required E2E specs. It intentionally runs the entire localized-defaults spec, not only the renamed navigation test.

```bash
bash scripts/check.sh e2e/skill-template-creation.spec.ts e2e/localized-template-defaults.spec.ts e2e/skill-category-taxonomy.spec.ts --project=chromium
```

`make check` is the full integration gate when release policy or expanded scope requires it. `make check ARGS=...` does not filter E2E. Do not run both broad paths reflexively after the required checks pass. Detailed prerequisites are in [research/verification-map.md](research/verification-map.md).

## Staffing and handoff

Default follow-up: one native `executor` owns slices 1–3 and integration; one `verifier` checks evidence after implementation. An `architect` reviews any scope change; a `critic` reviews this plan after the architect. Available relevant roles: `executor`, `architect`, `critic`, `verifier`, `test-engineer`, `designer`, `code-reviewer`. Prefer inherited models; suggest high reasoning for guard/navigation work and medium for bounded locale/test updates.

Parallel execution only pays after contracts stabilize: an executor owns components/helpers; a separate test-engineer owns E2E files and evidence setup, with locale files assigned to exactly one owner. Shared files require explicit handoff. No agent independently expands scope or reverts another agent's work. The integration owner reruns the cross-slice focused suites once.

If a later user explicitly requests OMX CLI execution, use `$ralph` for a single persistent owner; ensure `.omx/plans/prd-*.md` and `test-spec-*.md` entrypoints exist before implementation. For Team, a concrete launch hint is `omx team 2:executor "Implement .trellis/tasks/09-26-skill-library-template-entry/implement.md; allocate components/helpers to one lane and E2E evidence to the other; preserve file ownership"` (or `$team` with the same task/ownership text). These runtime commands are not launched by this plan and require OMX runtime availability.

Team verification path: each lane reports changed files, commands, failures, and artifacts; integration owner runs shared tests including the stable-page-session suite, lint/typechecks, and all three required E2E targets; verifier confirms every AC and ownership/build evidence before team shutdown. Ralph follow-up verifies the integrated diff and unresolved checks instead of repeating already valid identical runs.

## Risks and mitigations

- Draft loss on Desktop navigation or same-workspace query transitions: guard all Desktop adapter intents using original-gesture snapshots, keep one stable page-level dialog host, and cover both boundaries without changing tab-store semantics. Preserve request workspace affinity and recovery completion rules.
- Misleading counts/links: canonical pure matrices plus no-data versus cached-data query tests; preserve all matches and permission rules.
- Summary drift into persisted content: distinct summary field, unchanged source defaults/search/adoption paths, source-sync suite and request assertions.
- Narrow/translated overflow: four-locale rendering smoke tests; production wide/narrow/light/dark screenshots and keyboard checks. Use `visual-verdict` each iteration and persist its result at `.omx/state/skill-library-template-entry/ralph-progress.json` without activating an OMX runtime in App mode.
- Broad test-environment side effects: task-owned production services only; use the verified commands/prerequisites in the research map; report any unrun gate honestly.

Definition of done: AC1–AC10 satisfied with recorded evidence, no pending failures or accidental contract changes, changed-file/simplification/risk report prepared. Current deliverable stops at reviewed planning documents; product implementation, push, and task activation are not authorized. Planning-document disposition belongs to the parent workflow.
