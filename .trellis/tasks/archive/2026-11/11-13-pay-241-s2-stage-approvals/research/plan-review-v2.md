# S2 技术规划第二轮复核

结论：**首轮的主要事务、版本和客户端方向已收敛；S2仍不能启动。** 当前阻挡是尚未答复的产品规则，以及类型化测试报告和逐动作协议的少量具体入口。外部系统暂未就绪不妨碍实现S2内部事实合同，但不能因此把真实集成验收记为通过。

- 审查人：原生 `trellis-check`，`/root/s2_plan_check`；日期2026-11-13。
- 只新增 `plan-review-v2.md/json`，首轮报告保持原样。没有修改产品、规划、数据库或Git，没有运行产品测试。
- 审查绑定当前PRD/design/implement/JSONL摘要，见JSON。后续文本变化须按实际修订复核，不继承未经检查的通过结论。

## 首轮六项逐项处置

| 首轮项 | 第二轮结论 | 依据与仍需完成的内容 |
| --- | --- | --- |
| F01 原签字人代际 | 技术主体已解决；配置权限仍待决定 | `design.md:78`冻结原签字人participant/account/credential代际，禁止恢复账号复活旧签字；当前参与者集合约束与锁后重查可复用现有用户锁集合。仍未确定谁能变更角色/业务责任人，见P01。 |
| F02 语义版本与CAS | 设计方向已解决 | `:79`分离已审内容/代码候选与操作CAS，QA/部署/验证为追加证据，submission精确绑定候选。实施时此分层也必须适用于普通child，不可把child正常状态变化的revision误当代码变化。 |
| F03 stage-only最终检查 | 设计已解决 | `:80`明确私有同事务stage guard、NotApplicable/Prepared都执行最终检查、数据库后采样时钟和submit前后校验。原M1 token及replay语义保持。最终校验须识别合法before→after状态/生命周期推进，不能用“当前epoch等于旧epoch”误拒自己的完成。 |
| F04 建议与child准入 | 家族/唯一物化技术已解决；适用范围与已有任务处置待决定 | `:87–91`固定proposal家族、当前pending、唯一accepted、1–20child、原子创建及非派发。严格人工child准入比原需求更强，且已有child/重开后的准入恢复未定义，见P02/P03。 |
| F05 证据入口和资格 | 外部事实边界已解决；测试报告自动入口仍缺 | `:99–108`固定GitHub新鲜度/成功集合、VCS不足时unsupported、human_attested不代替部署、无外部事实时等待。类型化测试报告目前仍是字段描述，尚无实际进入路径或自动失败关联触发点，见T01。 |
| F06 客户端/恢复/事件 | 归属及机制已解决；需按动作冻结最终wire | `:112–118`明确承接M1协作typed消费、GET-onlystage恢复、最小确认及安全事件投影。逐动作确认字段仍应闭合，见T02；不再把已知M1缺口留给实施时发现。 |

## 尚需产品决定

主会话在本轮复核期间已补充询问：推荐AI建议物化必须审批，人类手建保留原权限；首次启用或重开时明确核对旧child/代码版本，不自动批准、重建或派发。角色配置推荐source owner/admin且当前对任务有权。以上均**仍待用户答复**；`:90`的“所有child禁止”仅是待选的更严格方案，不是已确认合同。本报告不把任何推荐写成用户决定。

### P01：角色配置权和“实现责任人”只能有一个确定来源

`design.md:17/19/78/95`明确了谁可签字的候选模型，但没有决定谁可新增/移除签字人、变更固定业务责任人、为代码候选指定实现责任人。当前 `workflowRoleAllowed`（`server/internal/service/collaboration_workflow.go:360`）没有新配置动作；不能以普通Edit/ControlAI兜底，否则QA可以换一个发布人甚至把自己移到发布角色，消解职责隔离。

建议在当前集中问题收敛时补一行明确授权表：项目策略仍由源workspace owner/admin管理；任务角色配置由被选定的明确人类角色控制，并与当前参与权/源权限相交；审批人必须先成为有效参与者，配置不隐式邀请。明确变更使哪些签字失效。

“配置或真实人类提交者”仍是两种不同规则。建议将无自评定义为**已配置的实现责任人不可签其代码评审**，记录真实候选提交者作为独立归因；不声称已证明Git提交作者与Multica账号相同。若允许候选提交者直接决定实现责任人，必须防止其换一个用户ID避开自评约束。这属于产品规则选择，不由实现者任取分支。

### P02：严格child准入与原需求相容，但明显更强

父PRD R02（`../11-13-pay-241-collaboration-acceptance/prd.md:28`）要求的是“未批准的拆解不能成为可执行子任务”。新 `design.md:90`进一步禁止**所有**人类直接创建或挂入child，连无Agent分派的人工子任务也必须先建建议再由产品批准。这能避免猜测文本来源，技术上自洽，但不是原句唯一可能的实现；已有“项目显式启用”建议不自动决定这项额外工作流负担。

建议将此严格行为明确纳入产品确认及启用说明；如果采用，就保留单一准入路径，避免另加可伪造的“manual”绕过字段。如果希望人工手建仍可直接进行，则必须另定可信真人/指定审批人路径如何形成精确版本决定，并保留自动化候选的准入证明；不能仅凭CreatorType、显示名或请求正文判断。

该决定只覆盖受管的collaboration root，不应把同项目的所有普通parent都纳入职业阶段。

### P03：项目启用时已有child与root重新打开的处置未定

`design.md:19`使项目启用后未配置的既有协作任务进入缺项状态；`:91`又要求自动拆解项有approved映射且root生命周期有效，但没有给启用前已有child、旧批准child在root重开后的重新准入入口。若直接加执行检查，这些合法既有普通任务可能被永久卡住；若缺映射被视为legacy允许，则又有旁路。

需要在下列事实间作明确选择：仅新root纳管，或启用时所有root纳管并提供显式已有child核对/准入；重开后原child是只保留历史、可由指定人重新确认，还是必须新建关联工作。不能让迁移自动制造批准。建议提供受管root的显式既有child核对/重新授权，保留同child身份和历史，不为补映射重复创建任务；若首期不做，应限制启用条件并清楚拒绝不可迁移的在途任务。

这涉及实际兼容行为，应与P02一起收口。S2当前产品问题未答复，不能用本审查替代用户选择。

## 需要补齐的具体技术合同

### T01：类型化测试报告由哪个真实入口进入，何时“自动关联”

`design.md:101/106`已经定义报告字段和读权，但“Agent只能提供新报告”仍未指向生产调用链。当前 `protocol.CollaborationRunOutcome`（`server/pkg/protocol/collaboration.go:109–119`）只有结果正文及record，不含逐测试case集合；`RecordCollaborationRuntimeOutcome`（`server/internal/handler/collaboration_runtime.go:242`）走严格解码，不能让实现者向旧body随意塞字段。`business_outcome=failure`也可能只是进程/Provider失败，并不能自动捏造失败测试case。

应在服务端lane里明确选择并冻结其中一个真实入口：

- 受运行business capability约束的独立typed测试报告写入，复用 `lockCollaborationRuntimeRun/currentAdmission/requireReservation`、原run/attempt/nonce/event去重，之后由已提交报告触发阶段失败证据投影；或
- 已存在的不可变record/附件中有明确版本的typed测试报告格式，服务端在实际结果入库或选取时解析它，区分自动解析与人工补录。

前者需将runtime handler/protocol/client/报告producer接线列入明确文件归属；后者必须指出确切格式及触发函数，并把人工补录写成human_attested。二者都不能把“人类挑一个run并填case名”记作自动失败关联。没有typed测试报告的普通超时/失败只显示运行失败和可得日志，测试结果保持未知。真实case结果、日志存在性、代码/构建绑定与输入来源分别验证。

新报告写入与已有QA签字分离；丢失回复仅恢复原报告事件，新报告不会自动替换QA已经选定的版本。测试新增一条“进程失败但无测试报告”负例，防止将S3运行故障错误升级为QA失败用例。

### T02：冻结逐动作请求/确认及最终writer接线表

`stage_confirmation`现在列action/proposal/candidate/version/digest/decision，但策略配置、角色配置、候选创建及拒绝未必都产生decision或candidate。应以判别联合分别列必填身份，不能填空字符串、零UUID或生成假的decision来凑通用schema。每动作写清方法/路径、真实actor、前置版本、成功/拒绝/进行中/404恢复形状；然后才能让服务端和Core并行。

实际完成接线可直接以既有入口制表：`PrepareDeliveryMutationInTx`、协作 `ChangeLifecycle`、`SubmitAcceptance`、R38及 `CheckDeliveryProjectChangeInTx/CheckDeliveryProjectDeleteInTx`。对普通child执行，已有可复用入口是 `TaskService.checkIssueExecution`（`server/internal/service/issue_admission.go:12`），它被 `EnqueuePreparedIssueTaskInTx`（`issue_task_transaction.go:51`）及真实任务Start事务（`task_start.go:70`）调用。该helper当前明确按实际source拒绝协作，**普通child保持自身执行合同**；S2的新条件应单独按可信受管映射检查，不能改成“只要parent是协作就一律禁止”。

child准入校验必须用调用者实际事务、在queued→started之前与root授权变更串行，不能只在池查询的enqueue前检一次。保持原workspace/catalog/W01/child锁顺序；必要的root/策略交叉锁用明确的NOWAIT或锁前收集，不能child→root与root→child形成阻塞环。映射以child的持久身份查，不能移除parent就丢失受管来源。

候选语义版本分层还需覆盖child的普通revision；最终stage guard需比较预期before→after合法变化，包括完成时lifecycle推进，重放分支不产生新guard消费。将这些加入A08/A09测试表即可，不需要第二套完成授权。

## S2与S4的责任分界：本轮认可

第二版明确了合理边界：S2持有审批、不可变外部事实的**内部数据/消费合同**和状态门禁；S4才实现具体支付、GitLab仓库CI、部署/回滚适配器与真实执行验真。没有合格外部事实时，生产流程停在等待结果。这不会重复Git物料自动合并任务，也不会让手填备注或测试flag取得发布成功身份。

S2内部接口应接收服务器已认证的集成上下文及精确外部operation/event/version，不能公开接受 `verified:true` 的HTTP入口。S4负责签名/当前信任代际/环境/批准消费和实际调用，使用S2合同登记事实；不要在S2另造假通用部署执行器。迟到的真实结果应作为绑定原操作的历史事实留存，是否仍可推进阶段另由当前门禁决定；记录已发生事实不等于恢复撤销的执行权。

合成positive fixture可以直接通过测试专用受控连接器或正常生产数据形状验证S2消费，且应真实经过对应业务handler/事务。其输出必须标明synthetic，**不能记为S4的真实发布/支付/回滚通过**。因此S2报告可以完成平台协议断言，同时让TC09–11、TC17–19的外部执行子断言继续PARTIAL/NOT_RUN。不能将S2的fixture资格偷换为父任务整例PASS。

GitHub当前snapshot确实分页到末页，并在跨页head变化/游标异常时失败（`server/internal/integrations/ghsnapshot/snapshot.go:139–201`）。它可作为30分钟内的数据库观察；其标准化对象只保留name/status/conclusion/details URL，不保留所有原生check attempt身份（`:47–57`），也不等于远端当前保护规则/发布验证。S2应保留这个事实边界，S4按目标实例补真实关联证据；不必为S2重做整套Git连接器。

## 文档收敛与验证

收到产品答复并补T01/T02后，把追加的F01–F06合并进正文：`:56`的“必要时Finish”、`:68`的“若发现客户端扩张”和`:47`的“时效待固定”已经被后文取代，应移除重复/矛盾措辞。PRD同步纳入已确认的严格child兼容边界；不要只在design里新增行为而让验收仍看不见。

- 首轮F02/F03已达到可实施的技术设计；F06机制与归属已达到该程度。F01/F04/F05的剩余项如上，不重复否定已经补齐的部分。
- JSONL路径/任务结构检查结果见JSON；本轮仅验证规划和源码事实。
- Lint / TypeCheck / 产品测试：NOT_RUN。本审查不把先前service50/handler31基线转算为S2通过。
- 新增两份v2审查；未改首轮结果、父矩阵或其他任务。最终 `ready_for_implementation=false`，原因是当前具体待决项，不是外部环境缺失本身。
