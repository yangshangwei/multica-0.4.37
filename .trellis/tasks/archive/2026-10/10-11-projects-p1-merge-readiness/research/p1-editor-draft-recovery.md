# P1 draft recovery verification (2026-10-12)

## Scope and environment

Investigated the remaining P1-L14 and P1-P05 failures without weakening either
browser test. The main session authorized expanding the repair from E2E specs
to the shared editor after the investigation identified a product race.

Baseline checkout: `/Volumes/artisan/code/2026/multica-projects-p1-merge-readiness`,
commit `6a892c3d0`. The main session verified the owned environment
`check-20261011123514-33760`: API `http://localhost:18951`, production Web
`http://localhost:13871`, Web build ID `bP-4Q6tRRJZPhCBoPBSxz`, matching listener
ownership and commit. No service was started, stopped or rebuilt by this lane.

## Reproduction and root cause

The unchanged browser tests ran ten times each with one worker and zero retries:
P1-L14 passed 10/10; P1-P05 passed 6/10 and failed 4/10 at
`e2e/projects-p1-progress.spec.ts:114`. Reopening the composer returned an empty
document instead of the complete final sentence. The previous run's
`error-context.md` showed access denial after workspace fixture cleanup; that
snapshot alone did not establish a permission regression.

A temporary, read-only event observer then recorded Tiptap lifecycle events in
ten P1-P05 executions without adding waits or changing assertions. Nine passed;
one reproduced the failure. The failing instance recorded:

| Time from page start | Event | Document | Draft persisted |
| --- | --- | --- | --- |
| 1071.1 ms | DOM attached | Empty | No |
| 1084.0 ms | `update`, before initialization | Complete final sentence | No |
| 1098.9 ms | `create` | Complete final sentence | No |
| 1167.4 ms | `destroy` | Complete final sentence | No |
| 1376.3 ms | Reopened editor attached | Empty | No |

All nine successful observed instances delivered `create` before `update` and
persisted their text before destruction. The compact, credential-free evidence
is `evidence/p1-draft-event-summary.json`; raw run logs, JSON reports and the
temporary observer patch remain under `.omx/reports/projects-p1-merge-readiness/2026-10-12/` in
`p1-draft-baseline` and `p1-draft-events`. They are local diagnostic evidence,
excluded from committed task artifacts. The compact JSON retains one successful
instance and every failure, with counts for all observed event orders.

Installed `@tiptap/core` 3.27.1 mounts its editable DOM before emitting `create`
from a zero-delay timer. `ContentEditor.onCreate` previously copied the current
document into `lastEmittedRef` unconditionally. If input arrived first, this
acknowledged unsent text as already emitted. Both the debounced autosave and the
opted-in unmount flush then discarded it as unchanged. Existing editor tests
called `onCreate` synchronously from their `useEditor` mock and missed this order.

## Repair

- `packages/views/editor/content-editor.tsx`: initialize the emitted-content
  baseline only when no debounced update is pending. The existing update retains
  its original Markdown base and its normal autosave or unmount delivery.
- `packages/views/editor/content-editor.test.tsx`: exercise input followed by the
  deferred create timer, separately verifying autosave and unmount delivery once.

The original E2E steps and assertions are restored byte-for-byte. No new helper,
dependency, retry, delay, public API or timing constant was introduced. The
default behavior of discarding pending text on unmount remains covered by the
existing editor suite.

## Commands and observed results

Run from the checkout above. The baseline command was:

```sh
PATH=/opt/homebrew/opt/libpq/bin:$PATH ENV_FILE=/Users/artisan/.multica/dev/envs/check-20261011123514-33760/check.env PLAYWRIGHT_JSON_OUTPUT_FILE=.trellis/tasks/10-11-projects-p1-merge-readiness/evidence/p1-draft-baseline/report.json make env-exec MULTICA_ARGS= ARGS="check-20261011123514-33760 -- pnpm exec playwright test e2e/projects-p1-lifecycle.spec.ts e2e/projects-p1-progress.spec.ts --grep 'P1-L14|P1-P05' --workers=1 --retries=0 --repeat-each=10 --reporter=line,json --output .trellis/tasks/10-11-projects-p1-merge-readiness/evidence/p1-draft-baseline/artifacts"
```

Result: 16 passed, 4 failed, no skipped or retried tests, 67.1 seconds. The
event-diagnostic run used the same environment/prefix with only
`e2e/projects-p1-progress.spec.ts --grep 'P1-P05'`, ten repeats and its own
`p1-draft-events` output paths; result 9 passed, 1 failed in 31.8 seconds.

| Check | Result |
| --- | --- |
| `pnpm --filter @multica/views exec vitest run editor/content-editor.test.tsx -t 'preserves input before deferred creation'` before repair | Both named regressions failed: `onUpdate` was called zero times; 36 unrelated tests filtered out |
| `pnpm --filter @multica/views exec vitest run editor/content-editor.test.tsx editor/content-editor-accessibility.test.tsx projects/components/project-overview-lifecycle.test.tsx` after repair | 3 files, 44 tests passed |
| `pnpm --filter @multica/views exec eslint editor/content-editor.tsx editor/content-editor.test.tsx` | Passed, exit 0 |
| `pnpm --filter @multica/views typecheck` | Passed, exit 0 |
| `git diff --check` | Passed |

Regression names are `preserves input before deferred creation through autosave`
and `preserves input before deferred creation through unmount`. The first red
run emitted an existing uninitialized-i18next warning under the filtered test
selection; the assertions failed specifically on missing callbacks. The full
green selection emitted no test warning. The installed package-manager command
also warns about existing `package.json` pnpm settings; no dependencies or package
configuration were changed by this lane.

Unit/static output is retained under
`.omx/reports/projects-p1-merge-readiness/2026-10-12/p1-draft-unit` as
`red.log`, `green.log`, `lint.log` and `typecheck.log`. The baseline command above
records the paths used when it ran; its outputs have since been moved to the
same ignored report directory.

## Final browser acceptance

Both unchanged browser cases passed ten executions each against the final
production Web build on commit `0d1c9ca58`. The environment remained
`check-20261011123514-33760`, with API `http://localhost:18951` (listener PID
77399) and Web `http://localhost:13871` (listener PID 81139, production build
`ryOc6YqV-OUNGZ5bo-IVU`). Before execution, this lane checked those identities
against the current running records and copied `api.running.json` and
`web.running.json` into the final evidence directory.

The run started at `2026-10-11T23:47:43.472Z` and finished at approximately
`2026-10-11T23:48:27.327Z` (`2026-10-12 07:47:43.472–07:48:27.327`,
Asia/Shanghai), taking 43.855 seconds. The command exited 0. The JSON report
confirms one worker, ten repetitions, zero configured retries, and retry index
zero for all 20 actual executions.

| Case | Passed | Failed | Skipped |
| --- | --- | --- | --- |
| P1-L14 | 10 | 0 | 0 |
| P1-P05 | 10 | 0 | 0 |
| Total | 20 | 0 | 0 |

Playwright also reported zero flaky tests. The exact command, run from the same
checkout with stdout/stderr redirected to the final `run.log`, was:

```sh
PATH=/opt/homebrew/opt/libpq/bin:$PATH ENV_FILE=/Users/artisan/.multica/dev/envs/check-20261011123514-33760/check.env PLAYWRIGHT_JSON_OUTPUT_FILE=.omx/reports/projects-p1-merge-readiness/2026-10-12/p1-draft-final/report.json make env-exec MULTICA_ARGS= ARGS="check-20261011123514-33760 -- pnpm exec playwright test e2e/projects-p1-lifecycle.spec.ts e2e/projects-p1-progress.spec.ts --grep 'P1-L14|P1-P05' --workers=1 --retries=0 --repeat-each=10 --reporter=line,json --output .omx/reports/projects-p1-merge-readiness/2026-10-12/p1-draft-final/artifacts" > .omx/reports/projects-p1-merge-readiness/2026-10-12/p1-draft-final/run.log 2>&1
```

The ignored raw evidence directory is
`.omx/reports/projects-p1-merge-readiness/2026-10-12/p1-draft-final`, containing
the copied API/Web provenance, `run.log`, `report.json`, `summary.json` and
Playwright's `artifacts/` output. This final verification changed only this
record; implementation, specifications and E2E sources were left intact.
Services remain running for the main session to close. Integration, PR and
task completion remain owned by the main session.
