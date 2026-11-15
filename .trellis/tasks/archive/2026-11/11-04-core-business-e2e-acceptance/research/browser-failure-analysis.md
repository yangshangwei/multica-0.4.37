# Baseline browser failure analysis

Read-only diagnosis of `evidence/legacy-full-1`, candidate `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`. No test replay, browser, build, database or service action ran. No product/test code changed. Original traces, screenshots, error contexts and test results were read; request headers, tokens and cookie values were not exported. `browser-failure-analysis.json` records archive hashes, safe extracted facts, ranked hypotheses and replay commands.

## G-E1: the first assignment never reaches a write

The failure is earlier than the `waitForResponse(PUT)` stack suggests. The owner clicked Unassigned, but the subsequent target-member click never found its row. The owner trace has **49 API requests:48 status200 and1 status204; zero issue mutation requests of any method**. Captured API duration max274.217ms is insufficient evidence for a load or server-timeout theory. The member-list response contains the exact target member.

Exact trace timeline (`0-trace.trace`, monotonic milliseconds):

| Time | Evidence |
|---|---|
| 1398586.678 | Title visible on UUID route; Unassigned trigger ID `base-ui-_r_1h_` |
| 1398638.647 | Actual pointer click on Unassigned starts |
| 1398728.866 | Click completes |
| 1398756.403 | Screencast briefly shows picker with the target member |
| 1398782.676 | DOM now at `/issues/CORE-1`; trigger ID is **new** `base-ui-_r_4c_`, `aria-expanded=false` |
| 1398801.440 | Member-row locator starts waiting; it never resolves before the150s test timeout |
| 1398826.873 | Screencast confirms popup has disappeared |

The route network request to `/issues/CORE-1` started at `2026-11-04T16:27:40.585Z`, returned200 in186.937ms, spanning the click. `packages/views/issues/components/issue-detail-route.tsx:40–44` intentionally replaces the UUID route with the identifier in an effect. The fixture at `e2e/core-business-workflow.spec.ts:175–179` waits for the title but immediately clicks without waiting for canonical navigation. Generated trigger ID replacement and the visible transient menu establish component reconstruction around the route change; the precise ancestor is not yet isolated.

Ranked falsifiable hypotheses:

1. **Canonical navigation remounts the detail during opening.** Waiting for the exact canonical URL before the same click should remove this race; delaying only the canonical route response after title visibility should expose it. This is the strongest explanation supported by captured evidence.
2. **A separate parent/layout remount coincides with navigation.** On a fully canonical URL, track trigger DOM identity and mount lifecycle during the same query updates. A recurring replacement there disproves a route-only explanation.
3. **Popover outside/focus dismissal contributes.** Capture `onOpenChange` reason and focus/pointer sequence. A close without unmount would support this, but it does not alone explain the new generated trigger ID.

Not supported: PUT changed to PATCH; missing target membership; failure in inbox assertions. No task write was emitted and the test stopped before its first assignment.

Minimal original replay, only after root restores/verifies the owned legacy environment and shared-machine handoff allows execution:

```text
python3 .omx/private/core-e2e-20261104/run-browser.py diagnosis-ge1-original-1 e2e/core-business-workflow.spec.ts --grep "^G-E1 assigning a teammate"
```

For a diagnostic variant, record URL, trigger node identity, popup mounting, pointer events and actual method/path. Then change only readiness to the canonical URL. Do not increase timeout, retry the click, or change the expected write method. Even if canonical-wait makes the business test stable, preserve the separate product question: can a real user's early interaction be lost during URL normalization?

The saved error-context shows onboarding after fixture cleanup, while both failure screenshots still show the two workspace pages. Do not interpret the teardown snapshot as the cause of the original interaction failure. The secondary context-close protocol error also occurs after the test timeout.

## Native triage English: unexpected input, insufficient native event recording

The assertion fails after ArrowDown and Enter at `e2e/fixtures/triage-duplicate-picker.ts:185`. The intended fill is `Resolved UI/UX upload failure`. The **actual failed screenshot and ARIA snapshot both contain `Resolved UI/UX upload failure e`**. The input remains focused and expanded, and the exact matching target option is still selected. The reason textbox remains unchanged. The page-errors attachment is empty.

`test.trace` records:

- fill finishes at1160129.2ms;
- the single-option count becomes true by1161049.8ms;
- ArrowDown runs1161054.5–1161061.8ms;
- Enter runs1161062.7–1161068.0ms;
- list closure fails at1166073.0ms after the original5s assertion.

There are **no Electron browser trace or network files** in this archive, only test API calls. `e2e/fixtures/project-p1-desktop.ts:77–85` launches its own Electron context, which is not automatically traced by the ordinary Playwright browser fixture. Consequently the trace proves the commands requested by the test, not the actual DOM keyboard/input/composition events delivered in the native page. It cannot establish whether a duplicate-mark API write occurred. The same baseline passed native zh-Hans and both Web locales; those facts limit scope but do not erase the English failure.

Ranked falsifiable hypotheses:

1. **Unexpected native input/composition changed the text, so Enter was not consumed as selection.** Capture `keydown`, `beforeinput`, `input`, composition events and the native `before-input-event`, with key/code/isComposing and this synthetic textbox's value. The added space/e must then have an observable origin. Current evidence does not establish whether this came from input-method processing, external keyboard input or another cause.
2. **cmdk active selection was not ready when Enter arrived.** Capture active descendant, selected option identity and callback at the exact key event. A correct noncomposing Enter with a stale/missing selected item supports this. The final selected option is not proof of selection state at keydown.
3. **Selection completed and a subsequent controlled input/focus update reopened the list.** Capture whether value first becomes the identifier and expanded becomes false, then flips back. `triage-duplicate-picker.tsx:91–96` reopens on input/focus/click; `:138–140` closes and changes to identifier on selection. No callback order was recorded, so this remains unproved.

Minimal original replay:

```text
python3 .omx/private/core-e2e-20261104/run-browser.py diagnosis-triage-native-en-1 e2e/triage-duplicate-picker-desktop.spec.ts --grep "\(en\)$"
```

For the next diagnostic capture, explicitly trace the **actual Electron context**, add bounded capture-phase events only around the synthetic target input, and record value/focus/active descendant before and after each key. Keep the keyboard path and5s closure assertion. Replacing Enter with a mouse click would remove the behavior under investigation. The existing component keyboard test (`packages/views/triage/triage-duplicate-picker.test.tsx:192`) covers ordinary DOM selection but not native key delivery or the one-result post-search sequence.

## Changelog: confirmed configuration containment guard

This case fails in97ms at `e2e/changelog.spec.ts:103`, before `useLocalApi`, before reading or replacing the feed and before publication behavior. The configured feed is the coordinator's `.omx/private/core-e2e-20261104/changelog.json`. The fixture requires a path under the **frozen checkout's** `.omx/state/desktop-changelog/live/` (`:8–11`,`:102–103`). This is a root setup mismatch, not evidence that live publication is broken.

Root should create the task-owned feed within the exact permitted checkout directory, make API `CHANGELOG_FILE` and test `CHANGELOG_E2E_FEED` identify that same file, preserve the previous file, and verify the restarted API/config binding. Keep the guard intact. Other changelog reader and native navigation cases passed but do not cover live publication.

```text
python3 .omx/private/core-e2e-20261104/run-browser.py diagnosis-changelog-feed-1 e2e/changelog.spec.ts --grep "generated publication reaches"
```

No replay or proposed fix has been validated in this diagnostic slice. All three original failure records remain unchanged; resource contention has not been inferred from a timeout.
