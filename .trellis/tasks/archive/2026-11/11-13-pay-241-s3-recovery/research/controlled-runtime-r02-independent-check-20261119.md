# Controlled runtime R02 independent check — 2026-11-19

**R02 NOT_ACCEPTED as a whole.** Actual reporter results are **2 PASS / 2 FAIL**, retry 0. The ten planned subcases contain **4 PASSED / 2 FAILED / 4 NOT_RUN**. The reached round has **six controlled Provider turns and eight original human inputs**, not the planned ten/twelve. Both withdrawal tops pass their complete cases; both notification tops fail on a records-button locator after the first actual failed execution. No cross-round PASS is assembled.

- Reviewer: `/root/notification_runtime_check`, existing native `trellis-check`; no spawning.
- Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`.
- Evidence root: `/Volumes/artisan/.multica-acceptance/pay241-s3-continuation-20261119-uwkcehly` (`B`).
- Frozen source: `B/source/client-harness-runtime-r02`, runtime fixture SHA-256 `b668eab5c0d00969e67871a66865029ed7a41bef6be8fd59603a4f045b085e7d`.
- Owned writes: this report and adjacent JSON only. Prior R01/failed-subscription reports, code, configuration, task/spec/matrix and Git state were not modified. Main executed all commands, services and read-only database captures; the reviewer reran no body or control.

## Findings (fixed)

### R02-F01 — R01's exact persisted refusals explain the initial issue pause

R01's original HTTP bodies remain **NOT_RETAINED**. Main subsequently captured the four exact existing operation receipts through a read-only query; this is persisted-result recovery, not an HTTP capture or replay.

`audit/runtime-initial-control-receipts-r01.json` SHA-256 `36065a47900b8da21b0119b490ed0e0676ae46543fcd1f0e0d614a130e1a90e7` contains exactly the four actor/issue/request tuples from the R01 report. Each has `kind=resume_queue`, `result_code=issue_paused`, stored status 409, target scope, paused queue, control version 1, protocol 2 and target-set version 2. SQL disk SHA-256 is `478630d04eb355aa3059819d6453b62433f31567801499460dcced0e485b1ce5`; removing its one final LF yields the original recorded SQL SHA-256 `11f3c424ff08e44bd13097778fb45fd24145f28a562451fd15be34fd8beade33`.

Actual `server-green-r02/internal/service/collaboration_control.go:221` refuses target Resume while the whole issue is paused. The owner changed only the authored fixture's initial Resume calls to issue scope and added the explicit helper scope argument/checkpoint name. Later target Stop/replays are unchanged. The R02 actual initial issue controls succeed. No backend permission or scheduling contract was weakened. Per the latest dispatch, the old R01 report remains unchanged; this later receipt evidence supplies its cause disposition here.

### R02-F02 — reached withdrawal identities close the scoped NR-P02 evidence gap

For all four reached withdrawal subcases, the reviewer independently linked the original run to its exact grant, live input, capability and reservation rather than relying on first-row selection. Every checked immutable tuple agrees before/after; the legitimate Stop claim-generation change 1→2 and revoked-grant epoch 1→2 are preserved explicitly.

| Platform / subcase | Original run ID | Original grant ID |
| --- | --- | --- |
| Web author withdrawal | `35d0079f-e579-4748-98d4-80910a02bf17` | `f60aab8f-c68a-43bf-930d-691bc46cc866` |
| Web grant withdrawal | `6a6bce6e-c81c-4c2a-a3c4-c943ee8f5dd4` | `e950ccdd-4758-46e3-b41a-6cd3b3085345` |
| Desktop author withdrawal | `12021e01-b66a-47b5-a40b-2c4ac906d86b` | `296064bc-d262-4286-bd2d-b55b4f2937f6` |
| Desktop grant withdrawal | `2769a83b-b1b5-4ee3-8411-57f4245aff2b` | `728f720f-800e-493e-85dc-37e29eef406b` |

Checks include:

- Exact issue/session/target/channel/batch/group/run and original command/input-event/author linkage; unchanged author/participant/account/lifecycle epochs, sequence and content hash for live and waiting inputs.
- Frozen original grant ID/epoch and resource owner/Agent/runtime/installation/binding/configuration; each public grant revoke request names that run's actual grant and expected epoch. Author withdrawal leaves the independent owner's grant unchanged.
- Actual issued business-capability row exists before and after with the identical original hash and creation identity: live before, revoked afterward. It is not an absent-row `not-null` shortcut.
- Same original reservation tuple persists, live before and released after proof. Original run/attempt/prompt/preparation identity remains; one group input is consumed by its exact run.
- Removed participant epoch advances exactly once; both queued commands remain cancelled, group-null and never consumed. The participant's two original comment GET receipts are content-free. Joining that same user to the separately created second issue does not revive the first issue's participation.
- Each of four runs has ten original-token protected 200 reads followed by two complete passes of ten exact 403 `business_capability_required` refusals: **40 positives / 80 denials**. Paths, methods and token hash match before/after, including original snapshot and memory-base POST replay. No wrong-token, missing-reader or unsupported-route substitute is used.

These satisfy the original-identity checks for the reached withdrawal cases. They do not accept the failed notification tops or the complete Extras-B round; R01 still has no executed identity evidence.

## Findings (not fixed)

### R02-P01 — both notification tops stop at a missing exact records control

Both fail at fixture `:688`, waiting for `View records for {runId}` under the already-focused run disclosure. Web run is `a30b43d1-ba40-4ead-868e-b3b8c08ff197`; Desktop run is `df0ef286-e5ec-4444-9c32-bf396af38cd4`. Actual failed outcome/proof, original-ledger recovery, three correct participant notices and the real run destination precede that failure. The later explicit success and Stop subcases never start, and the failed-result records panel is not accepted. Main's unique fixture owner owns locator diagnosis; no product interface or code was changed by this reviewer.

### R02-P02 — final monitor role labels can lose their previously observed role

The corrected kernel-path monitor captures full executables and descendants, fixing the old truncation. Its final loop can still overwrite `role` with `owned_other` when a latest `proc_pidpath` call fails while retaining the prior full executable. Final role counters therefore undercount known processes. Exact preserved paths and first-observed/lineage evidence recover the actual census without rewriting the original output:

- Ten daemon processes: four initial plus six original-ledger restarts.
- Ten `fixture-provider` process paths: four sampled readiness probes and six actual controlled turns.
- Six run-local CLI paths, each containing its actual original run UUID and parented by its Provider.
- Four native Electron main paths, matching the case lifecycle evidence.
- Three short observed processes have no kernel path and remain explicitly unclassified.

The final file records 77 PIDs and no raw arguments. It has the misleading final 9-daemon/5-Provider/2-CLI/3-native role counts; those values are preserved. Main prepared a separately versioned monitor fix afterward. No observer, old process record or actual launch denominator was relabeled by the reviewer.

## Actual round, original recovery and failure boundaries

Raw stdout SHA-256 is `ee1988baa174d05b7dc9999facf68f61de273d132c0148a7d1228dea7c3312aa`. The exact four tops have one terminal each at retry 0; all source/config/tool drift fields are empty/false. Attached case JSON equals retained case JSON. The two notification first-failure entries are FAILED, their later success/Stop entries NOT_RUN; all four withdrawal entries are PASSED.

All six runs have one authorized physical attempt, actual `host_exit_observed`, retained per-launch terminal observation and acknowledged original proof. Restart recovery preserves identity/proof/outcome-event/hash/launch facts and adds no new run/attempt or Provider turn. Repeated identical permit HTTP requests remain one attempt; a network replay is not a second physical launch.

Both natural failed notification outcomes are actually recorded and obtain their same original content-free outcome ACK after proof. The four withdrawal outcomes are locally staged but refused after authority loss; no original successful outcome ACK is claimed for them. Their original termination proofs still recover with the original termination authority. No unknown/failed outcome becomes success.

The passed withdrawal cases retain immutable NR-P01 per-scope checkpoints on both stages, including all author/controller client closures. No new consent, reenrollment of an old issue, new input, new Claim/Permit or implicit Resume repairs withdrawn authority.

### Six-run historical reservation closure

The notification failures occur before their post-proof audit. Main therefore executed the reviewer's separately requested bounded read-only six-run projection, not a business retry. `audit/runtime-r02-six-run-closure.sql` SHA-256 is `5a364c78e238c8b0e2ea317855f39bfb6d44e97add7eb824449915c64dd4adb2`; JSON SHA-256 is `81b7ca752cfcf90bf76b5e29e143294b52ba1434c9d04804f2186a2513f3836c`. The wrapper reports the exact `runtime_r03` database and `transaction_read_only=on`.

All six expected IDs have their exact run/group/command/grant/attempt/capability/reservation relations. All are settled with host proof, authorized/sealed sequence 1, retained terminal observation, actual revoked-capability timestamp and actual released-reservation timestamp. Frozen runtime/Agent/installation/binding identities match. Therefore the **six reservation rows are retained released history, not six live reservations**. The two natural failures retain claim generation 1/current grant 1; withdrawal runs retain original attempt generation 1 with current run generation 2 and applicable current grant epoch 1 or 2. No bodies, tokens/hashes, nonces or proof/launch JSON were selected.

## Source, configuration, cleanup and recommendation

The owner fixture delta is limited to initial issue-scope Resume. Matching `runtime-typecheck-r06`, nonignored targeted ESLint and exact four-top `runtime-list-r06` all exit 0 with unchanged bindings. Actual execution uses explicit baseURL/API/renderer configuration and the selected hashed executables; it never discovers a vendor account/CLI. Main's current API reports the actual built health identity `b1901a48f`, not a substituted newer Git commit.

API PID/PGID **62496**, port **64940**, deployment `6c22740c-2f22-4c76-b62a-377983342ec4` is stopped by its owned SIGTERM receipt with listener absent. All ten daemon process summaries/restarts and eight client sessions close normally; four native main PIDs exit 0/no signal. The monitor's final alive list contains only held renderer 25647. Main still holds Web 25150 and renderer 25647 at this R02 checkpoint; no all-frontends-stopped claim follows.

Post-stop audit SHA-256 `287fc7a9f4ade6ce048b2bf9c71d6f92243afc2a54154295720c279e10f8cea0` matches the admitted `runtime_r03` database: exact 855 migration entries and 663 unchanged ready/valid indexes, original creation/execution/host booleans restored with epoch 2→10, final connections 0. Actual retained rows include eight commands/comments/batches, six consumed group inputs/groups/runs/attempts/capabilities/terminations/reservations, six private input uploads and 54 collaboration notices. Ordinary inbox/subscriber/dispatch/attachment effects remain zero. Notice setup/removal/result kinds must not be collapsed into a single runtime-result count.

The defined TC-21 runtime indirect-read subassertions are proved by the four reached cases and exact original identities; the full-round acceptance still awaits completion of notification presentation and failure→fresh success→Stop. No matrix row is changed here. TC-22 successful-result/Stop notification subassertions remain NOT_RUN in R02. Real models, installed full Main, Agent-output upload, arbitrary descendant containment and all wider S3 requirements remain outside these controlled results.

## Verification

- Actual tests: **2 PASS / 2 FAIL, whole round NOT_ACCEPTED**, retry 0.
- Subcases: **4 PASSED / 2 FAILED / 4 NOT_RUN**; six turns/eight inputs observed, not ten/twelve.
- Scoped NR-P02: four original run/grant/input/capability/reservation matrices and 40→80 read/denial inventory independently pass.
- Original recovery: exact stored requests, hashes, proof/launch identity and no extra turn/attempt reconciled.
- Catalog/closure: actual six released reservations, all settled proofs, source/configuration/ledger/index identity, owned shutdown and connections 0 verified from retained Main captures.
- Lint/type-check/list: Main's matched exit-0 statics inspected; reviewer reran no command/body/control.
- Remaining: notification locator/presentation and later two-by-two notification subcases; monitor-label limitation preserved. Prior R01/ordinary-code/subscription failures and full S3 statuses remain unchanged.
