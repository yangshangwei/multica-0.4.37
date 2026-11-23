# S6 mixed mentions and durable notification journeys

Date: 2026-10-21. Acceptance: AC90, AC91, AC97 (P6 integration evidence; the backend/core retain the exhaustive authority and notification matrices).

## Change boundary

Existing coverage sends `@all` with one agent. It does not establish an H+A+B+S comment, an actual partial refusal after preview, or delivery to a closed browser followed by fresh-context notification reads and editing.

- `e2e/fixtures/collaboration-mention-journeys.ts` owns shared public-API setup and three bounded journeys. It reuses the existing resource-offer fixture's synthetic accounts, offline runtime and explicit enrollment. Offers, adoption, every owner's consent, comments, edits, notification reads and read/archive writes use real APIs or UI.
- `e2e/collaboration-mention-journeys-web.spec.ts` owns production-Web contexts and wide/narrow captures.
- `e2e/collaboration-mention-journeys-desktop.spec.ts` owns isolated real Electron profiles, deep links, native capture/200% zoom and restart.
- This report records exact test identities, checks and evidence limits.

No product code, existing fixtures, rollout gates, build infrastructure, provider processes/accounts, dependencies or notification rows are changed by this slice. The parent coordinates current snapshot builds and serial native execution. The preview fixture is deliberately not reused for setup: it clears target holds and inserts an earlier comment, which would invalidate the requested one-comment and fully held baseline.

## Intended evidence

1. A single UI comment selects H, independent A and B, and a distinct squad S. All resources come through offer/adopt/complete owner consent. Receipts and retained commands must agree on target, channel, batch, sequence and input event. H receives one located mention notification; creator, handler, status, participants and current handling stay unchanged.
2. UI preview and suppression preserve text without writes. After the final authorized A+B+H preview, a request barrier revokes only A's actual owner grant before continuing the real POST. B queues and H is notified; A has a visible refusal and no command. Original-key replay preserves the receipt, comment, queue and notifications.
3. H's browser/application closes before an explicit mention is submitted. A fresh authenticated context/profile reads the durable notification and opens the exact comment. Mark-read/archive preserve current handling. Comment and body edits add only newly effective human recipients, exclude the actor and nonparticipants, retain exact anchors, and enqueue no AI even when typed AI mentions are added.

The squad includes a human workspace member outside the collaboration. The fixture verifies that this person remains outside the issue and receives no private notification. Runtime rows stay offline; issue and target controls stay paused/held throughout. No multi-target execution or native containment claim follows from these journeys.

## Verification

The three source files are implemented. Each platform has three independent tests:

| Platform | Exact test identity |
| --- | --- |
| Web | `Web mixed H+A+B+S mentions retain one comment and exact queued target associations` |
| Web | `Web owner revocation after preview refuses only A and original-key replay stays deduplicated` |
| Web | `Web offline mentions survive a fresh context and edits notify only newly added people` |
| Desktop | `Desktop mixed H+A+B+S mentions retain one comment and exact queued target associations` |
| Desktop | `Desktop owner revocation after preview refuses only A and original-key replay stays deduplicated` |
| Desktop | `Desktop offline mentions survive a fresh profile and edits notify only newly added people` |

Checks run from the repository root:

```sh
pnpm exec playwright test e2e/collaboration-mention-journeys-web.spec.ts e2e/collaboration-mention-journeys-desktop.spec.ts --list
pnpm exec tsc --project /var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-mention-static-3g5de2rj/tsconfig.json
node packages/views/node_modules/eslint/bin/eslint.js --config /var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-mention-static-3g5de2rj/eslint.config.mjs e2e/fixtures/collaboration-mention-journeys.ts e2e/collaboration-mention-journeys-web.spec.ts e2e/collaboration-mention-journeys-desktop.spec.ts
```

- Discovery passes: **6 tests / 2 specs**. The first discovery attempt caught an extra parenthesis in the new Web capture helper; it was corrected before the successful reruns. This was a test syntax defect, not a product failure.
- Scoped ESLint passes with no diagnostics. The temporary config extends the repository base and declares browser/Node globals; it permits root dev dependencies for E2E fixtures and excludes type-only imports from the dependency rule.
- Strict TypeScript exits **2** solely on the imported existing `e2e/fixtures.ts:442`: `TS2532: Object is possibly 'undefined'` at `result.rows[0].count`. The temporary config extends the repository React library settings, retains `strict` and `noUncheckedIndexedAccess`, supplies the core source alias, and includes only these three entries. No new-file diagnostic is reported. The shared fixture was left to its owner; this is not recorded as a passing full typecheck.
- All four new files pass the equivalent untracked-file whitespace check (`git diff --no-index --check /dev/null <file>`; exit 1 with no diagnostics denotes the new-file difference).
- No browser/native journey has been executed by this implementer. No build, environment restart, migration or provider/account smoke was run. The coordinator will run final integration against the refreshed S4/S5 legacy snapshot and retain first-run failures.

Coordinator execution commands (inside the task-owned environment wrapper):

```sh
pnpm exec playwright test e2e/collaboration-mention-journeys-web.spec.ts --workers=1 --retries=0
pnpm exec playwright test e2e/collaboration-mention-journeys-desktop.spec.ts --workers=1 --retries=0
```

Run native tests serially with other Electron capture suites. The native fixture resolves production artifacts from `COLLABORATION_ELECTRON_SOURCE_ROOT`, exercises the real account shell/IPC harness, and verifies zero daemon starts. It does not establish packaged Main startup, other operating systems, provider execution, or containment. Web captures check actual wide/narrow viewport and PNG dimensions; native captures use the shared compositor/scroll helper at wide and 200% zoom. Final visual inspection and actual pass/fail results remain coordinator follow-ups.

## Independent control journey extension

The coordinator requested one additional Web/native control journey to complement the native Linux router/CLI/daemon proof tests. Its write boundary is three new files, with no changes to the six mention scenarios:

- `e2e/fixtures/collaboration-control-journey.ts`: public password account creation, signed installation enrollment/binding, scoped runtime registration/deregistration, two independent agent targets through offer/adopt/owner consent, and shared real control assertions.
- `e2e/collaboration-control-journey-web.spec.ts`: `Web controls recover partial Resume, isolate target lanes and stop the whole issue`.
- `e2e/collaboration-control-journey-desktop.spec.ts`: `Desktop controls recover partial Resume, isolate target lanes and stop the whole issue`.

The existing legacy fixture is intentionally not used here: `collaborationRuntimeTargetSupported` refuses `enrolled` grants with `runtime_upgrade_required`. The coordinator authorized the separate password/managed snapshot (API 18996, production Web 13916, native renderer 6090) and its task-only execution gate. Tests assert the live configuration and gate; they never change them.

The new synthetic owner signs the server's original enrollment/binding bytes with an in-memory Ed25519 key. The declared remote installation is Linux, its runtime stays offline, and it has no daemon process or executable. This identity is separate from the macOS Electron client. No database row, run, attempt, process proof, termination receipt or reservation is seeded. Registration and deregistration are real HTTP requests made by the fixture with the issued scoped MDT; credentials never enter evidence attachments.

The single journey verifies:

1. Target B Resume returns the real `issue_paused` refusal while issue and target holds remain unchanged.
2. A's owner revokes only A's current grant. B remains genuinely authorized.
3. Resume All commits `partially_resumed`. Both empty channels become active, independently of holds: A remains `grant_required`, while B receives `hold_released`. The UI must show the exact lane totals, the remaining held target, and both per-target decisions.
4. That Resume response is deliberately lost after the real server commit. Refresh retains the original draft; the existing “Check original result” button performs one correlated GET and no replacement POST. The displayed receipt and current API state must agree.
5. Stop B pauses its actual lane and leaves A's lane/hold and the issue control unchanged. Resume B restores only B's lane. Stop All returns the issue and both lanes to pause.
6. Historical original-key GETs retain the old partial/refused receipts without changing live controls. Exactly five UI control POSTs and one UI recovery GET occur. There are no commands, groups, runs or ordinary dispatches, and the runtime remains offline.

Static follow-up: discovery lists **2 additional tests / 2 specs**; scoped ESLint passes. Strict TypeScript now passes across all six mention/control entry files after the coordinator added the existing shared fixture's missing-row guard. That shared-file repair is not attributed to this implementation. The earlier failed typecheck remains recorded above as its original result.

Run these separately from the legacy mention suites, inside the coordinator's password/managed wrapper:

```sh
pnpm exec playwright test e2e/collaboration-control-journey-web.spec.ts --workers=1 --retries=0
pnpm exec playwright test e2e/collaboration-control-journey-desktop.spec.ts --workers=1 --retries=0
```

The control journey has not been executed by this implementer. Native UI capture uses the production helper at wide and 200% zoom; Web captures use wide/narrow viewport and PNG geometry checks. These are control-wiring tests for offline synthetic resources, not AI execution or macOS containment evidence. Actual E2E execution and final evidence linkage remain coordinator-owned.
