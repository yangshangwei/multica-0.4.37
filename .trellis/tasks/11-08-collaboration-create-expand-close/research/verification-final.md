# Expanded editor and native Close verification

**Passed:8/8 layout scenarios, native OS click/drag before–after proof, visual95/pass.** No product correction was needed after the first implementation. All owned Electron/Chromium processes and loopback listeners were closed.

## Native Close root cause and fix

The fixture used the actual production-built shared collaboration composer with mock API/memory drafts, hosted in a separate Electron39.8.7 hiddenInset BrowserWindow. An underlying48px `WebkitAppRegion: drag` strip matches DesktopAccountNavigation's titlebar behavior. No installed user app/window or real data was used.

macOS Accessibility was already trusted. Python ctypes called Quartz `CGEventPost(kCGHIDEventTap)` after the owned Electron window reported focused. These are OS mouse events, not renderer/CDP clicks.

| Action at expanded Close center (client855,38) | Before fix | After popup-local no-drag |
| --- | --- | --- |
| Native click | No pointerdown/mousedown/click received; dialog remains | All three DOM events target Close; dialog closes |
| Native drag32px right/24px down | Window moves from(80,55)to(112,79); no DOM events | Window remains(80,55); DOM pointer events received; release outside button correctly does not activate Close |
| Playwright/CDP click control | Dialog closes | Dialog closes |

Thus the prior browser-only successful click missed a real native drag-region interception. The existing DialogClose event wiring was sound. Final native evidence is `native-round-1/results.json`; unfixed receipt is `native-baseline/results.json`; screenshots `native-click.png` and `native-drag.png` accompany both. Window/content bounds and computed strip/popup app-region values are recorded. Native default-size OS clicking was not separately exercised; component Close/reopen behavior and unit tests cover lifecycle, while the native proof specifically targets the failing expanded/titlebar overlap.

A first Swift helper attempt hit the installed command-line toolchain's duplicate SwiftBridging module error. It was not used for evidence and its task-owned source was removed. The successful Python Quartz helper remains in `preview/native-pointer.py`; no package was installed and no accessibility permission was changed.

## Layout and retained state

Real Chromium tested901×1042,1280×900,390×740 and320×568 in both light/dark themes. Results: `browser-round-1/confirmed/results.json`.

| Viewport | Default short textarea | Expanded short textarea | With3long files | With advanced fields open | Long description |
| --- | ---: | ---: | ---: | ---: | ---: |
|901×1042|64px|517px (8.08×)|329px|64px|636px|
|1280×900|64px|375px (5.86×)|187px|64px|636px|
|390×740|64px|64px|64px|64px|1236px|
|320×568|64px|64px|64px|64px|1236px|

Narrow views have no spare space after necessary controls, so the textarea retains its natural minimum and the body scrolls. At901×1042, accepted AI output plus clarification and files leaves165px for the short textarea; at1280×900 it returns to64px. Header and footer remain visible throughout. Long-description textarea clientHeight equals scrollHeight in every case (internal range0); only body has scroll overflow.

Expansion/collapse preserves the actual textarea DOM node, title/description, selected files and live SSE controller. Keyboard toggle retains control focus. Creation Close works via component pointer input, returns focus to opener, and reopening retains the accepted text and3files. Clarification input is fully visible after focus. No document horizontal overflow was measured.

The test originally rejected computed textarea overflowY:auto. That is a native textarea default and did not prove an inner scrollbar. Diagnostics already showed clientHeight==scrollHeight in all cases. The calibrated same-frozen-build assertion checks real scroll range; it also waits for re-expanded state to settle before sampling AI results. The original false assertion receipt is retained at `browser-round-1/targeted/results.json` and explicitly is not a product failure. The calibrated8cases pass with zero page errors.

## Images and provenance

- Final cropped901 expanded editor: [dialog-expanded-901-light.png](browser-round-1/confirmed/dialog-expanded-901-light.png)
- Dark crop: `browser-round-1/confirmed/dialog-expanded-901-dark.png`
- Added sections: `browser-round-1/confirmed/expanded-questions-901-light.png`
- Narrow long description: `browser-round-1/confirmed/expanded-long-320-dark.png`
- Compact default: `browser-round-1/confirmed/default-901-light.png`

Baseline bundle `/tmp/collaboration-create-expand-close-baseline-22069` was frozen before product edits. Fixed bundle `/tmp/collaboration-create-expand-close-round-1-23645` supplied both native and browser proof. `final-source-check.json` reports no product/CSS/locale hash mismatch. The verifier script alone changed after bundling to correct geometry assertions; its exact current hash and all native harness hashes are recorded separately. No source was rebuilt during calibration.

Owned Electron and Chromium contexts were closed in finally; both HTTP listeners closed. No application stack was started, stopped or restarted. No real backend task creation, AI provider/quota, physical hardware touch, native IME or spoken screen-reader acceptance is claimed. OS Quartz hit-testing is stronger than CDP input but remains an isolated shell reproduction of the real desktop drag strip, not a full installed-client end-to-end run.

Browser worker changes are confined to this task's research harness/evidence and authorized visual state JSON; no product/spec/locales edits.
