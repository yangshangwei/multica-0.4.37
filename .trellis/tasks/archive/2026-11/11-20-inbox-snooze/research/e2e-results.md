# Inbox snooze production-client evidence

The E2E lane owns `e2e/inbox-snooze.spec.ts`, `e2e/fixtures/inbox-snooze.ts` and this evidence record. It changed no product files or generic fixtures and added no dependencies.

## Runtime boundary

The coordinator admitted the owned password API and migrated API database before the first network request. Configuration is loaded programmatically from the allocation's private environment file; credentials are never included in this document or retained traffic facts.

- Source: `/Volumes/artisan/code/2026/multica-0.4.37`.
- API: configured `http://localhost:61718`, verified by the coordinator at the corresponding owned loopback listener.
- Production Web: `http://localhost:61719`.
- Built renderer: `http://localhost:61720`, opened with the existing `openDesktop` native harness and a fresh private profile.
- API database: `multica_inbox_snooze_api`, separate from Go test databases.
- Private evidence root: `/private/tmp/multica-inbox-snooze-n9cwnxds`.
- Chromium: coordinator-bound browser cache; revision 1208 recorded separately in `browser-cache.json`.

Every browser command uses `pnpm exec playwright test e2e/inbox-snooze.spec.ts --project=chromium --workers=1 --retries=0 --output <private run directory> --reporter=line,json`. The two-case r05 adds `--grep 'Web group snooze|Web Chinese touch'`. The selected environment, exact argv, source hashes, exit code, complete Playwright report and raw log are retained separately for each numbered run. Existing 60-second test deadlines remain unchanged.

## Retained runs

| Run | Result | Evidence and interpretation |
| --- | --- | --- |
| r01 | Admission failure; five cases not run | The fixture expected a synthesized 127.0.0.1 API origin while the private configuration selected localhost. No database/browser/actor fixture began. |
| r02 | Six failed; zero skipped/retried | Browser requests to the localhost API from the 127.0.0.1 Web site carried no session cookie and returned 401. Native authentication and source inbox reads succeeded. The coordinator selected the already-allowed localhost Web/renderer aliases for the next run. |
| r03 | Six failed; zero skipped/retried | Five fixture assertions incorrectly required a slug on new mutations, which intentionally send exact source UUID plus an empty ambient slug. The remaining fixture's workspace-switcher name omitted its avatar initial. Actual source-bound snooze writes and initial Chinese touch screenshots are retained. |
| r04 | Three passed, three failed; zero skipped/retried | Passed actual failed-save/stale-undo recovery, cross-device/source-workspace producer delivery, and production expiry/archive protection. The remaining lifecycle assertion incorrectly treated historical siblings as read; touch geometry was sampled during the menu enter scale. Native revealed the real tab-store pathname-only deduplication gap. |
| r05 | Two passed; zero failed/skipped/retried | Full Web lifecycle/edit/undo/remind-now and Chinese touch/focus/Escape/custom/cross-year/theme journeys passed. Read preservation compares the per-ID snapshot, and the 44px checks retain initial and stable geometry under the original assertion deadline. |
| r06 | Five passed, one failed; zero skipped/retried | All Web cases passed on the final product candidate. Native custom Save time/Undo passed, then the fixture incorrectly expected the derived resource key in persisted tab JSON. |
| r07 | Five passed, one failed; zero skipped/retried | Native three-tab identity, view contents and reload passed. Its compact capture incorrectly attempted a touch-only trigger on a fine-pointer native device. |
| r08 | Five passed, one failed; zero skipped/retried | The complete native journey and actual ThemeProvider dark/light captures passed. The touch Web fixture opened the command shortcut before popup focus dismissal had settled. |
| r09 | Five passed, one failed; zero skipped/retried | Every business assertion passed, but native recorder cleanup reported pending response-body reads invalidated by reload. Original errors are retained; the next fixture joins those actual reads before reload. |
| r10 | **Six passed; zero failed/skipped/retried/flaky** | Complete suite on the same final production candidate, exit 0 in 31.95 seconds. Response reads join before reload, actual provider commands select themes, and all native/Web outcomes pass together. |

The private `e2e-run-summary.json` contains all named terminals. Original failures remain failures and are not replaced by later successes. Each screenshot index retains actual captures for the coordinator's visual review; `e2e-r10-screenshot-index.json` is the final complete 19-capture index.

## Verified outcomes before the native correction

- Two password actors use real questionnaire/completion requests and retain the same non-null onboarded actor identity. Each test creates its exact workspaces with public POST requests rather than `ensureWorkspace` fallback.
- Ordinary task creation, manual subscription and comment sources use `TestApiClient`. Original comment and notification IDs are frozen in per-test evidence.
- The Web group journey saves a two-notification snooze, preserves its original per-row read bits, advances selected detail only after success, reloads Snoozed, changes custom time, undoes the change, reminds now and restores the original unread group without cloning notifications.
- An injected save-response 500 keeps the actual custom input, selection and cached notification projections. A second live browser edits the plan; the first browser's visible Undo returns a real 409 and preserves the later state.
- An ordinary comment and @all notification stay snoozed without `inbox:new`. Direct comment/description mentions and assignment wake the group. The other-workspace browser receives the source workspace's personal event and a nonzero account unread-summary projection before switching back; both clients render the same resulting group.
- Exact owned-plan time control is the only product-data SQL mutation in the fixture. The real 15-second production sweep consumes the plan, sends the personal event, returns original notifications unread near the top, and leaves an archived plan cleared. No scheduler mock or inserted notification simulates expiry.
- Chinese narrow touch controls settle at 44px, keep arrows/Escape within the popup, restore focus and preserve a future cross-year custom input in both themes. Initial menu animation geometry remains recorded rather than silently discarded.

## Native gap and remaining gate

r04's real native renderer kept the Snoozed screen in an Inbox-labelled sole tab. Cmd-clicking the Inbox navigation link reused that tab because Desktop `resourceKeyForUrl` discarded every query parameter. The E2E owner reported this to the coordinator and made no product changes.

The native regression checks three real persisted frame IDs/URLs, each view's content, real open-or-activate deduplication, the localized new Snoozed title and reload restoration, preserving existing active/Archived labels. It also drives real native custom Save time, Undo and Remind now against a Web peer, captures the native compositor at wide and compact sizes, and verifies no Agent execution starts. Derived tab resource keys deliberately remain outside persisted-state assertions.

The coordinator admitted the rebuilt final candidate with API PID 51659, Web PID 69870 and renderer PID 69871; source/build/listener proof is `runtime-final-candidate.json`. r10 then passed all six journeys together with one worker and no retries/skips. `e2e-r10-invocation.json` binds exact argv and identical before/after E2E source hashes; `e2e-r10-source/` retains complete source bytes. The private `e2e-r10-report.json` and `e2e-r10-run.log` contain the full named results.

Theme switching uses the actual Search command palette and ThemeProvider setter. Earlier captures that manually added `.dark` alongside `.light` are retained as invalid fixture-theme evidence. The final native custom input has only `h-full dark`, computed `color-scheme: dark`, foreground `oklch(0.985 0 0)`, background `oklab(1 0 0 / 0.045)`, a nonempty readable value and `disabled: false`. Both compact themes have actual compositor captures. The native device remains a fine pointer; right-click drives its real context menu while the Web touch fixture drives the touch trigger.

Final native PID 73200 was audited not running after closure; its private profile remains retained. The final evidence count is 19 product screenshots with actor/workspace/comment/notification identities, real requests, personal events and source-workspace cache projections. Independent visual/spec review and overall task completion remain with the coordinator. No additional broad E2E runs followed the complete pass.

## Static check

`pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2024 --module ESNext --moduleResolution Bundler --esModuleInterop e2e/inbox-snooze.spec.ts e2e/fixtures/inbox-snooze.ts` passed. A first ES2023 invocation failed on existing Core use of `String.isWellFormed`; selecting the required ES2024 library passed without product changes. The two new source files also passed whitespace checks.

Synthetic fixture accounts/workspaces and private native profiles remain retained for inspection. Actual clients are closed; original failed evidence is preserved. Broader lint, package tests, Go checks, source/build identity and final cleanup remain owned by the coordinator.
