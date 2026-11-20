# R23 independent R22 restoration and reuse check

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`. Reviewer: `/root/r23_restoration_check`. Reviewed on 2026-11-17, with current process observations at `2026-11-17T03:25:31.661893+00:00`.

**Verdict: ACCEPTED_R22_RECORDED_RESTORATION_CURRENT_ARTIFACTS_AND_CONSUMERS_SCOPED. Blocking findings: 0.** The original R22 four cases remain **1 PASS / 1 FAIL / 2 NOT_RUN**, with zero retries. Full S2/A01–A12, the parent and real S4 acceptance remain open.

## Findings (fixed)

None. The reviewer wrote only this report, its JSON companion and exclusive private review artifacts. No product source, configuration, prior evidence, database row, artifact generation or process was changed.

## Findings (not fixed)

- `r22-final-restoration-audit-plan-20261117.md` refers to a JSON companion that is absent. This is a nonblocking documentation pointer issue outside the assigned original-document write scope. The Markdown and every actual evidence reference used for this acceptance exist and were independently checked; no nonexistent file was counted as proof.
- Source reuse applies to the recorded R22 consumers. Main must separately bind the new R23 test-only codec inputs, new configuration and fresh generations. This report does not certify the current entire ROOT checkout or a future listener/database state.
- The original R20 exclusive lock holder remains unproved. A succeeded controlled rollback record does not replace the failed R22 client journey or its unreached UI/receipt assertions.

## Verification

The new independent audit ran with exit **0**, without executing a test suite, request, SQL query, build or signal. Its exclusive [private acceptance seal](/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r23-control-opaque-version-r01/check-restoration-current-r01/acceptance-seal.json) has SHA256 `bbffd09de1059a1aa6241f984d2edbb9c5bcc77114a6df808ae66019d2f83bd6`. The [JSON report](r23-r22-restoration-current-check-20261117.json), SHA256 `a219abc9d4457fb2a281d99f39f1c0f896bfeefb193e22f566b35432adb91e32`, binds **102** exact references and the fresh observations.

| Check | Independent result |
| --- | --- |
| Original four-case result and raw reporter | Exact four identities and attempt mapping; 1 PASS / 1 FAIL / 2 NOT_RUN, exit 1, retry 0 retained |
| Artifact baseline and additions | R20 117 retained + 10 prior additions + 8 R21 slots = 135 prior paths; exactly 10 R22 execution additions = **145 current paths**, no omissions, unrelated additions or symlinks |
| Older generations and current slots | **64** prior directory inode/mode identities unchanged; only first two reused R22 slots sealed 0700→0500; remaining six stay empty 0700 with original inode/owner |
| Actual archives | **12** archives, each 2,048 bytes, one regular `stage-regression.json` member; metadata/end blocks, tar SHA256, exact sealed payload and database source revision agree |
| New artifact provenance | Original case2 approval/spec/receipt/source/hash/size fields match DB rows; sealed manifests match workspace, agent, task, child and task generation |
| Consuming inputs/products | **3,359** frontend inputs, **1,961** dependency links, **1,676** outputs, **5,047** Go inputs and **4** Go products freshly checked; zero drift |
| Recorded stopped database | Actual R05 identity/summary recomputed from rows; connections 0, original false/true/false gate at epoch28, no unsettled Host run, active lease/binding, unreleased reservation or incomplete artifact |
| Fresh owned process/socket observation | API32107 and bridge32101 absent; no API listener at64922 or R22 published socket; production Web86953/64925 and renderer86966/64926 still observed |

Lint, TypeCheck and tests were **not rerun**: this slice reviews retained evidence and performs no product-code edits. Previously accepted codec checks and the original runtime results remain separate evidence; no new test pass is claimed here.

## Preserved history and stop evidence

`owned-stop-r03.json` is bound at SHA256 `3c5a868684058dc3eb5bbf4394df1d7d26bf8827a60e8ed40c8c725b60b31cb2`; actual `database-after-stop-r05.json` is bound at `d0398103ea9ec1e2d63ae97ebe71f9e7d0d89992ec4b7f7e5aafbbbc94b1a6df`. The first lsof/stop-preparation error, subsequent log-path report error and their original scripts remain present. The mislabeled R04 live snapshot is excluded from the stopped gate. No repeated or foreign signals were sent by this reviewer.

The R05 recorded history contains **15 queues, 27 settled Host runs, 12 complete artifacts, 4 intents, 6 execution observations and 3 rollback metric observations**, with one old unresolved intent retained. After-four and after-stop tracked row values match exactly. Before-API→live startup recovery adds one old synthetic producer task and changes two old queue rows to failed with `runtime_reconnect_timeout` / `runtime_offline`; no old IDs disappear. Live-before→after-four adds the declared ordinary synthetic producer, four settled Host runs, two artifacts, two intents, three execution observations and three metrics; old rows remain unchanged.

The current artifact snapshot was sealed **before** handoff to main for next-generation allocation. [artifact-current.json](/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r23-control-opaque-version-r01/check-restoration-current-r01/artifact-current.json), SHA256 `704249bcf9c012851ff83551f3e6d2beab1afab22bf7fd7af9fc7f2271cbaa70`, is the 145-path pre-allocation baseline. Any following eight directories belong to next-run preparation and require their own binding.

Original rollback request `61b4733a-f70e-424c-8f4b-c337ea9c3f52`, durable succeeded intent and observation are preserved and were never replayed. No historical queue, unknown intent or failed case was removed or relabeled to obtain this scoped acceptance.
