# S2 规划独立审查

结论：**需修订后再实施**。复用方向正确：业务阶段独立于 `in_review`/`issue.stage`，最终完成继续进入 M1 的实际事务，不新增持久完成令牌。当前草案还缺少下列可执行合同；这些是规划缺项，不是已经发生的产品漏洞或测试失败。

- 审查人：原生 `trellis-check`，`/root/s2_plan_check`。
- 日期：2026-11-13。起点 HEAD：`feb87aee75fa6b9dd051a4e35bf7df8b101edb75`。
- 范围：本任务 PRD/design/implement、10项 curated context、对应真实源码落点。已读取被截断 hook 的完整保存文件；未改产品、原规划、其他任务资料或 Git 状态。
- 版本：本轮审查绑定 `plan-review.json` 中的规划文件 SHA-256；后续修订不自动继承本结论。

## Findings（未修复：需要主会话收敛设计）

### F01 / P1：批准必须绑定原签字人的授权代际

`design.md:28` 只明确决定中的角色/政策版本与真人身份，`:49`/`:60` 的当前读取规则不能替代原签字权重检。现有 `workflowInTx` 仅比较**本次调用者**的 participant/account epochs（`server/internal/service/collaboration_workflow.go:568`）。移除与重新加入同一用户会再次推进 participant epoch（`server/pkg/db/queries/collaboration_workflow.sql:13`、`:20`），角色UUID可以保持不变；若阶段消费只查“现在仍是指定人”，旧批准会在重新加入后复活。

修订要求：决定冻结签字人的 participant/account epochs，以及该动作采用的凭据身份/有效期/撤销依据；不得保存 bearer。消费时核对原签字人仍满足该快照和当前角色/参与权，不能用当前调用者的身份替代。原签字人的恢复读不重签、不续期。明确配置角色/业务责任人的授权主体、未配置与无效角色行为、撤销主体及理由。自评审约束必须明确比较的是哪一个可信的“实现责任人/提交人”；任意 `submitted_by` 或 Git 显示名不能建立人员映射。

还需把新角色/原签字人加入锁前身份收集及锁后重读，或提供证明其已被覆盖的约束。现有 `GetCollaborationWorkflowUserIDs` 只收 creator、handler、有效 participants、本次 actor/target（同SQL文件`:3`），不能在持有 issue 锁后再阻塞取得遗漏用户锁。

验收：签字→移除→重新加入同一角色仍旧批准失效；签字→密码/凭据撤销→恢复账号仍旧批准失效；配置更换后旧请求不获得新角色；并发撤权/推进两个提交顺序。

### F02 / P1：分离证据身份、阶段决定与操作 CAS，避免批准自失效

`design.md:26–28`、`:38–45` 没有定义“候选版本”与 issue/collaboration revision 的关系。合法交付本身会改变这些版本：`handoff` 写 status 和 handler（`server/internal/service/collaboration_workflow.go:218–230`），`UpdateCollaborationHandler` 增加 issue revision，`finishWith` 增加 collaboration revision 和 authority_epoch（`server/pkg/db/queries/collaboration_workflow.sql:24–31`）。若把候选绑定为当前全量 issue revision，提交正式交付后原批准会全部失效；若自动重绑定最新 revision，又可接受未审内容。

修订要求：固定一个不可变来源/代码候选身份及已审内容摘要，另用当前 workflow epochs/revision 处理请求 CAS。明确哪些来源字段变化产生新候选、哪些正常流转只推进 CAS。QA报告、部署结果等是后来追加的独立证据，不能靠修改初始代码候选的总摘要追加，否则所有前序批准循环失效。每种决定列出其精确依赖：候选、相关证据版本、前序批准 ID、角色/政策版本、签字权快照。正式提交必须与同一来源候选建立精确关联，不能只凭“当前阶段=验证”。

验收：正常研发→评审→QA→发布→交付→接受可走通；纯状态/处理人变化不冒充新内容；标准/代码/构建变化使相关批准失效；新增后序证据不会隐式重签前序批准；重新打开不复活上一生命周期决定。

### F03 / P1：stage-only 新完成需要自己的最终事务检查

`design.md:56` 的“必要时在原 Finish 中再次验证”不够具体。M1 off/observe 在 `server/internal/service/delivery_completion.go:304` 返回 `DeliveryNotApplicable`；协作和 R38 仅在 `DeliveryPrepared` 时调用 Finish（`collaboration_lifecycle.go:409–414`、`admin_collaboration.go:264–265`）。仅增加 Prepare 的 S2 查询，无法在 stage-only 路径阻止批准/证据于准备后过期；行锁不阻止时间流逝。

修订要求：为每个适用的 S2 新完成返回绑定实际事务的内部校验状态，并在实际写入/提交前执行统一最终检查；可以扩展既有准备结果或用借用事务的内部 stage guard，**不是第二套持久完成令牌**。M1有无绑定的分支都检查，off/observe/enforce 都到达最终检查；使用后采样数据库时间，而非事务开始时的 `now()`。原 consumed/原工作流回执先按既有当前读权恢复，不能重复推进。项目政策、角色、候选、PR/CI/附件/运行证据的锁或最终重检须有明确矩阵。

验收：stage-only + M1 off/observe 的真实迟到到期拒绝、全部写入回滚；M1 enforce 叠加原授权；角色/政策变化、PR关联插入/移除与当前head改变；submit/self-complete/admin、项目清空/转移/删除及旧客户端入口。

### F04 / P1：一次性拆解还缺稳定意图身份与创建/执行准入表

`design.md:27–28`、`:38`、`:58` 尚未固定建议的状态迁移、候选家族身份和通用 child 入口处理。`CreateInTx` 是正确创建原语，但它只重查普通 parent/project/admission（`server/internal/service/issue.go:444–477`），并不知道 S2 建议是否批准。仅约束新的批准 endpoint 不能保证 Agent/普通创建/重挂 parent 等入口无法绕开未批准拆解。

修订要求：定义稳定 proposal ID、不可变候选版本/内容及唯一当前待审版本；接受/修改后接受/拒绝/替代的状态表。接受后同 proposal 不能通过换 key 或换候选 ID 无意再物化一组子任务；若首期不提供已接受计划修订，明确拒绝该分支并保留旧 child 映射，不能标题去重替代。确定自动候选是受控运行产物，还是人类从已授权运行整理的候选；不能把 human-only workflow 开放给 Agent 来完成审批。

列出普通child创建、comment-to-child、后挂parent、赋Agent、显式运行及后台派发分别的准入规则。明确“审批生成子任务”与正常人工手建子任务的关系，避免为解决一句“未批准拆解”而禁掉所有普通子任务。批准物化必须原子，先验证真实 source membership/创建权限并在事务前解析 entitlement policy（`IssueService.CreateInTx` 的`:375–381`合同），固定数量上限、child ordinal与重复/故障行为。外部协作审批权不能隐含源空间创建权；plain accept 不派发。

验收：同键、换键、并发、旧版本、修改后接受、部分child失败的零部分写入；source membership撤销；已接受计划再次物化被拒绝或按明确修订合同处理；全部替代创建/派发入口。

### F05 / P1：冻结阶段证据入口和各来源能解锁的动作

`design.md:41–51` 对 run/失败用例/日志、CI时效和 `human_attested` 的准入仍停留于描述。现有 `CollaborationRun` 只有运行/结果/证明等事实（`server/pkg/db/generated/models.go:1088`）；`business_outcome` 不包含失败测试用例集合，选中 run UUID 也不等于“自动关联失败用例和日志”。GitHub/VCS镜像及现有artifact observer没有真实部署/回滚结果。

修订要求：冻结最小 typed 证据合同和生产入口：来源、issue/run/attempt、代码与构建身份、失败用例 ID、不可变日志/附件或record引用及摘要；服务端验证关联和读权。自动登记与人工补录分开，不根据任意报告正文推导自动通过。CI检查的非空集合、通过状态、完整性与时效边界必须固定；现有VCS按connection+SHA+context聚合的边界不足时拒绝/标未支持，不扩大成已验证跨仓CI。

给出来源→可解锁动作表：连接器观察、人类签署、合成fixture。合成fixture只在测试证据中标明，不暴露生产客户端可填的“模拟但视为真实”通道。人类登记是否足以记录部署/解锁验证与最终完成需要在合同中明确；无真实环境不是任意填写成功的理由。回滚批准绑定原部署+目标制品+指标证据，撤销与后来实际回滚结果不是同一事实。本包可验证固定平台协议，S4仍保留真实集成未验收状态。

验收：伪造run/attempt/log关联、Agent改写QA、缺失失败用例、日志不可读、空/旧/未知CI；批准但未执行不能呈现已部署；未授权来源不能提交部署/回滚成功事实。

### F06 / P1：已知客户端完成与恢复缺口必须指定责任，不能留给实施时发现

`design.md:68` 将 M1协作客户端消费端写成“若发现必须扩张”。这是已经查实的缺口：`packages/core/issues/completion-schema.ts:16–33` 只接受ordinary，而 `packages/core/collaboration/action.ts:27` 的complete变体没有completion绑定，现有两端不能直接走S2+M1 enforce的正路径。应在S2计划中明确承接适配，或明确其对应已有任务与可用的交接条件；不能用M1 off演示宣称enforce跨端通过。

同文件 `action.ts:83–96` 的 `recoverCollaborationAction` 在GET 404后会重发原POST，与新设计`:59`要求的GET-only恢复不一致。不得照抄名称推定行为；新阶段恢复需明确定义404/进行中/未知/拒绝的结果，并测试无自动新POST。另需固定最小 `stage_confirmation`（候选/决定/必要物化身份）及受保护重读，避免凭通用code退休了错误候选的请求。当前 `CollaborationWorkflowReceipt` 没有这些stage字段。

S2事件也需要显式消费者归属：`collaboration_activity.go:70–77`将新kind投影为other，丢弃明细；Views通知`:36–37`也只有旧动作映射。复用finishWith/outbox正确，但不是阶段审计已经可见的证明。增加明确允许的阶段事件摘要/引用和双语通知映射，不透传任意payload。

验收：Web/Desktop真实S2+M1 enforce接受；schema拒绝跨候选/提交绑定；丢失POST响应→GET原结果/404/进行中均无新意图；当前权限丢失只保留最小回执；阶段事件可追到真实候选/签字人，正文不越权泄漏。

## 建议实施增量

保持一个S2验收范围，可用三个有依赖的本地实现增量，全部仍受A01–A12约束：

1. **协议与建议准入**：确定待决产品选择、来源/授权代际/版本表、policy/roles/proposal/receipt schema；实现策略、角色、候选和原子物化，封住对应旁路。产品门禁保持默认关闭。
2. **阶段事实与实际完成事务**：实现PR/CI/测试/部署引用与决定、最终stage guard、submit/complete/project变化全部接线；normal/race验证所有拒绝及回滚。不能先交付一个可启用但只检查部分入口的策略。
3. **共享操作与联合证据**：Core/Views与M1协作typed消费、阶段原键恢复和审计、production Web/Desktop。独立检查整个S2差异后才关闭S2；前两个增量不能被单独记为S2通过。

服务端共享事务文件继续归同一实现者；客户端在协议冻结后可并行。E2E与只读证据汇总可独立持有文件，不把S4接入或S5性能强塞入服务端lane。

## Verification

- `task.py validate`：PASS，implement/check各10条真实路径，均存在。
- 结构检查：PASS，PRD有12个唯一A编号，父矩阵仍为24个唯一用例。
- 实际源码检查：上述Prepare/Finish、workflow/child创建、版本更新、协议和投影落点已核对；不是只转述研究。
- Lint / TypeCheck / 产品测试：**NOT_RUN（本次只做规划审查）**；不借用父会话正在执行的基线结果作本审查的产品通过结论。
- Findings（fixed）：无。以上属于产品/设计/跨层合同判断，按职责交主会话修订；本代理仅新增两份审查文件。

后续：用户答复与技术合同修订完成后，按修订版本复核F01–F06，再激活实施。继续独立的环境或恢复验证准备不受本报告阻挡。
