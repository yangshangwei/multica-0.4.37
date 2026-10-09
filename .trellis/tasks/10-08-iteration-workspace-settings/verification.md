# Iteration settings and business-page verification

Status: implementation and targeted repair verification are complete. The final combined browser rerun, scoped Git commits and task archival are pending restoration of the session's write/network permissions. This document does not mark the task complete.

The user explicitly authorized execution of [closeout-plan.md](closeout-plan.md). The validated candidate was assembled from `4afcfe0f745d786294f83fb757363c8f27351e23` plus the selected task patch in `/Volumes/artisan/code/2026/multica-iteration-closeout-20261009`. Root work continued concurrently; unrelated issue-property, progress/scope styling and triage changes were preserved.

## Delivered changes

- Workspace-owned iteration enablement replaces the deployment gate while retaining default-disabled workspaces, human/admin checks, atomic disable, historical facts and durable request recovery.
- Settings use the query-owned state as accessible live feedback. The stale enable-success paragraph was removed, so a normal or external disable cannot retain a contradictory success message.
- Detail headers expose the saved IANA timezone. Temporary first reads can be retried in-page; access denial/deletion stay distinct and remove protected cached details/charts.
- Overview and detail share the chart data table. Overview has a keyboard-operable disclosure and associated tab panels. Locale date parts preserve day suffix grouping; decorative glyphs use the existing solid semantic tone.
- Real Web/Electron settings tests cover same-mount normal disable without reload, separately from lost-response recovery. The new browser scenario covers 503→200 retry, searchable selection/clear/reselection, actual task membership and chart keyboard access.
- Two Go tests were aligned with the implemented contract: workspace opt-in routes replace the obsolete deployment-closed assertion, and the project-health fixture explicitly chooses UTC rather than depending on a midnight-sensitive default.
- Existing project E2E tests now use shared workspace timezone settings, constrained picker behavior, real 422/403 responses and the actual publish control. Risk IDs/counts remain strict; computation timezone is verified in response metadata, without restoring intentionally removed repeated UI text.
- Server/core specs and obsolete planning approval clauses were updated. [Parent acceptance](acceptance-matrix.md) and [business-page acceptance](../10-08-iteration-pages-design/acceptance-matrix.md) identify the canonical evidence per criterion.

## Verification evidence

| Check | Actual result |
| --- | --- |
| Independent full-task review | No remaining implementation blocker; its overview tab/panel fix was integrated. [Report](research/final-check.md). |
| Root pipeline static stage | 15 lint/typecheck tasks and UI export validation passed. Existing unrelated warnings retained. |
| Non-mobile TypeScript | 883 files, **10,340 tests passed**: core 2,771; docs 62; views 6,263; desktop 962; web 282. |
| Go | **70 package results passed** under the normal guarded race entry point; vet completed before API/Web startup. The isolated Go database was cleaned by the pipeline. |
| Builds | Desktop renderer and production Next build completed. Running API/Web provenance was retained. |
| Original complete browser run | **251 passed, 9 failed, 50 conditional skips**, zero retries. The failed run remains recorded; it is not relabelled successful. |
| Repairs of those 9 browser cases | **9/9 passed**, zero retries/skips, in two focused runs: the I1 case and eight P1 cases. TypeScript/lint for the repaired tests passed. |
| Final real UI inspection | One additional browser scenario passed; Chinese 1440px/900px, dark mode, saved timezone and chart-table keyboard assertions passed. Visual verdict **95/pass**. |
| Attempt after permission restriction | The root regression runner did not execute tests: pnpm could not verify its registry signature because fetches failed under the restricted network. No signature check was disabled. |

These are separate executions, not one final all-green E2E invocation. The manifest covers 6,720 source/config/test files. Comparison after the successful TypeScript/Go/build stages found changes only in the three repaired E2E files; production and unit-test inputs are unchanged. See [source verification](final-evidence/source-verification.json) and [test summary](final-evidence/test-summary.json).

Detailed follow-ups: [I1 browser](final-evidence/i1-e2e-followup.md), [P1 browser](final-evidence/p1-e2e-followup.json), [Go fixtures](final-evidence/go-followup.json), [contrast](final-evidence/contrast-followup.json).

## Screenshots

- [Settings](final-evidence/screens/settings.png)
- [Overview with data table](final-evidence/screens/overview-data.png), [900px](final-evidence/screens/overview-900.png), [dark](final-evidence/screens/overview-dark.png)
- [Planned empty state](final-evidence/screens/planned-empty.png), [current detail](final-evidence/screens/current-detail.png), [progress](final-evidence/screens/progress.png)
- [Visual verdict](final-evidence/visual-verdict.json)

The screenshots use synthetic fixture workspaces and actual API data. Fixture workspaces were cleaned. A one-day chart contains real points rather than an invented multi-day curve.

## Preserved failures and fixes

1. `full-check.log` stopped on the shared text-tone guard. Replacing one decorative opacity class with `text-faint-foreground` passed 23 contrast and 42 page tests.
2. `full-check-contrast-fixed.log` passed all TS tests and exposed the obsolete route assertion and UTC fixture assumption. Both precise race regressions passed after test-only repairs.
3. `full-check-final.log` passed static, TS, Go, vet and production builds, then reported the nine E2E failures above. Their focused repaired executions are green.
4. The initial visual harness had an incorrect relative import; it ran no tests. Correcting the harness path produced the successful visual execution. No application code was changed for that setup error.

Raw logs, source manifests and the selected-patch ledger remain under `.omx/state/iteration-closeout-20261009/`. The reviewable code patch is `verified-code.patch`, with its SHA-256 file, based on the candidate's fixed base. It is an artifact for the agent's later scoped commit, not an instruction to overwrite the concurrently modified root tree.

## Permission boundary and next action

The session changed to a managed profile: the main checkout is writable, `.git` and the sibling validation worktree are read-only, and network access is restricted. The local verification server was still listening on port 18792, but a normal tool connection failed. The package manager also refused to start when it could not fetch signature material.

No Git or sandbox workaround was used. The tested E2E repairs were copied/merged into the writable root, preserving concurrent history-presentation assertions. The task remains `in_progress`; no commit, archive or release is claimed.

After permissions are restored: reconcile the current HEAD/concurrent hunks with the selected candidate, run the complete browser suite with the repaired test files, verify the staged application files against accepted inputs, create Conventional/Lore commits, then archive the business-page child and this parent and record the journal. Unrelated tasks and changes retain their owners.
