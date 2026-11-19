# R24 independent failed-run and restoration check

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`. Reviewer: `/root/r24_after_restoration_check`. Reviewed 2026-11-17.

**Verdict: R24_FAILED_RESULT_AND_RESTORATION_ACCEPTED_SCOPED. Restoration blockers: 0.** The original R24 four-case run remains **3 PASS / 1 FAIL / 0 NOT_RUN, retry0, exit1**. Resource, source-record and artifact restoration are accepted for this stopped run. Main may use the new immutable current artifact baseline to prepare eight fresh R25 generations. Full S2/A01–A12, the parent24, S3–S5 and business sign-off remain open.

## Findings (fixed)

- File: private `after-independent-check/process-current.json` and verifier.
- Issue: the first private listener assertion assumed `lsof -Fp` prints only PID fields. The actual output also includes valid `f12`/`f16` descriptor fields.
- Fix: classify only `p<decimal PID>` fields. The original raw observation and preseal assertion are retained. The parsed listener owners remain Web86953 and renderer86966; API64922 is free.

- File: private `after-independent-check/verify-runtime.py`.
- Issue: the first reporter assertion assumed a failed run has no global reporter diagnostic. The original reporter contains `Testing stopped early after 1 maximum allowed failures.` even though all four actual cases have terminal results.
- Fix: retain that exact diagnostic alongside the four-case inventory and keep the original runtime failed. The failed verifier bytes and preseal assertion are preserved in separate files. Neither correction changed product code, original tests or original evidence.

## Findings (not fixed)

- Frozen `e2e/fixtures/collaboration-stage-execution.ts:78` still times out after the original20-second wait for a POST completion-preview response, after selecting Artifact evidence. The fourth case's whole-case timeout is480seconds and its actual duration is63.148seconds. Its source and failure remain preserved; main owns the separate helper diagnosis and fresh R25 execution.
- The successful exact submission is present, but the subsequent shared positive M1 preview/authorization, interrupted original completion, Desktop consumption, Web recovery and later R38 assertions were not reached. The retained shared transport trace has empty Web and Desktop write arrays. There is no completed lifecycle200 in this case.
- An earlier **self-completion negative control** did issue an authorization201, request `1acb641d-111a-4712-8b56-2d1740d7ac9e`, for `target.kind=collaboration_self`. It precedes the positive submission and is independent of the unreached shared submission-completion flow. A blanket “no M1 grant exists” claim would be inaccurate.
- This audit covers the captured runtime/history sets and original HTTP evidence. It does not grant full S2/A01–A12, parent24, real S4 integration, S3/S5 or formal human acceptance. No task, plan, matrix, policy, source or spec was edited by this reviewer.

## Verification

- Lint: not rerun under the explicit artifact-only assignment. The previously accepted R24 selector ESLint result/logs were rehashed; exit0, zero errors/warnings.
- TypeCheck: not rerun under the explicit assignment. The previously accepted strict TypeScript result/logs were rehashed; exit0.
- Tests: no suite or browser rerun. The original R24 runtime result is **FAIL**,3PASS/1FAIL/0NOT_RUN, with one attempt per case. Independent artifact, history, result, reference and process parsing checks pass.

The audit independently rehashed **107 decisive references**, parsed all four original terminals and four scenario evidence files, compared the recorded histories, and inspected current filesystem/process metadata. It issued **zero SQL/API/network requests, signals, launches, builds, test suites, real-account operations or Git mutations**. The complete historical source graph and already accepted code controls were not rerun.

| Original case | Actual status | Duration | Retry |
| --- | --- | --- | --- |
| Roles, human decisions and prepared reports across Web/Desktop | PASS | 72.965s | 0 |
| Release and rollback exact builds/decisions/controlled execution | PASS | 50.477s | 0 |
| Late authenticated deployment remains historical | PASS | 39.007s | 0 |
| Submission, M1 completion and administrative/project exits | FAIL | 63.148s | 0 |

All four reported timeouts are480000ms. The reporter has skipped0/flaky0 and retains its global max-failure diagnostic. All four scenario page-error lists are empty; their respective cleanup error lists are empty. The original R23 result remains unchanged at3PASS/1FAIL/0NOT_RUN, SHA256 `440d404d437fa34b1e808e56ffefa98bca30ea2768cb8970db3466f24db7a21f`.

### Successful submission and remaining runtime boundary

The repaired toolbar path opened the actual Submit result form. Its original POST to `/api/collaboration/issues/01a1e851-7e6e-795a-b3d1-8088000a1d91/acceptance-submissions` returned200 with request `ad4df2a2-e9a8-4aa4-9242-6ab67c270367` and immutable submission `83f6f4a3-fee7-4d9a-8c84-6ea9c50b7216`. Its request-body SHA256 is `1baee808d0634fcf628f038c6f6c9c66db6615248112a4bae5cd2b98b637ac53`, independently reproduced from the stored exact body. The response SHA256 is retained as `e4845a5c42eda7a79f5c4244a90cf5a6cea4aa9dcc3ec155a81bdf82dadda196`.

The original20-second `stageCompletionReview` failure follows that submission. No completion-preview HTTP fact appears in the case, and the bound after-stop log observations have no matching path records. These are retained observations; this reviewer did not issue a diagnostic request or replay any prior write. Steps from different failed runs are not combined into a successful case.

### Database and owned shutdown

The after-stop snapshot, SHA256 `2790b0b785b0c876a21b120dc37d0634ff914edf3ccf36e4a92ee6009dd45179`, is bound to the same original read-only query set, collector and private configuration as before-API/live-before/after-four. Its actual summary is connections0, restored gatefalse/true/false at epoch44,39 settled Host runs,26 complete artifacts, no bound preparation lease, unreleased reservation or incomplete artifact, and one retained unresolved old intent.

Before-API→live-before startup retains all20 original queue identities, adds four deferred retry rows and updates five old rows. Four previous synthetic producers become failed with `runtime_offline`; old deferred `01a1e799-2f60-7f17-8d76-4f1153d9b5c7` becomes failed with `runtime_reconnect_timeout`. Each new retry retains its original parent/retry-of identity. Runs, artifacts, execution intents, observations, metrics and gate/reservation values remain unchanged in that startup interval.

Live-before→after-four adds four ordinary producer setup rows, six settled Host runs, seven complete artifacts, four execution intents, six observations and three metrics. Every pre-existing captured row remains unchanged. After-four→after-stop leaves **all eight captured history sets byte-value identical**: queue, runs, artifact records, execution intents, observations, metrics, gate and reservations. Only the identity/connection observation changes; connections5→0.

The final28 queue rows retain20 failed,4 deferred and4 running ordinary synthetic producers. The four new running rows have provider `stage_artifact_fixture`, cloud mode, no parent/retry-of and no collaboration run ID. The frozen `ordinaryProducer` fixture explicitly creates these setup rows for actual local packaging. They are separate from the39 settled Host runs and were not cleaned or converted into successful history. Original unresolved intent `01a1e33a-cc80-74e5-bd70-6aa0a54e8168` and all historical unknown observations remain intact.

The owned-stop receipt has SHA256 `7b225240ead8249bbc33ee345b9b88b4d37b8b91c9d9141e27bc96aa460cbb51`. Its original intent, parent/argv/listener guards, one SIGTERM to bridge55648, absent API55652/socket and harness terminal `PASS` at stdout line46 are bound and independently checked. The actual R24 stopper differs from its bound original only in the output status literal; every ownership, signal, timeout and PASS guard is byte-identical.

Fresh read-only process observations at08:23:14Z show bridge55648/API55652, recorder57186 and Playwright57187 absent; API64922 has no listener. Web86953/64925 and renderer86966/64926 remain with their R20 production source cwd. The new R24 socket is absent. The old R23 socket is retained, and this check does not invent an orderly original R23-stop claim.

### Source and artifact preservation

All four fresh consuming-source audits—before-API, before-four, after-four and after-stop—use the same eight bound raw references and record **3359 frontend inputs /1961 links /1676 frontend outputs /5047 Go inputs /4 Go products**, drift0. Their26 internal workspace-link relocations and the single R23→R24 Submit result spec delta are preserved. All bound raw references and selected critical frozen helpers/schema, native products, production Web BUILD_ID and renderer/preload bytes were rehashed. This is a scoped retained-source acceptance; it does not certify every current ROOT file.

The artifact comparison uses the predecessor R23 snapshot and R24 preparation record. The actual captured state is **231 paths =188 unchanged prior paths +8 new slots +35 execution additions**. Every prior inode/uid/type/mode and file size/hash matches. Seven new slots retain their original identities and are sealed0500, with files directory0500 and manifest/payload0400; the eighth remains empty0700. Staging root0500 and output0700 are restored.

All26 actual tar files are2048bytes and contain exactly one regular `stage-regression.json` member. Each member equals its sealed file; archive digest/size and manifest `source_revision` match the corresponding DB artifact execution. The seven new executions explain exactly five new paths each. No old path or archive was removed or rewritten.

The reusable immutable baseline is:

`/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/after-independent-check/artifact-current.json`

SHA256 `e97b1beb004690893702643dc51a7fa2743a968bf036ae14570a3d3312265fac`. Its `entries` dictionary preserves the predecessor snapshot shape. It records the state before any later R25 preparation; main should append fresh generations and preserve all231 existing paths. Its detailed artifact audit has SHA256 `d7b62de46f29695035a966f421cad06ec0458b33c28eefb1d285d9f0407b0ee9`.

Detailed [JSON](r24-runtime-after-independent-check-20261117.json), SHA256 `d9cd81ab5c2e891e0fdb5d7893442a0e3feaa41ccd54587eef4b059965077eeb`. Matching private review, history/process/reference audits, current artifact baseline, preserved private checker errors and acceptance seal are under `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/after-independent-check/`.
