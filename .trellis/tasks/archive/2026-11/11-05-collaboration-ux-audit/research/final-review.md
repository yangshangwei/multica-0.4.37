# Independent final review — collaboration UX audit

Date: 2026-11-05. Reviewer: dispatched `trellis-check` / `final_review`.

## Final verdict

**PASS** for the accepted collaboration UX repairs. No unresolved P0/P1/P2 product defect found in the task-owned changes after source, regression, static and representative rendered review. The accepted findings are closed within the explicit verification limits below.

The reviewer did not implement the changes under review, did not spawn helpers, and made no product changes. Shared work and unrelated dirty files were left intact. Review covers the task PRD/design/plan, accepted `review.md` repairs, both independent assessments, project conventions, relevant Views/Core specs, product diffs and new tests. Accepted findings below use the original A/B research identifiers because `review.md` renumbers some B findings.

## Findings (fixed)

None during final review. The implementation already closes the source-level findings below.

| Accepted finding | Implementation and independent review conclusion | Regression evidence |
| --- | --- | --- |
| A1 / B3: shared-context initial failure and hidden original recovery | `memory-panel.tsx` retains a named region during loading/error/unavailable responses, gates current protected content on successful data, and exposes retained original operations independently. Attention opens all enclosing native disclosures. Denial still clears protected content and editor state through the existing access fence. | Initial failure → explicit refresh; unsupported/null response; failed refresh with retained editor; two collapsed ancestors with original-key GET, no automatic POST; incumbent revocation/conflict/recovery cases. |
| A2 / B5: referenced-file disappearance | `CollaborationCommentAttachments` distinguishes no references, pending/error reads and missing references. It renders only current successful metadata and uses the existing protected query. `CollaborationAttachmentList` derives the open preview from the current file set, so removal closes it and unmounts/revokes its blob. No workspace file fallback is added. | No-reference/no-request; held/error/retry; partial metadata excluding unrelated files; failed refresh, removed reference and access denial all remove private preview/content. |
| A3 / B2: intentional workflow focus | Workflow opening has a stable per-action opening identity; `useOpenedForm` opens enclosing disclosures, reveals/focuses its heading once, and restores the actual opener only when focus is lost. Explicit repeated comment preparation uses current text/files, clears the basis for a separate explicit confirmation, and does not submit. Refetches and edits do not refocus the form. Existing default helper behavior remains compatible. | First/repeated comment preparation, contents, all ancestors, no POST, no scroll on edit/refetch, return to latest opener; primary-action focus/draft retention; StrictMode default-opener regression. |
| B1: stale recipient commitment | The picker gates rows and paging while raw/debounced input differs or Query supplies placeholder/pending results. Pinned self/selected/recent accounts are filtered for nonempty input. Actual server matches remain authoritative, including matches by fields not projected into rows. Same-query authorized cached rows retain existing refresh-error behavior. | Keyboard and pointer attempts during debounce and delayed results; intended identity after resolution; pinned-row filtering; incumbent IME, IDs, paging, parent-disable, exact-lookup and authorized-cache tests. Core owns protected denial/epoch behavior. |
| A4: loaded-row search/count/empty semantics | Workbench normalizes outer search whitespace, distinguishes unknown/loaded/total/matches, retains explicit paging and shows its pending state. An incomplete empty page cannot claim an empty queue. Clear search retains sorting, saved list context and independently selected detail. Empty current view links to participating with the current issue intact. | Two-page search with a second-page-only match, clear/focus/state restoration, unknown initial count and incomplete empty page, Chinese whitespace, selected-detail retention. |
| A5: app-locale dates | Activity, comment, comment-history and command/cancellation timestamps use `useLocale()`; ISO `dateTime` values remain unchanged. | Both locales across activity, comments/history and command/cancellation timestamps. |
| B4: field-specific UTF-8 feedback | Revision reason, result and basis each have their own byte-limit error/invalid/describedby relationship. The unmodified Core byte contract blocks submit and retains text; overflowing one field does not invalidate the other. | Multibyte reason/result/basis overflow, no POST, field attribution and correction enabling explicit submit. |
| B8: knowledge destination recovery | `OtherTarget` uses current normal-directory queries and distinct pending/error/empty states with local retry. Selected IDs remain local; absent/error targets clear the effective proposal destination without silently switching project to workspace. Parent summary/content/source choices survive. An async source-resolution submission rechecks destination identity before preparing its intent. | Held/error/empty workspace catalog; selected project failed refresh/retry with all draft fields; removed destination while source read is pending causes no proposal or implicit workspace retarget. |
| B6: coarse-pointer target omissions | Local minimum dimensions cover source filter, knowledge inputs/digest/history, memory version links and the source/file/memory dialog close controls. Portal selectors are attached to their own content; shared primitives/tokens remain unchanged. | Source review; final coarse browser contexts measure the knowledge summary at 44px and assert creation-close width/height at least 44px. Other changed controls are source-reviewed, not all individually measured in this batch. |
| B7: narrow fixed-header density | Narrow title is visually hidden while retaining h1 semantics; view links occupy one row and creation/notifications share a wrapping action row. Notification trigger retains its accessible name and factual unread count. | Navigation/notification suites remain green. Final 320/390px captures show a 120px workbench header and accessible compact actions; baseline narrow image had multiple separate title/action/notification rows. |
| A6 from combined plan: admin raw status | `IssueMetadata` uses existing collaboration status labels in both list and detail; unknown codes keep the established fallback. No ordinary issue query or content authority is introduced. | Both locales list/detail plus zero ordinary issue reads; production admin-projection screenshot shows localized status. |

## Findings (not fixed)

No concrete in-scope product issues found. Broader observations in the initial assessments (comment-control density, notification chronology, closed-panel polling and lifecycle copy) were not accepted repairs and remain outside this bounded change.

Earlier development navigation/chunk failures and the stale WebSocket port were local environment failures, not final acceptance evidence. The parent served a successful production Next build and corrected the owned runtime origins/WebSocket endpoint without product changes. The final main run uses direct real API transport without the earlier CORS shim. The production admin run has four passing list/detail checks and no page errors; it uses a mocked administration projection with real authentication, not a live privileged administrative operation.

## Verification

Parent-run commands were inspected directly, without duplicating broad concurrent commands:

- Tests: **PASS**, 30 files / 312 tests, `/tmp/multica-ux-final-tests.log`; collaboration, collaboration administration and locale parity. The pre-change baseline was 281 tests.
- Lint: **PASS**, zero errors and 27 pre-existing warnings, `/tmp/multica-ux-lint.log`. No new task-file warning was reported. Unrelated warnings were not edited.
- TypeCheck: **PASS**, Views/Web/Desktop, `/tmp/multica-ux-typecheck.log`, `/tmp/multica-ux-web-typecheck.log`, `/tmp/multica-ux-desktop-typecheck.log`.
- Production Web build: **PASS**, parent-run `/tmp/multica-ux-web-build.log`.
- Static detector: **PASS**, `evidence/final-detector.json` is `[]`. This scan does not substitute for interaction or visual inspection.
- Scoped whitespace/static diff check: **PASS**, reviewer-run `git diff --check` over changed collaboration/admin/locale/spec paths.
- Browser: **PASS**, `evidence/after/browser-results.json` contains 54 captures, 11 recorded check groups, no failure and no page errors. The four variants are 1440×1000 English/light, 1024×768 English/dark, 390×740 Chinese/dark/coarse and 320×568 English/light/coarse. Every recorded document and scroll-root/dialog width is within its container. Tests assert intentional workflow focus, visible prepared-result position, no preparation write, cancellation focus return and injected memory-failure retry in each variant. Source-workspace detail and a 200% root-font layout perturbation were also checked; the latter is not browser zoom.
- Administration browser: **PASS**, `evidence/after-admin/admin-results.json`, four localized list/detail checks with no page errors or horizontal document overflow. Metadata projection is mocked and explicitly marked.
- Spec sync: `collaboration-detail.md` records named failure/recovery, exact intentional form focus, per-field feedback and loaded-search/date behavior; `collaboration-creation.md` records query-owned recipient commitment. `collaboration-knowledge.md` records destination-directory recovery and current target checks. These match the reviewed source. No API/schema/platform-template change requires another generated touch point.

## Independent visual inspection

Inspected the baseline wide detail and Assessment B narrow detail, then these final captures using the image tool:

- `after/wide-en-light-prepared-result.png`: prepared text visible, empty basis and disabled explicit completion, existing compact typography/surfaces retained.
- `after/narrow-zh-dark-detail.png` and `after/compact-en-light-list.png`: compact named actions and view context, no clipping or horizontal spill; Chinese requirements visible in the initial detail viewport.
- `after/compact-en-light-prepared-result.png`: result form is revealed below the fixed shell and long text wraps in the available width; lower fields remain in the normal scroll flow.
- `after/narrow-zh-dark-knowledge-proposal.png`: readable localized form and visible focused summary control.
- `after/compact-en-light-creation.png`: dialog, close and stable cancel/submit actions fit the narrow viewport; body content scrolls independently.
- `after/medium-en-dark-context-read-failure.png`: named shared context, readable factual error and nearby Refresh action.
- `after/medium-en-dark-detail.png` and `after/narrow-zh-dark-discussion.png`: consistent semantic theme, hierarchy and translated date formatting.
- `after/wide-en-light-source-enlarged-text.png`: shared source-workspace content remains in bounds under the recorded root-font perturbation.
- `after-admin/en-list.png` and `after-admin/zh-Hans-detail.png`: status reads Done / 已完成 while metadata-only authority remains clear.

This is representative image inspection plus the whole batch's geometry/assertion evidence, not a claim that every pixel of all 58 captures was independently inspected.

## Scope and limits

Changed product files remain shared Views collaboration components, the collaboration administration metadata page and the paired collaboration dictionaries; associated tests and three scoped specs document the behavior. No dependencies, backend permissions, stores, routes or storage formats were added. The changes simplify recovery by preserving one readable region and reuse existing focus, query, label and byte helpers.

This review does not certify installed Electron Main, native mobile, real AI-provider execution, physical-device gestures, spoken assistive technology, comprehensive contrast or a new full business-lifecycle acceptance run. Main browser data belongs to test-owned real API accounts and uses direct transport; fault injection and administration projections are explicitly separate from real-server outcomes. Coarse geometry measurements cover representative controls, not every changed disclosure/control. Existing server/Core boundary suites remain the canonical exhaustive authority/receipt tests.

Final recipient supplement: parent corrected the harness to use the actual accessible search name (Filter options) and assert the search popup rather than the zero-height empty list. Production browser with real API and450ms injected search latency passed rapid no-match Enter (no stale/self commitment) and settled exact-self-ID selection, errors0. See evidence/recipient-browser.json. Distinct recipient matrices remain component-test evidence.
