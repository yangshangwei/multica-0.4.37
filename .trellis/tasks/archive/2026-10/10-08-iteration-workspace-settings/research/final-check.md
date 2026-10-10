# Final full-task independent check

Reviewed 2026-10-09 **only** in `/Volumes/artisan/code/2026/multica-iteration-closeout-20261009`: fixed HEAD `4afcfe0f745d786294f83fb757363c8f27351e23` plus the selected task patch. All 122 selected artifact hashes matched `candidate-selection.json` before review. Workspace dependencies resolve to this validation checkout. Assignment/sidebar partial hunks were preserved; excluded issue-property styling, navigation reordering and other-session files were not imported.

**Independent source review passed after one local accessibility fix.** No unfixed implementation blocker was found. This is not the final build/browser/full-check verdict: the main session owns the frozen-source Desktop build, isolated DB/full `check.sh`, Web/Electron and visual acceptance.

## Findings (fixed)

- File: `packages/views/iterations/iteration-overview.tsx:53`
  - Issue: Overview tabs rendered only their tablist inside `Tabs`; the timeline was outside it and had no associated `tabpanel`. The active tab could not identify its content for assistive technology (IP-12).
  - Fix: Put the existing timeline under the shared `TabsContent` panels with accessible names. Only the active panel renders rows; metadata traversal, filters, pagination and active-only detail queries retain their existing behavior. No new state, dependency or public interface.
- File: `packages/views/iterations/iteration-navigation.test.tsx:565`
  - Added a regression proving active tab/panel association, keyboard focus movement plus Enter activation, visible history content and no history detail fetch. The original implementation failed because no `tabpanel` existed. The final page/chart rerun passed all 42 tests.

## Findings (not fixed)

- **Known W5 documentation follow-up, main-owned:** `.trellis/spec/server/iterations.md` still has old deployment-rollout wording at lines 3–6, 233, 264 and 287; some task-body planning wording/checklists also await closeout reconciliation. The newer approval/closeout plan is unambiguous. Left unchanged as requested; update current contracts without rewriting historical evidence.
- No other source change was required by this review. Full runtime/visual acceptance remains assigned to the main session and is not reported as completed here.

## Full-task coverage

| Area | Assessment against the approved contracts |
| --- | --- |
| Backend availability and authorization | Runtime `iterations_i1`/`Available` gates are removed from settings, issue assignment, triage and notification startup. Capabilities remain schema version 1; missing settings is disabled/revision 1. Human owner/admin enable checks remain inside the operation transaction. Ordinary human period maintenance is not incorrectly restricted to admins. Enable verifies saved timezone/revision and creates neither periods nor executions. |
| Atomic disable, frozen history, recovery | Complete preview selects every open period and enforces the 2,000-issue limit. Apply rechecks authorization/hash and commits closure snapshots, current-membership removal, settings, receipt and notification outbox together. Frozen reads use stored snapshot facts/statistics. Operation replay precedes mutable enabled checks and cannot re-enable a later-disabled workspace. History/recovery reads retain current-membership authorization and do not require enabled state. |
| Notifications and compatibility | New enable publishes after successful commit, not receipt replay. Delivery can resume retained outbox work after disable; new overdue scans still require enabled active periods. Schema/identity validation, old-server capability probing and qualified access-denied handling remain intact. Config samples/offline instructions remove deployment gating while preserving operator overlays and warning about previously hidden enabled workspaces. No migration/dependency added. |
| Core cache/catalogue/timeline | Local commands and iteration events invalidate iterations, issues, triage capability and shared timezone. Ordinary relevant events still invalidate projections before specific-event early returns. Catalogue checks complete pagination, duplicate identities/cursors, workspace identity and one bounded stale-cursor restart. Timeline preserves global upcoming identity, descending groups and conservative unfiltered gap inference. Session/access epochs and durable original payloads are retained. |
| Settings and shared timezone | Navigation is discoverable while disabled; sidebar requires confirmed supported/manual/enabled capability. Read-only member state, independent enable/timezone commands, dirty/pending/unconfirmed locks and recovery outside availability gates are present. The closeout fix derives the live status announcement from saved settings and removes stale enable-success text. Timezone confirmation reads, explicit retry and preserved dirty drafts remain separate from enable; no unsupported CAS promise is introduced. |
| Overview, detail and charts | Complete metadata powers compact planned/history rows and current-only statistics. Detail retains entity keys, mounted task filters and visited progress/event state. Saved period timezone is always shown. Transient first-read failures can retry without replaying writes; definitive denied/deleted reads remove protected content. Chart and accessible table share response points, frozen statistics take precedence, and cancelled/unknown data is not fabricated. Localized date parts remain intact. Overview tab-panel association was fixed in this review. |
| Forms, assignment and creation | Existing complete preview/hash/recovery pipelines remain. Expanded dialogs retain drafts and disable fresh writes after availability changes. Manual creation carries explicit workspace/period identity and the reviewed revision into the original create transaction, retains invalid selections for explicit correction, and does not silently switch to agent creation. The inherited searchable picker continues to own search/keyboard/clear behavior. |
| Shared Dialog | `keepMounted` is opt-in and defaults false. It forwards to Base UI's portal; installed Base UI still hides the unmounted-state popup. Existing regression covers closing/reopening an editor without losing its draft. No business logic entered the UI package. |
| E2E wiring | Formal settings tests now query the intentionally modal-hidden background switch only for the checked-state assertion. Normal enable/disable/re-enable stays on one mounted settings page; lost-response recovery is a distinct flow retaining request identity and sibling-client/history assertions. Web adds real read-retry, searched selection/clear/create and keyboard data-table wiring. Electron reuses the normal settings flow and existing closure recovery flow. These specs were inspected, not executed by this reviewer. |

## Verification

Logs: `.omx/state/iteration-closeout-20261009/final-check/`. Commands run from the validation root unless noted. Results below are separate runs, not an additive test total.

| Check | Exact command | Result |
| --- | --- | --- |
| Initial affected views regression | `pnpm -C packages/views exec vitest run iterations settings/components/workspace-planning-timezone.test.tsx settings/components/settings-page.test.tsx settings/components/settings-nav.test.ts layout/app-sidebar.test.tsx modals/create-issue.test.tsx modals/create-issue-dialog.test.tsx locales/parity.test.ts --maxWorkers=2` | Exit 0; 17 files / 316 tests; `views-tests.log`. |
| Core regression | `pnpm -C packages/core exec vitest run iterations api/iteration-client.test.ts api/iteration-schemas.test.ts projects/planning-timezone.test.tsx realtime/use-realtime-sync-ws-instance.test.tsx --maxWorkers=2` | Exit 0; 14 files / 159 tests; `core-tests.log`. |
| Shared package TypeCheck | `pnpm --filter @multica/core --filter @multica/ui --filter @multica/views typecheck` | Exit 0; `typecheck.log`. |
| Shared package Lint | `pnpm --filter @multica/core --filter @multica/ui --filter @multica/views lint` | Exit 0; 0 errors; 3 existing UI and 27 existing views warnings; `lint.log`. |
| Pure Go regression | In `server`: `env -u DATABASE_URL bash ../scripts/go-test-with-agent-cli-guard.sh go test ./internal/iteration ./internal/featureflags -count=1 -v` | Exit 0; 37 iteration + 7 featureflags tests, 0 skips; `go-tests.log`. |
| Go static analysis | In `server`: `env -u DATABASE_URL go vet ./internal/iteration ./internal/featureflags ./internal/service ./internal/handler ./cmd/server` | Exit 0; `go-vet.log`. |
| New accessibility regression, before fix | `pnpm -C packages/views exec vitest run iterations/iteration-navigation.test.tsx -t 'connects the overview tabs' --maxWorkers=2` | Expected exit 1: missing named tabpanel; `overview-tabs-red.log`. |
| After local fix | `pnpm -C packages/views exec vitest run iterations/iteration-navigation.test.tsx iterations/iteration-chart-closeout.test.tsx --maxWorkers=2` | Exit 0; 2 files / 42 tests; `overview-tabs-verified.log`. |
| Final affected TypeCheck | `pnpm --filter @multica/views typecheck` | Exit 0; `views-typecheck-final.log`. |
| Final affected Lint | `pnpm --filter @multica/views lint` | Exit 0; 0 errors, same 27 unrelated warnings; `views-lint-final.log`. |
| Diff check | `git diff HEAD --check` | Exit 0. |

`overview-tabs-green.log` preserves an intermediate test-authoring failure: the first version incorrectly expected arrow focus to activate a tab. Installed Base UI uses manual activation by default. The corrected regression presses Enter, preserving existing product semantics; the verified run is green. pnpm's existing root configuration notice is unchanged. No browser, service, DB-backed suite or ambient agent CLI was launched by this reviewer.

## Integration handoff

Source edits are finished. Integrate **only** these two files' review increments:

- `packages/views/iterations/iteration-overview.tsx`
- `packages/views/iterations/iteration-navigation.test.tsx`

`final-check/local-fixes.patch` contains the unstaged increment against the already-staged candidate, not the full original task patch. `final-check/changed-files.json` records before/after SHA-256 and confirms every other selected artifact stayed unchanged. Also retain this report and the verification logs. No commit, staging, archival or main-checkout source edit was performed.
