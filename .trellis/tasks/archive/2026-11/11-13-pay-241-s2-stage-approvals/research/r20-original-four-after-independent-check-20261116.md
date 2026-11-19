# R20 original four cases: independent after check

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Verdict

The retained result, current frontend inputs, artifact history and owned API stop are independently reconciled with **zero after-audit mismatches**. The actual four-case result remains **1 PASS / 1 FAIL / 2 NOT_RUN**, retry0. S2, A01–A12 and parent acceptance remain open.

The original first approval/QA case passed in58.741 seconds. Its unchanged source executes the source-denial assertion that failed in R19, and its retained A08/A10 steps, typed-report/QA history and cleanup complete without reported page or cleanup errors. This supplies actual runtime evidence for the R20 panel repair.

The second release/rollback case failed after19.920 seconds at the original `stageApprovalUI` assertion in `collaboration-stage-execution.ts:338`: Record approval in Approve QA remained disabled during the unchanged five-second expectation. The remaining two cases were not run under `maxFailures=1`. Investigation of that new failure belongs to the R21 diagnosis lane; this after review does not infer its cause or relax the form's constraints.

## Result and source bindings

The original reporter/stdout/stderr and result references match their recorded hashes. The four-case denominator, one passed result, one failed result and two unstarted results are retained. The actual case command's exit1 remains recorded separately from subsequent intentional API shutdown.

The accepted product/source/output review `2d3751e4…` and original entry review `d1d7cf94…` are reused. The bounded current frontend after check verifies3,359 declared source byte/size/mode identities,1,961 link targets and1,676 compiled outputs against those accepted inputs: zero mismatches. This did not repeat the complete13,428-file historical backend/ROOT audit.

An independent recursive check of the current result, final binding/permit, filesystem and database/stop records rehashed106 distinct file/hash bindings with zero mismatch. This is evidence reconciliation; no new test, API or database command was run by the checker.

## Filesystem and the two new artifact executions

All117 recorded before paths remain present. Existing file hashes, inode/owner/type identities and permissions are preserved except for the two assigned R20 staging directories intentionally sealed from0700 to0500. Those directories belonged to this run's new unused generation set. The old56 generation directories keep their recorded inode/mode identities.

The actual new filesystem path set contains exactly10 paths, matching the after record: two sealed input manifests/file trees and two output directories/tar files. The other six R20 generations remain unused. The staging root is0500; prepared manifests/files are0400, output directories0700 and tar files0600.

| Actual execution | Staging generation | Source revision | Tar SHA256 |
| --- | --- | --- | --- |
| `01a1e4a1-964f-73b1-b077-50c8f5301348` | `d59d4384-14e1-47e8-b1f1-c13aedcad097` | `8e82eaec439df9bf2f28024782d8fc41c9ab1e95` | `deb797a943bd9d6edff37595936ac859884b56ab13612324214b698c86dbc665` |
| `01a1e4a1-969d-7acd-99e6-93c4c37f229d` | `b0024b1b-2e6a-4a8b-b62f-e0330743685d` | `1eb38fb89cc07eef462fc8a5e2974b2c6f79ead1` | `2032b567c6227cb3dddee5f2717cb967a84a40c56c46d3b7172a35861d2c6d2a` |

Both recorded executions are `succeeded/packaged`, each tar is2,048 bytes, and each tar contains the exact105-byte `stage-regression.json` from its sealed staging input. Manifest source revisions match the corresponding execution rows. The spec/receipt digests remain bound to those rows; this review does not independently recompute the opaque receipt digest or label local packaging as external deployment.

The two manifests name the same new synthetic producer task `d27bbd39-745e-4fb6-8326-10541b201fdc` and its agent/resource, with distinct source revisions and input manifest digests. Its recorded `running` queue row uses `stage_artifact_fixture` and has no linked collaboration run. It is preserved preparation history and is not evidence of an unsettled Host run.

## Recorded database and process restoration

Recomputing the after-stop snapshot from its recorded rows confirms:

- 13 queue rows: 11 failed, 1 deferred, 1 running; every prior queue ID remains. No active prepare lease or execution binding remains. These retained rows are not claimed permanently inert.
- 23 Host runs, all settled; the 19 prior IDs remain and four new settled IDs were added. No reservation is unreleased.
- 10 complete artifact executions: all 8 prior IDs remain and the two new packaged executions are accounted for.
- One old unknown stage intent remains; no rollback metric was written because the metric path was not reached.
- Gate booleans are restored to false/true/false, with epoch increasing monotonically from 20 to 24.
- The before-stop after snapshot records six other connections; the separate after-stop snapshot records zero other database connections. Neither count is inferred from an empty table.

The stop receipt signals only API87271 with SIGTERM and records both owned API87271/bridge87268 absent, no API64922 listener and zero PAY API suites. New owned Web86953/renderer86966 and retained peer Web39744/renderer39748 remain on their respective ports. No platform signal, forced shutdown, history deletion or manual state rewrite is recorded.

The signal-driven API harness termination is separate from the Playwright case outcome and does not add another business-test failure. This reviewer did not stop or probe any process; the statements above refer to the hash-bound recorded stop/snapshot evidence.

## Handoff

R20 source/output identity and owned resource restoration are accepted for continuation. Preserve this run's first complete pass, second original failure, two unstarted cases, sealed inputs, raw reports, queues and unknown intent. Continue the existing R21 QA-form diagnosis using a new bound scope and output names. No full S2 or parent status is promoted by this after check.

Only this report and its JSON companion were written. Lint, type-check, tests, DB/API/browser calls, builds, installs, signals and Git operations were not executed by the checker.
