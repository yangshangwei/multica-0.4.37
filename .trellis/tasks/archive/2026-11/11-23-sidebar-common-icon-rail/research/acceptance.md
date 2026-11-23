# Common icon rail acceptance

The user selected the previously reviewed A proposal, requested a new task and
implementation, and explicitly required no commit. Implementation and
verification were completed with all source, tests and task records uncommitted.
The user then explicitly requested a local commit; the verified source/spec/test
change was committed as `76c61f95d`, followed by task archival and journal
recording. Unrelated working-tree changes remain outside this delivery.

## Result and boundaries

- Large Web/Desktop collapse keeps the shared 64px common-entry rail, original
  action/scope semantics and More navigation. Complete directory preferences,
  fixed items, capability gates and remembered expanded width remain owned by
  their existing code.
- More uses native Base UI menus and existing AppLinks/route icons. Pin menu
  items reuse resolved data without registering drag/unpin UI. Notification
  and chat observers retain their original scope; no new store, API, database
  migration or dependency was introduced.
- The generic primitive owns one inset icon width, centered 16px icons and
  44px coarse targets. Folding or compact transition cancels resize previews.
  Desktop subtracts the rail from its 200px toolbar clearance.
- Compact complete navigation scrolls search, Start work and directory
  together while the footer remains reachable. The real native 200-percent
  450×500 CSS-pixel failure drove this correction.
- Web collapse state remains route-provider-owned and nonpersisted; switching
  between account/workspace shells can expand. Stored expanded width still
  restores. This increment does not add B/C, a third hidden state, packaged
  Main/installer acceptance or native mobile UI.

## Fresh verification

| Evidence | Result |
| --- | --- |
| Shared sidebar/header/help/disclosure/resize/auto-collapse/wrapper unit suites | 7 files, 110 passed |
| Web account and scope shell suites | 2 files, 13 passed |
| Desktop layout and workspace gate suites | 2 files, 20 passed |
| UI, Views, Web and Desktop typecheck | Passed |
| Package lint | Passed, 0 errors; 31 pre-existing warnings outside changed files |
| Changed-file scoped ESLint and diff checks | Passed, no warnings/errors |
| Mechanical design detector | 0 findings |
| Production Web and actual Electron originals | 4 passed; 0 skips, retries or failures |
| Web/actual Electron 1100×650 coarse icon measurements | 44px targets, 16px icons, all measured centers dx=0/dy=0 |
| Actual Electron compact navigation at 200-percent zoom | Collaboration disclosure and child link clickable; footer reachable |
| Root preservation | Same HEAD, 0 staged paths, original next-env.d.ts and .env retained |

Runtime report: `artifacts/qa/sidebar-icon-rail-20261123/confirmed/report.json`.
Source/build/process binding: `artifacts/qa/sidebar-icon-rail-20261123/source-binding.json`.
Coarse measurements: `research/coarse-geometry.json`.
Screenshots: `artifacts/qa/sidebar-icon-rail-20261123/confirmed/browser-results/`.
Independent review: `research/check-review.md`.

Earlier failed runs remain under `r1`, `r2`, `r3` and `final`; they are not counted
as passing evidence. They exposed an overly broad tooltip selector, the current
enabled iteration default, trigger slot loss, coarse icon alignment and compact
scroll starvation. The final original assertions retain those behaviors rather
than force-clicking, changing viewport constraints or hiding failure.

Only task-owned acceptance processes are stopped. The isolated worktree and
database/evidence are retained for inspection. No staging or commit occurred during implementation acceptance. The later
explicit authorization permits only local work, task and journal commits; no
push, merge or release was requested.
