# R51/R52 shared view implementation

## Change boundary

The shared detail currently omits ordinary children and recorded run content.
Source copy actions still produce source-only URLs, and embedded child/batch
controls can reach ordinary edits before the collaboration guard.

The existing `CollaborationDetail` remains the sole presentation owner for
account, source, panel and native window mounts. It will compose the optional
ordinary-child projection and lazy, protected run-record pages. Execution groups
remain the owner of whole-squad outcomes; recording completeness, business
outcome and termination proof are displayed separately. Numeric telemetry stays
as recorded decimal text, with explicit missing values.

Expected files:

- `packages/views/collaboration/`: children, record disclosure, notifications,
  detail/execution wiring, localized copy, and focused composition tests.
- `packages/views/issues/actions/`, `issues/components/`, `search/`: shared
  account links, safe shared child rows, and batch guards before ordinary
  confirmation/transport. Existing ordinary interactions retain their owners.
- `e2e/fixtures/` and collaboration Web/Electron specs: real-API child changes,
  retained records, notification links and access-loss journeys.

No server/core edits, rollout changes, provider adapters, new dependencies,
ordinary-child execution changes or AC89 parent-wakeup changes are included.
Source selection-copy remains disabled for shared content (R56). A small branch
in the ordinary child row replaces its unsafe inline controls for independently
shared children; existing ordinary row tests protect its other behavior.

## Verification plan

Add failing regressions to existing component/action suites first. Core/server
own the wire, ACL and settlement matrices. Views own discoverability, readable
content, exact usage, lazy paging/retry, focus, safe navigation and composition.
Main owns production build/service lifecycle and actual Web/macOS Electron runs,
including screenshots and visual verdicts at narrow width and 200% zoom.

Evidence directory: `.omx/reports/cwc-r51-r52-20261019T110745Z/views/`.

## Results

Implemented:

- `CollaborationChildren` consumes only the detail projection; missing support
  differs from an empty list, source links use the returned slug, and outside
  references have no link/prefetch. Every detail mount reuses that composition.
- Shared copy/open actions use account UUID links. Shared children in ordinary
  detail retain selection and read-only summaries, with the external account
  component and no inline ordinary mutation controls. Mixed batch actions refuse
  before run confirmation, iteration entry, deletion or either transport path.
- Lazy run disclosures show recorded inputs/files, messages, tool calls/results,
  retained comments, squad activity and outcomes. Protected infinite queries own
  pages; a failed next page keeps prior entries and retries the original cursor.
  Current access denial removes the open content. The normal token formatter is
  intentionally not used: it accepts numbers and abbreviates values. Recorded
  decimal strings and costs are displayed exactly; missing fields stay missing.
- Run outcome, group status, termination proof and recording coverage have
  separate labels. Resource details are explicit frozen IDs/provenance, never a
  raw JSON dump or a request for the current resource catalog. Result notification
  links prefer optional `run_id`; older notifications retain issue destinations.
  Shared detail accepts/focuses the run anchor and copies it to an account link.
- Bilingual copy and shared real-API Web/macOS Electron journeys were added.
  The E2E setup creates human objects/private uploads publicly, then seeds retained
  three-run squad history (a successful early leader and a failed final
  continuation) with 60 recorded messages, exact usage and a result notification.
  No engine task, live runtime or executable is created by this UI fixture.

Executed checks (evidence under `views/`):

- `r51-red`: 8 new regression failures, 150 existing passing tests.
- `r52-red`: 4 new regression failures, 2 existing passing tests.
- `r52-anchor-red`: absent selected-run/copy wiring reproduced before its edit.
- `combined-unit`: `pnpm --filter @multica/views test collaboration
  issues/actions/__tests__/use-issue-actions.test.tsx
  issues/actions/__tests__/issue-actions-menu.test.tsx
  issues/components/batch-action-toolbar.confirm.test.tsx
  issues/components/batch-action-toolbar.test.tsx
  issues/components/issue-detail.test.tsx search/search-command.test.tsx
  locales/parity.test.ts` — 21 files / 281 tests passed.
- `typecheck-fixed`: `pnpm --filter @multica/views typecheck` — passed.
- `lint`: `pnpm --filter @multica/views lint` — 0 errors, 27 existing warnings.
- `e2e-discovery`: Playwright discovers both new named journeys without starting
  services or fixtures. Source/Electron reference screenshots were reviewed by
  the main session before implementation.
- `e2e-typecheck-final`: standalone strict TypeScript check of both new E2E
  journeys and their fixture passed (`ES2022,DOM,DOM.Iterable`, bundler modules).
- `git diff --check -- packages/views e2e` — passed.

All product/app/E2E source writes were frozen at 2026-10-19 11:55 UTC for the
main session's production build guard. Later changes require coordination with
the main session. Untracked research/evidence updates do not change that build.

Main-session integration commands:

```sh
pnpm exec playwright test e2e/collaboration-records-web.spec.ts --workers=1 --retries=0
pnpm exec playwright test e2e/collaboration-account-desktop.spec.ts --grep 'Desktop child links' --workers=1 --retries=0
```

Actual browser/native execution, screenshot verdicts and the repository-wide
quality gate belong to the main session. These UI fixtures do not prove runtime
ingestion or notification settlement; the server owner's supported static-runtime
and transaction suites own those claims. P7, general providers, release gates
and complete parent acceptance remain outside this increment.

## Integration and review follow-up

The main session's first Web journey exposed a fixture ordering defect: the
group-input trigger requires an in-flight command and an open group. The fixture
now records consumption first, then terminalizes the synthetic command/batch/group
inside the same transaction. It also separates the minimal target roster from
the frozen group roster. The server owner confirmed these invariants; no product
guard was weakened. The corrected initial Web journey passed in the main session
(`web/records-fixture-fixed.*`).

After the main session released the next source freeze:

- Source `IssueDetailRoute` now reads `NavigationAdapter.hash`, including native
  MemoryRouter windows, and reuses `parseAccountPath` to validate run UUIDs.
  `highlightRunId` passes through embedded detail and `SourceCollaborationDetail`.
  Source copy/open/search preserve a validated anchor only for the current issue;
  an unrelated selected row never inherits the current route's hash.
- Notifications reuse existing labels for assignments, participant changes,
  handoffs, information requests/returns, acceptance and comments. The removed
  recipient sees a content-free removal message without an open action; unknown
  event kinds retain a generic label.
- Hiding an anchored run clears only its matching account/source anchor through
  `replace`. The same result notification can therefore reopen and focus that
  run. The notification popover uses the existing `finalFocus` handoff pattern so
  it does not steal the run's focus; normal Escape dismissal still returns focus.
- Web now exercises a source `#run` route and canonicalization/copy/open plus
  repeated result navigation. Native opens a source `#run` popout via production
  IPC and checks the same selection/focus/copy behavior. Narrow screenshots now
  explicitly show recorded usage and the long recorded body, including native
  200% zoom, rather than only the controls above the record.

Fresh evidence:

- `views/review-fixes-red`: 7 reproduced gaps, 155 existing tests passed.
- `views/review-fixes-green`: 20 files / 285 tests passed.
- `views/repeat-anchor-red`: failed reopening reproduced by the composed
  notification/detail/navigation test. The initial fix exposed popup focus
  restoration; `views/repeat-anchor-focus-green` passes all 3 tests, including
  repeat open/focus and Escape return.
- `views/review-typecheck-final`: passed after correcting a test-only Testing
  Library option (`exact` belongs to Playwright's matcher).
- `views/review-lint`: passed, 0 errors / 27 existing warnings.
- `views/review-e2e-typecheck-final`: both E2E journeys and the shared fixture pass
  a standalone strict TypeScript check.
- `git diff --check -- packages/views e2e`: passed.

All owned product, unit-test, app and E2E source files were frozen again at
2026-10-19 12:34:26 UTC. Main owns the final production rebuild and browser/native
runs; the recovery UI checker owns the next independent bounded verification.

The independent UI pass then found that the isolated `resource-panel.test.tsx`
mount lacked a NavigationProvider, which real application mounts already supply.
Only that test wrapper was corrected; its three tests pass in
`views/resource-panel-provider.*`. All owned source files were frozen again at
12:38:15 UTC for the reviewer's final bounded suite, lint and typecheck.

## Native clipboard verification

The main session's full native run passed nine existing scenarios but the new
journey read the OS clipboard immediately after clicking Copy link, before the
asynchronous `navigator.clipboard.writeText` completed. No product code changed.

The E2E helper now activates and verifies the real native/content/document focus,
sets the approved test sentinel, and polls only a safe clipboard classification.
It returns clipboard text only after an atomic equality check confirms the exact
test-owned absolute URL constructed from `runtimeConfig.config.appUrl`. The
expected origin follows the same configured sharing policy as ordinary native
links. Arbitrary prior clipboard contents are never logged or asserted. Both runs
used the main session's JXA wrapper, which retains all clipboard formats in memory
and restores task-owned clipboard writes on exit.

`desktop/copy-diagnostic.*` passed 1/1 in 13.9 seconds. In both source/account and
source/run copies, native window, web contents and document were already focused;
the immediate clipboard classification was `sentinel` and the settled value was
`expected_absolute` at the configured loopback app origin. This distinguishes the
timing defect from relative URL construction or missing focus.

The native scenario also now explicitly hides the selected run and reopens the
same result notification. `desktop/copy-diagnostic-repeat.*` passed 1/1 in 14.6
seconds, with wrapper exit 0. It completed source child attach/detach and navigation,
source `#run` popout/copy, external references, repeated notification open/focus,
record paging and attachment preview, native 200% usage/body captures, restart and
main/popout revocation. The final E2E TypeScript check and diff check passed.
All source files are frozen again; main owns the independent final rerun and
visual verdict. The recorded process fixture remains seeded read/UI evidence.

The main session's final capture review identified transitional native screenshots
(score 86/revise): a closing notification overlay and a body image that still
showed the prior usage frame. Only E2E screenshot readiness was changed. The
scenario now waits for the popup to be hidden, checks the current target's scroll
alignment after two animation frames, asserts full viewport visibility and requires
different usage/body PNG digests before retaining the captures.

`desktop/visual-stable.*` passed 1/1 in 14.7 seconds with wrapper exit 0. Visual
inspection confirms a clear wide frame and a separately scrolled, wrapped long
body at native 200%. The readiness attachment records usage/body scroll offsets
2248.5 and 2786.75 in a 550×500 CSS viewport, with distinct SHA256 digests. The
standalone E2E TypeScript and diff checks passed; no CSS, product behavior or
timeouts changed. Source files are frozen for the main session's final verdict.
