# Frontend review fixes

2026-10-05. Responds to `../10-05-projects-p1-verification/frontend-review.md`. Read receiving-code-review and systematic-debugging guidance, checked each report against the current source, and retained the parent API contract. This is implementation/regression evidence; final production-browser and independent re-review verdicts remain separate.

| Finding | Reproduction / RED evidence | Fix and GREEN evidence |
| --- | --- | --- |
| FR-02 | New exact-wire test failed: `snapshot_version` was null while `version=A` was sent. | Serialize API snapshot version under `snapshot_version`; UI URL keeps `version`. `project-p1-client.test.ts` passes the exact outgoing-query assertion. |
| FR-03 | New real QueryClient component test loaded a clear overview, failed refresh, then failed to find any stale/error disclosure. | Retained data is explicitly marked stale; current healthy assertion and exact-risk buttons are suppressed until refresh succeeds. `project-management.test.tsx` refresh-failure case passes. |
| FR-04 | Barrier tests failed for both late update success and rollback: a previously cleared Project containing private description was restored. | Central access epochs guard project reads and all project/configuration/resource/progress writes; revocation scrubs project mutation variables/context/result/errors and workspace-scoped candidate queries before navigation. Description403 immediately hides its editor and clears drafts. Mutation barrier tests and the no-WS description403 UI regression pass. |
| FR-05 | Replayed the pre-fix `1a6e0eff4` ProjectDescription against the new regression: exit1, rejected local durable body remained after choosing server. | Explicit adoption uses a dedicated draft discard instead of the acknowledgement/rebase helper. Conflict → choose server → unmount/remount restores only server text and no retry affordance. |
| FR-06 | Replayed the pre-fix ProjectProgress against a debounce-faithful editor: cancel and navigate cases both failed to preserve the last text. | Opt into editor unmount flush, explicitly capture cancel input, and gate writes on access status. A deletion-only local-text flush captures pending keystrokes before copy-only transition; true revocation does not flush or re-persist. Four relevant UI cases pass. |
| FR-07 | Pre-fix ProjectProgress replay failed to expose the internal issue evidence link. Review also established that `/tasks/:id` does not exist. | Issue evidence uses the current workspace path builder/AppLink. Execution evidence uses the new authorized revision-specific reader and the existing AgentTranscriptDialog; no old `listTaskMessages` shortcut. Reader schema checks parent/task/message identities. API and UI wiring regressions pass. |
| FR-08 | Review demonstrated old404 followed the same erasure path as membership loss. No separate pre-edit mounted-UI RED run was claimed for this case. | `project_not_found` has a distinct deleted state: remove fetched/project/evidence data, stop submission, retain only unsent user text for copying. A subsequent true403 erases it; late reads remain404 and do not convert deletion into workspace revocation. UI/local-text and core late-read cases pass. |
| FR-09 | Review demonstrated normal property errors had no rendering or retained-patch path. No separate pre-edit mounted-UI RED run was claimed for this case. | Shared property recovery is wired to detail/list/card edits, retains attempted fields, displays validation errors, and fetches the latest revision only after explicit Retry. Status409, date422 and operation-only403 regression cases pass. |

Baseline replays temporarily loaded only this lane's earlier source into the same worktree, ran the named focused test, and restored current source in `finally`; no earlier implementation was left in the working tree. These were differential regression checks performed during review remediation, not claims that every new test preceded the original implementation.

## Permission distinctions

- Workspace/member read loss: `forbidden`403 or401 clears protected caches, mutations, drafts and copy-only recovery text.
- Evidence-only loss: `project_evidence_forbidden`403 retains the user's local text, removes any open execution-source cache and requires removing inaccessible evidence before previewing again.
- Operation privilege loss: `project_permission_denied`403 retains the attempted operation for review; workspace read access remains intact.
- P1 write disabled: `project_updates_disabled`403 retains drafts. These explicit backend codes avoid guessing from human-readable errors.
- An in-flight local DELETE has a self-event guard so its realtime notification cannot invalidate its own successful mutation before navigation.

## Notification and execution reading

Added the `project_update` inbox type/label and navigation to the correct workspace project plus `update` query parameter. The linked record can be older than the current page: its existing revisions endpoint is loaded independently. Notification details remain strings. Internal evidence has typed identity and nullable href; only URL evidence uses an external anchor.

Authorized execution reader: `GET /api/projects/{id}/updates/{updateId}/revisions/{revision}/executions/{taskId}`. The progress lane owns current-source authorization and the parent owns route registration; this lane parses its real task/messages and mounts the established transcript viewer.

## Visual follow-ups

- Conflict buttons: commit `1a6e0eff4` gives each project-specific comparison action its own wrapped, full-width button. Await final rebuilt screenshot.
- Narrow-screen footer: actual390px screenshot showed the floating chat launcher covering the timezone Save button; overview now reserves112px bottom scroll clearance. Browser lane owns nine-point hit-test confirmation.
- Risk scope now displays the response's reference date, timezone and calculation time.

## Executed final checks

- Core focused regression command from implementation-evidence.md: **14 files / 180 tests passed**.
- Shared project/review/inbox regression command: **7 files / 100 tests passed**.
- `pnpm --filter @multica/core typecheck` and `lint`: passed.
- `pnpm --filter @multica/views typecheck`: passed.
- Scoped ESLint for changed project and inbox components: passed.
- `git diff --check`: passed.

Remaining verification: rebuilt Web/Desktop flows, conflict/footer screenshots and hit tests, real execution-evidence click and notification target, complete reviewer re-audit. No production deployment or final UG completion is claimed here.
