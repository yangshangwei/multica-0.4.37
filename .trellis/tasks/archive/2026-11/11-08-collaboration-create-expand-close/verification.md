# Expanded editor and native Close repair

## Changes and root causes

`creation-form.tsx` previously resized only the popup. Its body stayed a block
layout, and the textarea kept its64px content-sized minimum, so most new space
was left below the form. Expanded mode now uses a flex column with natural-height
nonshrinking siblings, and the description/textarea receive the available growth
with auto content basis. Default compact mode and all existing mounted component
identities remain unchanged; no new persistent state or observers were added.

The desktop Close defect was native hit interception. An expanded popup begins
12px below the window top; Close occupiesy24–52, overlapping the shell's48px drag
strip. Its DOM event wiring was correct, but Electron intercepted the OS mouse
event as window dragging. A local `WebkitAppRegion: no-drag` on this popup repairs
the native input boundary without modifying shared Dialog or replacing Close.

Only this component and meaningful default/expanded Close tests changed. The
creation spec now records expanded editor allocation and native hit-test proof.

## Native before/after proof

An owned hiddenInset Electron window rendered actual shared components with a
realistic48px shell drag strip and mock APIs. Quartz `CGEventPost` at the OS HID
tap delivered mouse input to that owned window; Accessibility trust was available.

- Before: OS Close click produced no DOM pointer/mouse/click receipts, and the
  dialog stayed open. Dragging the same center moved the window32px right and
  24px down. A Playwright/CDP click closed it, demonstrating why browser-only
  tests missed the issue.
- After: the same OS click produced pointerdown/mousedown/click on Close and
  closed the dialog. The window remained stationary. Dragging from Close no
  longer moved the window; releasing outside the button correctly did not click.
- Baseline and fixed receipts are in `research/native-baseline/results.json`
  and `research/native-round-1/results.json`. Both owned Electron applications
  and loopback listeners were closed. No user app window or real data was used.

## Layout proof

At901×1042, description height increases64→517px; at1280×900 it becomes375px.
Three file rows consume spare editor space (329px/187px respectively); opening
advanced fields brings it back to64px. Long content grows naturally, while the
body scrolls and footer remains reachable. Actual textarea scroll/client height
equality proves no nested scroll range; the native computed `overflow:auto`
alone is not evidence of scrolling.

Final browser captures cover both themes,901×1042,1280×900,390×740 and320×568;
default/expanded/long text, files, advanced and AI clarification states; toggle
retention, keyboard Close/stream abort/draft recovery and visible footer/focus.
The report records exact source hashes and any fixture-only calibration.

## Limits

The native fixture proves Electron OS hit-testing with an underlying drag strip,
not end-to-end production backend or the installed app's full environment. AI
transport was mocked; no real provider/quota, real task creation, physical touch,
system IME or spoken screen reader acceptance is claimed. All product behavior
outside the corrected layout/native hit boundary remains covered by existing
regression suites.

Final checks and evidence links are appended before task closure.

## Final checks

- Six relevant regression suites: **81 tests passed**, including two new default/expanded Close/stream/draft/focus cases observed failing before the fix.
- Views typecheck passed; full Views lint passed with0errors and27pre-existing warnings. Scoped diff check and Impeccable layout detector passed (`[]`).
- Eight actual-component browser layout cases passed on the final frozen build after correcting a fixture assertion that wrongly treated native textarea overflow:auto as proof of scrolling. No product change was needed for that calibration; actual scroll ranges stayed0.
- OS HID baseline/fixed Electron proof passed. All owned browsers, Electron instances and loopback listeners closed. Final source manifest reports no product mismatch.

[Expanded editor](research/browser-round-1/confirmed/dialog-expanded-901-light.png) · [Expanded editor, dark](research/browser-round-1/confirmed/dialog-expanded-901-dark.png) · [Native Close evidence](research/native-round-1/results.json).

[Detailed final native/browser verification](research/verification-final.md). Visual verdict:95/pass.
