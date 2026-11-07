# Execution checklist

1. [x] Read operating contracts, workflow, E2E environment and database-isolation specs; record source and verified merge-base.
2. [ ] Complete business-diff and prior-defect mapping, reconcile unmerged repairs.
3. [ ] Freeze isolated source, install existing locked dependencies, create private task configuration and evidence runner.
4. [ ] Run core UI smoke on production Web; retain first result.
5. [ ] Run applicable make check stages without automatic retries; if a gate fails, retain it and continue independent coverage.
6. [ ] Run collaboration/handoff/memory/recall/knowledge and password/admin mode suites with effective config proof.
7. [ ] Build matching Desktop and exercise applicable native suites; record package/OS/provider blockers individually.
8. [ ] Add only necessary test gaps; no product fixes. Validate raw counts and collect persistence/recovery evidence.
9. [ ] Independently review coverage, severity and evidence; publish scope.md, coverage.csv/json, report.md, defects.md, evidence-index.md and remaining risks.
10. [ ] Verify cleanup and final source/worktree state; preserve unresolved failures and do not claim full acceptance.

## Commands
- PATH includes /opt/homebrew/opt/libpq/bin (installed psql was absent from initial PATH).
- Isolated guarded regression: ENV_FILE=<private task file> MULTICA_DEV_HOME=<owned registry> make check.
- Split fail-fast stages when necessary: pnpm exec turbo run lint typecheck --filter=!@multica/mobile --concurrency=1 --force; pnpm check:ui-exports; pnpm exec turbo run test --filter=!@multica/mobile --concurrency=1 --force -- --maxWorkers=2; bash scripts/test-go.sh --race; go vet -p 2 ./... .
- Playwright: pnpm exec playwright test --workers=1 --retries=0 --reporter=json with unique output/result paths and no source-assertion changes.
- All commands run under the recorded private environment; DB-backed TestMain is never probed without explicit task DATABASE_URL.

## Final assessment

Execution and delivery completed with failures/blockers. Acceptance remains NOT PASSED; no product repair or final candidate approval occurred. Gates and skip limits are disclosed in report.md.

## Gates and rollback
Before starting, review this plan. Before any code addition, load trellis-before-dev and applicable layer specs. Stop expensive/provider-dependent tests after a critical smoke failure but continue independent deterministic coverage. Preserve the original failing DB and use a new one for any full rerun. Cleanup uses ownership manifests, never broad process kills.

## 2026-11-07 authorized defect repair continuation

The user explicitly requested diagnosis and repair of the remaining2 E2E and4 frontend unit failures. This supersedes R08/no-product-fix only for QA-20261107-001,003,004 and necessary regression coverage. Preserve earlier acceptance evidence and unrelated platform/provider exclusions.

Acceptance: canonical diagnostic-path and Skill presentation suites pass without weakened assertions; production Web and real Electron full detail journeys pass the original exact620 restoration assertion on the repaired candidate; relevant focused regressions plus lint/typecheck pass. No paid model calls or unrelated refactors.

Execute the bounded repair plan in `research/failure-repair-20261107/plan.md`; original checklist/report remains historical.
