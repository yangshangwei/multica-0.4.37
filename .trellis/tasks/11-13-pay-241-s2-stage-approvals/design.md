# S2 技术设计

状态：产品规则已确认、技术合同已收敛，最终独立规划复核已通过（2026-11-13，research/plan-review-final.md）。用户已授权实现；本文描述待实施合同，不把它写成当前已有能力。

## 复用与边界

数据链：当前任务与配置 → 不可变建议/交付候选 → 服务器观察的 PR/CI/运行/制品引用 → 当前真人的精确决定 → 阶段变化 → 原交付验收与 M1 完成事务。`in_review` 保持交付验收，现有 `issue.stage` 保持父子执行分组；新增固定业务阶段不占用二者。

- 使用 `CollaborationService.mutateCollaborationWorkflow` 的真实主体、账户/参与者 epochs、锁顺序、拒绝回执、原键恢复、事件和通知。新动作须加入封闭 allowlist，并在动作内重查固定签字人。
- 普通子任务由 `IssueService.CreateInTx` 在批准事务内创建。批准、候选终结、所有子任务和映射原子提交；plain accept 不 enqueue、不发布执行事件。不能调用会提交并自动派发的 `Create`。
- 阶段完成检查加入 `PrepareDeliveryMutationInTx` 的新消费和无绑定新完成分支；在 M1 off/observe 提前返回之前执行。原 M1 授权仍独立适用，不新增第二套完成令牌。
- 历史 consumed 重放仍是历史回执；不重新执行、不要求重审当前阶段。当前读取权限和原操作者校验保持。
- Git 物料仓的定义、自动送审/合并和 D8 方案编排不属于本包；F2 报告仍是证据而非审批。

## 已确认的产品规则

项目显式启用、默认关闭；每任务固定业务责任人及产品/代码评审/QA/发布审批人。允许明确配置兼任，但代码候选绑定的实现责任人不得自评，QA 与发布审批人分开。最终验收沿用当前发起人，失能发起人的管理处置沿用 R38，不扩大为普通任意代理验收。上游版本、配置或当前权限变化需重新审查其适用性。

角色、稳定责任人与实现责任人的配置权按 PRD 已确认规则：来源 workspace 当前 owner/admin，且具备当前任务参与/管理权。实现责任人由该配置明确指定，候选复制其用户 ID 和配置修订；普通候选提交请求不能任填另一个实现责任人。真实候选提交者另作归因，不从 Git 显示名推断实现人，也不在缺配置时以提交者自动补齐。变更实现责任人使依赖旧配置的批准不再适用。

项目启用后未配置角色的协作任务应显示缺项并拒绝推进，不由迁移静默选人。项目关闭必须由当前 owner/admin 用理由和版本 CAS 明确执行；保留全部决定。阶段策略与 M1 策略、AI assisted/manual、Git D8 自动启动开关分别存在，不互相赋权。

## 持久事实

最小四类事实，不使用任意可编辑 metadata 承担权限：

1. **项目阶段策略修订**：workspace/project、固定合同、enabled、revision、操作者、理由、时间；无记录为默认关闭，缺表/不支持合同为不可用而不是关闭。
2. **任务配置与当前阶段**：唯一业务责任人、签字人、角色修订、当前候选、阶段、生命周期/版本。角色变更写不可变事件；处理人转交不改业务责任人。
3. **不可变候选**：建议或交付类型，任务、版本/前版、内容摘要、来源运行及服务器证据快照/摘要、提出者和时间。修改产生新候选，旧版保留；不能用同候选 ID 改写内容。
4. **不可变决定**：精确候选/内容/证据、角色和政策版本、实际人类、阶段/决定/理由/时间；撤销为新终结事实。建议接受保存固定 child IDs，候选 materialization 唯一约束防并发或换键重复。

所有表无外键；所有新索引单独使用 `CREATE [UNIQUE] INDEX CONCURRENTLY` 单语句迁移。数据访问使用工作空间与任务边界，历史分页有界。回滚优先显式关闭政策；降级不能删除审计来绕过阻挡。

## 证据与阶段

业务阶段固定为研发、代码评审、QA、发布、验证；返工以新的决定返回研发。批准、执行结果、验证结果分别保存。

| 流转/动作 | 必须重查的事实 |
| --- | --- |
| 建议接受/修改后接受 | 当前指定产品审批人、候选内容/来源版本、源工作空间子任务创建权限；修改后的具体内容先成为可见新版本；未批准零可执行子任务 |
| 进入代码评审 | 明确选择实际同任务/子任务关联 PR、当前 head 与集成 trust、非 reference-only、当前 head 的 CI 快照和检查集合；缺失/失败/未知拒绝 |
| 进入 QA | 当前代码评审人对同一候选/PR/CI 版本的有效批准；实现者不能自评 |
| QA 通过/退回 | 当前 QA 对绑定代码/构建/测试运行和日志作明确决定；失败源保持不可变，不因新 Agent 输出改写人工判断 |
| 发布批准 | 当前指定发布审批人、有效 QA、精确构建/目标环境；批准仅是授权事实 |
| 发布/回滚结果 | 保留来源、运行、目标、构建、真实结果与证据版本；只有 F05 的受控集成事实可证明已执行，human_attested 仅为说明；无合格结果不能显示已执行 |
| 回滚批准 | 精确原部署、回滚目标和指标证据，指定发布审批人明确确认；建议无执行权限 |
| 正式交付与最终完成 | 验证阶段事实完整、当前有效签字、原协作精确提交/发起人验收，以及适用时的原 M1 授权 |

GitHub 证据使用当前 PR/head、`snapshot_head_sha`、快照时间与逐检查状态；VCS 还须具备 F05 的仓库绑定，否则 unsupported。服务端生成检查集合摘要；客户端 `passed=true` 不作为通过依据。同事实刷新时间不应产生虚假内容变化，实际 head/检查/信任版本变化必须失效。GitHub 门禁要求非空完整集合、观察时间不晚于当前数据库时间且不超过30分钟、每项规范化 `status=completed` 且 `conclusion=success`；不能继承 UI 宽松 fallback。

协作测试运行先经 run→session→issue 校验；失败记录绑定原运行/尝试/日志。普通制品继续走 M1 原 observer，不把协作 run 伪装成普通 artifact。外部参加者能读协作不意味着能读源 workspace 的私有制品；涉及源证据沿用原权限，无法读时明确阻塞。

S2 的本地测试可以使用已标明来源的合成外部事实验证正路径，但不得将人填内容或合成事实当真实支付、CI、部署、回滚验收。S4 保留真实接入与签字证据责任。

## 绕过、锁与恢复

- 正式提交进入 `in_review` 前检查阶段；拒绝不得绑定附件或新增 submission。
- 阶段策略覆盖 M1 off/observe/enforce 的组合；新完成的两条 Prepare 分支都检查，实际 writer 在提交前必须消费同事务的私有 stage guard；不能只挂在 M1 Prepared 分支的 Finish 内。锁持有到真实 writer commit。
- 项目移动/清空/删除、候选子任务换 parent/证据变化不能逃逸。复用既有 project-change/delete 实际事务；阶段策略需显式关闭才能退出。不会自动给所有普通任务添加全阶段流程。
- 对未批准拆解的可执行 child 创建/派发入口列出具体防护：通用/Agent/批量/旧客户端不能拿候选文本或任意 ID 当作批准。批准创建使用内部借用事务的明确许可；不得公开可伪造 marker。
- 每次写入冻结原 request ID、payload digest 和预期版本。结果未知仅按原键 GET 恢复；变更载荷须新明确意图，不自动追最新候选重试。
- 读投影只包含当前可读事实；被撤权用户至多取得现有最小原操作回执，不恢复正文/证据。通知复用事件 ID 去重，不建立第二个 outbox。

## 客户端

在 `packages/core/collaboration` 增加固定协议/schema、受保护 Query 与原意图恢复；在共享 `packages/views/collaboration` 增加阶段面板和表单，由现有共享 detail 挂载。两种平台与来源/账户入口使用同一实现，不新增平台路由。

页面显示责任人、当前阶段、候选及版本、已发生决定、当前是否适用、缺项和下一步。服务器能力只控制操作可见性，所有权限仍由后端检查。新 API 严格解析，未知协议/畸形返回不可操作；旧服务端缺能力显示未知/不可用，不能显示门禁已通过。

API/CLI 入口与共享 UI 的完成授权范围仍由 M1 合同决定。本包明确补齐 F06 的协作目标 schema、证据选择与授权消费接线，保持 ordinary 变体；不强转 kind 或绕过证据选择。

## 验证与回滚

先验证现有交付、M1、撤权/幂等基线；新的阶段行为先失败回归，再实施。服务端 package normal/race 使用分开的任务数据库；浏览器另库、匹配源码 API/production Web/Desktop。详情、列表、回复及原键恢复均检查当前权限。所有失败、跳过、环境边界、构建哈希和实例身份保留。

实现与检查代理文件范围在 implement.md 固定。主会话整合结果与完整24例账本。阶段仅在必需断言通过及独立检查通过后关闭；真实外部缺项保持开放，不以本地绿色测试替代。

## 独立评审后的技术收敛（F01–F03）

- **签字人代际**：每个决定另存原签字人的participant/account epoch、真实凭据类别/服务器身份/版本与有限有效期；不存bearer，不用消费人的凭据模拟issuer。消费重新核查所有依赖签字人，移除再加入、角色移除再恢复、密码或PAT撤销均不能复活旧批准。复用M1现有issuer凭据核查语义。所有参与签字人必须属于当前参与者集合，使原workflow锁前用户收集覆盖；配置不能指向未参与/失效账号，锁后重新比对集合。
- **版本分层**：候选固定已审内容与代码基线摘要；当前workflow revision/epochs仅做本次CAS。需求标题/描述/验收标准、代码head或构建变化使对应候选失效；合法状态/处理人流转不改候选内容身份。普通child同样区分语义内容/代码摘要与它的普通revision，正常状态更新不能让已审代码自己失效。QA、部署与验证分别是追加证据，各决定绑定其直接依赖证据ID/摘要及前序批准ID，不把后来证据写入初始总摘要。交付submission明确绑定同候选，重开新lifecycle须新审核。
- **所有新完成的最终检查**：扩展原Prepare结果携带仅供同一pgx.Tx使用的私有stage guard；它不是持久授权/token。实际writer在新状态写入后、commit前，无论M1返回NotApplicable还是Prepared，都调用统一最终stage校验；使用数据库clock_timestamp()重核到期、当前角色与证据。guard保留before及该writer允许的after转换，核对真实submission/状态和本次合法lifecycle推进；不能要求完成后的全部epoch等于完成前值，也不能接受任意更新后的版本。原M1 Finish仍核其原授权。原操作重放只返回历史，不重做新完成。submit也必须在绑定附件/正式提交前后经同一阶段事实检查。

F04–F06按下列合同和已确认产品边界继续独立复核；不得将文档修订本身当实现或验收通过。


## F04：建议家族与子任务准入合同（最终规划合同）

- 稳定proposal_id归一个协作root；每版candidate_id/version/content_digest不可变，只有一个current pending。未决定版本可由有权提出者提交新版本，旧版变superseded；reject终结该版本并留理由。接受只能针对当前pending，accepted对整个proposal唯一；已经物化的proposal不支持在首期修订后再物化，明确返回proposal_already_materialized，需另建明确的新建议意图。
- 首期候选由有权人类从当前可读的Agent运行/评论整理，或直接手写；保存来源原run/comment及其摘要/版本。运行与Agent作者来自服务器，客户端不能冒充Agent直接签字。现有Agent输出协议保持原职责，本包不为自动解析自然语言而创建任务。要求中的接受、修改、拒绝各有明确界面，修改后接受先保存可见新版本再批准。
- 允许1至20个子任务；ordinal固定，标题/内容/责任/项目等沿用普通Issue边界。批准者必须同时有指定产品审批权和当前source workspace创建权。entitlement在开启借用事务前解析，事务内依原CreateInTx重核。任一个child失败回滚全部决定/映射/编号写入，不产生可执行前缀。
- 保留当前真实人类的普通create/comment-to-child/后挂parent权限；这些操作不能批准AI建议或建立其物化映射。受机器凭据/运行身份驱动的自动拆解在受管root下必须经已批准proposal的内部物化事务，否则stage_proposal_required；不能靠自报creator_type或可伪造HTTP marker取得许可。其他普通任务与未启用项目保持原行为。普通人工child要作为阶段交付证据时，仍须由指定产品人明确确认其身份和代码版本，不能因存在就自动成为已审范围。
- 批准物化不自动派发。Agent赋值/运行/后台启动仍走原当前invoke/triage/manual/Stop门禁；未有approved proposal映射或已失效root生命周期的自动拆解项不能被原有入口当作已经获批。每条替代入口须在实施测试表列出实际符号，不能只测新endpoint。

## F05：最小证据来源与解锁合同（最终规划合同）

代码候选的基线固定root生命周期、已审需求内容摘要、受权配置中的实现责任人用户ID及配置修订、普通child语义版本、PR provider/ID/repository/head/trust revision及制品绑定。真实人类提交者是独立归因，不能替代配置或证明Git作者身份。操作CAS与这些语义身份分开。

| 来源事实 | 生产入口与允许决定 | 不允许的推论 |
| --- | --- | --- |
| 当前GitHub PR/CI快照 | 现有连接器已存的实际PR关联/current head/snapshot head/非空完整检查集合；最多30分钟，全部必须明确success；服务器产生内容摘要，刷新时间相同事实不改变内容ID | UI rollup、客户端布尔值、空/unknown/skipped/neutral、旧head不能解锁评审 |
| VCS CI | 仅当能核对连接+仓库+head+检查身份时可用；现有connection+SHA缺仓库关联的事实明确unsupported，S4补真实仓库绑定后使用 | 不把跨仓相同SHA当相同流水线 |
| 类型化测试报告 | 保存issue/run/attempt/代码/构建、逐用例ID/outcome及原records或附件引用/hash；服务器校验同任务、当前读权、实际run/attempt与日志存在。失败run不可声明全成功；Agent只能提供新报告，QA人类决定另存 | business_outcome一个值不能代替失败用例；从自然语言猜通过无效 |
| 人工评审/QA决定 | 当前指定人对精确基线、报告与上游批准ID签字；人工新决定可纠正Agent结论，旧判断不被覆盖 | human_attested不冒充连接器实测、发布或回滚完成 |
| 外部执行事实 | 仅从服务器内部受控集成边界登记的不可变部署/回滚事实，绑定受信集成/信任代际、外部operation_id、精确批准、环境、前后制品、结果和观察版本；普通人类/Agent请求不能自行填verified | 发布批准本身、人工成功备注或模拟标记均不能成为已执行 |
| 合成测试数据 | 自有数据库/本地测试连接器建立同样的真实生产数据形状，测试报告明确synthetic及其版本；不增加生产调用方可选择的simulation bypass | 只能证明平台合同，不能填S4真实集成PASS |

测试报告的失败case/log引用与不可变run一同投影，引用无效或当前不可读时明确缺项；不能从不可读报告中泄露case名。新报告默认未选择，不自动替换已签QA依赖。QA明确选新版/退回后产生新决定，原失败和人审不改写。

S2保存外部执行事实的数据/内部消费合同与门禁；真实支付、GitLab仓库级CI、部署/回滚适配器及真实证据由S4承接。未有合格外部事实时生产流程停在等待执行结果，不允许用人工备注完成；S2正路径可用明确标记的隔离fixture验证。批准和撤销绑定精确原部署、目标制品与指标记录，实际回滚是另一个不可变事实。

## F06：共享客户端与完整事件接线（最终规划合同）

S2明确承接M1协作完成消费端，保持现有ordinary消费者：Core将目标建模为闭合判别联合，对collaboration submission/self/admin的不同约束使用已有服务器合同；普通schema的evidence issue=target规则仅属于ordinary，不能强转kind。新增协作人类完成preview/candidate选择入口时仍用原M1 observer/issuer检查与源普通child授权，不扩大外部参与者制品读权。共享workflow在完成时携带原M1绑定和精确submission；S2+M1 enforce正路径必须在Web/Desktop验收，不用off演示替代。

新stage命令复用受保护transport与Query，但单独定义GET-only恢复：原键GET404仍是未确定/待查，不自动POST；stored refusal显示其原status，进行中/畸形/未知结果保持原意图。用户明确放弃或依据权威新状态发起新操作，才能新建request ID。旧通用collaboration恢复行为不在此包偷偷重写。

最小stage_confirmation按实际action作封闭判别联合，不能要求所有动作都有proposal/candidate/decision。客户端验证原issue/project、request key、动作及该变体的精确身份后才退休请求。完整子任务映射经当前权限读取，不放入撤权后最小回执。opaque确认不带正文/附件元数据/日志/凭据。

| 动作变体 | 请求与成功确认必须对应的身份；非本变体字段不伪造 |
| --- | --- |
| 项目政策修订 | source workspace/project、原request、预期policy revision、明确enabled与理由；确认新policy revision，不要求issue/candidate/decision |
| 任务角色配置 | issue、原workflow观察与角色修订、明确配置；确认新role revision，不制造candidate/decision |
| 建议/代码候选创建或修订 | issue、原request、候选种类及父版本/内容摘要；确认实际candidate ID/version/digest，只有建议带proposal ID |
| 接受/拒绝、评审、QA、发布/回滚批准或撤销 | issue、精确候选/直接证据/前序决定及当前角色政策观察；确认相同依赖和实际decision ID，拒绝成功也是已保存的人类决定 |
| 旧child范围确认、阶段推进 | issue、当前lifecycle与精确范围/候选/直接依赖；确认实际范围/阶段修订，只有确实产生决定才返回decision ID |

创建类操作尚无服务器生成的ID，按原request、种类、父版本/内容身份及合法的新ID确认；不要求预先知道新UUID，不用零UUID补位。已有候选/决定的操作仍逐项比较原冻结身份。

项目政策使用源workspace项目政策写事务的权限/锁边界，原键与digest保存在自己的不可变修订中；不向现有只支持create/issue scope的collaboration_operation伪塞project操作。issue级动作复用原workflow operation和 `GET /api/collaboration/issues/{id}/operations/{requestId}`。所有新请求/确认字段、对应方法/路径与错误码在服务端首增量的wire-contract中逐动作列全后，客户端才并行；这是同一实施owner的接口交付，不是新的产品审批。新stage版本用规范十进制字符串，既有ordinary revision保持其原wire类型。

成功返回精确variant；已提交拒绝返回原status/code且不带成功确认；进行中为未终结；GET404为未知原结果，均不自动POST。项目政策的原键读取同样只返回该actor/project原操作的最小确认，不用“当前政策恰好相同”推断旧请求成功。

服务端stage事件显式进入activity的安全投影allowlist，只输出阶段、候选/决定引用、版本与实际操作者；不得透传任意payload或私有日志。Core schema、共享timeline与双语notification映射同步增加这些kind。finishWith复用既有event/outbox/通知去重，新的Query keys纳入现有realtime失效；Zustand不缓存服务器审批对象。


## 首次启用、重开与报告生产入口

- 项目owner/admin启用时只创建政策修订，不自动指定审批人或批准历史child；受管root处于needs_confirmation，仍可读历史、讨论、停止及人工手建。新阶段推进/新自动拆解须完成角色配置和指定产品人的明确范围确认。
- 产品人确认现有child时记录child IDs/当前版本、代码候选及root lifecycle；复用已有身份，不创建或重新派发。重开使上一lifecycle批准不能消费，需要新的确认；已有失败与原key历史仍可读。已在执行的旧工作保留原授权/终结链，不把启用当自动Stop或重新派发，旧无subject报告不具备新QA资格。
- F05采用独立POST /api/collaboration/runtime/runs/{runId}/test-reports、GET test-subject、GET by-request最小ACK及当前人类report读取；CLI使用collaboration test-report subject/submit/receipt，只接受既有mcb与完整run identity。
- 具体封闭JSON、大小限制、错误、日志与代际合同以 research/test-report-contract.md 第2–8节为实施依据。服务器从mcb推导attempt/nonce；报告本体复用run_event(kind=test_report)、固定每attempt producer key，不新增平行报告存储。
- 在create-only typed Agent input同事务绑定command的test_subject，first claim冻结唯一subject、prompt commands和真实Git preparation；不从上传时的当前候选推导。新增窄subject表及单条并发唯一索引；无Git冻结/旧run明确不可用于QA，不以客户端SHA补证。
- 每run/attempt一份不可变报告；failed/error由cases派生，skipped为incomplete；日志可用同run/attempt已保留record或首次有界inline，并使用现有redaction。报告提交发布原outbox hint，不变更business_outcome、创建评论/child、自动签QA或推进阶段。
- 新的typed报告不进入旧records判别联合。UI专门按原candidate/run/attempt显示自动关联的失败case/log和适用性；原运行之后失败/未settled时，即使报告全passed也不得QA通过。所有来源标runtime_reported，真实CI/外部执行仍由各自来源证明。

## 实际写入与准入接线清单

| 现有入口 | 本包必须加入的检查 |
| --- | --- |
| `IssueService.CreateInTx`，通用/快速/comment-to-child创建调用方 | 按真实凭据区分人工手建与机器拆解；后者须内部获批物化许可，拒绝全回滚；人工保持原权限 |
| `updateIssueAtomicallyOnce`（Update/Move/Batch共用） | 受管来源按child持久身份查，parent移除/替换不抹掉映射；项目/内容变化在实际事务中检查 |
| `TaskService.checkIssueExecution`，`EnqueuePreparedIssueTaskInTx`，`startTaskAtomically` | 保持原ordinary-source判定，另外核对自动物化来源及当前root生命周期；不能只在池查询enqueue前检查，必须覆盖真正queued→started之前 |
| `PrepareDeliveryMutationInTx`及普通writer/`WriteIssueStatus`/`ChangeLifecycle`/R38 | 两条新完成分支、所有M1模式均携带并最终核验stage guard；原replay不重新消费 |
| `SubmitAcceptance`，`CheckDeliveryProjectChangeInTx`，`CheckDeliveryProjectDeleteInTx`及相关真实删除事务 | submission绑定前后重查、项目退出/删除不能逃逸；保留原R38和审计删除拒绝 |

锁顺序保留原user/workspace/catalog/W01/iteration/issue前缀。上述普通child入口已持child锁，新增root/policy/配置交叉锁只能采用确定顺序的NOWAIT/锁前收集，锁后重读授权；不得形成child→root与批准root→child的阻塞环。保留普通人工child独立执行合同，不能以“parent为协作”一概禁用。A08/A09必须覆盖配置/重开与enqueue/Start两个提交顺序，以及合法完成的before→after转换。
