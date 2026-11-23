# Common icon rail review

## Scope and write boundary

Reviewed the current task diff in shared AppSidebar/navigation primitives,
Web account-shell wiring, Desktop shell geometry, bilingual layout copy and
`e2e/sidebar-icon-rail.spec.ts`. Read the complete saved native hook context,
task artifacts and CLAUDE.md. The main session requested read-only source review
while source-bound production builds run; this report is the only file written
by the reviewer. No staging, commit, reset or unrelated-file changes.

## Current review status

The final narrow read-only review confirms that both original P2 findings are
resolved in the current source and meaningful regression coverage. The confirmed
coarse icon offset passed actual Web and native measurements in r4. That run
exposed a separate compact native directory-height defect; its bounded shared
scrollport correction is reviewed and confirmed by the complete r5 run. No open
review finding remains. Original failed runs remain recorded and are not counted
as passed runs.

## Initial findings and resolution

1. **P2 — Complete visible keyboard names for the footer icons (R4).**
   `packages/views/layout/app-sidebar.tsx:638` renders the collapsed account
   trigger with its avatar, `aria-label` and native `title`, but no focus/hover
   Tooltip. `packages/views/layout/help-launcher.tsx:34` likewise only supplies
   a native title. Native titles do not visibly appear on keyboard focus, so
   these footer icons do not meet R4's written hover/keyboard name behavior,
   although their accessible names remain correct. Common route/action icons
   already use the shared Tooltip behavior. Add a narrow existing-primitive
   tooltip composition for icon mode and verify actual Tab-driven popup text;
   leave expanded/footer action behavior unchanged.

   **Resolved by the implementation follow-up.** HelpLauncher has an optional
   icon-only Tooltip composition; the account and web rail toggle also compose
   the real Base UI primitives. The rendered TooltipTrigger explicitly retains
   `dropdown-menu-trigger` or `sidebar-trigger` slots, avoiding the independently
   reproduced r2 slot regression. Real keyboard-focus, Help hover/menu action,
   single-control DOM and toggle action tests cover the behavior. The recorded
   `ui-toggle-slot-green.log` run passed 90 AppSidebar/Help/PageHeader tests;
   corresponding views typecheck and scoped lint passed. The reviewer inspected
   these exact current sources and regressions rather than inferring behavior
   from their component names.

2. **P2 — Wait for a visible More menu before negative feature assertions.**
   `e2e/sidebar-icon-rail.spec.ts:324` clicks More and then checks that Work,
   Iterations and Triage are absent without first waiting for a known menu/root
   to be visible. The popup-readiness spec explicitly requires the containing
   popup to be visible before absence checks or Escape; otherwise a deferred
   opening can make absence pass and outlive dismissal. Wait for the exact
   existing More popup or Resources menuitem before these assertions, then
   wait for real dismissal. Preserve the feature assertions and test deadline.

   **Resolved by the E2E follow-up.** The test awaits the visible Resources &
   connections menuitem before negative assertions. Setup uses the existing
   `iterationOptOut(api, second)` operation and verifies actual iteration
   capabilities plus Triage settings as disabled, both after setup and before
   opening More. Fresh workspaces currently enable iterations; the former
   assumption was the retained r2 Desktop fixture failure, not a product defect.
   r3 Web passed these assertions and continued through the remaining product
   steps before its final independent centering assertion failed.

3. **Runtime check requested — Coarse-pointer icon centering.**
   The shared MenuButton icon target now expands from 32px to 44px with 8px
   padding. Common NavRow, New Issue and notification/More triggers have no
   `justify-center`, so their single 16px SVG would sit 6px left of the target's
   center under the coarse rule. This is a concrete CSS composition concern;
   measure the actual compiled target/SVG centers in the parent's 1100×650
   coarse runtime probe before treating it as a verified visual defect. A
   local icon-mode alignment class is the smallest correction if confirmed.

   **Confirmed, corrected and measured.** r3 Web retained
   nine 16px icons at `dx=-6`, `dy=0` inside 44px targets. The primitive now adds
   `group-data-[collapsible=icon]:justify-center`, leaving full navigation
   alignment unchanged. `geometry-implementation.md` records 85 focused tests,
   UI lint/typecheck and diff check passing after correction. The originals
   retain the real centering assertion; a class-only assertion was not added.
   r4 measured all ten applicable Web and native icons at `dx=0`, `dy=0`.

4. **Pending planned work — Navigation contract synchronization.**
   `.trellis/spec/core/frontend/global-scope-navigation.md` still describes
   three always-visible Start-work controls and zero-width collapsed Desktop
   clearance. Main owns the planned spec update after runtime acceptance; the
   updated contract must distinguish full versus icon mode, More focus return,
   64px/136px Desktop geometry and route-owned nonpersisted Web fold state.

   **Resolved by the main session's spec update.** The current contract now
   distinguishes complete versus icon navigation, the one retained creation
   owner and More focus return, generic 64px width, native 136px clearance,
   compact-only content openers, route-owned nonpersisted Web open state,
   native menu pin rendering and truthful capability/popup readiness checks.

No verified P0/P1 source defect was found in review. r5 confirms the compact scroll
correction with complete production Web and actual Electron journeys. The r3
Desktop failure happened before its source
navigation handler was ready; the test now follows the existing original scope
journey by waiting for actual application navigation before dispatch, including
after Chinese reload. Failure-state captures preserve the original error before
window/profile/fixture cleanup. No sleep or increased timeout substitutes for
the readiness condition. r4 passed both original restoration journeys and the
complete Web rail journey; its native rail journey failed only after the earlier
large-screen/coarse steps, at the compact workspace directory.

## Compact scrollport follow-up

The retained r4 native 450×500 CSS-pixel workspace Sheet had zero directory
height with its actual Search/Start-work header and footer fixed outside the
directory scroller. This was a real product defect rather than fixture timing.
The current AppSidebar composes header and directory once, places both inside
one SidebarContent with a `shrink-0` content wrapper only while `isCompact`, and
keeps the original footer outside that scrollport. Large expanded/icon branches
retain the prior fixed header plus directory-scroller structure.

The reviewer compared the exact compact delta and checked the lifecycle:

- Header/directory variables are JSX values, not new component types/providers
  or route/context-dependent keys.
- The AppSidebar-owned collaboration dialog/open state, visible return-focus
  getter, original creation intent effect and workspace-selection handlers are
  unchanged; footer menus retain their existing owners.
- Actual-workspace versus preview query boundaries and the global modal/search
  hosts are unchanged. No view-state store or persistence was introduced.
- Sidebar already swaps its desktop and Sheet child surfaces when the compact
  breakpoint changes; this correction adds no new breakpoint remount boundary.
- The new regression first failed for actions outside the scrollport, then the
  recorded compact follow-up passed 91 AppSidebar/Help/PageHeader tests and
  scoped lint/views typecheck. No redundant broad test run was performed here.
- The spec now explicitly records centered coarse icons and the shared compact
  scrollport, preserving the original 450×500 native disclosure/link acceptance.

No unexpected state/action loss was found in this final read-only source check.
r5 supplies the actual native disclosure click and child navigation completion
evidence. The reviewer did not launch runtime or edit production source.

## Confirmed acceptance

The reviewer read and validated
`artifacts/qa/sidebar-icon-rail-20261123/confirmed/report.json`: all four originals
passed in 48.684 seconds, with zero skipped, unexpected, flaky or retried results.
These are the complete Web/native icon-rail journeys and the unchanged original
Web/native scope-restoration journeys. Native compact navigation succeeds at
450×500 CSS pixels under actual 200% zoom; theme/Chinese captures remain attached.

Decoded geometry attachments confirm ten applicable icons on both Web and
Desktop at `dx=0`, `dy=0` under real coarse emulation. Current AppSidebar and
primitive hashes match the reviewed compact/centering corrections above.
The design detector report is `[]`. Main-session verification records report
110 shared unit tests, 13 Web platform tests, 20 Desktop platform tests and
passing scoped lint/typecheck. No new test or production edit was made by this
final evidence review. There are no open findings; packaging/installer and
provider-execution acceptance remain outside this UI task's scope.

## Final narrow review source identity

- AppSidebar SHA-256: `7b2b8d39a5955a2c07649618152512ae91e9ffcbc28f3a22b5711b3f2fde4294`.
- AppSidebar test SHA-256: `d5c07f90697ef1df6f47f892d7a6af528172995ec9d7f3449abad272cd60596e`.
- HelpLauncher SHA-256: `3e9c50eab22ef71b2f41c0da1468c46ffcc1ac013fa53b282972266590a544c1`.
- Sidebar primitive SHA-256: `93b8c5968206f03aeee8263260c80d949dc1d5792dc05cde6fcaa7a4c795e81f`.
- New E2E SHA-256: `e6842201955a62e55f51eb209d3f3ac4e71e71aeaaddccd94d925c03ba7c3cc6`.
- Navigation spec SHA-256: `89344a46bc9a90a6c666c476e556865e73ff45dc5faaf80a4f5ce8caeff20851`.

The reviewer reran scoped `git diff --check`, read the latest source/tests/spec,
and inspected retained r2/r3 terminal reports and follow-up verification records.
Per the main session's instruction, no redundant broad test rerun or runtime
launch was performed during the final source-bound build. This report is the
only reviewer write.

## Contracts checked

- Common entries render independently of full-directory disclosure preferences;
  full compact sheets use `collapsed && !isCompact` correctly.
- Notification query ownership remains one protected observer; chat and workspace
  queries remain under actual route authority. Account preview capability reads
  remain disabled and do not install workspace identity.
- Ordinary New Issue retains its existing target-bound intent. Collaboration
  creation uses one retained dialog owner and the visible original/More ref.
- More reuses existing native AppLinks, route icons, capability gates and exact
  account destinations. PinMenuItem returns before sortable hooks, preserving
  resolved pin paths without registering menu drag/unpin controls.
- No server/API/schema/dependency/store/persisted-collapse preference was added.
  Scoped existing user modifications are left untouched.
- Collapsed resize startup is blocked and active drag previews are cancelled on
  folding or compact transition without persisting a bogus expanded width.
- Painted/reserved inset widths share 64px; Desktop remaining top clearance is
  136px and compact stays 200px. Existing auto-collapse transition logic remains.
- Every production Sidebar currently renders through AppSidebar icon mode.
  Production SidebarProviders are DashboardLayout, WebAccountShell and
  DesktopShell, so the `lg:hidden` page-header fallback does not strand a current
  generic offcanvas consumer. Future custom Sidebar slots must supply their own
  persistent opener; this is not a present-task blocker.

## Fresh verification

| Verification | Result |
| --- | --- |
| Shared AppSidebar/PageHeader/disclosure/resize/auto-collapse/wrapper suites | 6 files, 93 tests passed |
| Web account/scope-shell suites | 2 files, 13 tests passed |
| Desktop layout/workspace-gate suites | 2 files, 20 tests passed |
| `pnpm exec turbo typecheck --filter=@multica/ui --filter=@multica/views --filter=@multica/web --filter=@multica/desktop --concurrency=2` | 7 successful tasks; UI/Views/Web/Desktop checks executed, 2 dependent cached tasks |
| Scoped eslint in UI/Views/Web/Desktop package cwd | Passed, no lint errors |
| `git diff --check` for reviewed source/test scope | Passed |

pnpm's pre-existing warning about package.json `pnpm.onlyBuiltDependencies` and
`pnpm.overrides` appears during checks; no dependency settings were modified.
No production browser/native success is claimed by these static/unit results.
