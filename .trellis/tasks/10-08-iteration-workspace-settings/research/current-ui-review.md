# Current iteration UI readiness review

Reviewed on 2026-10-08 (Asia/Shanghai), branch `codex/projects-p1`, HEAD `ed418766266de8e1049f333a7d903fbec86df627` plus the existing worktree changes.

Scope: shared iteration settings, overview/detail, assignment and manual create integration, settings/sidebar wiring, and the optional shared Dialog retention change. This was an inspection. No product source, tests, plans, services, commits or task lifecycle state were changed by this reviewer.

The native check context, parent and child PRD/design/implementation artifacts, current specs and `verification-progress.md` were read. The approval comments and progress ledger supersede the stale planning-only headings.

## Readiness assessment

The settings and business UI are implemented, not merely represented by a prototype. The reviewed settings/layout revision passes targeted component tests and package checks. It is not yet accurate to call every acceptance criterion complete: one misleading status message remains, there are smaller presentation/recovery gaps, and real Web/Electron acceptance is owned by the main session.

| Feature | Actual implementation and evidence | Assessment |
| --- | --- | --- |
| Discoverable settings | `settings-nav.ts:107` registers Iterations after Triage, without an enablement gate; `settings-page.tsx:251` mounts the shared tab. Public `?tab=iterations` regression passes. | Implemented |
| Enable and read-only roles | `iteration-settings-tab.tsx:37` separates capability and workspace state; lines 41–74 enforce owner/admin UI and send the displayed timezone with the settings revision. The switch waits for the server. Member, lost-role, denied-access and malformed-count scenarios pass. | Implemented; stale success-message defect below |
| Independent timezone editing | `workspace-planning-timezone.tsx:29` preserves a local draft, blocks the switch during dirty/pending/unconfirmed states, confirms writes by reading shared settings, and retains failed input. Eight timezone tests and related settings tests pass. | Implemented |
| Workspace disable | The switch opens the existing controlled `IterationOperation`; complete preview, reason, explicit confirmation and same-request recovery remain connected. Cancellation preserves the switch state; lost responses and external disable are covered. | Implemented at component level; atomic server behavior reviewed separately |
| Sidebar and history | `app-sidebar.tsx:892` requires supported/manual/enabled; disabled settings retain a history link. Overview switches to history when disabled and uses the real settings URL. | Implemented |
| Timeline overview | `iteration-overview.tsx` consumes the complete core catalogue, derives planned/current/history groups, keeps the active row after the future-row limit, suppresses uncertain gaps, and only enables detail queries for active rows. Filters, stale-cursor restart and no-active behavior pass. | Implemented |
| Detail and task actions | `iteration-page.tsx` uses Tasks / Progress / Scope changes, retains mounted filters and visited panels, and keys entity boundaries by workspace/id. Empty plans offer existing-task and new-task actions. Edit and assignment dialogs retain drafts. | Implemented |
| Existing-task assignment | The current period remains the fixed target; selection leads through full preview and explicit confirmation. Pending/disabled inputs are guarded. Assignment suite passes. | Implemented |
| Manual creation in a period | The modal receives workspace/id/revision; scoped creation is manual, displays the selected period, submits membership in the create request and does not silently discard a stale or invalid period. The reviewed revision remains pinned until explicit selection. | Implemented |
| Frozen history and statistics | Detail and `IterationHistory` prefer `snapshot.statistics`; frozen events and live comparisons remain distinct. Null ratios, historical facts and labelled data tables are covered. | Implemented; real lifecycle evidence belongs to E2E |
| Shared Dialog | `DialogContent` adds optional `keepMounted=false`; only callers opt in. The installed Base UI implementation applies `hidden` to closed popup/backdrop. Real-dialog draft close/reopen regression passes. | No defect found in this change |
| Latest concurrent selector refinement | Another session added the searchable pill picker, pinyin lookup and keyboard selection to `IterationCandidate`, plus create-modal integration. This was outside the first 343-test snapshot. A separate final 66-test run passed with unchanged source hashes during that run. | Latest narrow regression passes; earlier lint/typecheck does not certify this later delta |

## Findings (fixed)

None. Source fixes were deliberately outside this inspection's write ownership. Only this report and new review logs were created.

## Findings (not fixed)

### UI-01 — P2: successful enable feedback survives a subsequent disable

- File: `packages/views/iterations/iteration-settings-tab.tsx:108`.
- Confirmed code path: the success paragraph checks `enable.isSuccess && !operationBlocked`, without checking the current enabled state or associating feedback with the current settings revision. `useIterationCommand` returns a normal mutation result; a separate disable mutation does not reset the earlier enable mutation.
- Reproduction: remain on one settings-page mount, enable successfully, then switch off, preview and confirm a successful disable. Once processing ends, the page can show the disabled switch/history link together with “Iterations enabled.” / “迭代已启用。”.
- Impact: contradictory state feedback; this does not imply the server stayed enabled or that disable lost data.
- Coverage gap: the existing disable test starts already enabled and does not cover enable → disable on the same mount.
- Recommendation: invalidate/reset prior success feedback when the authoritative state changes, and add that same-mount regression. Main session is checking the sequence in the real UI. No source change made here.

### UI-02 — P3: saved timezone is not always visible in the detail header

- File: `packages/views/iterations/iteration-page.tsx:98`; a similar condition appears in `iteration-overview.tsx:102`.
- The header renders `iteration.timezone` only when it differs from `planningTimezone`. Started/closed timestamps provide `title` tooltips, but an unstarted plan with the same timezone has no explicit saved-zone label in its header.
- Child PRD IP-05 and design line 46 specify the saved timezone in the detail header. This is a presentation/contract gap, not evidence of incorrect date calculations.
- This condition already exists in HEAD, and `iteration-navigation.test.tsx:167` intentionally tests the conditional display. It is not a newly introduced regression.
- Recommendation: main session should reconcile the intended compact display with the written contract; if the header must always show the zone, update that presentation and the current test together. No unilateral design change made here.

### UI-03 — P3: initial settings/detail read failures lack an in-page retry

- Files: `packages/views/iterations/iteration-page.tsx:52`, `:76`; `packages/views/iterations/iteration-error.tsx:5` is only an alert.
- Reproduction: open an uncached detail and fail the first `getIteration` read, or fail the first workspace iteration-settings read after a successful capability read. These branches return the alert before the page header and retry affordance are rendered; the queries themselves use `retry:false`.
- Impact: the user must revisit/reload or rely on a later external refetch. Cached background errors do have a retry and retain drafts, so those passing tests do not cover this initial-error branch.
- This behavior predates the layout rewrite. The child design nevertheless requires a retry for first-load failures.
- Recommendation: add a retry for recoverable first-load errors while keeping access denial/deletion separate. No source change made here.

### UI-04 — P3: overview chart has no nearby accessible data-table control

- Files: `packages/views/iterations/iteration-overview.tsx:108`, `iteration-progress.tsx:15`; the table exists at `iteration-history.tsx:110`.
- The overview renders the chart and current totals, but not the design's nearby “view data table” control. A screen-reader user can still navigate into the period and open Progress to reach the full labelled table.
- Impact: the specific overview accessibility entry in the design is incomplete; the underlying values and detail table are present. This is not missing statistics or a wrong curve.
- Recommendation: decide whether the existing deeper table satisfies final acceptance, or expose the shared table next to the overview chart. No speculative new chart abstraction was added.

## Verification

Logs: `.omx/state/iteration-workspace-settings/current-review-ui/`.

| Check | Result | Evidence |
| --- | --- | --- |
| Focused views regression | PASS, 16 files / 343 tests, 23.8 s | `focused-ui-tests-supported-options.log` |
| Two former navigation failures | PASS, 3 independent runs, 2 selected tests per run | `navigation-race-repeat.log` |
| Latest selector/create regression | PASS, 2 files / 66 tests, 6.1 s; tracked files unchanged during execution | `latest-selector-tests.log`, `latest-selector-test-source.json` |
| Views lint | PASS, 0 errors / 27 warnings in other existing files | `views-lint.log` |
| UI lint | PASS, 0 errors / 3 existing stepper warnings | `ui-lint.log` |
| Views TypeCheck | PASS (`tsc --noEmit --incremental false`) | `views-typecheck.log` |
| UI TypeCheck | PASS (`tsc --noEmit --incremental false`) | `ui-typecheck.log` |
| Source whitespace | `git diff --check -- packages/views packages/ui` passed when run | Tool result, no whitespace output |
| Web/Electron E2E, rendered layout/focus | Not run by this reviewer | Main session owns these checks |

The first test invocation used the unsupported Vitest 4 `--minWorkers` option and ran no tests. After checking the installed CLI help, it was corrected to `--maxWorkers=2`; the successful result above is from the corrected invocation. The unsuccessful setup invocation is retained as `focused-ui-tests.log`.

Focused command, with cwd `packages/views`:

```sh
pnpm exec vitest run iterations settings/components/settings-nav.test.ts settings/components/settings-page.test.tsx settings/components/workspace-planning-timezone.test.tsx layout/app-sidebar.test.tsx modals/create-issue.test.tsx modals/create-issue-dialog.test.tsx modals/quick-create-issue.test.tsx locales/parity.test.ts --maxWorkers=2
```

The two old failures were selected from `iterations/iteration-navigation.test.tsx` using `--maxWorkers=1 --testNamePattern 'returns to the task action when a plan becomes empty on another tab|locks a retained edit when another client disables iteration planning'` for exactly three runs.

Final concurrent-delta command, with cwd `packages/views`:

```sh
pnpm exec vitest run iterations/iteration-candidate.test.tsx modals/create-issue.test.tsx --maxWorkers=2 --reporter=verbose
```

Package checks were `pnpm --filter @multica/views lint`, `pnpm --filter @multica/ui lint`, and each package's `typecheck --incremental false`.

### Historical full-suite result is still a separate fact

The previous `logs/all-ts-tests.log` ends with 518/519 views files and 6242/6244 tests passing. Its two failures are the navigation cases above. They were not reproduced in the current focused suite or the three additional independent runs.

Timing supports, but does not by itself prove, a stale test snapshot: that full views process began at 20:45:16, `iteration-navigation.test.tsx` was modified at 20:45:36, and `review-final.log` was written successfully at 20:45:39. Do not call the old full suite green, and do not equate its failures with two currently reproduced product defects.

### Concurrent work and acceptance boundary

The first 343-test run began at 22:56:07. Another session modified selector/create/locale files after that check; a source manifest records this explicitly. Main-session coordination limited this reviewer to one additional selector/create run, which passed at 23:20:23, with before/after hashes recorded and no tracked changes during the run. Earlier lint/typecheck remain evidence for the checked settings/layout revision, not blanket certification of later concurrent selector edits.

Production Web/Electron navigation, response-loss recovery across a client restart, sibling-client updates, both locales/themes, 1440px/900px layouts and real keyboard/focus behavior must be assessed using the main session's current evidence. Prototype screenshots and jsdom tests cannot establish all those results. Mobile settings, automatic scheduling, capacity/estimates and automatic cycle advancement remain explicitly out of scope rather than missing features.
