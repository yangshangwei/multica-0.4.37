# Verification — iteration task-tab density

## Result

The four approved improvements are implemented and independently reviewed. No actionable code defects remain. The shared Web/Desktop view preserves lifecycle, snapshot and query ownership contracts.

## Executed checks

| Check | Result |
| --- | --- |
| Focused views regressions and locale parity | 156 passed across 5 files |
| Project lint | 6 tasks successful; 0 errors; existing unrelated warnings retained |
| Project typecheck | 9 tasks successful |
| Independent reviewer | Task-file lint, views typecheck and diff check passed; no fixes required |
| Full JS/TS suite, bounded concurrency | 10,659 passed in 897 files across 5 packages |
| UI export static check | 62 primitive/export files clean |
| Impeccable detector on changed markup | Exit 0, `[]` |
| Isolated production Electron build | Passed |
| Native audit and bilingual toolbar | 3 passed, 19.2 seconds |
| Native bilingual scope/history flows | 2 passed, 23.5 seconds |
| Visual verdict | 93/100, pass; no follow-up visual edits |

The first default-concurrency `pnpm test` run hit 5-second timeouts in untouched authentication, desktop tooling/runtime and workspace-timezone tests. No timeouts or assertions were weakened. The same complete test set passed with package concurrency 1 and four Vitest workers:

```bash
pnpm exec turbo test --filter='!@multica/mobile' --concurrency=1 -- --maxWorkers=4
```

Package counts: core 2,918 / 235 files; views 6,427 / 531; docs 62 / 9; desktop 970 / 86; web 282 / 36. Duration: 3m52.694s. Logs were retained under the task-owned external runtime directory.

## Native execution and evidence

Build and source identity: [desktop-build.json](verification/desktop-build.json).
The recorded 1,480-source fingerprint remained unchanged through native execution.
The compiled renderer and preload used isolated output and a task-only API, not the mismatched existing development environment.

Commands used the sanitized runtime wrapper; native specs require the explicit
`MULTICA_RUN_I1_E2E=1` test gate. An initial invocation without the gate skipped
three tests and is not counted as verification.

```bash
python3 /tmp/multica-iteration-density-20261010T115735Z-82e616/run.py exec env MULTICA_RUN_I1_E2E=1 pnpm exec playwright test e2e/iterations-audit-desktop.spec.ts --workers=1 --retries=0
python3 /tmp/multica-iteration-density-20261010T115735Z-82e616/run.py exec env MULTICA_RUN_I1_E2E=1 pnpm exec playwright test e2e/iteration-scope-business-desktop.spec.ts --workers=1 --retries=0
```

Screenshots and measurements retained:

- [Chinese planned layout](verification/tasks-zh-Hans-planned-wide.png)
- [Chinese narrow filters](verification/tasks-zh-Hans-filtered-narrow.png)
- [English planned layout](verification/tasks-en-planned-wide.png)
- [English narrow filters](verification/tasks-en-filtered-narrow.png)
- [Chinese metrics](verification/toolbar-zh-Hans.json), [English metrics](verification/toolbar-en.json)
- [Existing audit measurements](verification/audit-metrics.json)
- [Visual verdict](verification/visual-verdict.json)

At 2048 x 1088, the first task starts at y=387.5 and search measures 288 x 32 CSS px. At 680 x 900 with coarse input, the task panel has scroll/client widths of 605/605 and measured controls/chips are at least 44px tall. Two active conditions stay visible after the popup closes; individual removal and reset preserve the expected query/grouping state.

The supplied reference first task began near y=790. Its zoom was not instrumented, so this is an image-level density comparison, not a calibrated same-zoom pixel diff. The controlled new render independently satisfies the upper-half target.

## Acceptance mapping

| Criterion | Evidence |
| --- | --- |
| AC1 compact first screen | Both planned screenshots and toolbar geometry; reference zoom limitation above |
| AC2 all controls and narrow layout | Canonical detail tests, native grouping/filter controls and 605/605 overflow measurement |
| AC3 lifecycle-aware scope | Navigation/detail tests, empty-start and unknown-phase regressions; native scope lifecycle flow |
| AC4 unchanged whole-period counters | Focused regressions and real-API filtered planned count remains 2 |
| AC5 visible conditions and reset | Native two-condition popup-close, individual removal and reset with grouping retained; zero-match unit regression |
| AC6 retention/pagination/no traversal | Canonical detail/navigation tests, audit request observer and native scope tab-retention flow |
| AC7 meaningful rows/history | Unknown/null/rollover regressions, blocked screenshots and frozen-history native flow |
| AC8 localization/accessibility | Locale parity, both locales, real popup Escape/focus, coarse targets and existing contrast/focus audit |
| AC9 recovery states | Existing canonical loading/error/access/metadata tests and real frozen-current comparison |

## Scope and remaining limits

No API, database schema, dependency, native mobile or global primitive changes. A standalone production Next.js browser run was not repeated; shared view behavior was exercised through production Electron and Web wiring passed typecheck/unit tests. Go tests are outside this UI-only change.

Another parallel session subsequently added viewport and keyboard assertions to the existing audit test. Those additions were preserved in the working tree and excluded from this task's commit; this task's tested import, two toolbar cases and popup locator/focus changes are staged separately.

The task-only API was stopped and its disposable database, profiles and registry slot were destroyed after browser checks. Original development services and environment files were untouched. Selected evidence remains in this task; raw duplicate browser attachments and full logs remain in the external temporary runtime folder.

## Durable contracts

Added `.trellis/spec/views/frontend/iteration-task-list.md` and its index entry.
The large pre-existing core iteration spec, including unrelated edits, was left unchanged.
