# Research: PAY-241 S3 continuation prerequisites during S2

- Query: 独立核对 S3 block-contract v2 对原 requested changes 的关闭程度，列出 accepted S2 之后的最小实施批次及真正阻挡项。
- Scope: internal；当前 Active task 为 `.trellis/tasks/11-13-pay-241-s2-stage-approvals`，跨任务只读 S3/父矩阵/已归档证据，不激活或实施 S3。
- Date: 2026-11-15
- Write scope: 本文件及同名 JSON；未修改原评审、产品、测试、规范、任务状态或其他 owner 文件。

## Findings

### 1. 结论及证据级别

**`ready_for_implementation: false`，指现在直接派发完整 S3 产品实施；不等于原 timeout-proof 失效，也不等于全部研究需要重做。**

- **C01 的五条死路及 pending submission 子例均已有明确的 v2 设计修正**：真实单事务 repair 到非 blocked，再由人独立决定是否发 B2；不是改写 `previous_status` 或自动续跑。但尚不能记为产品修复/正式评审 PASS。
- **C02 大部分选择已收敛**：presence、4096-byte 文本、微秒时间、complete/actionable 分离、runtime v1 不变及旧快照字节不变。项目观测 envelope 与修复历史分页公共入口/DTO 仍未足够冻结，不适合让 Server/Core/Views 分别猜。
- 原评审 MD:77 仍为 pending；JSON:5 为 `changes_requested_v1_awaiting_v2`、JSON:45 为 `v2:null`。v2 MD:9 自称 coordinator-review recommendation，并非 released wire/product acceptance；本报告不回写旧审阅结论。
- 2c 的独立 correction PASS 允许同一 owner 继续 2d，不接受完整 S2。2d 的 F06 wire 已冻结，DDL 局部已验证，但没有本次读取范围内的 2d results/check 接受报告；不能将草稿、已迁移 schema 或客户端实现当作 accepted 2d writer。
- 新的 11月15日 S2 计划头 :7/:14 仍明确是保留的 2d 草稿和待完成全入口测试/独立检查；:8/:9 记录了 DDL r02 与新环境重建，不可用旧同名迁移的通过替代新字节。续接计划 :54 同样要求 accepted final S2 后才实施 S3。

### 2. 文件与引用约定

下表的短名及后文 `短名:行号` 均指这里的准确仓库路径；JSON 每项以 source ID 和 1-based line 绑定同一源文件。

| 短名 / source ID | 路径 | 责任与证据级别 |
| --- | --- | --- |
| Review / R | `.trellis/tasks/11-13-pay-241-s3-recovery/research/recovery-followup-review-20261114.md` | 原独立评审，C01/C02 requested changes 未覆盖重写。 |
| ReviewJSON / RJ | `.trellis/tasks/11-13-pay-241-s3-recovery/research/recovery-followup-review-20261114.json` | 原 status、v2:null 与 v1 SHA 绑定。 |
| V2 / V | `.trellis/tasks/11-13-pay-241-s3-recovery/research/recovery-followup-block-contract-v2-20261114.md` | 候选修正；不是已接受公共合同或实现。 |
| Prep / P | `.trellis/tasks/11-13-pay-241-s3-recovery/research/recovery-followup-preparation-20261114.md` | 保留的六主题、写入口 inventory、测试建议；冲突的阻塞规则由 v2 替代。 |
| PRD / S3P | `.trellis/tasks/11-13-pay-241-s3-recovery/prd.md` | R01–R09 原要求及不纳入结论的边界。 |
| Design / S3D | `.trellis/tasks/11-13-pay-241-s3-recovery/design.md` | 复用既有恢复原语，S2 稳定后做阻塞结构。 |
| Plan / S3I | `.trellis/tasks/11-13-pay-241-s3-recovery/implement.md` | :6/:7 分别要求组合测试及完整阻塞评审后再实施。 |
| 2cContract / C2 | `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/server-increment-2c-control-contract.md` | 已冻结 trusted source、单次 callback、历史回执合同。 |
| 2cCheck / C2K | `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/server-increment-2c-followup-check.md` | bounded correction 已接受；:4/:50 明确 2d/完整 S2 待续。 |
| 2dContract / D2 | `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/server-increment-2d-client-contract.md` | 已冻结 F06 read/activity wire，不代表实际路由/writer 接受。 |
| 2dDDL / D2M | `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/server-increment-2d-migration-results.md` | 861–864 DDL/retention 局部结果；:3/:33/:39 保留 runtime readiness/实际 writer 门禁。 |
| S2Current / S2I | `.trellis/tasks/11-13-pay-241-s2-stage-approvals/implement.md` | 新增 11月15日头部 :7/:14 为当前草稿/下一门槛，不替代冻结增量报告。 |
| ContinuationPlan / CP | `docs/plans/2026-11-15-pay-241-continuation.md` | :53/:54 为本次前置评审与 accepted S2 gate；:55 的 output-upload 主体/接口仍需澄清。 |
| Timeout / T | `.trellis/tasks/archive/2026-11/11-13-pay-241-s3-timeout-proof/report.md` | 已归档受控超时证明及其原源码，不重跑。 |
| TimeoutCheck / TK | `.trellis/tasks/archive/2026-11/11-13-pay-241-s3-timeout-proof/research/check-review.md` | T01–T06 bounded PASS，非完整 S3。 |
| Matrix / M | `.trellis/tasks/11-13-pay-241-collaboration-acceptance/test-matrix.csv` | 当前仍 4 PARTIAL / 20 NOT_RUN；本次只读。 |

### 3. Requested changes 逐项 closure

“已回应”只表示候选文本闭合该设计反例，不表示已实现、已测试或原 checker 已正式接受。下表完整展开 Review:26 的五序列及 Review:44 的公共合同问题。

| 项目 | 原 requested change | v2 精确回应 | 本次判定 |
| --- | --- | --- | --- |
| C01-A creator replacement | Review:28、:36 | V2:33 禁止 blocked 下无 parent 替换；:219 实际 R→非 blocked，独立 B2 保存真实 S；:115 精确 supersession relation。 | 已回应；待合同接受/红绿回归。 |
| C01-B recipient removal | Review:29、:36 | V2:220 用真实 removal 后 pointer=null 的 human-selected repair；:115/:119 禁止把 removal 的旧 supersession 追认给 R。 | 已回应；安全移除不因 metadata 被阻止，见 :245。 |
| C01-C legacy blocked→managed | Review:30、:36 | V2:221 明确 policy enable/project association；:86 human-selected work status，不伪造历史；:257 明确两种生效路径的回归。 | 已回应；待最终 S2 policy/source locks。 |
| C01-D incomplete/unreturnable parent | Review:31、:36 | V2:31/:33 完整 ancestry、cycle/missing provenance fail closed；:222 repair 不制造 parent；:258 要求 legacy/cycle/missing-parent 回归。 | 已回应；无任意新 depth cap，预算内无法证明即 unavailable。 |
| C01-E requester leaves/disables | Review:32、:36 | V2:43/:45 真实当前人类 Transfer+ChangeStatus、自目标可作状态修复；:223 不重邀旧 requester；:259 覆盖 removal/disable。 | 已回应；无当前合法 actor 则真实拒绝。 |
| C01 pending delivery / M1 | Review:38 | V2:129/:130 pending basis 优先，精确 D/event、当前 creator；:132 不改 submission/附件/stage/M1；:225/:226 与 :261/:262 区分正常修复及独立 R38 lifecycle。 | 已回应；不能选 in_progress/新 handler 绕过验收。 |
| C01 原身份、审计、真实状态变化 | Review:36 | V2:21/:24 一事务，不是两次 HTTP；:78 独立 kind；:113/:116/:117 真实 prior、事件及回执；:234/:237/:244 原键恢复/冲突/全回滚。 | 已回应；无新 repair ledger、自动 B2、Resume 或 stage advance。 |
| C02 absence/null 与旧 hash | Review:46 | V2:141 raw presence 后 pointer+omitempty；:76 repair observation 必须 present 可显式 null；:247 不用 zero-value struct；:268 含 duplicate/unknown/escaped-envelope 回归。 | 已回应；旧已提交请求先恢复，不补字段。 |
| C02 finite text / envelope | Review:44 | V2:78/:139 note/condition UTF-8、无 NUL、nonblank、4096 bytes；:268 六类 boundary/escaped JSON，沿用既有严格 64KiB handler envelope。 | 已回应；不是 4096 characters，也不新定 ETA 最大天数。 |
| C02 final stored timestamp | Review:47 | V2:143 timezone、0–6 fraction、拒绝 >6；持久化微秒值与最终 DB clock 比较，digest 保留输入字符串，旧过期 ETA 不使 replay 失效；:268 回归。 | 已回应；是新写入规则，不是已测试数据库结果。 |
| C02 complete vs actionable / unknown | Review:48 | V2:147/:156 严格 block complete/legacy/unavailable；:160/:175 当前 workflow 单独授权；:190 403/404/409/503 与 unavailable 非 empty；:267 回归。 | 已回应；完整历史不代表 requester/parent/target 现在可用。 |
| C02 selected-project options | Review:49 | V2:209 明选 project_id/current source-project authority，无 project 保留原 choices；:211 最终 writer 拒绝 managed blocked create；:271 回归。 | **部分回应**：`stage_policy_observation` 未列完整字段、类型、presence/null/unknown 形状及是否进入请求/digest；需公共 wire 冻结，不能由客户端推断 managed=false。 |
| C02 human current/history 公共读取 | Review:44、:48 | V2:194/:196 要完整历史及 paged repair affected set；:119 提供 DB relation+denominator。 | **仍缺公共合同**：未选该分页用哪个 route/query/cursor/response。当前 activity 只有普通 handoff DTO，而 human run-handoff 直接共享 strict runtime v1；不能随意把新 block/history 字段塞入后者。 |
| C02 runtime v1/source hashes | Review:50 | V2:201 固定 v1，不新字段/source text；:203 只允许新 live/new snapshot missing marker，旧 snapshot/note/hash/bytes不变；:205/:270 明确未交付完整 runtime metadata。 | 已回应；marker 本次源码搜索未见，不能称已发布 capability。 |
| 正式 review closure | Review:7、:77；ReviewJSON:5/:45 | V2:9、:288、:306 仍待 root review/owned handoff。 | **未关闭**；本报告是新的核对证据，不伪造原 checker 的 v2 PASS。 |

### 4. 源码交叉核验与 accepted S2 边界

- `server/internal/service/collaboration_handoff.go:21` 当前信息请求 DTO 无新两字段；`:184` 只为当前 recipient/handler 建 parent；`:230` 旧 requester 不可用则 return 拒绝。`server/internal/service/collaboration_workflow.go:255` 保存真实 prior status（赋值在 :257），`:289` 移除当前 recipient 会走 changeHandler/supersede。因此 C01 是 v1 **规划缺陷**，不是本次执行复现的故障。
- `server/internal/service/collaboration_workflow.go:411` 要真实人类；`:421` Marshal 原 payload，`:559` 先当前权限再原 key，`:599` account/lifecycle/participant 与 collaboration revision 判定。v2 必须扩展既有 owner/kind/receipt，不替换其恢复器。这里只核对既有模式；本轮新增 2d 接线仍是未接受草稿。
- `server/internal/service/collaboration_activity.go:29` 当前 handoff 仅 id/purpose/from/to/note；`:175` 用 kind/purpose/actor 精确匹配，不能用时间猜 repair。`server/pkg/db/queries/collaboration_handoff.sql:53` 只绑定信息请求到其原 event sequence，不是现成 repair-history API。
- `server/internal/service/collaboration_runtime_handoff.go:149` human RunHandoff 也返回 `protocol.CollaborationHandoffEnvelope`；`server/pkg/protocol/collaboration_handoff.go:256` 严格 typed fields，`:351` 只接受 version 1；`server/internal/service/collaboration_handoff_snapshot.go:128` source body 就是原 note，`:111` 重算 hash。这正是 C02 human-history transport 不能留给实现者猜的原因。
- `server/internal/service/collaboration_creation_fields.go:26` 当前参数只有 workspace/search；`:53` 仍列全部合法 nonterminal statuses。`packages/core/api/client.ts:1083` creation-options 也无 selected-project 参数。v2 已选“保留 unmanaged，不全局删 blocked”，但新增 observation envelope 仍需机械冻结。
- 2cContract:3/:5/:52/:82：trusted injected source、生产 nil unavailable、历史 receipt 不重建 callback、一个 consumed entry。v2 V2-18（:272）保持 **0 new command/input/group/run/Resume/launch/stage advance/approval consumption**；ETA 不是 execution deadline。不拿修复调用重发未知部署，不能放宽 2c 30秒 entry 或共享5秒 finalization。
- 2dContract:17/:50/:63/:65：当前 root + 原 artifact ACL、真实 issuer/consumer、numeric issue revision 与 decimal collaboration observations 分离，preview 是只读；每个真实 completion writer 最终重查，off/observe/enforce 都不能免 stage guard。V2:132 的 pending repair 必须原样保留这些绑定。
- 2dContract:69/:82：安全 stage activity 是闭合 metadata/hint，不可复制 block prose 或私有 evidence。阻塞 recipient 是本次处理人；S2 stable business owner 不变（V2:197）。无新 R38 repair capability。
- 2dDDL:33 明确 structural readiness 并不证明锁住864账本版本；:39 仍要求所有真实 writers/final checks/activation 的完成源与独立测试。当前 2d 草稿仅作为并行文件存在/整合风险，不作接受证据。

### 5. S3 实际剩余行为分类

| S3 要求与原行 | 稳定 accepted 2d 后可直接做的 bounded 行为/验证 | 仍缺的 plan / contract / 非本包证据 |
| --- | --- | --- |
| R01 :13；R02 :14 | 复用 T01–T06，不再造 timeout harness；补原 outcome 精确 ACK 与 changed/new late result 的拒绝、人工显式新 command 保留旧失败；评论/附件/结果分别计 ID/count。 | 真模型质量/账号仍非 S3 通过推断；新 ContinuationPlan:55 写 output-upload idempotency 但未指明主体/路由。runtime attachment 目前只有读接口；若要求 Agent 写能力，需明确 scope/contract，而非伪装输入附件测试。 |
| R02 :14；R08 :20 | 人类 multipart 上传到实际 API 后只丢 ACK，原 fingerprint/key GET 恢复、同键同字节显式 retry、changed-file conflict；comment/delivery binding 与 notice 分别去重；取消和 unknown 不发新键。 | 现有 U1/U4/U5 不是新产品层；重跑仅因 final-source 组合需要，不能重复归档 timeout 验收。U2/U3 实际端到端由主会话分配环境。 |
| R03 :15 | 55 个持久 comment ID，含 tombstone/reply/edit；实际 socket auth_ack→subscribe_ack，全部 limit/max_bytes/cursor，真实 Load more、quiescent final watermark，零 recovery POST。 | 不承诺恰好两页或跨请求 snapshot；byte budget 可多于两页。Web/匹配 Desktop 的完整 authored 场景及最终源码/环境绑定仍待主会话。 |
| R04 :16 | priority-vs-priority、priority-vs-handler 两提交顺序；协作 authority-first 403/409，普通 child 显式 numeric expected_revision 的 200/409，原 key refusal 不重算。 | 无需新产品合同；保留无 token 普通兼容控制，不能把 optional CAS 改成全客户端强制。最适合作首批两文件测试。 |
| R05 :17；R08 :20 | 撤一个 read basis 与撤全部分开；direct/page/cursor/attachment/native bytes/run/source/late GET 均当前核权；own minimal receipt 不授权私有内容。 | 已关闭 S2 ACL 缺陷不是新 blocker。并行 knowledge source closure/read contract 要从最终整合源重绑，不让 S3 owner 编辑 peer files。 |
| R06 :18 | 重复人/typed Agent/混合 mention，同 original event recipient/input 去重；edit delta/@all/new joiner、self/removed/display-only targets、通知与 actual launch 分开。 | 当前订阅是 active participation；独立 subscribe/unsubscribe 产品没有已确认合同，不通过插 ordinary issue_subscriber “补通过”。 |
| R07 :19 | 现有 note/recipient/previous_status/handoff/event 复用；v2 方案给四项和显式修复的实现方向。 | **尚不可直接产品实施**：root 接受 v2 + C02 两处公共 transport 冻结 + final 2d writer inventory/owned handoff。具体 migration 号码/readiness 必须重新分配。 |
| R08 :20；R09 :21 | 保留每动作原 key/body/digest、account/source/authority、失败与 link；新 source-matched API、production Web/匹配 Desktop、normal/race 及独立 check。 | R09 本身是执行验收义务而非已满足条件；不借旧 DB/installed Main/旧 timeout source 宣称 final candidate 通过。 |

原 timeout 边界：Timeout:9/:10/:11 已证明15秒 deadline、Stop 两顺序、丢已提交 ACK 后异 PID 原账本恢复及新 run 保留旧失败；:20 是 `feb87aee7` 加四 runtime/E2E 文件，另外两 unit 文件只在 root 跑；:30 / TimeoutCheck:66 未证明 Agent output upload、真模型、installed Main 或完整 S3。Matrix:15/:16/:17/:22/:23/:24 对应余项仍开放。

### 6. 最小、分离 write-set 的实现批次建议（不是本次实施）

**Batch S3-R04-01：仅两份新测试，先锁住已存在的 priority/handler/CAS 行为，不先做 R07。** S3 原 Plan:6 已要求这类组合评审；Review:6/:54 已认可测试主题，但派发前主会话仍应把本两文件范围绑定 accepted 2d source、批准计划及独立 package DB。

| Lane | 唯一 write-set（拟新增，当前均不存在） | 具体 testnames / assertions |
| --- | --- | --- |
| Server service tests | `server/internal/service/collaboration_recovery_priority_test.go` | `TestCollaborationRecoveryPriorityConflictBothOrders`（priority_a_first/priority_b_first）；`TestCollaborationRecoveryPriorityTransferBothOrders`（priority_first/transfer_first）；`TestCollaborationRecoveryPriorityOriginalKeyRetention`（same key/body、changed priority/target conflict、旧 refusal 保留）。 |
| Handler ordinary-child tests | `server/internal/handler/collaboration_recovery_child_revision_test.go` | `TestOrdinaryChildRecoveryPriorityAssigneeCASBothOrders`（priority_first/assignee_first，真实 PUT，同 numeric expected_revision）；`TestOrdinaryChildRecoveryMissingRevisionCompatibility`（省略 token 的原兼容语义单列，不要求409）。 |

- Service 复用 `newCollaborationQueueFixture`（`collaboration_queue_test.go:34`）、`workflowRequest`（`collaboration_workflow_test.go:23`）、`lifecycleCommitBarrierStarter`（`collaboration_lifecycle_test.go:592`）、`waitWorkflowBlocked`（`collaboration_workflow_test.go:454`）。不要改它们。旧 `TestCollaborationEditAndTransferSerializeAtCommit`（`collaboration_edit_test.go:304`）是现成两顺序模板，但其 :357 只断言 title/handler，所以新增测试必须只改 priority 且断言完整 winning pair。
- Handler 使用 `server/internal/testutil/db.go:38` / `http.go:22` 的 fixture/request；在真实协作 root 下由人工合法创建 ordinary child，不能插 synthetic stage approval 冒充受管路径。复用 `projectAssociationCommitStarter`（`project_association_concurrency_test.go:22`）及 `waitForWaiterBlockedBy`（`agent_builder_test.go:887`），核实 reached PID/lock waiter 两顺序，不用 sleeps 猜调度。原 `issue_revision_test.go:308` 使用 text bases，不是本批 numeric CAS 的替代证明。
- 每次比较原 ordinary row revision、collaboration revision、priority/handler、handoff/edit event/operation IDs、失败回执与执行账本 counts；不把前 handler 的403改成409，不把一次普通 child CAS 胜者称成 stage批准。
- 新测试以给定数据库不可用/配置缺失为失败，不继承旧 `t.Skip` 作通过。复用 fixture 的创建/清理接口，不改 TestMain、超时/pool、S2/peer 共享 writer 或生成代码。
- 执行形状：service `go -C <accepted-source>/server test ./internal/service -run '^TestCollaborationRecoveryPriority' -count=1 -json`；handler 用 `./internal/handler -run '^TestOrdinaryChildRecovery'`。每包先 normal，再各自独立 `-race`；只通过主会话按 package 分配的 launcher/新 DB 运行。本次没有执行这些命令。
- 本批若红，保留原日志并把最窄 root cause/所需 writer write-set 交回主会话，同一 server owner 获明确授权再修。**不是把红测试变绿所需的任何共享修改自动纳入本批。** 两 lane 可并行编辑，执行资源由主会话限并发，验收仍独立 trellis-check。

后续顺序：①现有 U/N/revoke 组合测试（优先新测试文件，非新增恢复器）；②单 E2E owner 新增 `e2e/collaboration-recovery.spec.ts` 与 `e2e/fixtures/collaboration-recovery.ts`，建议登记 `recovers 55 persisted comments after authenticated reconnect`、`recovers an uploaded attachment with its original key`、`deduplicates mixed mentions on original-event replay` 三场景；③C02/plan 冻结后才让一个 server owner 做 block persistence/repair/all-writer guards，Core/Views 消费同一 accepted DTO。所有名称均为建议，不是已 authored/passed inventory。

### 7. 真正阻挡项与交接

1. **B01 / dependency-owner gate**：2d 的最终 writer/final clock/M1/readiness/activation 未有可引用 accepted results/check；需主会话完成其既定复查、冻结组合 source/hunks并转交共享 write-set。不是重开已接受 2c 决策，也不是要求 S4 真支付才能写纯 S3 测试。
2. **B02 / acceptance gate**：v2 是 recommendation，原 review v2 仍 null。主会话/独立 checker 接受修正并记录引用，再冻结 S3 bounded 实施计划；不覆盖旧 requested-changes 或失败记录。
3. **B03 / creation transport**：冻结 V2:209 的 `stage_policy_observation` 完整字段/版本/nullable/unknown/source-project authority/请求关联。既定“managed 禁止初始 blocked、unmanaged 保持”不改；只需补机械公共合同。
4. **B04 / history transport choice**：冻结 V2:119/:194/:196 的 exact-history/repair-affected paging 入口、query binding/cursor/response，以及 activity 中 block/repair safe metadata 的闭合字段。既有 human run-handoff = runtime v1，故不能任选给它加字段。此处涉及接口选择，**交主会话，不自行选新 route 或变产品规则**。
5. **B05 / output-upload scope，条件项**：ContinuationPlan:55 的 output-upload 没有说明是既有人类上传输出物料，还是新 Agent/runtime 上传能力。前者可按原 upload/binding 合同做组合验证；后者当前无写入口，必须由主会话确认主体、准入与原键合同后另定最小 write-set。此歧义不阻止 R04 首批测试，但阻止把未验证上传能力算入完整 S3 通过。

不是 blocker：已归档 timeout-proof、已关闭 S2 ACL/C01/C02 correction、原 unattributed55P03 被后续 passing runs 覆盖的误解、F2 他任务清理注册失败、真模型/支付/生产部署/100人性能。它们保持各自失败/NOT_RUN/范围，不被本报告修复、归因或升级为 S3 全面通过。

## Related Specs

- `.trellis/workflow.md:7` / :9：先计划、持久研究；native implement/check 由主会话派发。
- `.trellis/spec/server/collaboration-foundation.md:28`：当前主体、参与/creator/handler 分离及既有原键 handoff。
- `.trellis/spec/server/collaboration-source-editing.md:53`：真实 source/member fence，普通 issue 与协作版本/写入口不混用。
- `.trellis/spec/server/collaboration-delivery.md:98` / :105：信息中断不替换 pending submission；只有原 initiator 的精确验收。
- `.trellis/spec/server/collaboration-stage-approvals.md:27` / :41 / :43 / :59：成员锁、原签字人、最终 DB clock、R1→私有证据 fence；不拿业务 owner 代替 actor/source authority。
- `.trellis/spec/server/collaboration-runtime-recovery.md:79` / :94：proof/telemetry 优先次序、原身份恢复，不再 Claim/Permit/Provider；unknown 不变新身份。
- `.trellis/spec/server/database-test-isolation.md:1`：新 package/normal/race/实际 API 环境分配由主会话负责，不能借 peer DB。
- `.trellis/spec/core/frontend/collaboration-workbench.md:64`、`.trellis/spec/views/frontend/collaboration-detail.md:126`：原草稿/key/fingerprint及晚响应 access fence；共享 Web/Desktop 表单，不造新 store/platform 分叉。

## External References

无外部引用或外部模型调用；版本/行为来自本仓库一手 code/spec/retained reports。PG `timestamptz` 微秒值与 Go persistable-value 的选择是原 Review:47 / V2:143 的合同前提，本次未连接 DB 或宣称验证真实数据库时钟。

## Caveats / Not Found

- 原 v1 Markdown/JSON及 Review MD 三份 SHA 绑定吻合；原 review status/v2:null 未变。v2 本次只有 MD，没有同名 JSON；其 :288 提到 JSON source hashes 不能当作现成 artifact，本同名 JSON 独立绑定本次来源，不冒充作者或 checker 的原结果。
- 保留两次 v1 final manifest 147 路径只读 SHA 比较：初次142匹配/5个文档漂移；末次129匹配/18漂移（6个文档、12个共享 server source/test），两次均0 missing。后者包括 router、handoff/workflow/lifecycle、activity、普通写入口等，原摘要不能继续当一致源码。38份本报告决定性来源相对自己的重绑定无新增漂移，行号已重解；完整旧/新 SHA 与时点见 JSON。**都不是最终 2d 全图构建或接受证明**，不靠相同哈希升级草稿。
- bounded `rg` 在 server/internal、server/pkg/protocol、Core/Views collaboration 搜索新两字段、repair action/relation/workflow/marker，没有匹配；当前 R07 产品行为未实施。没有编译或穷尽所有未来写入口的声明。
- 本次产品 tests/lint/typecheck/build、DB/API/浏览器/Provider/外部模型、git operation、任务状态操作均 **NOT_RUN / 0**；只读 source/spec/报告、哈希/矩阵/文档一致性验证。建议新测试不计为已 authored 或已通过。
- 路径/行号以 JSON 绑定的本次源码为准；未来 2d/peer 合并后必须重解 line 与 inventory，不将旧冻结源覆盖共享文件。父矩阵及 S3 状态保持原值。
