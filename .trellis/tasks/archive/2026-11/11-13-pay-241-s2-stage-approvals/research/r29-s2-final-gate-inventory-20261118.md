# Research: R29 S2 final-gate inventory

- Query: 为最终 S2 A01–A12/W01–W18/Q01–Q12 提供 canonical maps、accepted R29 与 retained 原路径的 applicability/source-default 依据、原第四场景及正常停止后审计清单。
- Scope: internal；filesystem-only 读取既有接受报告、冻结源码、CLI收据和 canonical 矩阵；仅此新 Markdown/JSON 对可写，最终0444。
- Date: 2026-11-18 (Asia/Shanghai).
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Findings

**本报告准备最终 A12 复核，不接受 S2。** 派发时 ROOT 指明 original4 在 `/Volumes/artisan/.multica-acceptance/pay241-r29-receipt-read-20261118-6dhi01s2/runtime/four-scenarios-r01` exactly once 运行；本研究未读取该轮终态或启动/重跑/停止它。ROOT 收集当前 outcome 和 post-stop；以下 pending 表示尚未纳入本报告的终态接受，并非断言对应文件不存在。R28 保持3PASS/1FAIL/0NOT_RUN,retry0；成功原 Desktop 完成不能重放，historical physical409 holder仍UNPROVED。

### Canonical 路径与不可混用的分母

| Canonical路径 | 职责 |
| --- | --- |
| `.trellis/tasks/11-13-pay-241-s2-stage-approvals/prd.md:30` | 权威A01–A12、当前真实主体/原M1边界、synthetic与真实S4/S5区别。 |
| `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/r24-final-acceptance-preflight-20261117.md` + `.json` | 完整12 A、18 W、12 Q；JSON的 `criteria/writer_map/transaction_map` 是本报告逐行来源。旧 runtime 标签仅历史。 |
| `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/r20-final-a01-a12-evidence-map-20261116.md` + `.json` | 原实际service116/321/263与handler99/326/275、router54/208/174的独立来源证明；旧W10缺口标签已被后续收据替代。 |
| `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/r28-current-controls-and-runtime-after-independent-check-20261118.md` + `.json` | 实际原控制58、R28部分第四场景/3-1结果、正常停止与历史制品恢复已独立接受。 |
| `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/r29-consumed-read-green-independent-check-20261118.md` + `.json` | ACCEPTED两产品delta、34原函数/3false body reduction、当前service76/directGet6 normal/race。 |
| `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/r29-root-runtime-preparation-check-20261118.md` + `.json` | ACCEPTED ROOT3/five matched products/5052 backend及原FRONT消费绑定；preparation≠live/fullfour。 |
| `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/r29-grant-control-inventory-20261118.md` + `.json` | 原handler5/router3及directGet3的确切named/leaf/selector和覆盖限制。 |
| `docs/plans/2026-11-18-pay-241-continuation.md:12` | original4 →42-row复核→normal-stop/history/artifacts→independentA12→S3；ROOT拥有进度与状态。 |
| `.trellis/tasks/11-13-pay-241-collaboration-acceptance/test-matrix.csv`、`gaps.md`、`roles-and-states.md` | 父24当前账本及业务角色/外部缺口，由ROOT更新；本研究读得0PASS/9PARTIAL/15NOT_RUN。 |

所有计数以下统一 **top/named/leaf，每模式**。42个A/W/Q行不是42个新业务案例，4个original E2E也不改变父24。旧service两个partition是 **37/128/95 +79/193/168 =116/321/263**，不能根据named减top推算leaf。

| 已接受组 | 包 | top/named/leaf | 当前使用 |
| --- | --- | ---: | --- |
| joint_completion | `./internal/service` | 37/128/95 | retained，实际命令与两模式结果在JSON |
| joint_foundation | `./internal/service` | 79/193/168 | retained，实际命令与两模式结果在JSON |
| handler | `./internal/handler` | 99/326/275 | retained，实际命令与两模式结果在JSON |
| router | `./cmd/server` | 54/208/174 | retained，实际命令与两模式结果在JSON |
| R27_binding_UTC | `./internal/service` | 2/8/6 | retained，实际命令与两模式结果在JSON |
| R28_preview_union | `./internal/service` | 7/26/19 | retained，实际命令与两模式结果在JSON |
| R28_original_controls | `./internal/service` | 7/58/38 | retained，实际命令与两模式结果在JSON |
| R29 consumed service union | `./internal/service` |7/76/54 | 当前实际normal/race76PASS、0FAIL/SKIP；13新named+63原receipt/M1/consumer，非旧58代替。 |
| R29 direct Get handler | `./internal/handler` |3/6/5 | 当前实际normal/race6PASS、0FAIL/SKIP；ordinary reopened receipt、self/submission/admin outside consumer、R38 current config。 |

精确R29 service selector：

```text
^(TestDeliveryCompletionConsumedGetReadCoexistence|TestDeliveryCompletionConsumedGetWriterFences|TestDeliveryCompletionConsumedGetCurrentAuthority|TestDeliveryCompletionReceiptRejectsMismatchedRetainedFacts|TestDeliveryCompletionReceiptValidatesClosedVariants|TestStageCompletionAllM1ModesAndActualWriters|TestStageCompletionIndependentConsumerAuthorityAndMinimalReceipt)$
```

精确R29 handler selector：

```text
^(TestDeliveryCompletionAuthorizationActualArtifactAtomicConsumeAndReplay|TestDeliveryCompletionCollaborationActualSourceAndOriginalRecovery|TestDeliveryCompletionR38AuthorizationHonorsCurrentConfiguration)$
```

新读回归3top/13named/10leaf：ReadCoexistence的r4_first/authorization_first/same_grant_siblings/operation_first/operation_after_authorization；WriterFences的writer_first/reader_first；CurrentAuthority的wrong_actor/changed_password/source_revoked。JSON保存完整新13、direct6和原所有选集的exact selector+两模式result/command/hash；不重复列出/执行完整116的321named。

### Accepted R29 delta 与保留依据

`G=/Volumes/artisan/.multica-acceptance/pay241-r29-receipt-read-20261118-6dhi01s2/source-green-r01`，`F=/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/frontend-base-r01`。两产品：`server/internal/service/delivery_completion_authorization.go` SHA `a8445340a055faa12fceaa04e70f62eb213e3aeac333b0bc5abff50520e4f0b0`；`delivery_completion_collaboration.go` SHA `50c048f059392bed2f1b5c99cedba605c6fade3facbc28b963fee2f0c586183b`，均0644。5052中其余5050 bytes/modes相同。558行新测试`7e927d27…`未改。只复用接受的34原函数/3exact reduction/2937-file caller证明，本研究没有重做那次完整扫描或实现复核。

- GetByID `authorization.go:210,215,218,222` 先 actual id-only SHARE NOWAIT，然后原generated getter/仅私有 `readCompletionAuthority`；同borrowedTx，经root/source child/sharedaccess/observer。共用三个validator只增加显式readProjection；不存在新Begin、public/context flag或marker。
- By-request `:227,235,239` 保留 UPDATE getter、原 `completionReadAuthority(...false)` wrapper `:330–331`；Create-key/revoke保留原false issuer entry `:374–375`。Prepare/Finish/terminal/actual writer、observer/default/QA/R38 advisory和配置仍原锁与谓词。三个false-common-body还原SHA依次 `0818e275…`、`afdbd77c…`、`27ebe62a…`，接受报告保存完整hash与原signature。
- `collaboration.go:92–124` 的原source entry为 `(exact,false)`，私有read为`(exact,true)`；**exact=false不是shared/unlocked**。freeze/revalidation继续exact=true/default-exclusive，原ordinary-kind/child/parent/project/revision校验保留。Consumer `false`是newAdmission=false，绝不是读锁选项。
- Schema/currentcredential/member/source/producer/execution、stored request/minimalreceipt、handler no-store/finalcredential/commit未知处理未变。直接handler6实际证明reopen/edit后仍返回原consumed receipt但今日todo/title不重写；真实outside consumer和admin否决/issuer独立读保留。
- 原handler五top只有AtomicConsumeAndReplay直接Get；另外四个writer top4/11/10可凭上述原路径证据保留。By-key/currentcredential/taskconsumer原router测试仍在99/54原PASS来源中；不能说handler原5独自覆盖这些GET。R29未改HTTP/router/DTO，因此不自动再跑conditional router3/6/5；只有实际authentication/selector/consumer HTTP wiring或handler finalization变动才需重新定界fresh包运行。

这构成 **保留116/99/54 +接受R27/R28补充 +当前R29 affected76/6 +本轮original4及after-stop** 的最终证据组合；不是“当前116全重跑”。不因同文件factoring或API重建再跑无关写矩阵。若最终scope/bytes/caller/default、schema或谓词/side effect发生新delta，保留证明失效，ROOT需按新delta定界相应检查。

### A01–A12：逐行最后需要什么

以下全A仍是final evidence待复核；“retained”表示source-bound逻辑/旧实际PASS可使用，不表示免于本轮UI/API/source/资源组合核对。

| A | R29 applicability / retained binding | 最终 evidence |
| --- | --- | --- |
| A01 | Roles/owner/handler predicates unchanged; use retained authority tests and fresh E01 role/history evidence. | E01 exact current source-admin/participants/accounts, stable owner vs handler and role history. |
| A02 | Proposal/decision/materialization/enqueue paths unchanged; R29 grants no new proposal or dispatch permission. | E01 propose/edit/accept/reject with exact version/key/child IDs, single materialization and zero unsolicited dispatch. |
| A03 | PR/head/CI/trust writer gates unchanged; GetByID shared observer preserves existing predicates. | E01 absent/failed/mismatched CI/head/trust definite refusal and unchanged stage; synthetic provenance explicit. |
| A04 | Human reviewer/self/machine/signer/rejoin/final-clock rules remain original false/default writer paths. | E01 genuine authorized reviewer plus self/machine/revocation/rejoin negative boundaries and original signed identity. |
| A05 | R29 adds receipt-GET coexistence with R4; report/run/case/log/QA mutation logic is retained. | E01 real Git-bound prepared CLI failed run/cases/logs, QA return/new run/new decision, retained old failure; E02 valid settled QA dependency. |
| A06 | Release/QA separation/build/issuer validity and execution defaults unchanged. | E02 exact build/QA/release authorization and controlled deployment fact with current issuer horizon. |
| A07 | Rollback proposal/approval/execution/metric/physical target contracts unchanged. | E02 original deployment/target/build/metric and separate approved rollback/executed receipt; single controlled call. |
| A08 | Current-authority original immutable receipt recovery is directly affected; accepted service76/direct6 bind new behavior. | E01/E02 keys/version/revocation; E03 late expiry/history/no renewal; complete E04 original grant/key/body GET404→Desktop consume→Web exact GET200. |
| A09 | Outer GetByID all variants/current R38 config are re-exercised by handler6; service76 includes40 M1 mode/writer names. Actual completion/revoke/clock/writer logic stays original default. | Complete E04 all7 composition groups, three-mode submit/self negatives, genuine enforce submission completion, R38 negative/restore and project exits; keep separate source issuer/outside consumer/backend positives. |
| A10 | Shared GET source/credential/actor fences and zero private denial views directly affected; retained cache/R1/R4/T4 scope stays bound to unchanged FRONT. | Fresh E01/E02/E04 source-denied/current-authority projections with no private cached resurrection; attach R29 wrong actor404/password403/source403 and direct consumer minimal receipts. |
| A11 | ROOT3/five binaries and unchanged FRONT products are accepted scope; current live binding is ROOT-recorded, not full journey acceptance. | Exactly one final current original4 with4PASS/0FAIL/SKIP/NOT_RUN/retry, production Web/native renderer+preload/API/CLI/harness/source/config/actor identities and PNGs. |
| A12 | No local test/build/source report closes whole S2; final review must integrate all retained and affected contract evidence. | ROOT outcome/after/normal-stop/history/artifact/final identity evidence, independent42-row scope review, ROOT parent/task/spec/bookkeeping updates; parent/external gates kept separate. |

### W01–W18：完整实际入口

每个actual entry path、canonical要求和已有service test names在JSON逐项保留；没有service名字的batch/delete/CLI行照样在分母，使用其actual handler/router/CLI证据。

| W | R29 applicability / source-default | 最终 evidence |
| --- | --- | --- |
| W01 协作正式交付submit | Submit writer/current final authority/default unchanged; service AllM1 mode matrix actually fresh. | E04 six submit/self × off/observe/enforce refusals with stage_blockers/effect equality, then actual immutable submission200 and exact candidate. |
| W02 协作精确submission完成 | M1 completion writer preserved; consumed grant GET/root/source fences change. | E04 original Web201/aborted-before-forward/GET404, same original Desktop key/body/hash one actual completion200/consumed200, Web operation200+grant200/equal receipt/no new POST. |
| W03 协作self-result完成 | Self-result/attachment/writer logic remains original; R29 direct handler covers consumed self reader. | Retained/fresh backend three-mode self positive; E04 incomplete self negatives and unconsumed enforce grant. Do not call E04 a self-positive browser proof. |
| W04 R38失能发起人管理完成 | Admin nomination/config/advisory/default writer unchanged; direct handler6 includes disabled admin consumer403 vs independent issuer200. | Backend three-mode real R38 positive/current organization/disabled creator; E04 admin authority, creator disable, incomplete-root409/blockers/effects equal and restore. |
| W05 普通PUT/旧client/Agent的完成请求 | Ordinary/legacy/Agent mutation and actual principal gates untouched. | Source-bound handler/router ordinary controls plus fresh W10 ordinary PUT refusal; installed old-client matrix remains separate. |
| W06 普通Move | Ordinary Move writer/semantic invalidation/default source exact=true unchanged. | Retained actual Move/ordinary atomic and child-semantic controls; no extra browser/destructive run needed. |
| W07 普通批量更新 | Batch per-item/committed prefix/duplicate binding/unknown policy unchanged. | Retained handler/router batch committed-prefix/duplicate-grant/key and mixed-root before-effect controls; no full-batch replay claim. |
| W08 Create/快速创建/comment-to-child/create-as-done | CreateInTx/AI proposal/human manual child/custom done unchanged. | Retained creation/machine proposal controls plus E01 approved single materialization/zero dispatch; no real-model proof. |
| W09 GitHub/VCS/system状态writer | GitHub/VCS/system signatures/trust/ordinary M1/root refusal unchanged. | Retained signed writer/root-refusal controls; real remote PR/CI/provider integration stays S4. |
| W10 完成CLI及run-local machine边界 | New matched CLI binary differs from prior historical binary; fresh ordinary refusal result is available and checked. | Use runtime/w10-cli-r29-r01/result.json +two actual409 logs +identical full before/after/eight counts/zero operation files. E01/E02 run-local report positives remain separate. |
| W11 协作root project move/clear | Project move/clear policy/actual ordering unchanged. | E04 root null/other project PATCH409 stage_policy_exit_required/effects equal; retain transactional order/off controls. Early R28 probes were reached. |
| W12 项目delete-impact和实际delete | Project impact/delete policy/race contract unchanged. | E04 impact200/blocker, managed DELETE409, unrelated empty project DELETE204; retain policy/race controls, no production deletion. |
| W13 managed root普通delete与batch-delete | Managed root/mixed/batch delete before effects, late attach/fault rollback unchanged. | Retained root/parent/mixed delete controls; no destructive new matrix required solely for R29 GetByID. |
| W14 普通child语义编辑/换parent/project/删除 | One source child validator is factored; exact=true/default false writer restores original parent/project/revision and physical fence. | Retained ordinary child persistent mapping, semantic edits/status distinction/parent/project/delete controls; freeze/revalidation never opt into read mode. |
| W15 真正enqueue与queued→started | Enqueue/Start/unapproved/stale/source authority/default paths unchanged. | Retained real enqueue/start both orders/unapproved rejection; E01 zero unsolicited dispatch and genuinely admitted prepared CLI. |
| W16 cancel与reopen | Cancel/reopen mutation unchanged; R29 handler actually reads old consumed receipt after reopen/edit without rewriting current todo/title. | Attach direct handler6 plus retained cancel/newscope; E03/E04 late/original receipt never renews an approval or implicit Resume. |
| W17 root/R38 completion candidates与preview | R28 candidate/preview read dispatch and R27 bound ID/UTC remain retained; R29 GetByID private mode is not a preview/authorization flag. | Retained F06 strict ACL/cursor/empty/admin plus accepted R28 preview26; E04 exact preview/child/build/submission/current grant. Preview POST is read; explicit201 alone authorizes. |
| W18 活动与原操作回执投影 | Safe activity/operation projection unchanged; new minimal GetByID recovery directly affected. | E01/E03 history and E04 original operation200/minimal consumed receipt equality, pending retirement/no duplicate POST/consumption; retain activity allowlist/frozen blockers. |

### Q01–Q12：跨事务/来源检查

| Q | R29 applicability / source-default | 最终 evidence |
| --- | --- | --- |
| Q01 | R29 service76 contains all40 mode/writer named nodes,24 leaves,12 verified positives +12 incomplete negatives. | E04 representative off/observe/enforce composition complete; no substitute for backend self/admin/outside-consumer positive matrix. |
| Q02 | New GetByID stays on caller pgx.Tx; no Begin/context flag/marker; Prepare/Finish wrong-Tx/private guard/single-use logic unchanged. | Accepted R29 Tx/reader/writer witnesses plus retained guard before/after/NA/finalization; E04 same original actor/header/request/body. |
| Q03 | Final DB clock/issuer/caller/source/CI/artifact/target/metric checks and strict consumption unchanged. | Retained final-clock/all authority writer controls +E03 late-deadline history/E04 current exact approval timestamps; do not turn same-Tx seams into commit-order proof. |
| Q04 | Stored request/receipt consistency unchanged; shared source physical fence changes only private read mode; immutable submissions locked by default writer. | R29 receipt closed/mismatch controls and current Get proofs; E04 real positive submission and same immutable ID/operation/collaborationrevision+1 receipt. |
| Q05 | Current wrong actor/password/source denies and outside consumer/R38 minimal Get adapters accepted; consumer false remains newAdmission=false. | Backend P≠E/no-source-membership remains its own evidence; E04 same-account Web/Desktop cannot be relabelled as that matrix; no private view on denied GET. |
| Q06 | GetByID adds compatible read fences; ten72-relation snapshots per service mode prove per-read/control no effects. | Preserve baseline after genuine fixture completion; whole suites legitimately create handoff/job/invalidation rows. E04 preview is read and one positive actual final completion count excludes earlier negative attempts. |
| Q07 | No HTTP/DTO/router/aliases/cursor/page budgets changed; retain99handler/54router and FRONT source/cache evidence. | R29 direct6/current denials plus fresh source denial/late-response fencing; router3 mandatory only if HTTP/current selector/consumer wiring/finalization actually changes. |
| Q08 | Consumed-after-reopen/current Get authority directly affected; by-request remains UPDATE/default and original write/unknown receipts unchanged. | Complete E03 late original fact and E04 pre-forward GET404→one Desktop original consume→Web doubleGET with exact receipt/no renewal/replacement POST; preserve R28 failed original result. |
| Q09 | R29 has zero SQL/generated/migration/readiness/role/epoch changes; no migration rerun required solely for GetByID. | Retain actual P2-R09 readiness/18-profile evidence and current schema binding; ledger853 entries through865 is count/stem identity, not physical DDL proof. |
| Q10 | Default source validator exact=true remains exclusive; project/submit/complete/physical target/ordinary child contracts unchanged. | Retained commit-order/target controls plus E02/E04 current exact target/exit/human-child observations. |
| Q11 | Zero R29 DDL delta; original six raw controls +immutable supplement/39 assertions and cleanup registrations stay scope-bound. | Use actual prior schema/index/noFK/history/readiness proofs and current ledger/source identity; old8 F2 cleanup omissions/Go1.26 Linux CI limits remain declared, not silently passed. |
| Q12 | ROOT owns one run and all lifecycle/audit mutations; preparation before-run equality alone cannot close cleanup. | Final source/product/link identity, live-after, one graceful owned stop, stopped DB/restored booleans+monotonic epoch/no active leases/reservations, old unknown/history/tar preservation and parent updates. |

### 原第四场景必须完整抵达的断言

冻结源 `F/e2e/collaboration-stage-execution.spec.ts:397`，标题exact `stage evidence gates real submission, M1 completion and administrative or project exits`。20秒动作/480秒case/五秒read保留。下表对应源码实际断言，case headline不能替代它们。

| group | 源码行anchors | 要求与证据 |
| --- | --- | --- |
| E04-01 | 419,424,425,434,435,436,437,438 | All three M1 modes submit/self stage_incomplete409+stage_blockers, equal effect snapshots; actual enforce grant on unfinished self root remains issued/unconsumed. Six negative response/effect pairs and explicit issued read; this grant is separate from the later positive root grant. |
| E04-02 | 441,446,448,449,450,451,452,453 | Root clear/move409 stage_policy_exit_required/effect equality, delete-impact200/blocker, managed project DELETE409 and unrelated empty project DELETE204. All early project probes must exist; they already ran historically before R28 spec572 and were not NOT_RUN. |
| E04-03 | 458,460,463,464,465,466,468,472,473,474 | Current QA/release dependency/target and one controlled verified execution; toolbar Submit result actual200 returns immutable exact submission ID. Release/controlled call counter/source/build/candidate plus real form POST200 delivery_confirmation.submission_id; local synthetic provenance remains explicit. |
| E04-04 | 475,479,480,485,487,488,495,496,499,500,501,503 | Shared Web preview binds exact child/build/artifact/source/submission/policy; one explicit positive-root authorization201 with its exact preview body. POST preview is a read; positive journey Web authorization count1 does not erase the earlier unfinished-self issued grant. |
| E04-05 | 504,506,507,508,510,514,516,517,518,519 | Web final intent interrupted before API forwarding, same original operation/grant/revision/body; actual original operation GET404, no new effects/grant/forwarded completion. Interrupted request key/body/hash and beforeCompletion equality; separately count one intercepted browser attempt. |
| E04-06 | 521,525,526,527,529,532,533,535,537,538,539,540 | Fresh native profile uses same actual P account without copied Web storage; exact same preview/imported original grant/operation issued GET200. completion_desktop_profile_sha256, actual native binding/account and original reference read; do not synthesize a new grant or body. |
| E04-07 | 543,549,550,551,552,554,555,556,557,558,559,560 | Desktop one actual final lifecycle200/code completed with exact interrupted key/body/hash; consumed Get200 exact original grant/operation/root/submission and collaboration revision+1; form retires and zero Desktop authorization POST. Completed/consumed typed bodies/request SHA and actual write trace; successful completion is immutable. |
| E04-08 | 564,565,570,571,572,573,574,575,576,577,579,580,583 | Web original-operation200 equals completed body, grant200/equal original completion receipt; pending recovery control disappears, no second POST/new grant, current issue done, completed native screenshot and whole journey record. The actual Core recovery requests are sequential operation then authorization GET; Promise.all response waiters are not proof of parallel HTTP. R28 failed at572;573-583 were not reached. |
| E04-09 | 585,587,589,590,591,593,596,599,601,602 | Set M1 off for independent R38 stage-negative scope; actual synthetic super_admin and current CAS disable of this scenario creator. Original admin/user operation IDs/action/target/actor/auth_version and applied state; simulated admin cannot become business sign-off. |
| E04-10 | 603,604,605,607,608,610,611,613,614,615,616 | Current real R38 complete option for unfinished self root; lifecycle409 stage_incomplete+stage_blockers/effects equal. Distinct admin actor/current organization/password and unchanged unfinished self-root effect snapshot; browser negative does not replace backend R38 positive. |
| E04-11 | 617,618,619,620 | Creator restore in finally after disable; successful scenario outcome/persist/close all finish, then runner records exactly one passing terminal. Restore applied operation, passed_representative_writers only after all assertions, persisted case evidence and no undeclared retries. |

另三原身份保持：E01 `collaboration-stage-approvals.spec.ts:27`（roles/proposal/strict CI/human reviewer/real failedreport/QA return/newrun/history）；E02 execution spec`:232`（exact QA/build/release/deployment/metric/rollback/single controlledreceipt）；E03`:348`（late fact只是历史、不renew/redispatch/endorse）。当前runner只选择这两spec中的原4，workers1/retries0/maxFailures1；`:81–89`要求每case仅一个passed/retry0且exit0。不得union不同失败轮次的最佳片段。

R28 retained fail定位572：Desktop原reference200、one lifecycle200、consumed200，然后Web operation200/grant409。573–583与585+未运行；**早期419–453的项目退出/删除负例实际已运行**。R29新通过不能覆盖原3-1结果，历史SQLSTATE/physical holder仍未证明。E04 same-P Web/Desktop transport不冒充backend P≠E，self/R38 browser negatives不冒充三模式positives，local packager/controlledadapter不冒充真实S4。

### 当前W10、产品与recorded live entry

Fresh W10 `/Volumes/artisan/.multica-acceptance/pay241-r29-receipt-read-20261118-6dhi01s2/runtime/w10-cli-r29-r01/result.json` SHA `90128baa73ed434ebcf3b10f134fcf6de3f388b27a5c2f5740262eb5dc759920`，实际当前CLI SHA`cd60ed4e…`/0755。两个命令 prepare/update均预期exit1，hash-matched stderr真实记录409 `delivery_completion_consumer_unsupported` /409 `unsupported_collaboration_action`。before/after fullbytes及parsed对象相同，SHA `60e8277c63465885073d4561d696504d68cff0b529064bf65be003e70428f314`；8counts=tasks0/events1/handoffs0/terminals0/operations0/evaluations0/submissions0/authorizations0，**旧event1保留而不是全8表0**，operation files0。根是unmanaged-stage普通协作；不声称managed-stageCLIpositive/realmodel/fullA09。

ROOT3/five binaries接受报告及JSON保存exact paths/SHA/modes：server `42eafbf4…`、CLI `cd60ed4e…`、router `5c039d1b…`、host `be777019…`、provider `8f7d4680…`；F生产buildID=`PpNZY2bAtTOUVfi6UgJKT`，3359 frontend inputs/1961links/1676outputs未重建或改变。`runtime/live-binding-current-r01.json`录API87493/bridge87488/origin64922、10111required file refs；既有Web86953/renderer86966由ROOT保留。本研究无live probe，不以prep/build/titlelabels认作A11完成。

### 正常停止与after审计：ROOT仍需提供

| audit | 最小具体 evidence |
| --- | --- |
| STOP-01 | ROOT-owned normal stop after immutable current result: one intended SIGTERM to bridge87488, parent/argv/listener ownership proof for API87493; harness normal terminal; no repeated/foreign signal. |
| STOP-02 | Owned API/bridge gone,64922 listener/published control socket absent; existing Web86953/renderer86966 remain owned and preserved or have separately explained intended lifecycle. |
| STOP-03 | Separate live-after from stopped-after selected RR/read-only snapshot. Stopped runtime DB identity correct/other_connections0; gate enabled=false/creation_enabled=true/host_execution_enabled=false restored with actual monotonic epoch relative to56. |
| STOP-04 | All new runs settled; no active lease/bound process/unreleased reservation/unsettled Host/incomplete artifact. Preserve legitimate historical running statuses without live lease and old unresolved intent1. |
| STOP-05 | Compare all retained49 runs/35 complete artifacts/16 intents/24 observations/12 metrics and queue history to pinned before; classify declared actual additions and legitimate runtime changes. Live-before already changed five old deferred tasks to failed while running4 remains; bind real chronology/reasons without inventing cause. |
| STOP-06 | Full staged/output FS descriptor/path/inode/UID/type/mode/size/hash audit and declared additions: preserve old292 paths plus eight initially empty0700 slots; staging root0500/output0700. Used slots sealed; unused slots stay empty0700. |
| STOP-07 | Actual new archive/tar SHA/member/source/spec/receipt/input-manifest bytes agree with DB execution provenance; preserve all prior35 complete artifact bytes. Outputs use execution IDs; metadata-only preparation/per-leaf72-table equality is not a tar or whole-suite audit. |
| STOP-08 | Final accepted GREEN5052 source/modes, ROOT selected3, five actual binaries,3359 FRONT/1961 dependency links/1676 production outputs/buildID, decoded configs/permits/listeners/processes remain correctly bound afterfour and afterstop. |

已有beforeAPI baseline49settled runs/35complete artifacts、无lease/unsettled/unreleased，gatefalse/true/false epoch56，unknown intent1；recorded live-before-fours的other_connections5是运行态，不是连接0停止证据。其queue38中failed34/running4，beforeAPI为failed29/deferred5/running4；这类actual runtime变化应绑定真实时间/原因并保留，不能做manual reset或宣称整个DB零变化。所有per-leaf72-relation/effect snapshots只界定读effects，真实fixture completion/Host历史会增加。

### 最小未纳入本报告的最终证据

| missing | ROOT交付/最终review要求 |
| --- | --- |
| M01 original-four terminal | ROOT deliver result/exit/raw stdout-stderr/reporter/started-once/command/permit: exact four original titles, one passed retry0 terminal each,workers1/maxFailures1,20s actions/480s cases/5s read bounds. |
| M02 case content | Four case-local JSON/HTTP/request/body hashes/actor/source/runtime/native/profile/PNG evidence; complete eleven E04 assertion groups, especially572-583 and585-617 that R28 did not reach. |
| M03 after-four/final source | Current complete source/product/config/link equality plus owned live-after DB/resources; pin actual source/build/profile and classify any legitimate writes. |
| M04 normal stop | STOP-01/02 owned graceful shutdown and preserved other platform processes; separate stopped-after DB with connection0 and restored gates. |
| M05 history/artifact preservation | STOP-04/05/06/07 settled new work, old unknown/history preserved, full FS/tar/source provenance and declared additions. |
| M06 whole-S2 independent acceptance | Independent A01-A12/W01-W18/Q01-Q12 applicability/source-default/current runtime/post-stop review, all evidence references resolved and no in-scope pending work. |
| M07 ROOT bookkeeping | ROOT reconcile authoritative S2 criteria and parent test-matrix/gaps/roles-and-states/progress/task state only from final scoped evidence; S3/S4/S5,parent24/real business sign-off remain separate. |

这是既有final gate的证据收集清单，不新增Go/SQL/browser/破坏性/remote执行任务。最终Trellis 2.2（workflow`:568`）须按全S2 affected contracts/packages独立复核，允许使用确切source/default的retained proofs；并非要求无差别重跑116或所有apps。ROOT负责spec/progress/task/parent bookkeeping；本研究不更新任何checkbox、矩阵或阶段状态。

## Files found / code patterns / related specs / references

Companion JSON保存所有上述canonical/report/result/command/source/spec SHA/mode、各选集exact selector/count、42-row applicability、原E04断言anchors、post-stop与minimal missing清单。重要spec：server `delivery-completion-enforcement.md:60–146`、`collaboration-stage-approvals.md:52–110`、`verification-environment.md`、`database-test-isolation.md`；Core `collaboration-workbench.md`、Views `collaboration-detail.md`及Core/Views/Web/Desktop indexes保持包边界和shared recovery合同。

无external network/API查询；第三方版本沿用冻结source/go.mod既有Go1.26.6/Chi5.3.0/pgx5.9.2声明，真实localGo1.27/macOS observations不静默认证Go1.26/Linux CI。全部synthetic externalfacts有其S2范围；S3剩余恢复/阻塞字段、S4真实I02–I09/E01–E07、最终S5 100人/10000传播/100回写与业务签字保持独立。

## Caveats / Not Found

- 本报告是final-gate research inventory，未读取/裁定currentoriginal4终态；ROOT当前结果/after/stop/A12仍由ROOT收集。所有A/W/Q“待复核”不能自动变成PASS。
- 没有执行Go/test/list/TestMain、DB查询/SQL/迁移、browser/API、runner/build/process/signals/Git，未写产品/测试/spec/progress/task/旧报告/父目录，也未派生代理。仅这两新report最终0444。
- 复用已接受34function/3default reductions与116/99/54/26/58来源证明；未重复全caller/source/原rawstream审计，不声称current116全重跑。
- 原R28 FAIL、成功完成、unknown/history保持不可变；parent24读得0PASS9PARTIAL15NOT_RUN。本inventory不关闭S2、父任务或任何真实S4/business sign-off。
