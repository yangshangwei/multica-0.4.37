# Iteration E2E alert-locator follow-up

Worktree: `/Volumes/artisan/code/2026/multica-iteration-closeout-20261009`.
Only source change: `e2e/iterations-i1.spec.ts`.

## Failure and repair

The full candidate run reached the intended initial detail GET 503, then failed at the global `getByRole("alert")` assertion. Its matches were the iteration read-error paragraph and Next's `__next-route-announcer__`. The original failure remains in the main `full-check-final.log` and this candidate's original `test-results/iterations-i1-I1-Web-retri-94786-pens-chart-data-by-keyboard-chromium/` directory.

Both pre- and post-retry assertions now share one locator scoped to `main`, role `alert`, and the exact business error text: `Could not load iteration data. Try again.`. No product code or functional expectations changed.

The repair was applied only after the main session's READY signal, following its production API/Web restart. No service/configuration changes, manual database cleanup or root-checkout edits were performed by this lane; the existing fixture cleaned up its own workspace.

## Verification

Owned environment: `check-20261008182302-93606`, API port 18792, production Web port 13712.

```sh
bash scripts/dev-env.sh exec check-20261008182302-93606 -- env MULTICA_RUN_I1_E2E=1 pnpm exec playwright test e2e/iterations-i1.spec.ts --grep 'retries detail reads' --workers=1 --retries=0 --output=.omx/state/iteration-closeout-20261009/e2e-followup/results
pnpm exec tsc --noEmit --strict --target ES2022 --lib ES2023,DOM --module ESNext --moduleResolution Bundler --esModuleInterop --skipLibCheck --types node e2e/iterations-i1.spec.ts
git diff --check -- e2e/iterations-i1.spec.ts
```

- Focused browser run: **1 passed, 0 failed, 0 retries, 0 skipped**, exit 0; 4.4 seconds total, 3.1 seconds for the case. See `focused.log` and `results/`.
- Strict TypeScript: exit 0, no diagnostics. See `typecheck.log`.
- Diff whitespace check: exit 0.

The browser run executed the unchanged 503-to-real-200 recovery without reloading, saved detail timezone, source-to-destination picker search/Enter/clear/reselection, submitted destination ID/revision, persisted target and empty source membership, zero agent dispatch, and keyboard opening/closing of the overview's real chart data.

The original full-run failure is retained. This focused result verifies the repaired case; it does not relabel that full run or claim unrelated E2E failures passed. No further changes are needed in this lane.
