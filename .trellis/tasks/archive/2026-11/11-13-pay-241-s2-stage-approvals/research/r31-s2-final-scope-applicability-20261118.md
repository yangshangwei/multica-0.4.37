# Research: R31 S2 final-scope applicability

- Query: R31 update to the canonical12 A/18 W/12 Q applicability and retained whole-scope evidence before final A12.
- Scope: internal; documents and bounded frozen-source files only; current runtime result/diagnostics/commands not read in this slice.
- Date:2026-11-18 (Asia/Shanghai).
- Active task:`.trellis/tasks/11-13-pay-241-s2-stage-approvals`.
- Output: this NEW Markdown/JSON pair only, sealed0444.

## Findings

**Retain the source/default/raw-bound original116 service,99 handler and54 router scope, and add the accepted R31batch-only37-name-per-mode evidence. A blanket whole-suite rerun is not justified by this delta.** Final S2 remains open: ROOT now reports current original four **3PASS/1FAIL/0NOT_RUN,retry0**. E04 Desktop completion and Web original-key recovery passed573–583; the later policy-setting call atspec587/fixture356 returned409. This inventory records that steering without reading current runtime files or attributing a native cause. One final independent full42-row A12 review is still needed after that actual failure and final runtime/after evidence are resolved.

### Factual evidence chain and scope retention

1. Original actual normal/race source-bound groups remain116/321/263 service (37/128/95+79/193/168),99/326/275 handler and54/208/174 router. Their exact named/raw/default proofs remain in the canonical R20/R24/R29 inventories; this report copies compact result/raw bindings without duplicating the full test-name matrix or execution.
2. Accepted R22capability,R26candidate,R27binding/UTC,R28preview and R29receipt/default supplements remain attached to their original scopes. R29’s34original-function/3false-body/caller/default proof and76service/6handler names remain valid for unchanged completion/authority paths.
3. R30 changed only test-only read diagnostics and its fixture: product0,11 normal/race names, original context/body/header/default/limits retained. Its original failed setup/native/business histories are preserved.
4. Accepted R31private GREEN `d63f8ed8db89091cb52be9f6cfd957ebaf5b3e3758054207e76f656990d46283` proves only batch ClaimTasksForRuntimes changes, plus one scalar SQL/generated bool getter and one error control. Other5051RED files/modes and original six-name regression remain equal. The old positive callback, outside-batch/singular/default functions and canonical queries remain unchanged.
5. Current R31 normal/race each pass19top/34named/29leaf service,1/1/1 temporal handler and2/2/2 cache controls: **37distinct package-qualified named tests per mode**, with exact source/permit/fresh-package/once-PASS proof in d63f8. Temporal handler reaches real batch finalization/credentials/generation/lease/started behavior. These checkpoints do not add named or business cases.
6. ROOT’s selected five integration and five matching program builds `eba476b5e65245524d1e9195c3cdd8d2d22c75b20be32a49a30b09839ab43b85` bind5055source and retained R24FRONT3359inputs/1961links/1676outputs/buildID. Preparation is not final runtime/full-workspace acceptance.
7. Current four content plus currentCLI/source/live-after/owned-stop/history/artifact/DB/permissions/default reconciliation and independent full42row A12 are the remaining acceptance layer. Current3/1/0 does not satisfy all-four-PASS.

The old batch no-work/error branch is intentionally different and is discharged by the accepted R31controls. Retention applies to the original positive/shared/singular/completion/authority/schema/transport/receipt contracts, rather than pretending every old code path is byte-identical. No additional unbound code dependency appears in the accepted frozen R31slice. Broaden verification only for an actual newly changed caller/default/type/schema/finalization/side effect or source drift.

### Actual batch-claim dependency chain

- **Managed grouping → batch client** — claimManagedTasks groups runtime IDs by managed workspace and calls group.workspace.client.ClaimTasks with remaining capacity. Source:`server/internal/daemon/managed_transport.go:243,253`.
- **RPC/HTTP transport → client** — Unchanged WS-first/RPC fallback uses batch client; old singular compatibility is not covered as repaired. Source:`server/internal/daemon/wsrpc.go:323,371; server/internal/daemon/managed_wakeup.go:115`.
- **Client → actual canonical HTTP route** — Client.ClaimTasks posts /api/daemon/tasks/claim; router registers h.ClaimTasksByRuntime. Alias/legacy singular routes remain unchanged. Source:`server/internal/daemon/client.go:298,310; server/cmd/server/router.go:1577`.
- **RPC → same batch HTTP handler** — Current daemon/session request context is forwarded into POST /api/daemon/tasks/claim and h.ClaimTasksByRuntime; it does not bypass the service. Source:`server/internal/handler/daemon_rpc.go:66,103,119`.
- **Actual router harness → original production handler** — stageExecutionRouter constructs NewRouterWithOptions with nil Redis; factories retain the supported nil-cache path rather than inventing a harness cache. Source:`server/cmd/server/collaboration_stage_execution_control_test.go:321,331; server/cmd/server/router.go:1316,1320`.
- **Authorized batch handler → changed service** — Existing managed source/workspace/daemon checks produce authorized runtime array; ClaimTasksForRuntimes then runs initial validation/promotion and the new due-only scalar probe. Source:`server/internal/handler/daemon.go:1812,1841; server/internal/service/task.go:3915,3995`.
- **Positive returned claim → unchanged atomic finalizer** — Handler invokes FinalizeTaskClaim and assigns response credentials/receipt only after success; existing temporal handler normal/race reaches this actual path. Source:`server/internal/handler/daemon.go:1911,1936; server/internal/service/task.go:3734`.

Actual runtime/static reachability is distinct from a current traffic witness. The production HTTP and RPC paths use the unchanged handler and changed service, and the current temporal handler tests execute the finalization path on real PostgreSQL. This report reads no current traffic/diagnostics. The old singular route remains a separate unchanged sibling.

### Every affected module in the final review

| Module | R31 treatment and matched evidence | Final scope condition |
| --- | --- | --- |
| Task service batch polling | R31_GREEN_37 structural proof and actual34 service names/mode; only reliable negative/error branch changes. Singular/shared/finalization/outside-batch functions and positive callback retain original defaults. | Bind current complete consumed source; singular idle sibling remains explicitly unchanged. |
| Agent SQL/generated DB queries | One12-line EXISTS/one25-line getter; canonical generation and original query blocks accepted in d63f8. Schema/readiness/migrations/canonical update/capacity/lease/generation unchanged. | Use current query4a661a.../generated2f6233... and ROOT generation/final-source proof; supersede old zero-SQL claim. |
| Daemon HTTP/RPC and finalization | Bound unchanged handler/router/RPC source plus temporal handler1 normal/race actual batch call/token/finalization. Old99handler/54router transport/UUID/auth/body/receipt/mint/refusal contracts. | Final actual API/router/client products and current runtime routing/actor binding; no new HTTP type or route. |
| Managed/password/runtime authority | Unchanged helpers; original source/admission/binding/history controls included in current service union. Current actor/version/owner/workspace/MDT/PAT and exclusive writer order; no authority cache/public read flag. | Current source/credential/runtime identities in four/post-stop evidence; new boolean is not permission. |
| Queued and reclaim schedule caches | Nil/failure control source retained; two actual owned-Redis controls/mode show complete-set/retry scheduling. All-runtime MarkChecked,30-second retry/90-second backstop/5-second margin and nil fallback. | Do not confuse private test Redis with runtime configuration; nil production/harness remains supported. |
| All delivery completion writers and receipts | Accepted R29 default34-function/3body-reduction/76service+6handler proof, R30product0 and R31 unchanged closure. Submit/submission/self/R38/ordinary/batch/create/VCS/revoke/cancel/project/source-child/commit-clock/unknown original contracts. | Complete current E04 operation/grant/body/submission proof and late project-policy failure;37 names do not replace three-mode matrix. |
| Roles/CI/QA/release/rollback/R4/T4/preview | Original116 + accepted R22/R26/R27/R28/R29 read supplements; R31 only removes empty ordinary batch writer fence. All current source/issuer/evidence/artifact/target/human decisions and actual writer defaults. | Current original four content and immutable old artifacts; no stale/private evidence or inferred sign-off. |
| Server/daemon runtime harness and test-only observer | R30 diagnostic11/default forwarding accepted; unchanged R31 harness source and eba476 five matching programs. Original deadlines,ctx,callbacks,diagnostic limits and controlled-source adapters. | Use current result/after records from ROOT only; old native holder unknown and read capture cap cannot name new policy cause. |
| Current CLI/run-local machine boundaries | CLI source boundary retained; current five-build record selects c1a8a4... CLI. Ordinary completion refusal and run-local report/approval identity allowlist. | Current R31 W10 exact CLI/source/env/actor/result/no-effects receipt required; prior R29 binary cannot alone prove current program. |
| Core/Views/Web/Desktop shared front and platform wiring | ROOT current consuming manifest binds3359 FRONT files/1961 links/1676 outputs/buildID; canonical source root unchanged. Strict schemas,source/access epochs,original pending key/body/GET recovery,shared components/platform boundaries. | Current rendered API/WS/native profile/actor/PNG and original four journeys; no new frontend rebuild/test required solely for R31. |
| Schema/readiness/history/artifact/lifecycle | No R31DDL/readiness/default changes; ROOT preparation keeps seven raw domain sets and persistent tasks with one computed freshness expiry. Original schema/CONCURRENTLY/noFK/readiness audits,old failed/unknown/history/archive bytes. | Resolve exact stopped-after/source/artifact descriptors/gate booleans and monotonic epoch plus declared additions. |

### A01–A12: R31-specific applicability

| A | Requirement | R31 source/raw treatment | Final evidence |
| --- | --- | --- | --- |
| A01 | Roles, stable business owner/current handler and authority | Role/owner/handler authority is unchanged. R31 changes neither signer selection nor configuration. | E01 current role/config/history evidence remains required. |
| A02 | Immutable proposals, decisions, single materialization/no dispatch | Proposal/version/decision/materialization writers are unchanged; negative batch preflight grants no enqueue. | E01 exact single materialization and rejected/unapproved no-dispatch evidence. |
| A03 | Exact PR/head/trust/strict CI refusal | PR/head/trust/strict CI and review refusal predicates remain original. | E01 exact failed/missing/mismatched CI/head/trust denial and unchanged stage. |
| A04 | Human reviewer, self/machine/revocation exclusion | Self/machine/revoked/rejoin/human reviewer and final signer clocks remain original defaults. | E01 genuine reviewer identity and negative human/authority boundaries. |
| A05 | Immutable actual run/case/log, QA return/retest history | R31 actual idle/read orders remove only empty batch account exclusion; report/QA/test/run writers and earlier read supplements remain retained. | Current E01 failed run/log/QA-return/retest history and E02 QA evidence, with source/current-R4 identity. |
| A06 | QA/build/release authority and transitive issuer validity | QA/build/release authority and controlled execution defaults are unchanged; new polling context does not approve release. | E02 exact QA/build/release/issuer/deployment evidence; test-synthetic scope remains explicit. |
| A07 | Target/physical conflict, rollback observation and single execution | Rollback proposal/decision/metric/physical target/execution constraints are unchanged. | E02 exact prior deployment/target/metric/approved rollback and one controlled receipt. |
| A08 | Versions, authority/expiry, original keys and late history | R29 immutable GET recovery/version/expiry defaults remain intact; R31 error is not empty success or renewed authority. | E03 historical late fact and E04 original key/body/operation/grant recovery; ROOT reports573–583 passed, raw/content acceptance remains final-checker work. |
| A09 | Stage plus M1 all writers/three modes/R38 and outside consumer | R31 claim/finalization supplement preserves real generation/binding/lease; it does not replace retained all-writer M1/off/observe/enforce/R38 evidence. | Current E04 completion/recovery passed per ROOT, but later project-policy PUT409 precedes final R38 branch. Resolve that actual in-scope failure without replaying completed writes. |
| A10 | Current source privacy and mid-revocation reads/cache | Initial current runtime validation remains before the non-authorizing bool; same-Q R4 coexists only for true-idle. Current source/privacy/cache/GET contracts remain retained. | Current source-denial/private-view/access-epoch evidence plus accepted R31 genuine actor/read/real-writer controls. |
| A11 | Matching production Web/Desktop real four-case verification | ROOT selected five integration/build/current FRONT identities are accepted preparation. Current original-four outcome reported3PASS1FAIL0NOTRUN,retry0 is not4PASS. | Current four raw/content/profile/PNG/actor/API/CLI evidence and resolved late-policy failure; no accepted full journey yet. |
| A12 | Independent complete evidence, scoped diff/after and parent ledger | This inventory updates applicability only. Original proofs+accepted R31+matching programs need one final independent full42-row/current-after audit. | Four current cases all PASS,owned lifecycle/currentCLI/history/artifact/DB/source evidence and final independent A12 acceptance before ROOT bookkeeping. |

### W01–W18: actual writer-entry applicability

| W | Entry | R31 source/raw treatment | Final evidence |
| --- | --- | --- | --- |
| W01 | 协作正式交付submit | Submit current authority/final clock/materialization unchanged; R31 adds no stage or M1 admission. | Retain three-mode submit matrix; current E04 negatives and exact positive immutable submission. |
| W02 | 协作精确submission完成 | Actual submission completion/default M1 consume is unchanged; R29 receipt-read proof retained. | ROOT reports Desktop original completion and Web recovery573–583 passed; final raw/key/body/receipt/content reconciliation remains required, with no replay. |
| W03 | 协作self-result完成 | Self writer/result/attachment/consumer behavior remains unchanged; same-P recovery is not a backend self-positive matrix. | Retain backend three-mode self positives and current E04 self negatives. |
| W04 | R38失能发起人管理完成 | R38 admin/default issuer/organization/disabled creator gates are unchanged. Current E04 policy setup409 occurs before remaining admin branch. | Retain backend R38 positives; current full-case R38/disable/restore composition remains unresolved until the late policy failure is settled. |
| W05 | 普通PUT/旧client/Agent的完成请求 | Ordinary/custom-done/Agent/legacy completion writer and principal gates unchanged. | Retained handler/router matrix plus exact current R31 W10 ordinary refusal; old installed-client limits separate. |
| W06 | 普通Move | Ordinary Move/source semantic invalidation/default writer unchanged. | Retained actual Move/child semantic atomic proofs. |
| W07 | 普通批量更新 | Completion batch committed prefix/key/grant/mixed-root behavior unchanged; this is distinct from ordinary daemon task batch claim. | Retained original99/54 batch completion/refusal/unknown controls; no new full batch suite. |
| W08 | Create/快速创建/comment-to-child/create-as-done | Create/manual-child/proposal/admission remains original; new reliable false reclaim precheck still reaches queued/deferred claims. | Retained creation/no-unapproved-dispatch plus current queued/deferred/capacity controls and E01 materialization. |
| W09 | GitHub/VCS/system状态writer | GitHub/VCS/system signed completion/trust/root refusal unchanged. | Retained signed writer/default proof; real external integration remains S4. |
| W10 | 完成CLI及run-local machine边界 | CLI source allowlist and ordinary completion path are retained, but binary identity is current eba476/c1a8a4 rather than R29cd60ed. | ROOT exact current W10 CLI/actor/env/results/no-effects receipt; E01/E02 run-local report positives separate. |
| W11 | 协作root project move/clear | Root project move/clear completion/exit semantics unchanged. Policy-setter failure is a current Q10 dependency, not evidence of a changed Move default. | Retained move/clear transaction proof and current E04 early project probes; diagnose late UpdateDeliveryPolicy409 independently. |
| W12 | 项目delete-impact和实际delete | Project delete-impact/actual delete ordering/current authority unchanged. | Retain managed refusal/unrelated empty project delete and prior races; no destructive replay. |
| W13 | managed root普通delete与batch-delete | Managed root/parent/mixed batch delete-before-effects and rollback unchanged. | Retained original delete/raw controls; no new matrix solely for R31. |
| W14 | 普通child语义编辑/换parent/project/删除 | Child semantic/parent/project/delete and R29exact-source default remain unchanged. | Retained child mapping/semantic edits/ordinary lifecycle/default checks. |
| W15 | 真正enqueue与queued→started | Actual daemon HTTP/RPC authorized runtime array reaches changed ClaimTasksForRuntimes; false continues queued, true keeps exact real writer/finalization. Enqueue/Start remain original. | Accepted34 service+1 temporal+2 cache names/mode plus retained enqueue/start orders. Bind actual current source/router/client/harness; singular idle sibling remains unchanged. |
| W16 | cancel与reopen | Cancel/reopen/Resume/generation/admin cleanup defaults and historical consumed receipts unchanged. | Retain cancelled/reopened/newscope/current GET history; no original successful completion replay. |
| W17 | root/R38 completion candidates与preview | R27binding/UTC,R26/R28preview/R29receipt reads retain exact source/access/transaction predicates; idle preflight adds no preview authorization. | Retained F06/read supplements and current E04 exact preview/child/build/submission/issued reference. |
| W18 | 活动与原操作回执投影 | Activity/minimal operation/grant projection and Core recovery wire/schema remain unchanged. | Current E04 original operation/consumed receipt/pending retirement equality per ROOT report; preserve old failed/unknown history. |

### Q01–Q12: transaction/source applicability

| Q | Scope | R31 source/raw treatment | Final evidence |
| --- | --- | --- | --- |
| Q01 | Three modes × submit/submission/self/R38 verified and incomplete; enforce missing grant remains refused | R31 does not alter any completion mode/actor guard. Current37 claim-related names do not replace R29all40 M1 mode/writer names or original matrix. | Retained source/raw three-mode writer positives/negatives and current E04 representative composition. |
| Q02 | Owning Tx/before/allowed after, wrong Tx, missing/unfinalized/reuse, NA finalization, original actor/header/result | M1 owning pgx.Tx/guard/Prepare/Finish stays original; new scalar absence read is intentionally before optional reclaimTx, with actual positive callback unchanged. | Accepted R29Tx/default and R31real-writer/idle/error controls; final actual actor/header/body/source identity. |
| Q03 | Last DB clock after receipts/audit, issuer/caller/PAT/source/CI/artifact/target/metrics; real orders distinct from same-Tx seams | New EXISTS uses DB now and original90-second recovery/lease predicates; final completion/approval/issuer/consumer/CI/target clocks remain unchanged. | Current temporal handler both modes plus retained final-clock/writer orders/E03 history. |
| Q04 | Exact immutable submission/candidate/lifecycle/scope/operation/handoff locks and legacy/corrupt refusal | Submission/candidate/lifecycle/original receipt binding remains unchanged; new bool returns no private task/evidence and cannot backfill history. | Retained immutable/refusal/mismatch controls; current actual E04 exact submission/grant/operation receipt. |
| Q05 | P ≠ E without source membership, real issuer authorization, minimal grant, revoked/expired roles and frozen refusal recovery | Current source/consumer/issuer/PAT/MDT/admin checks remain original. Same-Q R31 and same-P browser recovery do not establish backend P≠E. | Retained distinct-principal/minimal/private-denial matrix with current authority identity. |
| Q06 | Read-only source/root preview and self attachment inspection, zero business-write witnesses | Accepted R31 genuine idle/error/blocked76-relation+stage-exit witnesses and exact allowed reclaim generation/lease changes supplement prior72-relation read proofs. | Final current four/after audit must classify real writes and artifact additions; per-call equal snapshots do not prove whole DB unchanged. |
| Q07 | Strict objects/aliases/limits, root/source/viewer/revision cursor, empty/cached/mid-revoke orders | HTTP/DTO/router/aliases/cursors/budgets/FRONT parsing unchanged; actual temporal batch handler re-exercises positive credential finalization. | Old99/54 remains source/raw-bound; new source/caller/default delta alone discharges with R31current controls,not full rerun. |
| Q08 | Lost/unknown commit, original refused/consumed key and current read authority; late external fact history | Probe error wraps/returns before MarkChecked/claim; true reclaim/lost response preserves original execution and generation; grant/operation unknown recovery unchanged. | Accepted one sentinel control/temporal2mode and current completed Desktop/Web original receipt chain; late policy failure cause remains separate. |
| Q09 | Exact864 ledger and readiness dependencies; missing state is unavailable, P2 remains false | Prior R29zero SQL/generated statement is superseded: one EXISTS/one getter added. No migration/schema/readiness/role/epoch/default change; dependencies are a subset of original reclaim columns. | Current canonical generation143outputs/current query/generated hashes plus retained exact readiness/ledger/physical-DDL evidence; no migration rerun solely for R31. |
| Q10 | Policy/project/submit/complete ordering, physical-target uncertainty, human-child independence | Policy/project/submit/complete/physical target and child locking defaults remain unchanged; actual late E04project delivery-policy PUT409 is unresolved current business evidence. | R32 bounded source/CAS/native evidence diagnosis; do not attribute CAS/lock/holder cause from nearby reads or change policy/locks/retries without proof. |
| Q11 | Four exact835/864 schemas plus source shapes/cleanup map; root-owned six raw controls + one immutable supplement,39 assertions | R31 has no DDL/index/FK/cleanup/readiness delta; canonical old six controls+supplement39 assertions stay applicable. | Final current schema/history identity; keep old cleanup/Go1.26/Linux/true external limits declared. |
| Q12 | Immutable inputs/configs/owned cleanup collected | Current five products5055source/retainedFRONT/current run and stopped-after identity are required. ROOT reports one owned stop,0otherconn/gate74/64settled48complete/history unknown1. | Final raw/currentCLI/live-after/stopped-after/source/archive/permission/gate restoration and independent42-row audit; parent24 unchanged. |

All42 canonical row IDs/requirements are preserved from the hash-bound prior inventory. Their current source/raw aliases and exact prior JSON pointers are in this companion JSON. None is marked PASS by this research.42 rows,37current named tests per mode,4original E2E and24parent cases have independent denominators.

### Exact remaining final-check conditions

| Gap | ROOT/final checker requires | State |
| --- | --- | --- |
| F01 Current original-four raw/content acceptance | ROOT deliver current exact four result/reporter/raw/one-start/permit/config/case-local HTTP/actor/profile/PNG; every original case must PASS once/retry0. ROOT reports3/1/0 with successful E04completion/recovery but later policy409; report does not parse current runtime files. | actual current failure remains unresolved |
| F02 Late policy failure/current R38 composition | Resolve E04spec587/fixture356 UpdateDeliveryPolicy409 with exact input/current revision/policy/lock/result evidence; no completed Desktop/Web write replay, no cause attribution or blanket rerun. Remaining R38/admin branch cannot be inferred from earlier success. | R32 diagnosis next; no product authorization |
| F03 Current program/CLI/platform/source binding | Bind exact R31currentCLI W10,API/rendered origins/WS/native account/profile/harness/current five products plus5055source and FRONT3359/1961/1676 before/after identities. ROOT selected five integration is not whole-workspace equivalence. | ROOT final evidence receipt needed |
| F04 Owned stop and stopped-after domains | ROOT reports API35740/bridge35737 already stopped once,0otherconn/gate74/64settled48complete. Final reviewer verifies exact owning stop,absence/listeners/preserved platform processes,selected DB,restored original gate booleans/monotonic epoch and no live lease/bound/unsettled/unreleased work. No additional signals from research. | ROOT attestation available; raw stopped-after audit not consumed here |
| F05 Immutable history/archive/permission identity | Verify old338 descriptor/source/history records and declared381 current descriptor set,old unknown1/new archive/tar/input/spec/receipt bytes,staging/output modes/UID/type/path/inode/source,current complete48 artifacts and every declared addition. Computed freshness is classified without rewriting raw data. | ROOT attestation available; full final audit remains |
| F06 One final independent full-S2 A12 | Combine every42 A/W/Q row and all affected modules from this applicability map with original raw/default chains,accepted R31+ROOT integration/builds,current original-four/resolved failure and after evidence. Broaden tests only if real unbound code/default/schema/caller or failed actual contract demands it. | not accepted by this inventory |

Prior Q09 “zero SQL/generated delta” must be replaced by the R31one-query/one-getter fact; there is still no DDL/readiness/migration delta. Prior A11ROOT3/old program hashes must be replaced by current selectedfive/5055/build/consuming bindings. The new source enters actual HTTP claim through unchanged adapters, rather than a standalone test helper. The updated server R31contract at `.trellis/spec/server/collaboration-stage-approvals.md:128–186` records batch-only scope, reliable negative/error/positive writer behavior and unchanged timing/history.

ROOT now reports one normal stop of API35740/bridge35737,0other connections,gate74/64settled runs/48complete artifacts,old338/current381artifact descriptors,22intents/33observations/18metrics/unknown1. Those facts still need their exact final source/DB/permissions/archive/declared-addition receipts in independent A12; preparation and this steering record do not substitute for that audit. No extra signal or historical write replay is requested.

### Current source and canonical evidence bindings

| Evidence alias | SHA256 | Purpose |
| --- | --- | --- |
| PRIOR_42 | `5a30432b9ceadca7fb5291fd5ae4d783f6a3b6e51f7a3e79f4b3d46142e2dcf2` | `r29-s2-final-gate-inventory-20261118.json` |
| CANONICAL_42 | `8cc9195cd50d415b51ff3552f6dd01abd10f00a370988a17371a6fc94600cacb` | `r24-final-acceptance-preflight-20261117.json` |
| ORIGINAL_GO | `01162af46f6d74d5e3a145a7283e891bd9279cce4005c9193afb5b8ad45f2153` | `r20-final-a01-a12-evidence-map-20261116.json` |
| R29_76_6_DEFAULTS | `662d3113fd5820f3d6771cdbb521ef3be8d5bcdadb62d63d6bdf5bb3e02e7031` | `r29-consumed-read-green-independent-check-20261118.json` |
| R22_CAPABILITIES | `72de7a9a42ca0e1577ad21eb36232acb8278acfcc3af6f59bcffb43db60d8b5f` | `r22-capabilities-read-red-green-check-20261116.json` |
| R26_CANDIDATES | `ebc30fde6b5a456cb6a3681247f1b8311d77606780037fcf671b6124a1f76e5c` | `r26-candidates-read-green-independent-check-20261117.json` |
| R27_BINDING | `ef03680d53d69bef593c7c49072ba522595ab397bf84aaf55c7037fe70bc30d9` | `r27-preview-binding-utc-followup-independent-check-20261117.json` |
| R28_PREVIEW | `d2a5017e76662f94d4490eb3abc88f1d72898caafa9c92616ac1e8b10626419e` | `r28-preview-read-green-independent-check-20261117.json` |
| R28_HISTORY | `661baeba237af31309ec99beefe690cbdeccf425d2e46facfbefebc7e6f4f7a5` | `r28-current-controls-and-runtime-after-independent-check-20261118.json` |
| R30_DIAGNOSTIC | `2191a2eefbb91599473e44c7dc5cea6f02a5669f718509805e1585782dc8a99c` | `r30-read-diagnostics-independent-check-20261118.json` |
| R31_GREEN_37 | `d63f8ed8db89091cb52be9f6cfd957ebaf5b3e3758054207e76f656990d46283` | `r31-idle-claim-green-independent-check-20261118.json` |
| ROOT_FIVE_PREP | `6ac5365d18f395b30d3091af4ca536dbf204317314080b72eb48578d2a3d6b9c` | `r31-root-runtime-preparation-check-20261118.json` |
| CLAIM_ORIGINALS | `aa2dd4e25ab19fddd9ed65718b1da73bc6479cf2eec91270498764570580584f` | `r31-idle-claim-original-control-inventory-20261118.json` |

| Frozen source path | Raw SHA256 | R31 scope |
| --- | --- | --- |
| `server/internal/service/task.go` | `bcfb7399e75040f8eeb809cada849e11095ba1a105b9553d322d450397fdd7bc` | Only batch ClaimTasksForRuntimes changes; outside-batch/singular/shared/finalization remains covered by accepted structural proof. |
| `server/pkg/db/queries/agent.sql` | `4a661a0c1374525177d4cd103b3d47aa39ea67c1a0a32c3eb4bf4067aa6494be` | One non-authorizing necessary-condition EXISTS; canonical claims/reclaim/admission/lease/capacity SQL retained. |
| `server/pkg/db/generated/agent.sql.go` | `2f6233d3d901d96c03f3964b69319932cf8246f65402c2123c1d9a5c2eb7c794` | One canonical generated25-line bool getter block; other142 generated outputs retained. |
| `server/internal/service/task_idle_claim_read_concurrency_test.go` | `b07397c51e304ef51e95d3c3757143600a8a37528b96005e5641df9f765f300b` | Unchanged six-name genuine idle/reclaim regression; ROOT integrated new test. |
| `server/internal/service/task_idle_claim_preflight_failure_test.go` | `1cb4fac1359b96fd29a0b4812a7e461250752b42b1eb7d56e1f750a3c8c8db8a` | One exact sentinel preflight-error/no-authorizing-effect regression; ROOT integrated new test. |
| `server/internal/service/managed_runtime.go` | `325251ecb8a8adeaf3c8c3a97ef53302750fce9a8c163d579087e25f12288e96` | Original current source validation/exclusive writer default/namespace/admission runtime locks. |
| `server/internal/auth/password_session.go` | `69530f6da81e79658f47e28f4db801d651a6603b9fd7bea621aaecf8e393d33c` | Original current source version and password-session account fence. |
| `server/internal/service/reclaim_check_cache.go` | `7fa0f3a8ec6568e4d6bf3abcdc2088ac131f79656715f19fead5ea78b0c8dfe3` | Original due/MarkChecked/backstop/retry implementation. |
| `server/internal/service/empty_claim_cache.go` | `71d40bbf6d1b936532868c6562ad3b3147170f6552f16fdfaf5e408652841430` | Original queued-only empty cache/version behavior. |
| `server/internal/handler/daemon.go` | `239693e1a475a72d3706b565d9a7e219f3fb1f13ec2fd1a2a87ddefba581fa73` | Unchanged HTTP batch handler, response/token/comment preparation and finalization. |
| `server/internal/handler/daemon_rpc.go` | `5fb862e72fff347a5fa15b0c71954608b8467bb49404c4abd5c7796fa967da55` | Unchanged RPC forwarding into the same HTTP batch handler with current context. |
| `server/internal/handler/daemon_ws.go` | `4fffa90be8343fabf1f4a2b4f04310468e169f4493da54296da5961f0b734e4b` | Unchanged daemon connection runtime/current identity path. |
| `server/internal/daemon/client.go` | `ba1b521622c9e6ad2461271ac30ea8517da59c871ad78128421fa0ca310e83ec` | Unchanged machine-level client request and five-second batch deadline. |
| `server/internal/daemon/managed_transport.go` | `0252432eb59fbe39c270b6122e83896ede8e942aca5c1aee2dd3b643755ecf2e` | Unchanged managed workspace grouping and calls to batch client. |
| `server/internal/daemon/managed_wakeup.go` | `d1a3f29fc4a1bcda667e8f4a93925a90d807ea220695841c4943626a85c2ef37` | Unchanged managed WS-first claim transport. |
| `server/internal/daemon/wsrpc.go` | `31ca2ceb3173f1e0278e8ae74b4ac16742af5e3fbfb22e0a0bcb53dbc1f372ab` | Unchanged RPC/HTTP fallback to batch client; singular compatibility remains separate. |
| `server/cmd/server/router.go` | `99b8add17fa3251ec830a0069e8477c67c74a83626a62a829228685142bf4b11` | Unchanged canonical batch route/alias and both cache factories. |
| `server/cmd/server/collaboration_stage_execution_control_test.go` | `5a8dceb725802a80d61438e0f6b1f2055eefffa88b964ab203f6b8a05fca579e` | Original actual router/harness entry with nil Redis; no source R31 harness delta. |
| `server/internal/service/delivery_completion_authorization.go` | `a8445340a055faa12fceaa04e70f62eb213e3aeac333b0bc5abff50520e4f0b0` | Accepted R29 receipt read/default/issuer/current consumer predicates unchanged by R31. |
| `server/internal/service/delivery_completion_collaboration.go` | `50c048f059392bed2f1b5c99cedba605c6fade3facbc28b963fee2f0c586183b` | Accepted R29 exact-source read/default writer predicates unchanged by R31. |
| `server/internal/handler/managed_claim_recovery_test.go` | `e85a824c8f13ef97f5dd4bb7cd65eb5b035252730d427fecc6e1a48d11c41460` | Current actual temporal handler normal/race proves finalization/credentials/generation/lease/started boundary. |
| `e2e/collaboration-stage-approvals.spec.ts` | `f8f0c32f7622ef4cb521bdfd6aec423906d7f955e3b9be7aea940f73d8883e73` | Unchanged canonical R24 FRONT source; full3359/1961/1676 identity retained through ROOT_FIVE_PREP/current consuming manifest. |
| `e2e/collaboration-stage-execution.spec.ts` | `6d08bb644417d3c23b579564b7110a6d234773df7e68e187bfa03ab1175573f4` | Unchanged canonical R24 FRONT source; full3359/1961/1676 identity retained through ROOT_FIVE_PREP/current consuming manifest. |
| `packages/core/collaboration/operation.ts` | `b9cd10c98e42be139046d8c672643e3d969df2694965dc66206947555a05b79c` | Unchanged canonical R24 FRONT source; full3359/1961/1676 identity retained through ROOT_FIVE_PREP/current consuming manifest. |

The JSON binds compact original normal/race result/raw references, all six current R31 accepted result/raw references,ROOT selectedfive/binary/consuming references,unchanged source subsets,all42 row pointers and the module/callgraph map. No raw Go stream,whole5055/3359scan or accepted constant-default proof was re-executed or reconstructed.

### Related specs and external references

- `.trellis/workflow.md`: Full-scope final2.2 at568.
- `.trellis/spec/server/collaboration-stage-approvals.md`: Updated R31batch-only probe/current-R4/error/realwriter contract at128–186.
- `.trellis/spec/server/delivery-completion-enforcement.md`: M1 borrowed Tx/original-key/consumer/default all-writer semantics.
- `.trellis/spec/server/managed-installations.md`: Current binding/owner/PAT/admission/history/generation/lease.
- `.trellis/spec/server/database-test-isolation.md`: Task-owned package DB/TestMain and actual evidence.
- `.trellis/spec/server/verification-environment.md`: Final source/command/resource/configuration/JSON inventory.
- `.trellis/spec/core/frontend/index.md`: Core package boundary.
- `.trellis/spec/views/frontend/index.md`: Shared Views boundary.
- `.trellis/spec/web/frontend/index.md`: Web platform boundary.
- `.trellis/spec/desktop/frontend/index.md`: Desktop platform/native identity.
- `.trellis/spec/core/frontend/collaboration-workbench.md`: Shared original-intent/source epoch recovery.
- Local versions/first-party entry points: [sqlc1.31.1](https://docs.sqlc.dev/en/stable/), [pgxv5.9.2](https://github.com/jackc/pgx), [go-redisv9.18.0](https://github.com/redis/go-redis). No external retrieval occurred.

## Caveats / Not Found

- This is an updated applicability inventory,not independent finalA12 acceptance. R31current3/1/0 and after facts are ROOT steering attestations,not a fresh research result audit.
- 37 package-qualified current names per mode do not replace old whole-scope writer/source/DDL/transport matrices or four business cases.
- Retained whole-scope semantics apply to original positive/default contracts; the intentional batch negative/error change is separately bound to d63f8 and current controls.
- No unmatched code dependency found within the accepted frozen R31slice; late E04policy409/current runtime and final after/source/artifact acceptance remain unclosed.
- Original singular idle polling is unchanged. Historical physical holder is unknown; do not attribute it or the new policy409 to the preflight by proximity.
- Current source/ROOT five integration/build/preparation is not a whole-workspace or final live audit. FRONT3359/1961/1676/buildID remains R24bound.
- Only the two new report files written/sealed0444. No successful completion/policy write replay,permission request,archive or parent/status update.
- Parent24 remains **0PASS/9PARTIAL/15NOT_RUN**. R30 stays2/1/1 and R28 stays3/1/0,retry0; successful earlier completion and oldunknown/history/artifacts are retained. No S2/A12/parent/S3–S5/external business sign-off is accepted or archived here.
