# I1 final UI and compatibility audit

Audit date: 2026-10-07. Branch: `codex/projects-p1`; base HEAD `115b4cd28`.
**Bounded UI review: PASS after the fixes and final production Web/Electron
rerun.** This report supplements the parent acceptance matrix and recommends UG
acceptance for the documented scope. It does not independently close UG, FCG or
VG; the integration owner retains source-provenance, database, performance,
migration and final gate ownership.

## Evidence and review boundary

- Read the original `iterations-prd.md` §§6–8, ITR-010/012/019/032–035, the
  parent design/API/test contracts, client verification and iteration frontend
  spec. Inspected the shared views/core implementations, route integrations,
  test assertions and final screenshots from the accepted full check.
- The accepted full-check log has four successful I1 E2E cases at lines
  8286–8289, and `249 passed`, `49 skipped`, `All checks passed` at lines
  8515–8520. This audit reuses that evidence for unchanged behavior. The root
  session established its source parity; it is not evidence for changes made
  below after that run.
- Preserved the five reviewed screenshots in `browser-evidence/*-accepted.png`.
  `accepted-screenshot-manifest.json` records exact source paths and SHA-256.
  `visual-verdict-accepted-history.json` is a **94/pass** comparison against the
  retained prior Web/Desktop history renders. The Desktop tab is now named and
  uses the iteration icon; history dates are readable on the saved clock; the
  390px Chinese Web and 680px native table layouts remain usable. This score is
  for captured history surfaces, not unobserved settings or confirmation flows.
- No official pixel-exact design reference exists in this task. The comparison
  is against previous rendered integration evidence and the written product
  requirements. It does not claim pixel-identical design reproduction.

## Confirmed gaps and bounded fixes

The audit found four small Web/Desktop omissions. The integration owner
explicitly authorized fixing these in the existing files, without replanning
the I1 task or changing its protocol:

| Gap | Fix | Evidence |
| --- | --- | --- |
| §6.3 settings did not explain manual mode | `iteration-page.tsx` shows that periods never start/end automatically and planning never starts/stops executions; EN/ZH copy remains in the existing locale namespace | Existing real closure fixture now asserts the explanation before enabling |
| ITR-007/034 per-task terminal retention showed only UUIDs | `iteration-operation.tsx` labels each checkbox with explicit retain semantics plus the protected preview identifier/title; absent facts retain the UUID fallback; master wording includes both completed and cancelled tasks | Existing terminal-choice test now selects by readable task identity, retains independent choices through refreshed preview, then verifies revocation hides task identity |
| ITR-010 scope events did not identify the affected task | `iteration-events-view.tsx` displays the persisted after-facts title/identifier, falls back to before-facts for deletion and to `issue_id` for uncaptured/malformed fields; no current-task request was added | New mounted assertion covers updated facts, deleted facts and malformed/ID-only history and asserts zero `api.getIssue` reads |
| ITR-012 preview timestamp was raw RFC3339 | `iteration-operation.tsx` uses existing `formatInTimeZone`, active locale and semantic `<time>`; source-period preview timezone is authoritative; workspace-wide previews show the saved affected zones (UTC when there is no period) | New mounted assertion proves 18:05Z renders 02:05 in saved `Asia/Shanghai` and retains exact machine-readable `datetime` |

The mobile notification click omission was handed to the integration owner and
its separate mobile lane: iteration notices with `issue_id:null` previously only
marked themselves read, without explaining that Web/Desktop is required. The
lane's `mobile-final-compatibility.md` records the added native supported-client
explanation, guarded workspace/kind/UUID routing and successful 196-test mobile
suite, typecheck and lint. This reviewer made no mobile source changes and does
not claim to have exercised the native Alert on a device.

The source scope for this reviewer is exactly:

- `packages/views/iterations/iteration-page.tsx`
- `packages/views/iterations/iteration-operation.tsx`
- `packages/views/iterations/iteration-events-view.tsx`
- `packages/views/iterations/iteration-operation.test.tsx`
- `packages/views/iterations/iteration-details.test.tsx`
- `packages/views/locales/en/projects.json`
- `packages/views/locales/zh-Hans/projects.json`
- `e2e/fixtures/iterations-i1.ts`

The source was frozen after successful narrow checks and handed to the root
session for production Web/Desktop builds and the four I1 E2E cases. The final
correctly configured rerun passed all four in 24.2 seconds, with zero skips or
retries. The root session confirmed all 6,696 runtime/test source hashes still
match the frozen source. No new dependencies, data structures, API calls or
lifecycle transitions were added.

## Original UI requirements mapped to code and assertions

| Requirement | Current implementation and concrete evidence | Review result |
| --- | --- | --- |
| §6.1 workspace entry; disabled explanation and admin enable | `app-sidebar.tsx` gates on confirmed support; `IterationWorkspace` only renders enable for owner/admin, shows timezone and disabled copy. `iteration-page.test.tsx` verifies old-server unsupported mode has no write controls. Real closure flow enables through the UI. | Covered; manual explanation added above |
| §6.1 current/future/history; no false current; earliest-future tag | `IterationList` obtains current/future from the complete protected catalogue independently of paged history; earliest planned start date determines the tag. Navigation tests use 50 historical rows, verify current/future still visible, and verify no-current/upcoming semantics. | Covered |
| §6.1 history search/date/status/direct links | List sends search/from/to/status/cursor and renders localized completed/cancelled status. Route links are shared `AppLink`; stale cursor retry returns to the first page without reissuing the failed cursor. | Covered by source and navigation assertions |
| §6.2 heading fields and shareable link | Detail shows name, dates, saved timezone, status, coordinator (including missing member), mode, actual-start time and platform-derived copied link. Navigation test checks the share URL and manual/coordinator text. Accepted Web/native screenshots show these fields. | Covered |
| §6.2 summary counts, two ratios and whole-iteration scope | `IterationHistory` prefers snapshot statistics and renders current/original/cancelled/effective/completed/remaining, scope-event counts, net change and both ratios. Tests assert every counter and null ratios; 61-task live filter test leaves original count 61 while filtering to one item. | Covered |
| §6.2 five task filters/groupings | `IterationIssueList` supports actual status key, assignee, project, priority and captured label ID. `iterationGroupedIssuesOptions` walks the complete filtered result and rejects duplicates, changed scope/total and incomplete traversal. Live 61-task test proves 31/30 priority groups, then high+label yields one. | Covered; all five wiring branches inspected, not five separate browser matrices |
| §6.2 current vs original and historical values | Task scope switches between current and original. Closed lists use frozen API projections. `IterationCurrentComparison` reads current data only when opened and preserves separate historical/current values. Real history E2E edits title/priority, renames/deletes label, deletes task and retains the snapshot. | Covered |
| §6.2 ranges, events, chart, history controls | Summary and events are separate; chart has effective/completed/original series, dashed original line and same-value table with column/row headers. Frozen end type/reason, processing time, destinations and counts remain available. Screenshots show the table on narrow Web/native. | Covered; task identity added to events above |
| §6.3 detail/create/batch/project/T1 entry points | Issue detail mounts assignment and participation, create carries target revision and explicit completed acknowledgement, batch passes the resolved full selection, project page filters by current iteration, T1 only offers the candidate field with `iteration_assignment`. Mounted 1,000-task assignment regression proves two complete preview calls and no per-UUID reads. | Covered by wiring, canonical component/API tests and live assignment |
| §6.3 prior participation and unavailable functionality | `IterationParticipation` is separate from current assignment and remains available while planning settings are disabled. Real history E2E navigates from a task's participation to the frozen period. Unknown modes remain readable but do not offer new manual actions. | Covered; mobile guidance handled in separate lane |
| ITR-001/002 enable/create/edit/default calendar | Explicit timezone confirmation and shared P1 timezone write; creation uses 14 inclusive saved-zone calendar dates. Form validates Unicode code points without truncating input and warns on open-period overlap. Calendar and mounted form regressions cover DST dates, oversized astral names and conflict preservation. | Covered |
| ITR-003/004 and §8 lifecycle/date edit restrictions | Action availability follows planned/active/closed status; started start date is disabled/omitted; closed edit writes only name/description with reason. Start/today and terminal selections use full server preview. Handoff uses explicit next target and atomic confirmation. | Covered; server uniqueness/date/empty-baseline proofs belong to parent matrix |
| ITR-005 overdue only | `iterationIsOverdue` only marks active periods after the saved local end date; labels keep status active. Pure calendar regression includes local date and non-active cases. | Covered; reminders/execution invariants are backend-owned |
| ITR-006/007 admission and terminal choices | Shared candidates require supported manual planned/active periods. Picker excludes cancelled and hides done unless explicitly allowed with an active target. Assignment preview carries the actual source/revision. Unknown modes stay read-only. | Covered; terminal readable labels fixed above |
| ITR-008/009 execution separation and whole selection | Exact selected set is validated against complete server preview; no UI path starts an execution as a planning effect. Live closure checks zero dispatches, preserved pointers/counts, and native stale409 keeps destination/reason before re-preview. | Covered; running execution preservation is backend/service evidence, not a real-agent test here |
| ITR-010 readable scope history | Events display kind, actor, saved-zone business time, reason, explicit known-null versus uncaptured source/target, guarded period links and frozen task identity. Existing cancellation regression distinguishes period cancellation from task cancellation. | Covered after bounded identity fix |
| ITR-011 count semantics | Two ratios, count-not-workload explanation, parent/subtask statement, cancelled separately, fixed-original line and table. Filter operations do not recompute summary. | Covered; A–J formula truth remains canonical Go/history evidence |
| ITR-012 complete close preview | All preview issues show identifier/title/project/assignee/category/running count/rollover count; full affected total, completed/cancelled/remaining and destination totals are visible. Per-task planned destinations support bulk default plus individual override. No default action submits. | Covered; final preview time and rendered confirmation proof refreshed after this fix |
| ITR-013/032 recovery | Core persists exact original operation payload/ID. Unknown results check original receipt and only retry the same payload. Definitive conflicts permit editing; access denial clears protected queries, payloads and pending storage. Dialog/recovery survives closed or disabled refetches. Native E2E exercises real409 and committed-response loss. | Covered |
| ITR-014 rollover and ITR-015 frozen release | UI displays prior/current counts and >=3 review guidance; frozen destinations show before→after count and named guarded source/target. Live closure asserts remaining count 1, terminal count 0 and release; reopening/title changes do not mutate snapshot. | Covered; count arithmetic/replay truth backend-owned |
| ITR-016 cancel/delete/disable | Planned/active cancellation have distinct target behavior; delete is only offered for planned and server rejection explains used plans; disable only owner/admin, complete preview and recovery independent of enabled state. Real handoff case disables through UI and verifies old history retained/new active closed. | Covered; >one-page disable and rollback proofs backend-owned |
| §8.2/8.3 saved time/date and immutability | Date-only strings are rendered directly; actual times use saved period timezone and locale. Existing plans retain timezone; coordinator missing state is explicit; closed metadata edits cannot rewrite snapshot data. | Covered |
| ITR-019 immutable metadata and deletion | Frozen priority/labels may be absent/null and remain unknown; missing values are never filled from live directories. Current comparison is explicitly separate. Deleted/inaccessible task message retains frozen row and metrics, and no new task is created. | Covered by real 61-task history case and mounted legacy tests |
| ITR-020 adjacent handoff | Shared dialog chooses one explicit next active and per-task planned destinations, shows new baseline including carryover. Live handoff asserts new active original=2 from existing planned+carried task. | Covered |
| ITR-033 old client/API drift | API schemas reject malformed success receipts and wrong identities, accept optional legacy unknown fields and safely retain unknown enum read-only display. Qualified workspace-denied404 is never old-server fallback or a replay POST. Mobile ordinary title edit test omits iteration fields while parsing them from response. | Covered at API/schema/ordinary-write layers; physical old binaries not exercised |
| ITR-034 failures/accessibility/large lists | Native HTML controls and shared Dialog preserve keyboard flow; live fixture presses Enter for start/final confirmation. Long names wrap in screenshots, list names have full `title`; loading/processing/conflict/denial/unknown states are distinct; input locks prevent stale preview/selection mismatch. | Covered at stated layers; no full assistive-technology certification |
| ITR-035 capacity | Complete previews are independent of visible pages; 1,000-item mounted regression rejects truncation and avoids N+1 issue reads. Root-owned P95 artifact measures actual 1,000-task paths; UI immediately marks preparing/processing and disables repeated submit. | Performance thresholds/provenance remain in `performance-evidence.json` |

## Verification from this audit

All test commands used `scripts/go-test-with-agent-cli-guard.sh`. These mounted
tests use mocked API fixtures and no database; no server/browser/DB was started
by this reviewer.

1. RED: operation/details suites — **3 failed, 19 passed**, exposing the three
   behavioral omissions above (`ui-final-audit-red.log`).
2. GREEN: operation/details/navigation/locale parity suites — **100 passed in
   four files** (`ui-final-audit-green.log`). These are the final run's actual
   counts, not a sum of overlapping historical runs.
3. `pnpm --filter @multica/views typecheck` — exit 0
   (`ui-final-audit-typecheck.log`).
4. Scoped ESLint for the three modified views and two modified test files —
   exit 0 (`ui-final-audit-lint.log`).
5. `git diff --check` — exit 0. Existing pnpm configuration deprecation warnings
   remain informational and are not caused by this change.

## Final rendered verification

Root-owned `.omx/logs/i1-verification/final-e2e-corrected.log` reports **4 passed
(24.2s)**: real Electron closure/recovery, real 61-task historical metadata,
real Web closure/Chinese narrow history, and Web atomic handoff/workspace
disable. All new manual-mode/preview-time/event-task assertions ran on those
production builds. Earlier retries involved a mismatched verification
environment and a source-fingerprint guard; they are retained by the root
session and are not product failures or the basis of this signoff.

This reviewer visually inspected all 12 non-attachment screenshots from
`final-e2e-corrected-results`. Copies and hashes are in
`browser-evidence/final-screenshot-manifest.json`; the strict verdict is
`browser-evidence/visual-verdict-final.json`: **94/pass**, threshold 90.
The same result, limits and previous review records are preserved in
`.omx/state/iterations-i1/ralph-progress.json` without activating a runtime mode.

- Web/native confirmation previews show readable `Previewed at` plus saved UTC,
  full affected count, per-task destination, status/execution/count details and
  unchanged-execution guidance. The native screenshot also shows the final
  confirmation button; shorter Web windows scroll the same dialog, and the
  actual keyboard confirmation succeeds in E2E.
- Task events now distinguish the two frozen task titles and their stable UUIDs
  where the event never captured a readable identifier. Names, source/target,
  actor, reason and local business time remain readable; no live title is used
  to enrich the event after closure.
- Wide Web/native history retains long period names, proper native tab name and
  icon, two completion rates and explicit count semantics. Chinese Web 390px
  and native 680px keep the chart table and controls within the viewport.
- The filtered history screenshot still shows one matching task while the
  frozen chart table retains 61 tasks; the deleted-task explanation remains
  visible after label rename/deletion and live task deletion.
- No remaining demonstrated UI defect is left unfixed in this review's scope.

## Remaining verification boundaries

- Post-fix rendered signoff uses only the correctly configured final run above.
  Earlier `*-accepted.png` screenshots remain preserved as prior references.
  Per-task terminal retain choices/revocation have mounted canonical coverage;
  the live handoff renders the master retain control but does not exercise the
  full terminal-choice matrix. Manual-mode text has real-flow assertions but no
  dedicated settings screenshot.
- The 49 full-suite skips are accounted for in `skipped-tests.md` and
  `skipped-tests.json`: separate password/device-auth/platform-admin/managed
  publishing/compiled-CLI/provider and other native feature fixtures were not
  configured. All four I1 cases actually ran in the accepted suite; their
  environment guards did not skip them. This does not establish those separate
  integrations or relabel their skipped cases as passes.
- Mobile full iteration editing remains explicitly deferred by I1. Compatibility
  is established through response/write fixtures and current source, not a
  physical-device installation of an older released build. The separate mobile
  report verifies its supported-version policy and source wiring; native Alert
  rendering/tapping remains an explicitly recorded limitation.
- Screenshot coverage is English wide Web/native history, Chinese 390px Web,
  native 680px chart/table and filtered frozen metadata. It is not a claim of
  exhaustive theme/browser/OS/screen-reader coverage or multi-day chart visual
  comparison. Canonical data/time tests cover those business calculations.
- No release flag was enabled by this reviewer. No commit, push, merge,
  deployment or external notification was performed.
