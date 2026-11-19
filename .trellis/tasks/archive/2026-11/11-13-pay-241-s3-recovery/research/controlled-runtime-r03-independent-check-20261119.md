# Controlled runtime R03 independent check — 2026-11-19

**R03 ACCEPTED for the complete controlled Extras-B round.** The fresh Web/Desktop run has **4 PASS / 0 FAIL / 0 SKIP / 0 flaky**, retry 0, and **10 PASSED / 0 FAILED / 0 NOT_RUN** subcases. Independent reconciliation establishes **12 original human inputs, 10 intentional runs, 10 physical controlled Provider turns and 0 recovery launches**. There are **zero blockers within this round's defined scope**. The earlier R01/R02 failures remain failed records; this verdict uses R03's own complete evidence.

- Reviewer: `/root/notification_runtime_check`, existing native `trellis-check`; no spawning.
- Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`.
- Evidence root: `/Volumes/artisan/.multica-acceptance/pay241-s3-continuation-20261119-uwkcehly` (`B`).
- Frozen source: `B/source/client-harness-runtime-r03`; runtime fixture SHA-256 `1669c8356a4cc01720753e5df7280e76fdcf58b19e66da620748fe5ab6c3959c`.
- Owned writes: this new report and adjacent JSON only. Main ran the recorded checks, clients, Provider, services and database captures. The reviewer inspected retained source/raw files and recomputed their identities, projections and hashes; no body, control, API, database query or service was rerun.

## Findings (fixed)

### R03-F01 — the owner corrected the exact already-open records locator

The sole fixture delta from frozen R02 is at `e2e/fixtures/collaboration-recovery-runtime.ts:687`: the records button matches `^(?:View|Hide) records for {original run UUID}$`. The destination still must name the original issue and `#run-{UUID}`, and the same disclosure must receive focus. The existing `aria-expanded` guard retains an already-open panel. The fixture then requires a visible Run records region, `Direct launches: 1`, truthful `Standard host execution`, and the actual failed/success result text.

This closes R02's two records-control failures without changing product code or relaxing run identity. The initial issue-scope Resume correction is retained from R02; subsequent target Stop and its exact original replay are unchanged. The reviewer made no source correction. The old R02 report remains **NOT_ACCEPTED**, with 2 PASS / 2 FAIL and 4 PASSED / 2 FAILED / 4 NOT_RUN subcases.

### R03-F02 — fresh original-identity evidence closes the runtime acceptance omissions

All four withdrawal subcases independently bind the original run, live input, grant, actual issued business capability, attempt and reservation. Their immutable tuples agree across pre-withdrawal, post-withdrawal and recovered snapshots. Each public grant-revoke request selects that run's original grant and expected epoch; no first-row substitute is used.

| Platform / actual subcase | Original run ID | Original grant ID |
| --- | --- | --- |
| Web failure | `bbc54544-08e5-4c97-b65b-f0457d49ef00` | `fce90db2-0142-4f62-8ebe-1002f2fe9d1d` |
| Web fresh success | `e38d2da5-08f2-47f5-80ba-f74da99cb311` | `fce90db2-0142-4f62-8ebe-1002f2fe9d1d` |
| Web explicit Stop | `4f220473-1d7b-48fd-9286-863b145ab7ed` | `fce90db2-0142-4f62-8ebe-1002f2fe9d1d` |
| Web queued-author withdrawal | `5daee610-92cc-49e0-8348-c1949b928d07` | `a31028fb-9d32-42f1-b9ae-c3973c7d7818` |
| Web live-grant withdrawal | `1efcd391-0f72-434c-9440-e7cf6aa58f9a` | `da683fc8-67e8-4138-9c99-757cf65913e0` |
| Desktop failure | `3a216948-a089-41d3-9c1c-6903b0a7b7e9` | `eb3fa76e-d123-45fc-b64d-1be31de516b8` |
| Desktop fresh success | `e0faf278-f0a4-4702-902d-2e8a50b28b2f` | `eb3fa76e-d123-45fc-b64d-1be31de516b8` |
| Desktop explicit Stop | `ea65eb39-8c7d-491e-adc7-6ccdf80da4a7` | `eb3fa76e-d123-45fc-b64d-1be31de516b8` |
| Desktop queued-author withdrawal | `d21a3250-fbb6-4e19-800b-7e93d725d58e` | `310a2c8a-9dd2-4bca-981a-6b1b8fc7f821` |
| Desktop live-grant withdrawal | `c8d4c7f2-fba1-4989-bdd7-06251b967201` | `66db0102-d7a5-40f3-89a6-543ca87bae26` |

Checks preserve exact issue/session/target/channel/batch/group/run identity, original command/input-event/author identity, sequence/content hash and participant/account/lifecycle epochs. The actual original capability row exists with the same token hash and creation identity: live before, revoked after. The reservation keeps its original execution workspace/Agent/runtime/owner/grant/binding tuple and is released after proof. Claim generation legitimately advances 1→2 after Stop; the original attempt remains generation 1. Grant withdrawal advances only the selected grant epoch 1→2; author withdrawal preserves the independent owner's grant.

The two original waiting inputs remain cancelled, group-null and unconsumed. Each removed participant epoch advances once. Rejoining the user to the separately created second issue does not restore participation in the first issue. Original removed-author comment receipts are content-free, while the exact original termination authority continues to recover proof. No consent, new input, Claim/Permit, reenrollment or implicit Resume repairs lost authority.

## Actual result, notification and late-result evidence

Each notification top executes **failure → distinct explicit fresh success → explicit Stop**, in that actual order. Its three original requests/commands/runs remain distinct. All six executions require an unknown/unsettled run and **zero notices for that run** while its genuine termination proof is held. After committed proof, each original result event produces exactly one notice for each of the creator/resource owner, handler and participant; the outsider receives none. Each notice binds the exact issue/run and one common original event sequence.

The notification tops therefore contain **18 result notices**: three recipients × three proven outcomes × two platforms. Across all four tops, the retained audit contains **28 result notices: 6 `ai_failed`, 6 `ai_completed`, 16 `ai_stopped`**. Its **74 total collaboration notices** also include setup, comments and controls; 74 is not the result-notice denominator. Original notice IDs/recipients/events remain unchanged through receipt replay and recovery. Both clients actually follow each result notice to its original focused run and visible records panel.

The reviewer independently compared **12 complete before/after audit pairs**: six original input/Stop replay pairs and six late-result pairs. Every selected `database`, `issue_ids`, `rows` and `counts` projection is identical within each pair. In all six settled notification snapshots, every earlier row remains an exact member of the final corresponding table. This preserves old failed/successful command, run, attempt, event, notice, upload and operation facts while later explicit runs are added. The executed source separately asserts the complete original records response and original notices for each prior run remain unchanged during later subcases.

All four recorded natural outcomes replay their original request to a content-free 200 `{code: outcome_recorded}` ACK. Changing that already-recorded outcome returns **409 `outcome_conflict`**. All six notification fresh late events and all four withdrawal late events return **403 `business_capability_required`**. The Stop/withdrawal locally staged outcomes are rejected and carry no claimed original successful outcome ACK. No late refusal creates a comment, attachment, result notice, dispatch, claim, permit, attempt or launch. No failed or unknown outcome becomes success or replaces a fresh current result in the UI.

## Current permission and recovery evidence

Each of four withdrawal runs uses its same genuinely issued original business token for ten successful protected reads before withdrawal, then repeats those exact paths/methods twice afterward: **40 positive 200 reads → 80 exact 403 `business_capability_required` denials**. Protected paths cover context, original preparation, input-file bytes/version, comment page, original comment content/revision, memory, live handoff, original frozen snapshot POST replay, original memory-base POST replay and a genuine handoff source chunk. Original token hash, supported path, original request and source identity agree; no wrong-token or unsupported-route control substitutes for a denial.

Author/grant withdrawal commits while the original Provider is still live. The unknown run and original proof barrier remain until actual host exit. Removed-author recovery establishes a new authenticated socket, shows the access-denied UI, removes the private issue heading and obtains no restored issue subscribe ACK. The second denial pass follows real recovery. The private content cannot be rehydrated with the withdrawn original business authority.

All **10 runs** settle with `host_exit_observed`, authorized/sealed attempt sequence 1 and exactly one physical launch. Each original private ledger preserves run identity, original proof receipt/hash, staged outcome event/hash, launch/terminal facts and cleanup. A different daemon PID recovers that same ledger, and the original proof request receives its same accepted receipt. All ten recovery checkpoints add **zero turn/attempt/launch**. An identical permit HTTP replay can appear twice among transport facts while still authorizing one attempt and one physical launch.

The complete retained business denominator is **12 comments, 12 commands, 10 groups/runs/attempts/capabilities/reservations/group inputs, 6 grants and 6 human input uploads/references**. Commands are **2 failed / 2 succeeded / 6 stopped / 2 cancelled waiting**. Runs are **2 natural failure / 2 natural success / 6 forced stopped**. All ten capability rows are revoked and all ten reservation rows have actual non-null `released_at`; these are retained released history. Ordinary dispatch, ordinary attachment, source inbox and source subscriber effects remain zero.

NR-P01's immutable per-withdrawal-scope client checkpoints preserve both stages' actors and closure. All **8 client sessions** close without page/cleanup errors. The four native Electron main PIDs are **72215, 72238, 72388 and 71783**, each exit 0. The human input uploads are not Agent-output uploads.

## Findings (not fixed)

### R03-P01 — mutable final monitor labels differ from original observation roles

`runtime/runtime-processes-r03/final.json` retains **73 observed PIDs**, no raw arguments and final alive `[]`. Its final role counters label 11 controlled Providers, 3 readiness probes and 11 daemons. The last process sample can lose the readiness/daemon guard flags, so those mutable counters cannot establish physical turns. Original `first_observed` events, retained full kernel paths and actual restart histories establish:

- **4 readiness probes**.
- **10 controlled Provider turns**.
- **10 run-local CLI PIDs**, whose ten unique executable run UUIDs exactly equal the ten case runs; each CLI's parent is its actual controlled Provider.
- **14 daemon PIDs**, four initial plus ten original-ledger restarts.
- **4 native Electron mains** and the **1 owned API**.

The original outputs and labels remain unchanged. This is an observer-label limitation outside the reviewer's write scope, not an eleventh executed turn or a missing proven launch.

### R03-P02 — one short observed PID has no kernel executable path

PID **71547** is the sole truly null-path short observed process. It remains unclassified. The proven Provider/CLI/daemon/native identities above have their actual retained paths and matching case histories. No executable identity or extra turn is inferred for 71547. This sampling limit does not prevent acceptance of the ten independently reconciled launches.

No bounded R03 blocker remains. Wider S3 capability and final integration decisions belong to Main; this report does not archive S3 or change the parent matrix.

## Source, resources and service closure

Raw reporter stdout SHA-256 is `51eff997718ef1d8ff49a99494a9bbd7b7c4283dd75e069417ea299bdb91c21e`. All four tops have one passed retry-0 terminal. Each retained case JSON exactly equals its corresponding reporter attachment. The four verification bindings have empty source/tool drift and false config drift; each complete **257-entry source-before/source-after/current** manifest matches. Execution uses the recorded explicit API/Web/renderer configuration and selected hashed controlled executables.

API PID/PGID **70147**, port **64940**, deployment **`8ae4cdfc-092a-4e0d-a8a4-36b9bdc45375`** reports the actual built health commit **`b1901a48f`**. API binary SHA-256 is `0c8ca9898c5b022df40bf2230849c956ac2c14ee5fe59d4d87b02f805306f999`. The built health identity is not replaced with the newer ordinary-notification Git commit `8b1f3dcf6`. Each case binds its actual source/execution workspace, actors, runtime, installation and binding.

`audit/runtime-extras-b-after-r03.json` SHA-256 is `53e717710e9806713be28e88251d19489a9fc241e59d9ea7f7340318157e0db3`. The exact database `multica_pay241_s3n_20261119_uwkcehly_runtime_r04` has zero other connections. Its **855-entry migration ledger and 663 ready/valid index definitions** are unchanged from `audit/runtime-database-admitted-r04.json`. Original booleans restore to creation=true, execution=false, host=false, with monotonic execution epoch **2→10**. Setup and historical rows are retained, not deleted to produce zero counts.

Main stopped only the owned API 70147, Web 25150 and renderer 25647 with their exact SIGTERM receipts. Final owned ports **64940/64941/64942** have no listener. The old unrelated frontend ports **64931/64932** are explicitly untouched. The monitor final SHA-256 is `3977c37cca59d1272fc34fe47bab7ed6c5cda2819df6f2f8b87ff97f521f42da`; final listener audit SHA-256 is `72d59d01c0200d4c34d8ecd5dc46143fa515417f543c458ee4f0e8bdcae943c5`.

## Defined parent-case recommendation

The exact parent CSV outcomes and the agreed [remaining acceptance map](s3-remaining-acceptance-map-20261119.md) were read. **Recommend PASS for both defined TC-21 and TC-22** using the independently accepted union below. Neither case has a concrete unproved required subassertion remaining within its agreed implemented flows. Main owns the actual CSV edits.

| Accepted reference | Relevant proved scope |
| --- | --- |
| **A** — [Extras-A R05](extras-a-r05-independent-check-20261118.md) | 6 tops / 18 subcases: typed human/Agent collaboration mentions and original events/notices, human attachments and source/participant withdrawal, and surviving independent read-basis controls. |
| **N** — [ordinary notification GREEN](ordinary-notification-green-independent-check-20261119.md) | Normal/race each 35 tops / 57 names: literal original nonzero-comment fanout/replay identity, exact recipients and controls, preserved read/archive/legacy rows. |
| **R05** — [ordinary subscription R05](ordinary-subscription-r05-independent-check-20261119.md) | Fresh 2 tops, retry 0: actual Subscribe, first ordinary unkeyed comment, original notice/read/anchor, physical reconnect/reload, Unsubscribe, distinct second comment and nondelivery. |
| **R03** — this report | Fresh 4 tops / 10 subcases: exact current-authority indirect reader denials and original proof recovery, complete actual failed/successful/stopped result notices, original event/recipient/run/records/replay identity. |

**TC-21 — `跨工作空间任务附件及Agent间接读取均依当前权限隔离`: PASS recommended.** A supplies the human cross-workspace attachment/source/participant controls and surviving read basis. R03 supplies the previously missing actual run credential denominator: ten formerly authorized protected routes for each of four original tokens, author/live-resource-grant withdrawal and post-recovery denials, while original proof remains recoverable. The matrix's named remaining real-runtime indirect-read gap is closed. A's held authorized response was cancelled at the access fence; it was not delivered late to the renderer. That retained timing limitation does not add an unproved required flow to this defined case. No new Agent-output upload path was exercised or introduced; the separate S3 R02 output-writer requirement is not silently counted as passed by TC-21.

**TC-22 — `人员Agent提及和订阅通知正确且同事件去重`: PASS recommended.** A establishes typed human/Agent mentions and original collaboration-notice identity. N establishes ordinary `new_comment` event convergence for the same persisted nonzero comment UUID under sequential/concurrent/fresh-bus replay with current active subscriber controls. R05 supplies the missing actual ordinary subscriber delivery/read/reconnect/unsubscribe journey. R03 supplies the missing actual proven runtime result notices, frozen recipients, original event/run destination, no pre-proof delivery and no extra notices from original/late replay. The matrix's two named omissions are closed.

These PASS recommendations apply to those defined cases and admitted flows. N does not promise deduplication for every generic event, ordinary `mentioned` writer, mixed old/new server, or automatic ordinary-composer submission; those were expressly outside its incremental guarantee and are not newly added parent gates. Real-model timeout/quality, installed production Main, Agent-output uploads, ordinary UI CAS and complete S3 R01–R09/final integration remain separate acceptance work. **Full S3 and full PAY-241 remain open.**

## Verification

- **Lint: PASS** — retained `runtime-eslint-r07`, nonignored selected fixture/spec, zero warnings, exit 0; raw bytes and bindings independently verified. Reviewer did not rerun it.
- **TypeCheck: PASS** — retained `runtime-typecheck-r07`, selected TypeScript profile, exit 0; complete source/raw identity verified. No repo-wide TypeScript claim is made.
- **List: PASS** — retained `runtime-list-r07`, exactly four intended tops, exit 0.
- **Tests: PASS** — retained fresh `runtime-extras-b-r03`, 4 tops / 10 subcases, retry 0, no failure/skip/flaky. All four case/attachment pairs, original identities, protected-read denial tuples, immutable replay/history projections, ledger recovery, process census and cleanup were independently reconciled.
- **Reviewer artifacts: PASS** — only this new Markdown and its adjacent JSON were created; old failed reports, source/config/spec/CSV/Git state were not rewritten. JSON records the artifact fingerprints and Markdown hash.
