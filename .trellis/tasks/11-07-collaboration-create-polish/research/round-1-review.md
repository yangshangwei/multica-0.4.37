# Round 1 visual review

Verdict **86/revise** persisted before correction. Current source hashes matched the completed first build `/tmp/collaboration-create-polish-round-1-22016`; browser/listener closed; zero page errors.

## Screenshot judgement

Compared directly with user-reference.png, `browser-round-1/dialog-default-722-light.png` has a substantially lighter title focus line, smaller empty description, full-width one-line recipient ID, no bold empty attachment legend, and a single desktop footer row. This addresses the visible form density; it is not merely a numerical test improvement.

Three bounded correction suggestions:

1. Empty722 light/dark: summary ends at y490 while footer starts at y561, leaving71px of unused lower body. Body content/client are both463px and scrollTop0. Reduce default36rem height by32px to34rem: still39px after summary, no need for initial scrolling. This also reduces the comparable desktop dead area.
2. English320 light/dark: Cancel top431.59, Create top483.59. Their widths67.36+172.67 plus8px gap total248.03px against248px available, so a rounding-size overflow forces another row. A smaller gap or bounded two-column action arrangement should keep both together and save52px.
3. Long source name from the supplied screenshot is visibly clipped in the current192px selector. The adjacent breadcrumb has space at722. Adjust caption font sizing or max width without altering native control behavior; check the cropped image rather than treating a nonzero width as enough.

## Measurements already sound

- Primary722×660 empty both themes: advanced summary fully visible, no body vertical overflow (463/463,scrollTop0), description64px, full UUID one line, empty attachment legend visually hidden while sharing guidance remains.
- Title indicator is genuinely bottom-border2px, no ring/outline, effective opacity1, measured contrast3.73:1 light/4.76:1 dark.
- Desktop and390/320 coarse full-ID, body/footer,24-step keyboard checks pass.32files across six viewport/theme cases pass. Upload chooser/cancel/recover/retry still pass. Advanced summary, refusal and pending-search checks pass.
- 200% equivalent reflow captures show header/actions/advanced summary visible and no document overflow in all4locale/theme combinations. This uses640×450CSS,DPR2 for1280×900physical pixels; it is not native browser zoom.

## Harness issues, not product defects

Five create-success/recovery checks failed because this task changed the fixture's actor to the realistic screenshot UUID but inherited the old creator identity in collaborationIssueFixture. Core correctly refuses an issue whose creator is not the current actor (creation.ts:95). Fixture now overrides creator.user_id with actor. This is a mock consistency repair, not a product change.

Four zoom cases failed only the last focus-reveal assertion: the test manually scrolled while title remained focused, then called focus on the already focused title, so no focus event fired. Header/actions/summary/overflow checks all passed. The test now moves to attachment entry then focuses title, matching actual keyboard navigation. Both calibrations are prepared for the one confirmation build; no extra product batch or repeated failure bundle is needed.

Raw first result is29passed/11failed; that count includes2realEnglish-footer failures and9harness issues above. Retain it honestly, then use the corrected final run as acceptance evidence. Cropped final delivery target is `dialog-default-722-light.png`; before screenshot is local-only.
