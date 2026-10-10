# I1 post-baseline quality review — 2026-10-10

This is a scoped review of the changes after the original accepted product head
`e516b15c29328c893d8f384b6e7bd8d7a9dc9c39`. It supplements the dated 2026-10-07
acceptance; it does not replace that evidence with a claim that the entire
current checkout passed the original full pipeline.

Review HEAD: `f34cb736ac186be6aa2d45343be906f0a2b861f7` on `codex/projects-p1`.
HEAD moved from `8777f75e2` to this documentation-only archive cleanup while the
review was starting. No product source was edited by this reviewer. No push,
merge, deployment or workspace enablement was performed.

## Findings and evidence reuse

No I1 product defect was found in the reviewed scope projection, metric
selection, protected complete reads, frozen-source selection and platform-link
paths. Counts still come from the service/snapshot; the new projection labels
stored transitions and marks incomplete evidence rather than replacing those
counts. Start sequence boundaries and planning/lifecycle exclusions agree with
the canonical history path in `server/internal/iteration/history.go`.

| Commit | Effect on I1 acceptance | Evidence assessment |
| --- | --- | --- |
| `6d3bf906d` | Changes the shared scope/activity UI, adds the core scope projection and five real-API Web/Electron scenarios. It affects phase semantics, task/event count explanations, filtering, frozen history, navigation and accessibility. | The [scope business task](../archive/2026-10/10-09-iteration-scope-business/verification.md) records 10,564 unit tests, 11 static tasks and 5 production browser scenarios. All 17 SHA-256 entries in its `browser-evidence/verification-source.json` match the current checkout and the implementation commit. All six retained raw-log hashes in `evidence/checks-summary.json` match. Its final Playwright report has 5 expected, 0 unexpected, 0 skipped, 0 flaky and no top-level errors, in 34,656 ms. This evidence is reusable for the unchanged paths. |
| `c6e647d90` | Changes the independent triage history presentation and filter feedback. It does not change iteration admission, assignment, API or backend behavior. | The [triage timeline task](../archive/2026-10/10-10-triage-history-timeline/verification.md) records 122 triage/parity tests, platform typechecks and fixture-browser evidence. Its product paths are unchanged after that commit. The current focused tests/typechecks below check the combined frontend source. Fixture-browser evidence must not be called a live API or native Electron run. |
| `b2403ae5f` | Archives prior iteration detail presentation evidence; no application or E2E source changes. | The [detail UI record](../archive/2026-10/10-09-iteration-progress-scope-ui/verification.json) retains 20 bilingual production Desktop captures and keyboard/activity evidence. This is older evidence with its own source identity. The newer scope task above supplies the relevant current scope evidence. |

After `6d3bf906d`, the only committed product-path changes through review HEAD
are the triage paths from `c6e647d90`. The I1/scope source, tests, locales and E2E
fixture retained the recorded bytes throughout this review.

The original I1 task/handoff still describes `e516b15c2` as the final product
head and says the `iterations_i1` deployment gate remains off. The historical
claims were true on their recorded date, but they are not the current product
contract: `cba6ad302` removed that deployment gate and retains default-disabled
workspaces with explicit human administrator enablement. Current
`iteration_settings.go` reports supported capability and the persisted
workspace `enabled` value. The [workspace settings verification](../archive/2026-10/10-08-iteration-workspace-settings/verification.md)
records the separate acceptance and its preserved failures. The main session
should add a dated evidence mapping and current remaining-gate statement to the
parent/verification handoff; it should preserve the original historical record
and not restore the removed feature gate.

## Commands executed in this review

All commands below exited 0. Counts overlap the earlier focused probe and are
not summed as independent new scenarios. Output was read from the Codex tool
session; this review did not create replacement raw logs for these commands.

| Command | Result |
| --- | --- |
| `pnpm --filter @multica/core exec vitest run iterations/scope.test.ts --reporter=dot` | 1 file / 79 tests passed. |
| `pnpm --filter @multica/views exec vitest run iterations/iteration-events.test.tsx iterations/iteration-details.test.tsx iterations/iteration-navigation.test.tsx --reporter=dot` | 3 files / 100 tests passed. |
| `pnpm --filter @multica/core exec vitest run api/iteration-schemas.test.ts iterations/activity.test.ts iterations/candidates.test.ts iterations/catalogue.test.ts iterations/command.test.tsx iterations/grouped-issues.test.ts iterations/scope.test.ts iterations/timeline.test.ts --reporter=dot` | 8 files / 203 tests passed. |
| `pnpm --filter @multica/views exec vitest run iterations --reporter=dot` | 15 files / 185 tests passed. |
| `pnpm --filter @multica/views exec vitest run triage/triage-history-dates.test.ts triage/triage-history-filter-summary.test.tsx triage/triage-history.test.tsx triage/triage-page.test.tsx triage/triage-ui.test.ts triage/triage-import-dialog.test.tsx --reporter=dot` | 6 files / 38 tests passed. |
| `pnpm --filter @multica/views exec vitest run locales/parity.test.ts --reporter=dot` | 1 file / 64 tests passed. |
| `pnpm --filter @multica/core typecheck` | Passed. |
| `pnpm --filter @multica/views typecheck` | Passed. |
| `pnpm --filter @multica/web typecheck` | Passed. |
| `pnpm --filter @multica/desktop typecheck` | Node and renderer checks passed. |
| `pnpm --filter @multica/core lint` | Passed. |
| `pnpm --filter @multica/views lint` | Passed, 0 errors / 27 warnings outside the reviewed iteration/triage changes. |
| `git diff --check` | Passed. |

The views iteration run retained React test warnings in the existing form and
settings tests (`act` wrapping and a render-time settings update). pnpm retained
its existing warning that the `package.json` `pnpm` field is no longer read.
Neither is reported as a warning-free run. No assertion failed.

## Remaining boundaries

- This reviewer did not run remote CI, a new complete `make check`, Go tests,
  migration/performance rehearsals, production builds or browser/Electron
  scenarios. The identified new product commit `6d3bf906d` has no backend,
  migration or dependency changes; its verified unchanged source and retained
  logs allow reuse of that task's affected checks.
- The original VG remote-CI statement needs reconciliation by the main session
  with later release evidence and the source under review. This review neither
  claims remote CI is absent from every later release nor marks VG complete.
- During the review, another session added unrelated changes in Desktop tabs,
  agent views, skill views and desktop locales. Those changes were preserved.
  Consequently this is not frozen whole-checkout validation, even though the
  reviewed I1 and triage paths remained unchanged. `apps/web/next-env.d.ts` was
  already modified and was also preserved.
- Packaged Windows/Linux clients, mobile device/assistive-technology coverage,
  deployment and pilot activation are not established by this report.
