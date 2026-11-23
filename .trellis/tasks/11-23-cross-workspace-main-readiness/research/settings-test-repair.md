# Settings trigger stale-assertion repair

Date: 2026-11-23. Scope owner: native `standards_review` child, explicitly assigned this mechanical test repair. Candidate: private `.omx/private/qa-work/main-readiness-20261123/src` at `73a28ed90fdcfd9eda1ac5d2970df5af2c486f94` plus the parent's uncommitted readiness changes. Only `packages/views/settings/components/settings-page.test.tsx` was edited in that checkout; no product/source component, root counterpart, task status, staging or commit was changed.

## Cause and retained failure

The parent's `.omx/private/qa-work/main-readiness-20261123/ts-tests-r2.log:6558` records the sole Views failure: settings shell expected `xl:hidden`, while the shared trigger emitted `lg:hidden`. The run finished 630 files: 629 passed / 1 failed, 7,844 tests passed / 1 failed. This original result remains unchanged.

The `lg` contract was confirmed independently before editing:

- `76c61f95d` intentionally changed `CollapsedNavTrigger` in `packages/views/layout/page-header.tsx:47` to `lg:hidden`, so large-screen collapse retains the rail's own reachable control.
- Canonical `packages/views/layout/page-header.test.tsx:78` already expects `lg:hidden`.
- `packages/ui/hooks/use-mobile.ts:7` sets `COMPACT_BREAKPOINT=1024` (Tailwind `lg`).
- `.trellis/spec/core/frontend/global-scope-navigation.md:228` explicitly assigns the rail toggle to `lg` and larger screens and makes page/account content openers compact-only.

The settings test still described the older `xl` permanent-column model. Its assertion, test name and two-line comment were updated to the current shared contract. All settings interaction, grouping, permissions, route and focus assertions are retained. No runtime behavior was changed to satisfy the test.

## Fresh bounded verification

From private `packages/views`:

```sh
pnpm exec vitest run settings/components/settings-page.test.tsx layout/page-header.test.tsx --maxWorkers=1 --no-file-parallelism
pnpm exec eslint settings/components/settings-page.test.tsx --max-warnings=0
```

- Vitest v4.1.0: **2 files / 32 tests passed**, exit 0; settings 24 plus canonical page-header 8. Run started 19:41:04, duration 5.49 seconds.
- Scoped ESLint: exit 0, no lint warnings/errors. pnpm emitted its pre-existing package-configuration notice before both commands; it did not fail either command.
- From the private checkout root, `git diff --check -- packages/views/settings/components/settings-page.test.tsx`: passed.
- Diff: 4 insertions / 4 deletions (name, comment and breakpoint expectation only).

Final changed test SHA-256: `cb3eec98d89e745ea55c49aaf5951c29fd9ecc632ea9e46b7304dc176a451bb9`.

This is a targeted successful replay after the retained 630-file failure, not a claim that the full Views suite was rerun. The parent owns reconciliation with the unchanged-suite evidence and separate incoming-marketplace replays. No additional test, product redesign, dependency or suppression was introduced.
