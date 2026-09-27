# B1 cache hit-rate evidence

- Worktree: `/Volumes/artisan/code/2026/multica-upstream-b1`
- Baseline: `7845de31e6bc7d021bba6005bccdab44fc9958e6`
- Approved upstream: `470fd1fc0dcbbb8b86a8d13c6b4c43b6f2a10e1f`
- Implementation scope: exactly the six source/test paths in this task's PRD. No dependencies, locales, core APIs, prices, schemas, database, ledger or commits changed by this executor.
- Logs: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-upstream-b1-execution-amgeklaq/`

## Baseline

Command: `corepack pnpm --filter @multica/views exec vitest run issues/components/issue-usage-dialog.test.tsx runtimes/components/usage-section.test.tsx runtimes/utils.test.ts --maxWorkers=2`

Exit 0: **3 test files, 106 tests passed**. Log: `b1-cache-baseline.log`.

## Observable red

Added the upstream issue-dialog regression before changing production code. The fixture reports 0 plain-input tokens, 72,000 cache-read tokens and 28,000 cache-write tokens. It expects the rendered `72% hit rate`.

Command: `corepack pnpm --filter @multica/views exec vitest run issues/components/issue-usage-dialog.test.tsx -t 'includes cache writes in the cache hit-rate denominator' --maxWorkers=1`

Exit 1: **1 failed, 5 skipped**. The failure DOM explicitly contained **`100% hit rate · 72K read`**, proving the old arithmetic error rather than a missing helper or compilation error. Log: `b1-cache-red.log`.

## Green and scope checks

Ported the approved six-file patch and its canonical tests; the initial red test is retained once, without a duplicate matrix. Both views now use `cacheHitRatePercent(input, cacheRead, cacheWrite)`. Output tokens stay outside the denominator, zero input-side usage displays unavailable, and only a complete hit displays 100%.

- Repeated the baseline command: exit 0, **3 test files, 113 tests passed**. Log: `b1-cache-green.log`.
- `corepack pnpm --filter @multica/views exec eslint issues/components/issue-usage-dialog.test.tsx issues/components/issue-usage-dialog.tsx runtimes/components/usage-section.test.tsx runtimes/components/usage-section.tsx runtimes/utils.test.ts runtimes/utils.ts`: exit 0, **0 errors, 12 warnings**. Log: `b1-cache-lint.log`.
- Re-linted the three warning-producing files from baseline Git blobs via ESLint stdin: the same **3 + 4 + 5 warnings** already exist. They are pricing subscription dependencies and unused disable directives, outside this fix. Log: `b1-cache-lint-baseline.log`.
- `corepack pnpm --filter @multica/views exec tsc --noEmit --incremental false`: exit 0. Log: `b1-cache-typecheck.log`.
- Scoped `git diff --check`: exit 0. The stable patch ID for the local six-file diff and the upstream diff is identical: `c427bcdabce593e22e479b6dbadfa55b2bedadd9`. Diff size: **6 files, 142 insertions, 19 deletions**. Log: `b1-cache-static.log`.

No global tests, services or packaged-browser checks were run by this executor. Leader review, integration verification and commits remain separate responsibilities.
