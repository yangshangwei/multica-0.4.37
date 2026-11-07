# Final component browser verification

**Status: passed within the isolated component scope. Visual verdict: 94/pass.**

The full confirmation batch passed 26 scenarios and exposed the one remaining cancellation-focus defect in its light/dark cases. After the authorized minimal cancellation handoff fix, both focused cases pass. This is 28 scenario contracts covered by the retained full-batch evidence plus the focused final delta, not a claim that the entire matrix reran after that last change.

## Evidence

- Full confirmation: `browser-round-2/results.json` (26 passed; the two historical cancellation failures are retained honestly).
- Final focused verification: `browser-final-focus/targeted/results.json` (2 passed, zero failures/pageErrors).
- Exact final-source comparison: `final-source-check.json`. The only product hash delta from the full confirmation is `packages/views/collaboration/attachment-picker.tsx`; the verifier script also changed to add focus tracing. Every final recorded product/harness hash matches current disk.
- Final visual verdict: `visual-verdict-final.json`; live state `.omx/state/collaboration-create-composer/ralph-progress.json` records 94/pass and no pending actions.
- Root-cause trace retained at `cancel-focus-trace-round-2.json`: Cancel removal caused dialog focus, recoverFocus briefly selected Attach files, then delayed modal restoration took focus back. Final trace now shows Cancel click → Attach files focus → Cancel removal, with no subsequent dialog focus.

## Verified behavior and geometry

- Desktop 1280×900, coarse 390×740 and 320×568 in both themes: composer hierarchy, retained header/footer, independently scrolling body, no viewport horizontal overflow, audited targets >=44px, visible 24-step Tab sequence.
- Full account IDs and same-name account distinction remain visible in candidate rows and selected trigger. The narrow popup now fits available height and scrolls.
- English 320px header: source width increased from 50.31px to 212px; its selected text needs 83.50px. Breadcrumb wraps. The title hint is initially visible at y198.41..214.41; the body retains 224.19px height. Known-source title receives initial focus in both themes.
- 32 long filenames across all six viewport/theme combinations: checkbox list remains in body, footer position stable, last file reachable by focus, no horizontal overflow.
- Advanced configured count and invalid auto-expansion; source lock; specific quota refusal; original-key recovery; exact recipient selection; parent debounce; default shared picker/dialog compatibility remain covered.
- Native/CDP composition: composing Enter makes zero POSTs; a subsequent ordinary Enter makes exactly one request preserving plain title/description. No copied Cmd/Ctrl shortcut was introduced.
- Final focus cases: empty file selection sends no upload; Cancel upload, successful original upload recovery and retry all leave focus on Attach files. Cancellation's visible focus is recorded by light/dark screenshots. Enabled recovery focus indicators measured 3.73:1 light / 4.76:1 dark; those numbers do not describe the disabled cancellation state.

## Representative screenshots

- `browser-round-2/desktop-light.png`, `desktop-dark.png`
- `browser-round-2/english-320-empty-light.png`, `english-320-empty-dark.png`
- `browser-round-2/coarse-320-light-picker.png`, `coarse-320-dark-picker.png`
- `browser-round-2/attachments-32-320-light-bottom.png`, `attachments-32-320-dark-bottom.png`
- `browser-final-focus/targeted/attachment-cancel-light.png`, `attachment-cancel-dark.png`
- `browser-final-focus/targeted/attachment-recovery-light.png`, `attachment-recovery-dark.png`

## Reproduction and cleanup

```sh
node .trellis/tasks/archive/2026-11/11-07-collaboration-create-composer/research/preview/build.mjs final-focus
node .trellis/tasks/archive/2026-11/11-07-collaboration-create-composer/research/preview/verify.mjs final-focus attachment-focus-light,attachment-focus-dark
```

The final bundle was built to `/tmp/collaboration-create-composer-final-focus-95198`. The driver created only its own ephemeral loopback listener and Chromium contexts; every run records `listenerClosed: true` and `browserClosed: true`. No application API/Web/Electron process was started or stopped. No new dependencies, real accounts, real submissions or external API transport were used. Two existing CSS optimizer warnings for the custom `::highlight` selectors appeared; production builds completed.

## Boundaries

The harness compiles actual shared components/CSS/locales, uses mock fetch responses and a page-local memory StorageAdapter, and blocks external browser traffic. It proves component rendering, DOM/AX relationships, synthetic keyboard/touch, native browser form events and query wiring. It does not prove real backend acceptance, Next navigation, Electron Main/native packaging, physical touchscreen, spoken screen reader, real OS file chooser or actual system IME. Ordinary composer source is the primary reference; a saved earlier e2e screenshot was inspected only as a supplemental appearance reference because its current-build provenance was not established.

Browser worker changed only this task's research harness/evidence and the authorized visual state JSON. No product source, locales, tests or project specs were edited by this worker.

## Archival portability note

After browser verification, the parent changed only the build helper's provenance-path construction to resolve its own three fixture paths relative to its current directory. This lets the fixture run after Trellis moves the task into the archive. `node --check` validates that bookkeeping change; no product, CSS, locale, fixture-rendering, or verification-scenario bytes changed. The saved final-source-check records the pre-archive helper fingerprint at its original verification time; runtime product hashes remain exact. Run the reproduction commands from this task's archived path after archive.
