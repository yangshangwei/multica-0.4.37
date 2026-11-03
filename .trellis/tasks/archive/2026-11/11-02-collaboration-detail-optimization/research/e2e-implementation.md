# Task-owned detail journey E2E

## Write boundary

Owner: native `e2e_journey` implementer, continuation 2026-11-03.
Only `e2e/collaboration-detail-journey.spec.ts`,
`e2e/fixtures/collaboration-detail-journey.ts`, and this research note are owned.
These files were absent at dispatch. Server, Core, Views, routing, locale,
environment and other test changes belong to the root session and other agents.

The missing evidence is a complete client journey through the current public
creation and locked information/result/decision APIs. The fixture will reuse
`TestApiClient`, the existing native `openDesktop`/`nativeScreenshot`, and the
recipient picker helper. It will bind observations to the exact returned issue,
information-request, submission, decision and original operation identities.
Permission, malformed-wire and admission matrices remain in their canonical
Server/Core suites.

## Runtime boundary

Initial implementation preparation was authorized without runtime execution.
The root session later supplied the real/current runtime permit described in
the continuation below. This agent has not started/stopped services, allocated,
migrated or cleaned a database, or used a real model/provider account.

The fixture must read `COLLABORATION_DETAIL_PROVENANCE_FILE` before creating
actors. The credential-free record must identify the active task, exact source
root, API, production Web, native renderer and allocated API database, plus each
listener's launch/build/source identity. It must agree with the task allocation,
process listeners, current API health, production Web artifacts and the exact
built renderer/preload bytes. A configured mismatch fails before fixture writes.
Credentials remain in the private runtime environment and fixture closures.

Root owns startup/build, the explicit runtime permit and retained database
cleanup. Client contexts and fresh native profiles are the test's teardown scope.

## Planned proof

Each platform gets one complete manual-task journey: external current handler;
ordinary one-action comment with a real validated preflight and no handoff/AI;
exact human information request/return; text-only result and basis; immutable
author/time/submission ID; initiator revision with reason and explicit eligible
actual-author recipient; resubmission; acceptance of the exact second submission;
readable first result and returned decision. The source entry resolves the same
UUID and status, and the ordinary parent stays open and unchanged.

The client journey also exercises explicit comment-to-result preparation,
retention through source-comment edits/deletion, self-result completion without
self-transfer, both locales, long content at wide/intermediate/narrow sizes,
keyboard/form focus, exact comment links and account scroll restoration.
Screenshots and receipt/identity evidence are attached without tokens or cookies.

The Electron fixture compiles selected production Main routing/IPC functions
with test-owned stubs, loads the current built renderer and preload, and uses
real native windows/compositor capture. It does not run installed Main, daemon
lifecycle, provider accounts, kernel containment, installers or the updater.

## Coordination

Root accepted the provenance shape implemented by `detailJourneyProvenance`
and added `data-submission-id` to the shared result record. The record contains
`runtime_permitted: true`, task/source/origin/database identity, `api`, `web` and
`renderer` launch proofs (`listener_pid`, `commit`, `source_id`, `command`),
production Web `build_id`, renderer `artifact_hashes`, and `source_hashes`.
Existing `time[datetime]` exposes the exact server timestamps.

Requested a check of the remaining current-permission context beside Participants.
No product file was changed by this agent.

## Verification

The two spec identities are written. Each platform exercises the full chain and
Chinese self-result completion/reopening. One actual committed submission loses
only its response, then reload and the original GET confirm it without another
POST. Web obtains both signed cookies from real send-code/verify-code responses;
write observations also verify the CSRF header and absence of a bearer header.

Read-only static checks ran with Node `v22.23.2`, TypeScript `5.9.3`:

- `ts.createProgram` over both owned files with strict, noEmit, bundler resolution,
  noUncheckedIndexedAccess and the installed dependency graph: **0 diagnostics**.
- TypeScript syntax inspection: **0 parse diagnostics**.
- Both new sources: **0 trailing-whitespace lines**.
- No test module was executed by those checks.

Prepared source hashes:

| File | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-detail-journey.ts` | `ae86327e740fcb0a8f765b35aca8810141080ee931e7286add77b8818c288c8f` |
| `e2e/collaboration-detail-journey.spec.ts` | `5e6b3b61488671e31c618ee840132e95026ee0e2c221a9789425e047da458d04` |

Run these sequentially from this checkout only after root provides the permit
and parses the private `application.json` as data into the child environment:

```sh
pnpm exec playwright test e2e/collaboration-detail-journey.spec.ts --project=chromium --workers=1 --retries=0 --grep 'Production Web'
pnpm exec playwright test e2e/collaboration-detail-journey.spec.ts --project=chromium --workers=1 --retries=0 --grep 'Electron Desktop'
```

The environment must explicitly provide `COLLABORATION_DETAIL_PROVENANCE_FILE`,
`DATABASE_URL`, `NEXT_PUBLIC_API_URL`, `COLLABORATION_E2E_API_URL`,
`PLAYWRIGHT_BASE_URL` (or `FRONTEND_ORIGIN`),
`COLLABORATION_ELECTRON_RENDERER_URL`, `COLLABORATION_ELECTRON_SOURCE_ROOT`, and
the configured development verification code. Do not source the private data as
shell code or print its values.

**Preparation runtime status: NOT_RUN.** The initial handoff had no actual
browser/native acceptance, screenshot, public creation or database fixture write.
The prepared captures cover
both locales at 1280/900/430 CSS pixels, current delivery, the beginning/end of
the long retained first result, and self-completion/reopening. Actual image
dimensions, viewport state, immutable receipts and native fixture scope enter
the per-test attachment only when executed. Root owns visual-verdict and the
before/after comparison against its separately retained baseline.

Focused client portions of A07/A12/A14/A15/A16 supplement their canonical suites;
the spec does not claim the complete permission, legacy, provider/proof,
protected-byte or auxiliary-governance matrices were exercised in a DOM.

## Runtime continuation and retained Web run 1

Root supplied `/tmp/multica-detail-verification-20261103/runtime-provenance.json`
and the mode-600 `runtime-env.json`. The actual source/working directory is the
frozen worktree `/private/tmp/multica-detail-verification-20261103/source`.
The recorded listeners were API on 18800 (PID 50574), production Web on 13720
(PID 52287, build `A5UUePCzWN61a25sog50P`), and renderer on 5894 (PID 39674). Root owns their lifecycle
and each numbered run's current source/harness provenance.

Root executed `Production Web manual detail: information, delivery, revision,
resubmission and exact acceptance` once. Actual result: **1 failed**, 11.1 seconds,
at A03/A12, `openJourneyAction` line408, missing form `Submit for acceptance`.
Start work, information request/return and the ordinary single-action comments
had passed before this failure. Retained originals:

- `/tmp/multica-detail-verification-20261103/journey-web-run1.log`
- `/tmp/multica-detail-verification-20261103/journey-web-run1-artifacts`

Diagnosis used the actual failed source, log and frozen label/form owner.
`labels.action("submit_acceptance")` returns `delivery.submit` (`Submit result`),
which `WorkflowForm` uses for its accessible name. The E2E copy map still read
`workflow.acceptance` (`Submit for acceptance`). The comment branch presses
`Use in result` once; no second trigger click exists. Its successful options GET
was initiated by the mounted form. This is verified selector drift, with no
product correction required.

Changed only the helper's submit label mapping, preserving every result/basis,
identity, ordinary-comment/no-write, retention and exact-acceptance assertion.
The spec now also writes a durable credential-free `detail-journey-evidence.json`
and failure UI/compositor artifacts before closing clients. It does not retain
tokens, cookies or the private runtime environment.

Copied only the two owned E2E files from the original worktree into frozen source.
Compared all non-E2E paths in root's product hash map before/after: **0 product
files changed**. Strict no-emit TypeScript compilation in the actual frozen
working directory: **0 diagnostics**.

| Corrected file | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-detail-journey.ts` | `65705e6f66a58d91fec0ab8618395183771a9641b78e0877200c64090b52efc9` |
| `e2e/collaboration-detail-journey.spec.ts` | `f37ad32b09c42a248b4e8bc3d74916927f980921b1786dacc963178415cda859` |

The corrected hashes were sent to root for the numbered run-2 provenance update.
The original run-1 failure/log/artifacts remain unchanged.

## Retained Web run 2 and navigation correction

After root updated the harness provenance and explicitly authorized execution,
this agent ran Web with one worker, zero retries, `--grep 'Production Web'`,
`--reporter=line,json`, parsed `runtime-env.json`, and the exact frozen cwd.
The process handle was PID 59822 / exec session 37293 and was reported at startup.
The actual result was **0 passed, 1 failed, 0 skipped, 0 flaky**; report duration
17.8 seconds. No Electron run was started.

Retained evidence:

- `/tmp/multica-detail-verification-20261103/journey-web-run2.log`
- `/tmp/multica-detail-verification-20261103/journey-web-run2-report.json`
- `/tmp/multica-detail-verification-20261103/journey-web-run2-artifacts`

The real client chain passed through exact acceptance before the A12/A17 scope
restoration assertion failed at helper line671. The durable JSON records issue
`01a19f68-8474-70fd-8323-c9d30042dc08`, first submission
`8a797eb7-59b9-408d-85da-4594c0e562da`, resubmitted/accepted submission
`cbb39002-1709-4bc1-8b6c-3e35c5a3a0eb`, source `DETA-2` with status `done`, and
ordinary parent status `in_progress`. Original-key lost-response recovery,
source-comment edit/deletion, exact revision, resubmission and acceptance checks
had passed. This is useful partial client evidence, not a passing complete case.

Failure UI/screenshot shows the workspace Issues board. The fixture's
`openAccount` used `page.goto`, and A12 then performed another `page.reload`.
`packages/core/navigation/scope-history.ts` explicitly says "Browser-tab
lifetime only" and has no persistence; those document replacements erase its
workspace path. `resolveScopePath` correctly returns the workspace Issues
fallback. This is fixture-induced store reset, not a production restoration bug.

Root approved the correction against that documented contract. Initial actual
login/full navigation and the A03 lost-response reload remain. Subsequent Web
account/source moves now use the production `multica:navigate` bridge and await
canonical URLs. A12 uses the existing Participants > Refresh control, whose
owner invalidates `collaborationKeys.issue(scope,id)`, parses the actual
authorized detail GET, and verifies the same ID/status/delivery without a full
reload. Exact source-title, aria-current, repeated-anchor and saved-scroll
assertions remain unchanged.

Copied only the two E2E files into frozen source; **0 product hash paths changed**.
Strict no-emit TypeScript check there returned **0 diagnostics**. Final run-3
hashes sent to root for binding:

| File | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-detail-journey.ts` | `9361430ac2f2adbf23c35d94250473dcd4a9a532bdf969cd021dd97e0e0eae91` |
| `e2e/collaboration-detail-journey.spec.ts` | `c59cba84131b1436439ad6f6b6415de086318a96569373f0b96fa4479e6fe987` |

Every original run-1/run-2 log, failure and screenshot remains retained.

## Final complete Web and Electron runs

Root bound the corrected harness in `runtime-provenance-web-run3.json` and the
current permit, then explicitly authorized Web3 and sequential native execution.
Each launcher verified both current E2E source hashes before the test could write
fixtures, parsed the private environment as data, and used the frozen cwd.

| Actual run | Exit | Passed | Failed | Skipped / flaky | Duration | Handle |
| --- | --- | --- | --- | --- | --- | --- |
| Production Web run3 | 0 | 1 | 0 | 0 / 0 | 18.49 s | PID 63034, exec 97624 |
| Actual Electron run1 | 0 | 1 | 0 | 0 / 0 | 20.97 s | PID 63396, exec 34535 |

Commands used one worker, zero retries, `--reporter=line,json`, and the respective
`--grep 'Production Web'` / `--grep 'Electron Desktop'`. Both exec sessions closed.
The native fixture closed both owned Electron clients and removed their fresh
profiles. Root's API/Web/renderer listeners and retained database were left to
root's lifecycle ownership.

Both **complete** cases passed manual external handling; exact information
request/return; ordinary single Publish with a real preflight and no transition
or AI work; explicit source-comment preparation; nonblank text-only result/basis;
real committed-response loss and original GET recovery with one POST; preserved
snapshot after source-comment edit/deletion; exact initiator revision to the
explicitly resolved actual author; a new resubmission; and acceptance of that
exact current submission from the same source issue. Both retain the first
result/returned decision, leave the ordinary parent `in_progress`, repeat the
source-comment anchor, restore **620 px to 620 px**, complete a Chinese self result
atomically, and reopen it while retaining its accepted record.

Exact immutable delivery correlation from the saved public DTOs:

| Platform | Submission | Author ID | Submitted at |
| --- | --- | --- | --- |
| Web first | `b3968f09-4a4a-47e8-9604-8e6c600ee64e` | `f38ff413-c11c-4ed0-8b74-0981c09e3795` | `2026-11-03T11:38:31.963706+08:00` |
| Web accepted second | `b04b4117-6726-4907-bb78-c487d390b406` | `f38ff413-c11c-4ed0-8b74-0981c09e3795` | `2026-11-03T11:38:34.12327+08:00` |
| Electron first | `6937bf89-9c31-4ae0-b1da-57cdae08c8a0` | `a11d373c-9579-42e2-911d-29722725c169` | `2026-11-03T11:42:33.197846+08:00` |
| Electron accepted second | `435df4f2-2126-4f01-8dbe-b95a716d486e` | `a11d373c-9579-42e2-911d-29722725c169` | `2026-11-03T11:42:35.785234+08:00` |

Web issue/source UUID: `01a19f7c-9ee5-73e4-a46e-43fe0d244c5d`, `DETA-2`, `done`.
Acceptance actor `ef3dc173-faff-4daf-a6f1-5a617b4c89af`, event sequence `12`, exact
time `2026-11-03T11:38:35.480905+08:00`, bound to the Web second submission.
Electron issue/source UUID: `01a19f80-46ce-740f-99d9-d600a1c42364`, `DETA-2`, `done`.
Acceptance actor `b9275282-a67c-4558-8ed6-26f10fbb39e7`, event sequence `12`, exact
time `2026-11-03T11:42:37.755067+08:00`, bound to the native second submission.
Returned-decision binding, author/time and retained history were asserted through
the actual authorized history DTO and visible `time[datetime]` records.

Durable logs/reports:

- `/tmp/multica-detail-verification-20261103/journey-web-run3.log`
- `/tmp/multica-detail-verification-20261103/journey-web-run3-report.json`
- `/tmp/multica-detail-verification-20261103/journey-desktop-run1.log`
- `/tmp/multica-detail-verification-20261103/journey-desktop-run1-report.json`

Capture and `detail-journey-evidence.json` roots:

- Web: `/tmp/multica-detail-verification-20261103/journey-web-run3-artifacts/collaboration-detail-journ-f64b8-ission-and-exact-acceptance-chromium`
- Electron: `/tmp/multica-detail-verification-20261103/journey-desktop-run1-artifacts/collaboration-detail-journ-c170c-ission-and-exact-acceptance-chromium`

Each passed case produced **19 captures** in `en` and `zh-Hans`: manual before;
accepted/header and current-result views at 1280/900/430 CSS pixels; start/end of
the long retained first result at narrow width; and Chinese self completion and
reopening. Actual PNG dimensions matched the retained viewport geometry at
1280x900, 900x900 and 430x900, pixel scale 1, on both platforms. The native PNGs came
from the physical Electron compositor through `nativeScreenshot`.

Native evidence identifies `darwin`, the current production renderer/preload,
selected compiled production Main routing/IPC functions and fixture services;
both actors recorded `daemon_starts=0`, `provider_account_access=false`, and
`installed_main=false`. The native fixture SHA-256 is
`f9021ab07635f2f48d0f90613fcd3c8558cb7262b3bdd2aca98edb31654475d2`.
It does not prove installed Main, provider execution, kernel containment,
installers or update behavior.

After both runs, read-only comparison against every current provenance source
hash returned **0 mismatches**. This lane changed only the two original owned
E2E sources and this note; copied only the E2E sources into frozen source; added
no dependency, product change, service operation, allocation, migration or commit.
Root-owned visual-verdict/baseline review, broad quality gates, final artifact
retention and database/service cleanup remain follow-ups outside this lane.

## Bounded capture-framing correction

Root's review found that `desktop-accepted-en-wide.png` showed long requirements
instead of the intended accepted delivery. Before changing a fixture, this agent
read the visual-verdict skill, viewed the retained native accepted/result images
and the correct `web-result-en-wide.png` reference, and persisted the initial
**score 32 / revise** verdict in
`research/continuation-20261103/visual-verdict-initial.json`.

The capture call had no explicit accepted-result target. Separately, the native
capture helper applied its final scroll after its last render wait. Retained
native frames show the preceding scroll position: the accepted image shows
mid-requirements while its DOM metadata says `scrollTop: 0`, and the result image
shows the page header while its metadata says `scrollTop: 1579`. This is verified
fixture framing/paint timing; no production UI change was justified.

The single fixture batch declares concrete target heading/content anchors,
scrolls the actual account-detail container to the complete target bounds,
waits after that final scroll, and asserts every target rectangle against the
intersection of the scroll-root and native content viewport before/after PNG
capture. It retains target text/rectangles, actual native content size and stable
scroll geometry. Accepted wide captures cover the formal accepted record;
smaller accepted captures cover the complete decision block; result views cover
result and basis; long-result views cover heading/first paragraph or final
paragraph; self-result captures cover the complete short record. Original
journey, identity and saved/restored 620 px assertions remain.

Only the two E2E files were copied into frozen source. Product hash paths did
not change; strict no-emit TypeScript check there returned 0 diagnostics.
Harness hashes sent to root for the one bounded Web4/Desktop2 confirmation:

| File | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-detail-journey.ts` | `56181bfef4c16cddfee5daf5a9f1ecafe0f6f6e49a7016670dc608226a685219` |
| `e2e/collaboration-detail-journey.spec.ts` | `269ed34d94c6a4a12bbedd13cd6a5e19364262a8035e289f73e095e328a2863e` |

All earlier behavioral successes, failure logs and images remain unchanged.
Confirmation is pending root's updated source provenance; root owns the final
`.omx` verdict, service lifecycle and final retention.

The first confirmation launch guard found the current `runtime-provenance.json`
still held the previous helper/spec hashes (`9361430a...` / `c59cba84...`), despite
root's binding message. It stopped before starting Playwright, opening a browser,
writing a fixture, or creating a run4 output/log folder. Both original and frozen
E2E files matched the new hashes above. The exact path/keys and actual hashes were
reported to root for correction. This was a preflight refusal; the one bounded
confirmation round had not executed.

## Single capture confirmation: Web4 then Electron2

Root corrected the exact current provenance map, retained the previous map as
`runtime-provenance-before-capture-fix.json`, and saved the task copy
`research/continuation-20261103/runtime-provenance-capture-confirmation.json`.
The launchers verified the two frozen harness hashes above before any fixture
write, parsed `runtime-env.json` as data, and used the explicit frozen cwd.
Exactly one Web/Electron confirmation round then executed sequentially.

| Confirmation | Exit | Passed / failed | Skipped / flaky | Duration | Handle |
| --- | --- | --- | --- | --- | --- |
| Production Web run4 | 0 | 1 / 0 | 0 / 0 | 19.43 s | PID 77846, exec 10438 |
| Actual Electron run2 | 0 | 1 / 0 | 0 / 0 | 23.23 s | PID 83593, exec 32369 |

Both complete journeys retained all business, exact-receipt, history, permission,
anchor, self-result/reopen and **620/620 px** restoration assertions. Every one
of the 19 captures per platform recorded `targetContainedBefore: true` and
`targetContainedAfter: true`, stable target text/rectangles and unchanged scroll
geometry after the PNG. Both exec sessions closed; the test-owned clients and
profiles were closed by the fixture. Root-owned API/Web/renderer services and
retained database were not stopped or cleaned.

Actual commands (private environment loaded as data by the retained Python
runner, never sourced or printed):

```sh
pnpm exec playwright test e2e/collaboration-detail-journey.spec.ts --project=chromium --workers=1 --retries=0 --grep 'Production Web' --output /tmp/multica-detail-verification-20261103/journey-web-run4-artifacts --reporter=line,json
pnpm exec playwright test e2e/collaboration-detail-journey.spec.ts --project=chromium --workers=1 --retries=0 --grep 'Electron Desktop' --output /tmp/multica-detail-verification-20261103/journey-desktop-run2-artifacts --reporter=line,json
```

The original successful output lines remain in the raw logs:

```text
1 passed (19.4s)
{"run": "journey-web-run4", "exit_code": 0}
1 passed (23.2s)
{"run": "journey-desktop-run2", "exit_code": 0}
```

Logs and JSON reports:

- `/tmp/multica-detail-verification-20261103/journey-web-run4.log`
- `/tmp/multica-detail-verification-20261103/journey-web-run4-report.json`
- `/tmp/multica-detail-verification-20261103/journey-desktop-run2.log`
- `/tmp/multica-detail-verification-20261103/journey-desktop-run2-report.json`

The Python runner's exit JSON was returned by the closed exec handle; the
Playwright success lines are retained verbatim in each raw log. Neither report
has unexpected, skipped or flaky tests.

Each capture root contains `detail-journey-evidence.json`, complete target text/
rectangles/native content dimensions, and its 19 PNGs:

- Web4: `/tmp/multica-detail-verification-20261103/journey-web-run4-artifacts/collaboration-detail-journ-f64b8-ission-and-exact-acceptance-chromium`
- Electron2: `/tmp/multica-detail-verification-20261103/journey-desktop-run2-artifacts/collaboration-detail-journ-c170c-ission-and-exact-acceptance-chromium`

Accepted English wide capture geometry at the actual compositor capture:

| Platform | Target | Target bounds (left,top)-(right,bottom) | Content viewport bounds | PNG |
| --- | --- | --- | --- | --- |
| Web4 | Complete accepted delivery `9dd9c2e1-d5ea-4aab-99d9-25c5abbcdd37` | `(384,132)-(1248,541.75)` | `(352,116)-(1280,900)` | 1280x900 |
| Electron2 | Complete accepted delivery `8b324c85-b189-4731-b3e9-d2adb0fc90c7` | `(392,180)-(1229,589.75)` | `(360,164)-(1261,892)` | 1280x900 |

Native content size is 1280x900 with zoom 1 and pixel scale 1. The native heading
rect is `(392,180)-(1229,204)`; the complete formal record rect is
`(392,224)-(1229,589.75)`, including author/time, result, basis and accepted
decision. The before/after geometry snapshots match exactly. The native PNG now
visually shows those targets rather than the previous requirements/header frame.
This agent also inspected the corrected native Chinese narrow result/basis
and long-result English start PNGs; they show their complete declared anchors.
Root owns the final visual verdict and `.omx` persistence.

Post-round read-only comparison found **0 current provenance mismatches**.
The frozen/original E2E files match the two final hashes above. The recorded
main/frozen non-E2E differences are
`server/internal/handler/collaboration_delivery_test.go`,
`server/internal/service/admin_collaboration_test.go`,
`server/internal/service/collaboration_ai_policy_test.go`,
`server/internal/service/collaboration_handoff_test.go`, and the known
`server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md`
doc-only pointer wrap. They were reported as separate root-owned deltas; this
lane did not copy or revert them and did not change production code. All earlier
failure/success artifacts remain retained, with no additional confirmation run.
