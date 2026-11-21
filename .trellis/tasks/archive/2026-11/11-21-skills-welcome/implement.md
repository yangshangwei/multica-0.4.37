# Skills Welcome Execution Plan

Status: implementation authorized on 2026-11-21 by the user: “批准按此方案进入实施”.

## Phase Gate

- [x] Load Trellis and the requested Impeccable skill; run its context launcher once.
- [x] Obtain task-creation consent and create `skills-welcome` in `planning`.
- [x] Inspect current collection, create dialog, template separation, docs, locale conventions and comparable welcomes.
- [x] Converge PRD and draft design, execution plan and real context manifests.
- [x] Present the latest final planning summary and obtain subsequent explicit implementation approval.
- [x] After approval, loaded Phase 1.4, validated context and ran `task.py start`.

## Implementation Sequence

1. **Capture a narrow baseline.** Recheck the target files' current diffs because this checkout has extensive unrelated work. Load Phase 2.1 context and relevant specs. Before UI edits, read Impeccable `reference/craft-floor.md`. Do not rerun its already-executed context launcher or broaden this addition into identity work.
2. **Dispatch a bounded Trellis implementation agent.** Its prompt begins `Active task: .trellis/tasks/11-21-skills-welcome`; use native context injection with child-side loading fallback. Ownership is limited to new `skill-welcome*` files, `skills-page.tsx`, its focused tests and both `skills.json` locales. Keep implementation and review separate; no unrelated file cleanup.
3. **Lock state behavior before implementation.** Extend the existing query test rig with explicit status/data/error fields. Add meaningful regressions for successful empty data, pending and paused unknown data, first/refresh errors, populated built-in-origin records, auxiliary failures and filtered-empty recovery. Keep parsing/filter matrices in their current canonical layer.
4. **Implement the local welcome.** Add all three scenes, five real method labels, manual selection controls and both locales. Wire the existing creation session/button ref and embedded docs path. Suppress only the duplicate primary header create action while visible, retaining market access. Preserve the virtualizer, current list/card behavior and creation recovery semantics.
5. **Verify unit and static contracts.** Run the focused and existing Skills suites, locale parity, views typecheck and scoped ESLint. Check package boundaries and absence of new dependencies/API writes/automatic carousel timers.
6. **Coordinator owns focused browser acceptance.** Add a focused welcome journey using existing TestApiClient fixtures, distinct test workspace and real production-Web runtime. Cover dialog open/cancel/focus, workspace docs and template navigation, all positions, no-match recovery and empty-to-populated transition. Do not use account data or delete existing skills to manufacture an empty fixture.
7. **Batch visual evidence.** Capture Chinese light desktop at 1440×900 against the reference; English dark desktop; English and Chinese narrow states at 390×844 and 320×568; include middle and final scene states in the batch. Test reduced motion, clipped preview containment and reachable controls. Perform a Desktop smoke for the actual scroll container. Retain source/build/runtime provenance and distinguish renderer verification from packaged Main acceptance.
8. **Inspect and review.** Run visual-verdict for the captured batch and save `.omx/state/skills-welcome/ralph-progress.json`. Run Impeccable detector once over the finished changed UI files. Use the shipped fresh finish-reviewer with the reference, design, screenshots, craft-floor path and detector results. Address material findings in one batch and confirm with at most one further inspection round; report any remaining review disposition precisely.
9. **Independent quality check and finish.** Dispatch Trellis check over the final bounded diff, run applicable review/verification skills and preserve evidence. Update the relevant Skills spec only for actual implemented contracts. Follow Phase 3 commit/finish rules with an atomic scoped Lore-protocol commit if authorized by that phase; stage no unrelated changes. Do not archive until verification and owned-resource cleanup are complete.

## Proposed Commands

Run focused commands during iteration; broaden only when an observed regression or changed boundary justifies it.

```bash
pnpm --filter @multica/views exec vitest run skills/components/skills-page.test.tsx skills/components/skill-welcome.test.tsx skills/components/create-skill-dialog.test.tsx locales/parity.test.ts
pnpm --filter @multica/views exec vitest run skills/
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views exec eslint skills/components/skills-page.tsx skills/components/skills-page.test.tsx skills/components/skill-welcome.tsx skills/components/skill-welcome.test.tsx
```

The `skills/` path filter scopes the broader run to the Skills domain. JSON locale parity is the canonical structural check.

For browser acceptance, follow `.trellis/spec/web/frontend/e2e-run-environment.md` and pin a task-owned production runtime:

```bash
make status ARGS="--json"
make up C=api,web ARGS="--web-mode production"
make env-exec ARGS="-- pnpm exec playwright test e2e/skill-welcome.spec.ts e2e/skill-library-accessibility.spec.ts --workers=1 --retries=0"
```

Confirm that the selected environment belongs to this checkout and task; if it is shared, allocate a separate task environment with the repository allocator rather than restarting it. Existing processes and databases are not task-owned by default. A dev-server capture does not satisfy production acceptance.

```bash
/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json packages/views/skills/components/skill-welcome.tsx packages/views/skills/components/skill-welcome.module.css packages/views/skills/components/skills-page.tsx
```

## Review Ownership and Rollback Points

- Implementation agent: only the bounded Skills UI/locales/colocated test slice.
- Coordinator: planning, e2e fixture/journey, environment provenance, final evidence, spec updates and integration decisions.
- Check/review agents: independent review unless explicitly assigned a bounded correction.
- Roll back only newly introduced domain code and exact scoped hunks. Preserve existing list behavior and all unrelated dirty files.
- If the inherited malformed-success fallback becomes a requirement to distinguish, stop scope expansion and record it as separate API-contract work.

## Evidence to Retain

Validation command/results, exact changed-file list, screenshots opened and checked, detector output, independent code/visual verdicts, production runtime provenance, Desktop verification scope and any known gaps. Planning-time context validation is not product-test evidence.

## Accepted Outcome

Implementation, unit/static checks, both isolated builds, complete Web/native journeys and independent code/visual review are complete. Visual disposition is `ship` at 94/100 with no material fixes. The user resumed the task for closure; the feature was committed after fresh scoped checks. See `commit-plan.md` for the commit boundary, `evidence.md` for acceptance, and `research/resume-check.md` for the pre-commit recheck.
