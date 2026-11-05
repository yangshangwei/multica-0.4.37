# Research: Final collaboration touch-target supplement

- Query: Verify B6 rendered coarse-pointer targets and supplement B1 recipient keyboard evidence after the audit fixes.
- Scope: internal, bounded production-browser supplement; no product edits.
- Date: 2026-11-05
- Build: production Web supplied by the parent at `http://localhost:13493`, API `http://localhost:18573`; parent reports product commit `5a7440952`.

## Findings

The source filter and five supporting controls meet the 44px target floor in Playwright Chromium with `hasTouch: true`, `isMobile: true`, and a 390×740 viewport. Each measured control was in the viewport; the document had no horizontal overflow. The script verified `matchMedia('(pointer: coarse)').matches === true` for every target. Browser page errors were empty.

| Control | Rendered dimensions | Evidence source | Screenshot |
|---|---:|---|---|
| Source-workspace External collaborators filter | 44×44 | Real API, source issues list | `live-source-filter.png` |
| Attachment preview close | 44×44 | Real UI; controlled attachment metadata and text-byte responses | `fixture-attachment-close.png` |
| Shared-context historical version link | 44×44 | Real UI; controlled memory metadata/content | `fixture-memory-version.png` |
| Shared-context version dialog close | 44×44 | Real UI; controlled memory metadata/content | `fixture-memory-close.png` |
| Knowledge Content SHA-256 disclosure | 324×44 | Real UI; controlled candidate/list/detail projections | `fixture-knowledge-digest.png` |
| Knowledge Version history disclosure | 324×44 | Real UI; controlled candidate/list/detail projections | `fixture-knowledge-history.png` |

Fixtures were read-only browser route responses containing visibly labelled `SUPPLEMENT FIXTURE` records and a screenshot badge. They were not written to the server. Real issue identity, account authentication, application bundle, components and CSS were retained. The exact replaced read paths and measured rectangles are in `results.json`. Source filter inspection used direct CORS; no CORS shim was necessary.

The live recipient case remained **unverified**. Opening the real creation dialog and recipient picker succeeded, and the current self account was displayed. The filter textbox locator timed out before rapid-Enter assertions. One bounded selector correction also stopped at the picker locator. No keyboard-commit pass is claimed. The source's real recipient discovery returned no other accounts, so a distinct-account selection would have required additional setup; this supplement did not create any account. Component tests remain the canonical proof for stale/debounced recipient commitment.

The source-workflow dialog close was **not verified** in this supplement. The currently available collaboration row context menu exposes open/copy actions, so no source edit/lifecycle dialog was reached in the bounded pass. Its source and component checks belong to the parent verification, not this browser claim.

## Files found / relevant code patterns

- `packages/views/collaboration/source-filter.tsx:14` — local coarse-pointer minimum width/height on the source filter.
- `packages/views/collaboration/source-bridge.tsx:54` — local close-target styles on source workflow dialogs; not reached in the browser.
- `packages/views/collaboration/attachment-panel.tsx` — protected preview dialog with local coarse-pointer close styling.
- `packages/views/collaboration/memory-panel.tsx` — 44px historical version trigger and portaled close control.
- `packages/views/collaboration/knowledge-review.tsx` — coarse-pointer minimum height on digest/history summaries.
- `packages/views/collaboration/account-picker.tsx` — current-query pending and selection ownership; live supplement stopped before these assertions.

## Artifacts

- `results.json` — primary batched pass, rectangles, fixture paths, errors and request-method inventory.
- `recipient-correction.json` — bounded follow-up harness limitation; does not claim B1 browser acceptance.
- `browser-supplement.cjs`, `recipient-correction.cjs` — reproducible scripts, with credentials read only from the parent's private temporary session file.
- Six target screenshots listed above; `recipient-harness-limitation.png` records the unresolved picker locator state.

## Related specs

- `.trellis/spec/views/frontend/collaboration-creation.md`: coarse-pointer target, recipient identity, keyboard and query ownership contracts.
- `.trellis/spec/views/frontend/collaboration-detail.md`: shared native surfaces, focus and rendered viewport evidence.
- `.trellis/spec/views/frontend/collaboration-knowledge.md`: live exact review and source identity.

## External references / versions

No external documentation or new dependencies were used. Installed Playwright Chromium drove the current local production app. Impeccable context/detector were not rerun.

## Caveats / Not Found

- This is emulated coarse-pointer geometry plus synthesized taps, not physical-device or spoken-assistive-technology evidence.
- Only the source filter uses wholly real product records. File/history/knowledge screenshots are controlled fixture evidence, not API persistence or privileged-action acceptance.
- The request inventory calls all non-GET requests `writes`; observed requests were client usage telemetry and issue-table read endpoints (`/api/issues/table/facets`, `/api/issues/table/rows`). No collaboration creation, comment, invitation, file upload, lifecycle, or knowledge-decision operation was submitted.
- Both owned browser contexts and Chromium instances were closed. No server was started by this supplement. Services remain under parent ownership.
- Output is under the task's `research/supplement/` because the Trellis researcher role permits writes only in `research/`; the parent may integrate it into final evidence.
