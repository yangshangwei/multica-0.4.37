# Assessment B — detector and independent browser evidence

Method: isolated assessment `/root/blocked_evidence_review`. Assessment A's report was not read. Detector findings were withheld from the parent until the parent confirmed A complete. This assessment changed no product source.

The final scoped checks pass. The disabled helper text exceeds 3:1 in both themes, same-state helpers have identical computed color and opacity, the recipient hint remains adjacent to its picker, and the footer/submit control remains reachable in all sampled language, viewport, theme and recipient states.

## Target and version

- Target: `packages/views/modals/quick-create-issue.tsx`, with `quick-create-actor-picker.tsx`, `creator-recommendations.tsx` and `issue-description-assist.tsx`.
- Final isolated Web URL: `http://localhost:60677`.
- Build ID: `yPiCLeEob9_90sBlHK-1y`, confirmed by the browser owner and independently observed in the served HTML. See `artifacts/qa/quick-create-blocked-20261120/review-b/build-binding.json`.
- Final panel SHA-256: `9bf089caf226d7baef748e95ad9c532874cc8cea4ad792496685556161183e94`.
- Final recommendation SHA-256: `319e6b39f691352ba2542b57cff0bd6ad807613c0d4909fcbbd0a14d29162820`.
- All four hashes are recorded in `review-b/final-source-hashes.json`.

The first browser attempt reached the application's error boundary because another session rebuilt the shared `.next` while the previous server still served it. The browser owner diagnosed that build mismatch and replaced it with a private candidate. `review-b/initial-page.json` preserves the invalid attempt; it is not acceptance evidence.

## Deterministic detector

`impeccable detect --json` ran once on the four TSX files before the final hint move: exit **0**, findings **[]**, empty stderr. After the panel's markup changed, the parent authorized one new scan of those changed final bytes: exit **0**, findings **[]**, empty stderr. No repeated scan of an unchanged version was performed.

There are **0 CLI findings, 0 rule names and 0 file locations to triage**. The two runs and hash boundaries are preserved in `detector-run.json`, `source-hashes.json`, `detector.json`, `final-detector-run.json`, `final-source-hashes.json` and `final-detector.json`. `.impeccable/critique/ignore.md` does not exist.

## Independent browser matrix

Fresh isolated headless Chromium contexts exercised **16 combinations**, each measured empty and filled, for **32 samples**:

- English and Simplified Chinese.
- Light and dark themes.
- `1280×900` and `375×812`.
- A usable recipient and an empty recipient directory.

The real application dialog and editor were mounted. The supplied synthetic review account was authenticated through a private file; browser-local route responses selected language and empty-directory state. No credentials were written into artifacts. Creation and recommendation requests were guarded with browser-local responses, and the measured blocked paths issued **0 creation and 0 recommendation requests**. No daemon ran.

### Actual helper contrast

The measurement reads the real button's computed color and ancestor backgrounds, converts browser CSS colors to sRGB through a one-pixel canvas, composes the actual opacity, then calculates WCAG luminance contrast. It does not compare opaque token values while ignoring disabled opacity. Screenshots show the same rendered surface.

| State | Light | Dark | Both helpers match |
| --- | ---: | ---: | --- |
| Enabled | 5.9126:1 | 6.3398:1 | Yes: same color, opacity 1 |
| Disabled | 3.7365:1 | 4.9521:1 | Yes: same foreground color, opacity 0.5 |

With no recipients and a filled description, recommendation remains disabled while refinement is enabled. Their different colors in that case are expected because their states differ.

### Hint grouping

The picker-to-hint gap is **4px in all 32 samples**. The hint-to-description-label gap is larger throughout:

| Available recipient | English empty / filled | Chinese empty / filled |
| --- | ---: | ---: |
| 1280px width | 32 / 12px | 12 / 12px |
| 375px width | 80 / 60px | 60 / 60px |

The no-recipient notice intervenes before the description, producing hint-to-description gaps of **152–264px**. The local picker-to-hint gap remains 4px. This confirms the final grouping fix on the actual final build.

### Footer and blocked feedback

All 32 samples have `footerFits`, `submitFullyVisible` and `submitHitTarget` true; the document scroll width equals its viewport width.

| Viewport | Dialog bounds (x, y, width, height) | Footer bottom | Submit bounds |
| --- | --- | ---: | --- |
| 1280×900 | 304, 230, 672, 440 | 670px | English: x791.72, y630, w168.28, h28; Chinese: x813, y630, w147, h28 |
| 375×812 | 12, 86, 351, 640 | 726px | Both languages: x235, y670, w112, h44 |

For empty descriptions, the submit button is not natively disabled, carries `aria-disabled="true"`, and its accessible description is the correct localized reason. A real pointer click redirects focus to the editor with no request. For the empty recipient directory, the reason identifies the missing recipient and pointer activation focuses the first setup link. The notice displays its explanation and both setup/manual exits.

A separate real keyboard Tab check focuses the submit button and exposes **“Write a task description first”**. A separate mouse hover exposes the same visible text. The tooltip width/height is nonzero, and the always-present `aria-describedby` points to the same reason. See `reason-evidence.json` and its two screenshots.

Measurement caveat: `measurements.json` contains an exploratory `focusedReasonVisible:false` field based on `getByRole('tooltip')`. This is an invalid probe for this component: rendered tooltip content has `data-slot="tooltip-content"` and no explicit `role="tooltip"`. The subsequent real Tab/hover inspection checked the actual element and passed. It supersedes that field; it is not a product defect.

## Browser detector visualization

Native CUA had no browser surface and the in-app browser was unavailable in the sibling assessment's concrete attempt. The fallback used a new isolated headless Chromium browser and never attached to human Chrome or Desktop.

Mutable injection preflight passed: the owned page title changed and an appended script executed. The bundled live server started on port 8400, `detect.js` injected successfully, and detector console output and an overlay screenshot were captured. The server was stopped with `impeccable live-server stop --keep-inject` from the owned artifact directory; stop exited 0. The browser closed. No human-visible overlay is claimed.

The captured console group reports **“[impeccable] 1 anti-pattern found.”** Its filtered log does not identify which banner entries count as primary versus advisory. The screenshot `detector-overlay.png` contains these concrete labels:

| Overlay label | Observed target/evidence | Scoped interpretation |
| --- | --- | --- |
| OVERUSED FONT | `Primary font: inter (100% of text)` across the rendered Issues page and modal | Generic detector preference, not a task defect. Inter is the existing product typeface; replacing it exceeds this refinement. |
| FLAT TYPE HIERARCHY | `body 14px, h1 14px, h2 16px`, largest adjacent step `1.14:1`, target `1.25:1` | Whole-page semantic/type-scale heuristic. The compact task-operating interface uses the existing role tokens; this scan does not establish a regression in the four edited files. |
| SKIPPED HEADING LEVEL | Banner begins `<h1> "Issues" followe…`; remaining text is clipped | The identified h1 belongs to the background Issues page beneath the dialog. The exact subsequent element is not captured. This is a page-plus-modal semantic observation, not a proven new task blocker. |

These labels have no source-file locations in the overlay evidence. The CLI returns no source findings. Do not equate the three visible banner labels with three primary findings or invent a machine rule ID. No scoped P1 was established by the browser overlay. The known dialog accessible-title and typography questions remain outside this approved task.

## Artifacts and limits

Full measurements: `artifacts/qa/quick-create-blocked-20261120/review-b/measurements.json`. The directory retains all 32 matrix screenshots plus the detector and reason evidence. For a compact committed visual set, these eight screenshots are sufficient:

1. `en-light-1280-available-empty.png`
2. `en-light-375-available-empty.png`
3. `zh-Hans-dark-1280-available-filled.png`
4. `zh-Hans-dark-375-available-filled.png`
5. `en-light-375-no-recipients-empty.png`
6. `zh-Hans-dark-375-no-recipients-empty.png`
7. `blocked-reason-keyboard.png`
8. `detector-overlay.png`

The full numerical evidence should remain even if redundant screenshot files are omitted from the commit. `measure.cjs`, `reason.cjs` and `overlay.cjs` reproduce the probes given a private `REVIEW_AUTH_FILE`; they contain no token. `credential-check.json` records zero exact-token or JWT-pattern matches in the owned artifacts.

This review does not claim physical touch testing, assistive-technology speech output, backend dispatch, upload/autonomy/version matrices or native Electron behavior. Those require the task's separate behavior verification. The narrow no-recipient state intentionally scrolls the description region while preserving the warning exits and fixed footer; this assessment verified the requested footer and focus reachability, not a new editor-height contract.

Questions skipped: the user approved the scoped completion and archive; no additional design decision is required.
