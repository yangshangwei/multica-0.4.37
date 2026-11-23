# Final visual and component verification

**Status: passed within the isolated component scope. Visual verdict: 94/pass.**

The corrected full batch passed 40/40 after fixture calibration and captured the complete retained behavior matrix plus the new density/reflow cases. A final eight-case subset then verified the only subsequent product delta: the source selector header cap changed to `sm:max-w-64` (256px) to keep a long source name readable while preserving breadcrumb proximity. Final subset passed 8/8 with zero page errors.

## Visual result against the user screenshot

The final crop `browser-final-header/targeted/dialog-default-722-light.png` is the primary after image; the supplied before image remains `user-reference.png`. Compared with the before, the final composer:

- reduces the empty description to 64px;
- puts the required-title hint beside the title;
- replaces the heavy title rectangle with an opaque 2px bottom border (no ring/outline; effective opacity 1; measured contrast 3.73:1 light and 4.76:1 dark);
- keeps the full recipient UUID on one line in the 722px reference case;
- removes bold empty attachment chrome while retaining sharing guidance;
- keeps the advanced-fields summary fully visible at scrollTop 0;
- groups attachment, draft status, Cancel and Create in one footer row;
- reduces the bounded dialog height to 34rem, leaving a measured 39px breathing margin after the summary before the fixed footer.

The final dark crop is `browser-final-header/targeted/dialog-default-722-dark.png`. The desktop light/dark captures show the bounded source selector and adjacent breadcrumb; English 320 captures from the full batch show Cancel/Create sharing one row after the 6px gap correction.

## Evidence

- Full corrected matrix: `browser-round-2/results.json` — 40 scenarios, zero failures/page errors.
- Final header subset: `browser-final-header/targeted/results.json` — 8 scenarios, zero failures/page errors.
- Final hash proof: `final-source-check.json` records the user-approved focus integration; prior header-only proof is retained in `header-source-check.json`. The only compiled product delta since final header evidence is `creation-form.tsx`. The DOM regression test is newly hash-recorded but not imported into the browser bundle. All final recorded hashes match current disk.
- Final state/verdict: `visual-verdict-final.json` and `.omx/state/collaboration-create-polish/ralph-progress.json` record 94/pass.
- Representative after images: `browser-final-header/targeted/dialog-default-722-light.png`, `dialog-default-722-dark.png`; `browser-round-2/desktop-light.png`; `browser-round-2/english-320-empty-light.png`; `browser-round-2/attachments-32-320-light-bottom.png`; `browser-round-2/zoom-reflow-200-en-light-top.png`.

## Verified behavior and geometry

- 722×660 Chinese empty draft with long source, default self/AI: body content/client both431px, scrollTop0, advanced summary y470–506, footer y545–602, no initial scroll or document overflow.
- 390×740 and 320×568 in both themes/locales: title hint, footer actions, body reachability, full identity and coarse targets remain usable; English 320 Cancel/Create share the same top (measured y483.59).
- 200% equivalent reflow uses a 640×450 CSS viewport at DPR2 to represent a 1280×900 device-pixel canvas. All four locale/theme cases keep header/actions/summary reachable with no document/body horizontal overflow. This is CSS reflow evidence, not native browser-toolbar/OS zoom or physical-device evidence.
- Full 32-file long-name states across desktop/390/320 and light/dark retain body scrolling, fixed footer, full controls, recovery and visible focus.
- Existing source locking, rejection messaging, original-key recovery, exact recipient identity, advanced retained/invalid states, shared picker compatibility, native Enter and CDP composition guard, and compact upload cancel/recover/retry focus remain green from the full matrix.

## Provenance and cleanup

The harness compiles actual shared Views/UI/Core components, CSS and locale bundles with mock fetch and a page-local memory StorageAdapter. It blocks external browser traffic and never starts the full stack. Each build records source hashes; each run creates an owned ephemeral loopback listener and fresh Chromium contexts, then closes them in `finally`. Final subset reports `browserClosed: true` and `listenerClosed: true`. No product preview servers remain, and no real accounts, backend submissions, Electron Main, OS file chooser, physical touch, spoken screen reader or system IME were used. Two existing Tailwind optimizer warnings for custom `::highlight` selectors appeared; builds completed.

The browser worker changed only this task's research harness/evidence and authorized visual state JSON. No product, locale, test or project-spec files were edited by this worker.


## User-approved focus integration

After visual verification, the user explicitly authorized the concurrent initial-focus patch and its DOM regression test to be included and verified. The patch guards queued focus recovery so a live user choice wins and closed dialogs cannot regain focus. No appearance classes changed; the final header crops remain the visual evidence and the visual score remains94/pass, without a new taste iteration.

A current-source production rebuild ran only seven affected scenarios, all passing with zero page errors:

- Delayed source responses in light/dark at390×740 with touch input. One deterministic subcase temporarily queues requestAnimationFrame callbacks after page load, opens while a background input owns focus, selects/focuses the second source, resolves source data, then restores native scheduling. The source retains focus both before and after queued frames; no title steal occurs.
- A separate native-scheduling delayed-known-source subcase opens from a background input using touch: focus stays within the dialog while waiting, then moves to the title after source resolution.
- No preset source initially focuses the empty source control. Close returns focus to the opener in all delayed/no-source subcases.
- Normal known-source initial title focus in English320 light/dark remains correct.
- Compact chooser/cancel/recover/retry focus in light/dark remains correct.
- Composing Enter still sends zero create requests; the subsequent ordinary Enter submits exactly once with the original plain content.

Evidence: `browser-final-focus-integration/targeted/results.json` (7passed/0failed), current build `/tmp/collaboration-create-polish-final-focus-integration-64660`. Product hash delta versus the final header build is only `packages/views/collaboration/creation-form.tsx`; its test is newly recorded in the manifest but is not part of the browser bundle. Fixture source and verifier/build scripts changed only to add delayed-response/focus cases and hashes. All final hashes match current disk. Both `browserClosed` and `listenerClosed` are true; the checker was notified that browser resources are released for the serial unit suite.

The rAF-held subcase is controlled browser scheduling evidence, not a claim of naturally reproducing the exact timing on physical hardware. Separate native-scheduling touch cases cover ordinary interaction. No new full-matrix or visual-polish pass was run for this behavior-only delta.
