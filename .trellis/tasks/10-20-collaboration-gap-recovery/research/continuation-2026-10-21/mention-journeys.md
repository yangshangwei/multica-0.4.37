# Continued AC90/91/97 mention journeys

Task: `10-20-collaboration-gap-recovery`. Branch: `codex/cross-workspace-collaboration`.
Starting HEAD: `8ecb98883504443a6ec1a5fb4a4c6295a985ae04`.

## Change boundary

The three existing untracked mention E2E files are inherited implementation, not new work from this continuation. Their original bytes/hashes are preserved in `/tmp/multica-mention-continuation-baseline/`. Only those three files and this report are writable in this lane. The coordinator owns builds, running environments, shared fixtures, product repairs, gates, ledger/spec integration and commits.

The inherited six tests cover a single H+A+B+S comment, read-only preview and suppression, owner A's revocation between preview and the original POST, B's retained queue association, exact-key replay without duplicate effects, offline H followed by fresh browser/native notification reads, comment/body anchors, read/archive and edit recipient deltas. The squad's human member remains outside the issue. All targets remain offline, paused and held; this is queue and UI wiring evidence, not completed AI execution.

One local repair was needed before live execution: both offline cases now perform an actual legacy authentication API login after the offline comment is committed, verify the same H identity, and install that resulting session into the fresh browser context/native profile. This distinguishes login-after-send from simply restoring the old token. Interactive login-page coverage remains outside these bounded tests.

## Static verification

All commands below run at the repository root.

```sh
pnpm exec playwright test e2e/collaboration-mention-journeys-web.spec.ts e2e/collaboration-mention-journeys-desktop.spec.ts --list
pnpm exec tsc --project /var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-mention-static-3g5de2rj/tsconfig.json
node packages/views/node_modules/eslint/bin/eslint.js --config /var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-mention-static-3g5de2rj/eslint.config.mjs e2e/fixtures/collaboration-mention-journeys.ts e2e/collaboration-mention-journeys-web.spec.ts e2e/collaboration-mention-journeys-desktop.spec.ts
```

Before the local repair, discovery passed (six tests in two specs), strict TypeScript exited 0 and scoped ESLint exited 0. The strict config includes the three adjacent control journey entries as inherited from the prior session. pnpm emits its existing package.json configuration migration warning; no check diagnostic was emitted.

After the local login repair, all three commands passed again: six tests/two specs, strict TypeScript exit 0, scoped ESLint exit 0. A direct whitespace check of all four owned files also passed. No runtime result is inferred from discovery.

## Live execution

The coordinator handed over the task-only legacy API at `http://127.0.0.1:18985`, production Web at `http://127.0.0.1:13905` (build `SrpeaQzGK_I-BlKn9Vy0e`) and production native renderer on port `6079`. Production snapshot source and all 6994 application source files match the root tree at handoff. See the sibling `source-environment-manifest.json`, verification `*running.json` and `native-artifacts.json` records. Credentials are injected by the existing wrapper and never copied here. No service startup, rebuild, migration, gate change or provider invocation is performed by this lane.

The first two Web attempts use these exact commands; stdout JSON and stderr are retained alongside each output directory under `.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-web/`.

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/verification-test.py pnpm exec playwright test e2e/collaboration-mention-journeys-web.spec.ts --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-web/attempt-1
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/verification-test.py pnpm exec playwright test e2e/collaboration-mention-journeys-web.spec.ts --grep 'Web owner revocation' --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-web/attempt-2
```

- Web attempt 1: **2 passed / 1 failed / 0 skipped**, 35.95 seconds. Mixed H+A+B+S and offline-login/edit/anchors passed. Partial refusal stopped at `selectedA.uncheck()`: its real click invalidated and removed the old preview, and Playwright's checked-state postcondition waited for a retired element. Preserved text and disabled Send are visible in the saved page snapshot. This is a test-harness failure before any comment write, not a product refusal/deduplication failure. Raw JSON, snapshots, screenshots and trace remain intact.
- Root-cause correction: the shared journey now asserts the initial checked state, clicks, requests the new preview and asserts its actual suppressed/checked state. No force click, product behavior or assertion was removed.
- Web attempt 2: the failed partial-refusal identity alone **passed** with that correction in 7.01 seconds. Raw `attempt-2.json` is retained.
- Web attempt 3: **3 passed / 0 failed / 0 skipped / 0 flaky**, 19.79 seconds.
- Desktop attempt 1: **3 passed / 0 failed / 0 skipped / 0 flaky**, 24.64 seconds, real Electron on macOS with isolated profiles. No daemon starts or ambient workspace headers were observed by the asserted native harness. All applications/profiles were closed/removed and the exclusive native slot was released immediately afterward.

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/verification-test.py pnpm exec playwright test e2e/collaboration-mention-journeys-web.spec.ts --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-web/attempt-3
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/verification-test.py pnpm exec playwright test e2e/collaboration-mention-journeys-desktop.spec.ts --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-desktop/attempt-1
```

### Unavailable-preview clause

Final criterion review found that the first six green cases displayed paused/waiting and suppression plus a changed-state refusal at commit, but did not separately display an already-unavailable AI target during a fresh preview. The coordinator requested that straightforward coverage extension within the same partial-refusal journey. After proving exact-key replay, it now previews the unchanged A+B+H text again: A is blocked with null target/command IDs and a visible disabled, unchecked control containing the unavailable reason; B remains waiting; recipients remain H and owner D. A full before/after state comparison proves that this preview adds no comment, command, notification or human workflow change. No second send occurs.

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/verification-test.py pnpm exec playwright test e2e/collaboration-mention-journeys-web.spec.ts --grep 'Web owner revocation' --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-web/attempt-4
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/verification-test.py pnpm exec playwright test e2e/collaboration-mention-journeys-web.spec.ts --grep 'Web owner revocation' --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-web/attempt-5
```

- Web attempt 4: **0 passed / 1 failed / 0 skipped**, 11.09 seconds. API assertions and disabled A state passed; a new `has` filter incorrectly reused a form-rooted locator relative to a list item. The saved snapshot shows the correct visible unavailable A label. This is a test locator defect; the repair directly asserts that same visible control's accessible name.
- Web attempt 5: **1 passed / 0 failed / 0 skipped / 0 flaky**, 7.40 seconds, including the unavailable preview. Strict TypeScript and scoped ESLint passed after the final locator change. The other two journey functions were not changed by this extension.

The coordinator requested complete platform reruns against the final shared fixture hash for the current-source audit. Web attempt 6 passed **3/3**, with **0 failures / 0 skips / 0 flaky**, in 19.95 seconds. Desktop attempt 2 passed **3/3**, with **0 failures / 0 skips / 0 flaky**, in 24.61 seconds. All unavailable-preview clauses are included on both platforms. All six final executions use the exact source hashes below; no further test code changed. Native applications/profiles were cleaned and the exclusive slot was released directly to the control lane immediately afterward.

Before that final native rerun, the coordinator found both inherited static renderer helpers had exited and restarted only those known owned processes, without a rebuild or source change. The environment event is retained separately in sibling `native-renderer-restart.json`; the legacy renderer is PID 95773. This lane independently verified HTTP 200 with the exact 1256-byte index SHA-256 `337c08a228217fb574b77bb5fb6942bcbc40f2f1ce42587ceec5264204f3b35c` immediately before launching native attempt 2. No mention test ran against the unavailable renderer, and this environment event is not a product failure.

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/verification-test.py pnpm exec playwright test e2e/collaboration-mention-journeys-web.spec.ts --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-web/attempt-6
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/verification-test.py pnpm exec playwright test e2e/collaboration-mention-journeys-desktop.spec.ts --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-desktop/attempt-2
```

Final E2E source hashes (SHA-256):

| File | Hash |
| --- | --- |
| `e2e/fixtures/collaboration-mention-journeys.ts` | `aa256fb98809c7fcf55944557bbda44e16b551416db38e32bdf075d6eb76d2c7` |
| `e2e/collaboration-mention-journeys-web.spec.ts` | `67a6cdd7da37d5840dbaca0f0b3396c7f22671e74c4a388554536d40b7d63d47` |
| `e2e/collaboration-mention-journeys-desktop.spec.ts` | `515eb616f099e313002727a9bbf16d33727b9e83993942c570a1999cba4304e8` |

### Exact identities and current receipts

| Platform | Exact test identity | Current completed evidence |
| --- | --- | --- |
| Web | `Web mixed H+A+B+S mentions retain one comment and exact queued target associations` | attempt 6: comment `d59dcf1c-7d5e-4e81-91b0-b8b643b9c750`; three waiting commands on distinct target/channel identities, each exact input event/batch/sequence; H notification `9b151bb5-ac7c-47aa-885c-d414402bba7c` |
| Web | `Web owner revocation after preview refuses only A and original-key replay stays deduplicated` | attempt 6: A blocked, B command `45e5f0c3-e955-452b-aaba-9cba25e0e733`, H notification `14d8cb79-e74d-42cb-9972-39a1e1af6aa6`, identical replay, unavailable preview read-only |
| Web | `Web offline mentions survive a fresh context and edits notify only newly added people` | attempt 6: comment `3d5baa0e-a0d0-41f1-b29d-a16fb257e54a`; H notification `b536dd6b-a899-45c4-b9ba-a9e11e75c93e` read and archived; fresh API login; comment/body deltas and anchors |
| Desktop | `Desktop mixed H+A+B+S mentions retain one comment and exact queued target associations` | attempt 2: comment `32847c17-f302-4931-b1ad-07a640f0a7e4`; three waiting commands; H notification `3b150c07-a562-44fd-ad55-8fd44a71154c` |
| Desktop | `Desktop owner revocation after preview refuses only A and original-key replay stays deduplicated` | attempt 2: A blocked, B command `3e42a1ff-15cc-4ded-a703-0f6125befcc7`, H notification `57055171-623d-45ca-a321-6145c7231ce7`, identical replay, unavailable preview read-only |
| Desktop | `Desktop offline mentions survive a fresh profile and edits notify only newly added people` | attempt 2: comment `3e68ace7-3490-463b-8f6c-4565e3480ad8`; H notification `5414b19d-c35a-4845-a78b-e1e87ea9d2d1` read and archived; fresh API login; both edit deltas and anchors |

For both offline cases, surrounding-text-only edits add no notifications. Adding owner D in the comment adds exactly one `mention_added` notification with the original comment anchor; the first body edit notifies H once and records the exact description change event, then adding D notifies D once. Actor P and the outside squad human X receive none. Typed AI mentions in edits create zero AI commands or runs. Human participants, creator, handler, business state and current handling remain unchanged by notification read/archive or any edit.

### Artifacts and visual inspection

Raw JSON/stderr and original attempt output directories are retained at `.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-{web,desktop}/`. Decoded credential-free inline identities/geometry, run summaries, source hashes and PNG hashes are in `.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-safe-evidence/`. The first failed attempts remain available; no retries were enabled.

The safe export retains all eight attempts and the six final identities. `web-6-{1,2,3}-*identities.json` and `desktop-2-{1,2,3}-*identities.json` are the current receipt/notification evidence; the grouping/geometry attachments use the same prefixes. Its 21 final PNGs comprise ten Web captures and eleven native captures, including both unavailable previews.

| Safe manifest | SHA-256 |
| --- | --- |
| `runs-final.json` | `53354dcc63cca4b9a6bbdb6760e3b71e5b10a73e32604cee57c006c74fe947c8` |
| `png-hashes-final.json` | `f6ff5566fa394f2897e2e42742c4211b86b180597a06d662c89fb2ebc49bfb48` |
| `source-hashes-final.json` | `3c81bd1312f4ad651dc7cdc45d12021c10f24d92693873bbe9c12d4dd2b21b2a` |

Web captures assert actual 1280×1000 and 390×844 viewport PNG dimensions and no horizontal overflow. Native captures use the compositor at 100% and 200% zoom with actual scroll-root coverage, including two overlapping parts for the long comment at 200%. All four contact sheets from Web attempt 3 and Desktop attempt 1 were visually inspected: the intended preview, per-target refusal, comment and body are readable with wrapping, no clipped target text and no horizontal overflow. No product visual edit was necessary.

The final Web attempt 6 unavailable preview at 390px and final Desktop attempt 2 unavailable preview at 200% were also inspected: disabled A's identity/reason and B's paused state are readable with normal wrapping. Final strict TypeScript, scoped ESLint and source/whitespace checks passed after the only shared fixture change. The main coordinator owns independent review, ledger/spec updates and Lore commits.

## Files changed and simplifications

- `e2e/fixtures/collaboration-mention-journeys.ts`: replace retired-preview checked-state waits with the actual next-preview contract; add the bounded read-only unavailable preview and returned proof.
- `e2e/collaboration-mention-journeys-web.spec.ts`: perform real H login after offline send and verify the same identity before the fresh context.
- `e2e/collaboration-mention-journeys-desktop.spec.ts`: perform the same login-after-send step before the fresh native profile.
- This report: retain exact commands, original failures, environment provenance, current receipts, hashes, results and boundaries.

The tests reuse existing fixture authentication, APIs, renderer and native capture helpers. No abstraction, dependency, product behavior, shared fixture, release gate, task state, commit or parent archive was added or changed by this lane.

## Remaining boundaries

These six cases use actual Web and Electron renderer/account-shell flows, public APIs and synthetic offline resources. They do not establish provider process execution, packaged full Main startup, containment, other operating systems, interactive login-page behavior or full acceptance of the 97-row parent contract. Production creation/execution gates remain outside this lane's authority.
