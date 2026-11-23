# Research: R22 final restoration and next generation audit plan

- Query: Reuse accepted R20/R21 artifact-history/owned-stop methodology for R22's final after-stop classification and the next fresh eight-slot run.
- Scope: Internal, read-only retained task JSON/case/source records. Main owns process stop, network/DB probes and final filesystem/tar audit. No runtime/network/DB/process/Git commands or codec review by this researcher.
- Date: 2026-11-17
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Findings

### Current stopped evidence and original outcome

Original R22 remains **1 PASS / 1 FAIL / 2 NOT_RUN**, retry0. Its new codec diagnosis and any later codec repair do not alter that result. Source/result/historical artifact identity and resource restoration are separate acceptance questions.

Main's recorded stop evidence is now available:

- `r22-runtime-capabilities-read-r01/runtime/owned-stop-r03.json`, SHA256 `3c5a868684058dc3eb5bbf4394df1d7d26bf8827a60e8ed40c8c725b60b31cb2`: absent owned API32107 and bridge32101, absent API listener/published socket, terminal harness PASS and retained production platforms. It records the earlier signal rather than sending another signal.
- `runtime/database-after-stop-r05.json`, SHA256 `d0398103ea9ec1e2d63ae97ebe71f9e7d0d89992ec4b7f7e5aafbbbc94b1a6df`: other connections0, gate false/true/false epoch28,27 settled Host runs,12 complete artifacts, no unreleased reservation or active prepare lease/binding,15 retained queue rows and one old unresolved stage intent.
- `runtime/owned-stop-preparation-failure-r01.json`, SHA256 `cac801b387c5dce32306e6e9f641d8067bf678235f6df6d6f7e29652d70b7f73`: preserves the first stop preparation failure and the R04 snapshot's actual live classification. That mislabeled snapshot is excluded from the final stopped-state gate.

The subsequent stop attempt sent SIGTERM but its report serialization failed on a nonexistent literal log name. R03 uses the actual configured `logPrefix` and records already observed stop/no repeated signals. Preserve both failures and their scripts; do not count them as additional original business-case failures. This research verified the three exact supplied file hashes; it did not independently probe processes or the database.

R22 `database-after-four-r03.json` is a valid **live after** record, with5 other connections. It cannot substitute for R05 after-stop. The final inode/tar/path-set audit remains main-owned and pending.

### Accepted methodology to reuse

`research/r20-original-four-after-independent-check-20261116.*` accepted independent reconciliation of:

1. Original result/reporter/stdout/stderr, consuming source/link/product bindings and actual after records, preserving the four-case denominator and failure.
2. Every previous filesystem path's type/inode/owner/permissions and each file's size/hash, with only the new assigned generation directories allowed to seal0700→0500.
3. Exact additions: input manifest/file trees and output execution directories/tar files, with no removed history, symlinks or unrelated additions.
4. Real tar size/hash and its single `stage-regression.json` member compared with the sealed input and execution-row digests. No recomputation claim for opaque receipt digests merely from a filename/hash.
5. Retained queue/run/artifact/intent identity sets, all required settlement/lease/reservation predicates, gate restoration, owned-process/port/socket evidence and a separate zero-connection after-stop record.

The exact historical references are R20 `runtime/after-files-history-r01.json` (`76eed561…`), `artifact-filesystem-before-four-r01.json` (`d618bab3…`), `database-after-stop-r02.json` (`5ab003e7…`), and `stop-owned-api-after-four-r01.json` (`60e6fca8…`). R21 `runtime/generation-and-config-preparation-r01.json` (`776c6724…`) supplies the next eight untouched generation IDs and64 older directory inode/mode identities. Its original directories and earlier accepted tar histories remain immutable.

## Exact R22 artifact inputs

Trusted roots are:

```text
/Users/artisan/.multica/pay241-r11/runtime-r01/artifacts/staging
/Users/artisan/.multica/pay241-r11/runtime-r01/artifacts/output
```

R22 `runtime/runtime-input-preparation-r01.json` records the eight slots, initial0700 mode, owner501 and inode. Its IDs equal the accepted R21 allocation exactly; R22 did not allocate a second set. R22 first two slots were consumed:

| Execution | Generation | Expected tar SHA256 | Expected input SHA256 |
| --- | --- | --- | --- |
| `01a1e574-a443-7937-9e8c-52d3be761978` | `710cd971-43eb-4514-aeaa-358879dc54e9` | `ee8e03a8180cc18648b74891cf7ebab384debe3e542b2f9b89d54928cdf0ea4f` | `55d2d6359f855f49df987c08771953ff50f14d6e5f3c186c0b303f56771f53a2` |
| `01a1e574-a49c-7743-a731-042f9cee38c3` | `a7bb6cf9-8100-4f23-a850-7b9c52a6430f` | `60943f59f79eb25da63cdb407dd224b0e2b1eef5fa8c5d55b8b958b21ef9dae0` | `bddee3d37cdfc1617610ee98a73282653a9d8ec801b7e18a0375167ee26fb0ea` |

For each row, actual expected paths are staging `<generation>/manifest.json`, `<generation>/files`, `<generation>/files/stage-regression.json`, output `<execution>` and `<execution>/artifact.tar`. Both tar files are2,048 bytes according to original case2 steps3/4 and the recorded DB rows. Compare against those exact steps' approval/spec/receipt/source/producer fields, not a newly invented artifact.

The first two slot inodes were178740913 and178740914. Final expected directory mode is0500 with the same inode/owner; manifest/input files0400, output directories0700 and tars0600. The other six slots retain0700, their original inodes and empty contents:

`ebd6ee66-34b8-439a-814d-d5b2925bb248`, `62c445ea-3446-463f-82f8-f0cc78180bfc`, `60b742d7-df95-4f16-91bb-323909eb9562`, `767290b4-f2e6-4656-9655-b0db4d76f3e8`, `56cb94ab-a4a9-49a4-b190-93197d5b6584`, `00eb9f2d-470c-4fe5-8518-3d76050fa79c`.

The expected complete pre-R22 path map is R20 retained117 + new10 + R21 eight new directories = **135 unique paths**. This original R22 run adds exactly10 input/output paths, giving **145 unique paths** before a next-run generation allocation. Directory sealing changes mode, not path count. This count is derived from retained records, not a new filesystem traversal.

For main's final traversal, reconstruct the old baseline from R20 `retained_filesystem` and `new_filesystem`, plus R21/R22 slot records. Ignore historical `new_generation_allowed_to_be_sealed` labels: those belonged to R20's old run and do not authorize any older generation's mode/content to change now. All64 older generation directory inode/modes and all prior file hashes/sizes/types must remain exact. The ten older artifact execution IDs and tars must remain present and retain their recorded fields/hash bytes. Preserve older empty output directories too; their legacy naming is not permission to repurpose them.

## Retained DB history classification

Compare row identity sets using task `id`, Host run `id`, artifact `execution_id`, stage intent `intent_id`, stage observation `observation_id`, and rollback metric `observation_id`. Do not use an invented run ID field or demand empty history tables.

Recorded transitions are separate:

- **Before API→live before four:** queues13→14; new deferred task `01a1e55f-fb39-776a-817f-d18c11fe9a9d` belongs to an older `stage_artifact_fixture` producer with no collaboration run. Old task `01a1e491-bd4f-73c8-9a0f-818e0ff51933` changes deferred→failed with `runtime_reconnect_timeout`; old R20 producer `d27bbd39-745e-4fb6-8326-10541b201fdc` changes running→failed with `runtime_offline`. Preserve these recorded startup recovery facts rather than calling them this case's leaks or manually restoring old statuses.
- **Live before→after four:** queues14→15 with new running synthetic ordinary producer `abe879c4-0183-4c99-a1a8-4aaf90686e23`, matching case2 artifact steps3/4, provider `stage_artifact_fixture`, source child `01a1e574-a2bc-7897-968e-420abf816d19` and no collaboration run. Host runs23→27, artifacts10→12, intents2→4, observations3→6 and metrics0→3; no old IDs are removed.
- **After four→R05 after-stop:** queue/artifact/intent/observation/metric identity sets and task statuses are unchanged. Host run IDs must similarly remain27 settled. The actual other-connection predicate changes5→0.

The new deployment and rollback intents/three observations and three metrics are retained real controlled-source evidence. The final rollback committed despite the client codec failure; it must not be marked unknown or removed. **One older unresolved intent remains**, as before this run. Preserve its exact original intent ID and observations independently of the new terminal rollback. The collector explicitly does not claim full original RPC-frame capture; do not upgrade selected-row equality into full request/receipt reconstruction.

Queue rows are retained history, not permanently inert declarations. Require current leased/bound work0, Host unsettled0, unreleased reservations0, incomplete artifacts0 and owned connection/process restoration; do not delete the running synthetic producer or the old deferred/failed rows merely to make counts zero.

## Main-owned final acceptance checklist

1. Keep original result `a6a9b58f…`, both case evidence files, raw stdout/stderr/reporter, binding/permit and gate history unchanged. Reconcile `consuming-files-before-four-r01.json` versus `consuming-files-after-four-r01.json` and their exact accepted input/product references; this is consuming-source identity, not whole ROOT equivalence.
2. Use stopped R03/R05 as the final resource evidence; retain/exclude mislabeled R04 and both stop-preparation/report failures. Confirm no repeated/foreign signals and preserved production/peer platforms from actual records.
3. Traverse only the two trusted artifact roots without following symlinks. Compare the135 prior paths plus exactly10 allowed additions, two allowed slot-mode seals and unchanged64 old directory identities.
4. Read the two tar archives in memory, with no extraction. Require the single regular `stage-regression.json` member, expected content hash/size and exact sealed input bytes; compare each manifest's source/task/resource identity and each output digest/spec/receipt binding to original case/DB evidence. Inspect all prior retained file identities too.
5. Recompute all history/settlement/lease/reservation/gate predicates from R05's recorded rows. Retain15 queues,27 settled runs,12 complete artifacts,4 intents/6 observations/3 metrics and the one older unresolved intent. No manual cleanup/status reset.
6. Save a fresh exclusive audit report with exact references and declare only restoration/source/artifact-history acceptance. Full S2/A01–A12 and the original four-case result remain incomplete.

## Next fresh eight-slot allocation

Finish and bind R22's filesystem/tar audit before allocating the next eight slots when possible. With an already recorded new allocation, list those eight added directories separately as next-run preparation; do not mislabel them R22 execution effects.

With owned API/bridge stopped, main may temporarily grant write permission to only its own staging root, create eight completely new UUID directories as owner-matching empty0700/non-symlink paths, and restore root0500 in a guaranteed cleanup path. Capture prior inode/type/owner/mode/file hashes first and prove them unchanged afterward. Never alter old sealed manifests/files or reuse this run's two consumed slots. Retain the six unconsumed R22 slots and every old tar/generation even though the next run's configured list is replaced with new IDs.

Configure exactly the new eight IDs, a new suffix/log/result/permit binding and the accepted new test input. Keep constructor identity/roots, retained DB history and blank model/account configuration unchanged. Output execution directories are created by the packager under **execution IDs**; do not require or pre-create output paths under staging IDs. The preserved old unused output directories are historical preparation, not part of the new eight-slot contract.

Expected next preparation path count is145+8=153 if no other declared task-owned additions occur. It is not a claim of a completed next run or permission to replay a sealed generation/original unknown execution.

## Exact input files / related specs

The JSON companion binds R20 accepted after/stop evidence, R21 generation preparation, R22 runtime-input slot record, before/live/after/stopped DB snapshots, owned stop/failure records, original result/case evidence and before/after consuming-source records. Stage and database verification specs require preserved history, explicit owned resources and separate actual case/restoration conclusions.

No external references were needed. No full environment, DB URL, credential or bearer value was printed or copied.

## Caveats / pending resource questions

- Final inode/path-set/tar validation has not been performed by this researcher;145 is a record-derived expectation.
- Main's fresh stopped-state records are verified file evidence, not new process/network/DB observations by this helper.
- Native socket/ready cleanup can intentionally retain the ready JSON while removing its published socket; use the stop receipt's actual contract rather than demanding all files disappear.
- A future heartbeat freshness change or later queue transition needs its own observed/source-bound explanation; retained queues are not asserted indefinitely inactive.
- Codec review, W10 execution, new generation mutation and source integration remain other owners' responsibilities. Only this plan and its JSON were written.
