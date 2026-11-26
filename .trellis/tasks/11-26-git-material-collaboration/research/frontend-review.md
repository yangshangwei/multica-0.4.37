# Frontend review — 2026-11-27

Reviewed the complete task changes in `packages/core`, `packages/views` and
`packages/plugin-sdk` against the approved requirement-to-plan scope, API
contracts, package boundaries, and settings/access recovery rules. Other review
lanes own backend/provider behavior and root owns the final integrated run.

## Fixed findings

- **Transient reads discarded active work.** A failed detail poll replaced the
  entire detail component, discarding review reason/comment state and closing
  its editor. A failed settings refresh similarly unmounted the configuration
  form and discarded unsaved configuration. `material-detail.tsx` and
  `project-materials.tsx` now reuse `getSettingsQueryState`; a retry notice is a
  stable sibling of retained authorized data. Access-denied data remains hidden.
  `material-controls.tsx` disables the read retry during an active request.
- **Offline writes could resume automatically.** Material mutations inherited
  TanStack Query's default network mode, so a click while offline paused until
  reconnect instead of returning to original-intent recovery. All three hooks
  in `core/materials/queries.ts` now explicitly use `networkMode: "always"` and
  `retry: false`. Transport failure retains the original request; reconnect
  alone does not send it again.

The canonical component suite now proves retained input DOM nodes, focus and
text through failed reads, plus offline approval failure/manual retry with an
identical original request. Each regression failed before the corresponding
production correction. The shared Web/Electron journey also checks a real
detail read failure via a 503 route fixture, retains the focused review input,
captures that state, then explicitly refreshes before approving.

## Verification performed by this reviewer

| Command | Result |
| --- | --- |
| `pnpm --filter @multica/views exec vitest run projects/components/project-materials.test.tsx plugins/surface-bridge.test.ts plugins/plugin-sdk-handshake.test.ts` | 3 files, 28 tests passed |
| `pnpm --filter @multica/core exec vitest run materials projects/access-lifecycle.test.ts paths/paths.test.ts` | 4 files, 43 tests passed |
| `pnpm --filter @multica/core --filter @multica/views --filter @multica/plugin-sdk typecheck` | All 3 packages passed |
| `pnpm --filter @multica/core --filter @multica/views --filter @multica/plugin-sdk lint` | Exit 0; 27 pre-existing views warnings, no errors |

Existing regression coverage checks scoped API identity, malformed/default
responses, original request replay, version-pinned approval, explicit false
auto-start, current baseline binding, private cache/draft cleanup, shared
navigation, Chinese UI copy, and the mounted issue/version plugin allowlist.
No additional unresolved defect was found in this review scope.

## Remaining verification and spec handoff

- Root must run the changed shared browser journey against freshly built Web
  and Electron renderer outputs. This reviewer did not launch that environment
  or claim native visual verification. The layout itself is unchanged; the
  failure state now keeps its active content visible alongside existing feedback.
- Root owns the full repository test run and backend contract verification.
- Include material input retention and explicit original-request recovery after
  offline failure in the task's final spec update. No new dependency, SDK action,
  or backend interface was introduced by these review corrections.

## Terminal planning recovery follow-up

The backend review added explicitly authorized human retries for terminal
planning attempts on the existing issue. The current shared view already shows
`Start planning` for a known material state whenever `can_start_plan` is true,
including a blocked plan, and does not dispatch on mount or reconnect. No
production frontend change was needed. Added a component regression for this
case, including the unchanged issue link before/after the command.

`pnpm --filter @multica/views exec vitest run projects/components/project-materials.test.tsx`
passed with 18 tests after this addition. Existing production builds remain
valid for this test-only follow-up; backend terminal-state semantics are
verified separately by the backend lane.

## Browser harness correction

The first root-run integrated test exposed two problems in the new outage
assertion: a page-wide alert selector also selected knowledge errors and the
Next.js announcer; Electron's fulfilled-route response event was not observed
even though its captured page already showed the material read error and the
retained review form. Source inspection confirms Desktop calls renderer `fetch`
through the same `ApiClient`; Main only supplies endpoint configuration here.

The test now scopes the material alert by its displayed text and measures the
actual intercepted GET/fulfilled 503 instead of relying on that response event.
It triggers the real header Refresh path, holds the response while restoring
input focus, then releases it and verifies the same input node, text and focus.
After removing the fault it uses the displayed error's Refresh action and
checks that the error disappears with the same input/text retained. A JSON
attachment records the interception and fulfillment counters. The product code
and production bundles did not change. `playwright test --list` parses both
journeys; root owns the rerun and its observed result.

The next root replay passed the complete Electron journey. Web exposed a
further test-only scope assumption: its material header is nested under `main`
and therefore has no `banner` role. The Refresh lookup now starts at the
material heading's actual shared title-row parent. Cleanup releases the held
response and awaits `unrouteAll({behavior: "wait"})`, so teardown cannot race a
late fulfillment. Production source remains unchanged; root is rerunning Web
and Electron with this final selector correction.
