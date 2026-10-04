# T1 UI implementation progress

Worktree: `/Volumes/artisan/code/2026/multica-triage-t1`, branch `codex/triage-t1`.
Owner: native executor `triage_ui_impl`; parent owns final integration and browser/visual validation.

## Implemented

- Shared `packages/views/triage/` view and exported Web/Desktop session routes; `paths.workspace(slug).triage()`, route icon/tab identity, search keywords and editor link routing.
- Default-off workspace settings with owner/admin gate, explicit save/revision, supported-vs-error distinction, pending/snoozed disable block and queue link. Settings/page mount their draft state under workspace identity.
- Enabled-only sidebar with global actionable count. Queue URL carries view, filters, sort, offset and issue selection. Global action/import history uses the immutable history endpoint.
- Resizable list/detail, compact navigation preserving filters/sort/scroll, shared IssueDetail reuse, pending content/comments/attachments without execution affordances. Review actions live in a fixed footer.
- Manual intake reuses title/content editors, attachment uploader and draft pickers, creates no formal task or run hint. Request ID persists over transport retries of an unchanged submission.
- Atomic full-field acceptance with required-priority focus, optional note and separate explicit accept-and-execute. Reject/reopen reasons; formal same-workspace duplicate picker and pasted link; actual local-zone snooze presets/custom instant; reviewer reassignment and unsnooze.
- Failed/pending execution remains visibly accepted with original-actor retry. Immutable history displays changed product fields, localized enums, related identities and actor/time/source; storage metadata/JSON is not shown.
- Selected-only batch preflight, explicit valid-row confirmation, frozen input snapshot, per-row outcomes and unsuccessful-only retry preserving row request IDs.
- UTF-8 fatal CSV decode, size limit, server preview/mapping, warning/error rows, selection and per-row duplicate override, progress, partial results/GET reconciliation, retained unselected rows, download failures. `?view=history&batch=<id>` opens a durable batch result.
- English and Simplified Chinese locale resources registered; named controls, semantic tokens, compact/coarse 44px targets, queue-only shortcuts with editable/portal guards and post-decision focus recovery.

## Test-first evidence

Initial `triage-ui.test.ts` failed on missing helper implementation and settings IA tests failed on missing triage navigation. The subsequent history field-diff test failed before its implementation. Component tests cover required inputs, retained reason/request identity, no advance before server success, workspace settings gates, queue keyboard scope, direct pending detail, CSV encoding/selection/duplicate override and selected-only partial retry.

## Executed checks

- `pnpm --filter @multica/views exec vitest run triage locales/parity.test.ts settings/components/settings-nav.test.ts settings/components/settings-page.test.tsx layout/app-sidebar.test.tsx issues/components/issue-detail.test.tsx issues/components/issue-detail-route.test.tsx`: 12 suites / 225 tests passed (latest rerun tracked below).
- `pnpm --filter @multica/core exec vitest run paths`: 8 suites / 84 tests passed.
- Scoped views ESLint initially had no errors and one unstable-empty-array dependency warning; replaced with stable constant before final rerun.
- Views typecheck earlier passed own source and then saw only parent inbox changes in progress; final views/Web/Desktop checks tracked below.
- `git diff --check`: passed before final formatting-only tweak.
- Prettier ran from a temporary tool cache on new triage source only; no application dependency/lockfile changes.

## Integration references and remaining verification

Incumbent reference: `/acme/inbox`, `packages/views/inbox/components/inbox-page.tsx`; settings uses existing SettingsTab/Section/Row; IssueDetail owns scrolling/comment rendering.

New routes: `/{slug}/triage`, `/{slug}/triage?issue=<id>`, `/{slug}/triage?view=history`, `/{slug}/triage?view=history&batch=<id>`, `/{slug}/settings?tab=triage`.

Accessible English test locators: region `Triage queue`; row button `{identifier} {title}`; `Create pending task`, `Import CSV`, `Enable triage`, `Save settings`, `Reason`, `Confirm selected valid tasks`, `Import selected rows`. CSV file input: `#triage-csv-file`.

No screenshots, real-browser results, visual verdict, release or commit are claimed by this lane. Parent owns the production Web/Desktop browser pass, final visual verdict and full acceptance audit.

## Final lane check (2026-10-05 00:54–00:57 Asia/Shanghai)

- Views targeted regression rerun: **12 suites / 225 tests passed** (exit 0).
- Core path registry/route tests: **8 suites / 84 tests passed** (exit 0).
- `pnpm --filter @multica/views typecheck`: **passed** (exit 0).
- `pnpm --filter @multica/web typecheck`: **passed** (exit 0).
- `pnpm --filter @multica/desktop typecheck:web`: **passed** (exit 0).
- Scoped ESLint for triage, direct IssueDetail/CommentInput, settings/sidebar, search/link routing and localization registrations: **passed with zero warnings** (exit 0).
- `git diff --check`: **passed**.

Source handed to parent as stable for production build. No application dependency was added. No commit performed. Remaining proof is parent-owned real-browser Web/Desktop flow, screenshot/visual verdict, full repository checks and complete acceptance ledger.

## Search palette integration follow-up (2026-10-05)

The parent full suite exposed the palette test's incomplete hand-written workspace path mock. Replaced it with the real path builder and added product regressions for supported/enabled, disabled, unsupported and settings-not-loaded cases. Those three unavailable cases failed before adding the capability gate. Search navigation now includes triage only when support and enablement are both confirmed; all other registry destinations retain their prior behavior. Focused palette suite: **36 tests passed**.

## FR09 / creator filter audit follow-up (2026-10-05 01:58 Asia/Shanghai)

- Added `triage-execution-preview.tsx`: read-only agent/squad/runtime/project/resource queries compose the shared invocation permission and runtime-binding checks. The accept-and-execute confirmation now names the resolved agent (or squad leader), runtime, project, and repository/local-directory resources, including execution mode and machine applicability.
- Confirmation and the submit handler both reject unresolved, failed, archived/missing, unauthorized, offline/unavailable or invalid-resource prerequisite state. Hidden runtimes use the agent's privacy-safe availability signal; invocation is not incorrectly gated by runtime-binding permission. Ordinary acceptance retains its no-execution behavior.
- No generic pending-task trigger endpoint, enqueue, remote preparation or write runs during preflight. Project local resources are summarized as configured resources: a daemon uses its own matching directory or the existing repository/default path, so this UI does not invent a new mandatory-machine rule.
- FR04 creator selection now includes member user UUIDs and agent UUIDs with distinct localized labels; reviewer/processed-by selectors remain human-only.
- Test-first evidence: all six initial execution-confirmation regressions failed before the implementation; creator selection failed before agent options were wired. Final execution tests also cover squad-leader resolution and hidden shared runtimes.
- Final verification: **10 suites / 128 tests passed** (`triage`, locale parity, search palette), Views typecheck **passed**, scoped triage ESLint **passed with no warnings**, `git diff --check` **passed**. All command exits were 0.

Source stable again for parent production build/browser validation. No commit performed.

## Production-browser / visual findings follow-up (2026-10-05 03:05 Asia/Shanghai)

Parent recorded visual verdict 86/revise before this pass at `.omx/state/triage-t1/ralph-progress.json`; original reproduction is `/tmp/multica-triage-focus-diagnostic.log` and `.omx/triage-t1/e2e-focus-diagnostic/`.

- Confirmed search-clearing race: the input and asynchronous decision callback rebuilt URL replacements from the adapter's earlier search snapshot. Added view-local `use-triage-route.ts`: synchronous latest intended parameters control the UI; only one platform replace is outstanding and its acknowledgement releases the latest queued intent. External route/history changes are still adopted. No fixed delay, retry timer, core navigation edit, or weakened browser focus assertion.
- Local rejection validation now refocuses the reason field. The concurrency explanation/refresh action is restricted to actual revision/concurrency errors; network/unknown failures retain their original message without falsely claiming concurrency. Refreshing an actual conflict clears the resolved error and retains the draft.
- CSV row status now preserves created/failed/skipped outcomes, then distinguishes invalid rows, duplicate external IDs and warnings; only clean ready rows say Ready. Explicit accept-and-execute no longer displays ordinary acceptance's no-execution explanation.
- Added actual `TriageExecutionStatus` component proof: accepted-but-failed banner and reason remain visible, only the original authorizer can retry the existing action ID, and retry never accepts again.
- Test-first evidence: seven assertions across four suites failed before these fixes (search intent, misleading validation, CSV effective status and explicit-execute copy). Final targeted verification: **10 suites / 97 tests passed**, Views typecheck **passed**, triage ESLint **passed with zero warnings**, and `git diff --check` **passed**; all exits 0.

Source handed back stable for fresh production builds and parent-owned browser/visual revalidation. No screenshots or real-browser success are claimed for the corrected build yet.
