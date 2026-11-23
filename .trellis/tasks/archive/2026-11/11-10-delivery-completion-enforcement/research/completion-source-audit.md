# Research: 完成写入口、事务边界与未决业务合同

- Query: 核对既有 completion-writers 清单，查找当前源码中的其他 done 写入口；定位可复用权限、政策、观察和事务能力，明确实施前尚缺的业务决定。
- Scope: internal；只读源码研究，仅写本文件与 `completion-source-manifest.json`。
- Date: 2026-11-10
- Worktree: `/Volumes/artisan/code/2026/mc-delivery-completion-enforcement-20261110`
- HEAD: `a37bee7cd1c05122383a6c40e157ef9ed5890bf4`，分支引用 `refs/heads/codex/delivery-completion-enforcement-20261110`。
- 依据: 直接读取 worktree Git 元数据确定 HEAD；未执行 Git 操作、测试、数据库启动、真实模型调用、外部请求或提交。

## Findings

### 1. 当前结论

上一包 `research/completion-writers.md` 的六组入口与当前源码一致；当前没有全局完成门禁，也没有 enforce 或 skip 实现。政策解码只接受 `off/observe`（`server/internal/handler/delivery_policy.go:58`），原评估和提示仍始终不授予完成权限。

本轮从 sqlc 查询、非测试 Go 的查询调用者、内联 `UPDATE issue`、创建调用链和 done/category 引用交叉搜索。未找到超出下表、可以将普通/独立协作任务实际写成 done 的另一个当前生产入口。这个结论限于所扫描源码；它不是运行时完整验收或未来代码的保证。

需补入规划的事实：协作 `handoff` 也调用 `UpdateIssueStatus`，但各 action 的目录只允许非终态；普通更新有单独迭代分支，但明确拒绝与 status 混写。两条均应有排除证明。另有**不写 status、却改变政策归属或已完成内容的独立入口**：协作项目编辑、项目删除后解除关联，以及内容/附件/标签等更新。它们是否也需重新验收，取决于“已 done 编辑/换项目”尚待用户决定的语义。

### 2. 实际完成写入矩阵

| 路径 | 实际写点与事务所有者 | 当前事实 / 门禁接入约束 |
| --- | --- | --- |
| 普通 PUT | `handler/issue.go:3771` `updateIssueAtomicallyOnce`；`3885` 调用 sqlc `UpdateIssue`，`3916` commit | `updateIssueAtomically`（3571）构造主体并在事务中复核；`lockIssueWriteFences`（3703）先 workspace/subscriber/member/catalog/W01，随后 iteration/issue UPDATE，再项目 SHARE NOWAIT（3864）。门禁必须在这一实际写事务内，不能调用一个先结束事务的 hint HTTP 入口。 |
| move | `handler/issue_move.go:17` 字段白名单、`109` 注释、`141` 调用 `UpdateIssue` | 同次可改 status/project；锚点解析与更新不是同一事务，实际完成仍归普通写点。若增加显式完成绑定，白名单及转发不能漏掉它。 |
| batch | `handler/issue.go:4961` 逐项循环；`5118` 调普通写事务 | 每项各自提交。当前部分错误立即结束 HTTP，部分只记录并 continue（5123），最终只返回 `{"updated": N}`（5215）；前面的提交不会因后项失败回滚。每项必须单独重新检查政策、主体、资源及绑定，不能用一次 batch 预检替代。 |
| 普通创建即 done | `handler/issue.go:3194` → `IssueService.Create` → `CreateInTx`（`service/issue.go:382`）→ `516/538` 两个 INSERT | `p.Status`可为内置或自定义 done；自定义状态在 catalog 锁下再次 Resolve（429）。父任务可补齐项目（443–463），最终项目锁在465；政策必须使用解析后的实际项目。新 issue ID 在 INSERT 参数处生成（517/539），不存在本 issue 已有制品的自然路径。 |
| 评论转子任务 | `handler/source_context.go:667`，状态解析672–679，`746` 调 `IssueService.Create` | 手动模式接受创建状态及项目，复用 CreateInTx；不是一条可以忽略的“只是评论”操作。AI quick-create 后续实际创建也进入普通创建服务；不能仅修改页面。 |
| GitHub / VCS 自动完成 | `handler/github.go:1897` `advanceIssueToDone` → `service/issue_status_write.go:26` `WriteIssueStatus` → `85` UPDATE | VCS复用同一函数（`handler/vcs_webhook.go:274`）。事务明确 RC（42），workspace/catalog/W01/iteration/issue UPDATE（46–59）；现无政策项目锁/人类评估主体。PR聚合只在没有open且存在merged close intent时允许（github.go:1898–1910）。不能借系统身份读取他人私有 key。 |
| 协作发起人验收 / 自完成 | `service/collaboration_lifecycle.go:340` → `prepareCompletion`（424）→ `changeLifecycle`（480）→ `recordStatusChange`（547/552） | 事务由 `mutateCollaborationWorkflow`（workflow.go:388）持有。只有当前发起人可 complete（workflow.go:371–374）；外部交付检查精确 submission、available evidence、当前 review/responsibility、信息请求（lifecycle.go:429–460）；自完成写结果与 accepted decision（462–476）。门禁必须与这些操作、队列关闭/Stop及回执共同提交。 |
| R38 管理员协作完成 | `service/admin_collaboration.go:144` 独立事务；`225` 直接调用 `changeLifecycle`，`251` commit | **不调用 `prepareCompletion`**。当前要求专门的平台管理员/组织/禁用创建者身份，并保留真实管理员；不是workspace owner/admin政策管理权。新门禁不能伪造发起人验收，也不能擅自让普通 skip 取代R38身份。 |

表内 `handler/`、`service/` 路径均相对 `server/internal/`。完整路径哈希见同目录 manifest。

### 3. 本轮排除核对

| 候选写入 | 排除依据 |
| --- | --- |
| 协作 handoff 的 `UpdateIssueStatus` | `service/collaboration_workflow.go:218–226` 有写入；其调用者 request/return information、submit/return acceptance 均先 `validateStatus`（`collaboration_handoff.go:170,239,314,388`）。`collaboration_lifecycle.go:61–79` 将这些 action 限为 blocked、非终态、in_review、in_progress；只有complete允许done。 |
| 协作通用 ChangeStatus | `collaboration_lifecycle.go:385–420` 调 `recordStatusChange`，但 `change_status` 的允许类别排除所有终态及in_review（67–68）；不是旁路完成接口。 |
| 普通 PUT 的迭代分支 | `handler/issue.go:3951` 分流；`issue_iteration_assignment.go:164–183` 只准 `current_iteration_id/expected_revision/iteration_reason/allow_completed`，与status混写400；`241–299` 只提交membership change。 |
| triage accept | `handler/triage.go:325–327` 设置只准active backlog/todo；`triage_actions.go:196–198` 再次校验所选状态，`269` 的内联status写入受此限制。reject/duplicate固定cancelled（479），reopen固定backlog（509）。 |
| autopilot 创建 | `service/autopilot.go:790–795` 直接 `CreateIssueWithOrigin`，固定todo。 |
| onboarding 两个直接 INSERT | `handler/onboarding_shim.go:275–280,446–451`，固定todo。 |
| channel 创建 | `internal/integrations/channel/engine/router.go:1051–1059`，固定todo并进入IssueService。 |
| 普通lifecycle handoff follow-up | `handler/lifecycle_handoff_transaction.go:398–405` 创建固定todo；原handoff记录metadata/评论/后续义务，不负责完成。 |
| 独立协作创建 | `service/collaboration_creation_fields.go:72–87` 持catalog锁Resolve并拒绝非nonterminal；实际INSERT复用IssueService（`collaboration.go:313`）。 |
| failed-task 后台恢复 | `service/task.go:5957` 只调用WriteIssueStatus目标todo；`issue_status_write.go:114` 限当前in_progress且无活跃task。 |
| child-done父唤醒 | `handler/issue_child_done.go:18–46` 的评论/阶段唤醒；没有 `UpdateIssue`/`UpdateIssueStatus` 调用，不自动完成父任务。 |
| 普通接口访问独立协作 | `handler/collaboration_boundary.go:61–72` 拒绝普通POST；普通PUT只保留position及expected_revision（121–146）。`WriteIssueStatus`（63–72）也明确拒绝协作。 |
| 状态目录修改将类别改成done | `handler/issue_status.go:80–81` key/category不可变；目录归档不修改issue.status。无需把目录编辑当成当前完成writer，但必须在准入时锁catalog并严格解析。 |

### 4. 已 done 的关联/内容变更不是单个 PUT 能覆盖的

- 普通 `UpdateIssue` 可同次改 status、project 和内容；sqlc `issue.sql:159–254` 对实际变化增加revision。原观察绑定旧 issue/project revision，不能不加说明地把它用于“先改字段，再完成”的新状态。
- `service/collaboration_edit.go:34–42,100–112` 提供独立项目/结构及内容编辑；`collaboration/access.go:99–101` 的 Edit/EditSourceStructure 不依赖open，因此已done协作仍可由具备权限的负责人编辑。`collaboration_source_fields.go:180–188` 只锁变化后的有效项目。
- `handler/project.go:978` → `queries/project.sql:122–124` 会将项目内所有issue的project置null并递增revision；该路径不写status，也不进入普通move。删除政策和评估历史另有留证合同，不能随新门禁擅自禁止或清除。
- 若“已done编辑必须重验”覆盖一切revision变化，还会涉及 `UpdateIssueContentOnly`（queries/issue.sql:616）、metadata、附件、标签、属性、iteration等独立writer。应先明确哪些变化影响验收对象；不能仅检查普通PUT后声称覆盖所有已done编辑。
- `issuestatus.Resolve`（issuestatus.go:338）严格验证新的active目标；已持久化自定义状态可能已归档，读取其当前类别应复用严格的 `service.IssueStatusCategory`（issue_status_write.go:100）或当前协作catalog事实。`Effective`（issuestatus.go:283）查询失败返回原key，不能当准入判定的严格解析器。

### 5. 可复用能力与必须保留的事务边界

| 现有能力 | 复用位置 | 不能省略的约束 |
| --- | --- | --- |
| 原始主体及凭据 | `handler/agent_artifact.go:62` artifactPrincipal；114 artifactCredential；188 artifactUsers | JWT/PAT/verified task身份来自认证上下文；user/member角色与密码/PAT/managed绑定保持当前；伪造headers不能授予人类权限。人类source membership与collaboration跨账号参与权不是同一能力。 |
| 项目政策 | `delivery_policy.go:81` currentDeliveryPolicy；147 deliveryPolicyInTx | owner/admin管理已确定；独立CAS revision；虚拟revision0/off。政策写锁project UPDATE NOWAIT（164），观察读锁project SHARE NOWAIT；第一条policy还不存在时也靠project行序列化，不能只锁最新policy行。 |
| 原评估恢复 | `delivery_evaluation.go:128,201` | namespace是workspace+原人类+key（151）；先找原键，再判新准入；以当前锁定issue/project版本调用原观察器复核权限（208–230），原观察不可变。不能把“旧版本”提前返回当成权限检查完成。 |
| 原观察器 | `delivery_evidence.go:157` | 原制品审批/执行/producer/task/runtime/各人类权限、spec/artifact/receipt摘要以及当前数据库PR head；缺失、unknown、过期、撤权不成为pass。固定观察算法不能偷换为远程Git实时验证。 |
| PR并发篱笆 | `delivery_evidence.go:192–204` | 四张PR/link表短SHARE NOWAIT覆盖新link插入及排序改变。移入实际完成事务会拉长锁存活，必须实测竞争/超时；不能删除此锁而只锁已知PR行。 |
| 生命周期/issue锁 | `queries/lifecycle.sql:5`、`issue.go:3738`、`queries/collaboration.sql:162` | 普通写使用issue FOR UPDATE；观察是NO KEY UPDATE NOWAIT；协作NO KEY UPDATE。复用局部验证应借用现有tx，不能再开一个独立tx来验证同一issue。 |
| 制品原主体锁 | `queries/task_token.sql:16–29`，`agent_artifact_execution.sql:63–75` | joined issue/task/agent/runtime、originator/owner users+members、approval/execution及凭据；原绑定丢失返回非成功，不泄漏历史。 |
| schema与时间 | `delivery_policy.go:65`、`agent_artifact.go:79–100`，`delivery_evidence.go:324–338` | 当前wrapper自行Begin并设置250ms lock_timeout、检查schema；新borrowed-tx接口必须保留等价检查。真正写状态时再检查DB时间/有效期，不能拿先前matching/checked_at当有效期保证。 |
| 协作事务/恢复 | `collaboration_workflow.go:388–585` | 依次当前users/account、workspace、catalog、W01/iteration/issue、collaboration/session/channels/targets、participants/epochs；业务拒绝可写原operation receipt；成功重放不执行新变更。未知commit不重试。 |
| R38事务/恢复 | `admin_collaboration.go:259–360`、144–256 | 管理平台fence与真实管理员/组织/创建者禁用状态、版本、catalog、独立双账本；不能借普通workflow封装开嵌套事务，不能借skip自动增加普通参与者权限。 |

现有观察/评估/制品权限helper是 **handler私有方法**，而创建、系统状态写入、协作事务在service层。不能让service反向import handler，也不能复制第二套观察算法。后续技术方案可在用户语义明确后选择下沉现有事务局部能力，或注入显式的借用tx验证接口；后者必须审计所有构造位置，nil不得导致enforce静默绕过。该选择是技术建议，未在本轮实现。

重试合同也不能直接拼接：普通更新/Create的 `RetryProjectAssociationTransaction`（issue.go:249）只重试完整回滚的55P03；观察/提示不自动重试；协作workflow当前可重试事务内40001/40P01/55P03，但commit失败立即返回（workflow.go:406–425）；R38也不重试commit（admin_collaboration.go:251–254）。新拒绝/未知消费不得被转换成一次新的尝试或另一个key。

### 6. 仍需用户决定的业务语义

以下是未决事项，不是采用建议后的合同。`research/enforcement-decisions.md` 明确未收到skip答案，源码也没有能代表用户未来意图的实现。

1. **最终验收责任与受限代办**：是否只允许原人类评估者完成；其他人/agent/GitHub是否可以在新明确授权下代办；私有评估是否保持不可转移并另建不同的授权事实。当前 `GetDeliveryEvaluation` 按原人类key查询，系统没有人类主体，跨账号协作发起人又可能不是source workspace成员，因此不能自动选择共享/冒用身份。
2. **skip**：谁可发起、谁可消费，理由/期限、确切issue+policy revision+submission/spec/artifact/receipt范围及一次消费。owner/admin管理项目政策不等于skip批准权；R38平台管理员也不等于workspace管理员。
3. **创建即done**：强制项目内拒绝、先创建再提交验收，还是具备明确的创建豁免流程；不得借父任务或另一个issue制品满足本issue绑定。评论转子任务及quick-create必须同义。
4. **哪些变更触发重新验收**：进入done、done→done、自定义done切换、已done内容编辑/附件/结构/iteration变更的范围；移项目、解除项目、同次换项目并done使用来源、目标或两份政策；项目删除解除关联如何处理。需要可列举的规则，避免只围绕HTTP端点决定。
5. **协作**：源项目政策与跨账号参与权限如何组合；精确submission/自结果是额外必要条件；R38关闭是否另需skip，且绝不能伪造发起人acceptance。
6. **旧客户端和batch**：缺少绑定的明确拒绝协议；batch保留逐项提交后，怎样返回已提交、拒绝、未尝试及未知结果。当前统一updated计数不表达每项拒绝，不能将它描述成全批原子。
7. **模式转换/回退**：何时允许政策选enforce，observe→enforce是否使旧observe评估只能stale（当前算法会这样），是否提供专门新的enforce评估准入；回退保留哪些已完成/skip/拒绝账本。当前hint对新mode返回409，原评估applicability只接受observe（delivery_evaluation.go:244）；不能只在政策enum中加enforce便算接通。

### 7. 建议的最小独立验收边界与激活顺序

以下仅供主会话在答案到达后收敛，不构成业务批准或本轮实现承诺。

1. 当前可独立完成的是**源码/归档对齐与规划检查点**：保留此writer矩阵、未决责任、锁图和验收矩阵，由独立check复核，任务保持planning。它不是M1功能交付。
2. 产品最小包应先明确可支持的验收主体和拒绝范围，再完成**借用真实writer事务的公共验证能力、不可重放的完成记录/绑定（若决定需要）及真实认证验证**。复用原观察器，不复用历史current作为许可。未接齐所有入口时不能开放项目全局enforce选项；不能用默认允许的nil callback冒充安全落地。
3. 普通PUT/move/batch和CreateInTx是可分所有权实施的两条接入线；同一package必须明确本包覆盖与仍未覆盖入口，旧off/observe行为回归。未决创建done不能临时放行来完成首条线。
4. 系统VCS与协作/R38分别接入自己的tx；受限代办、source policy及R38例外按已获答案实施。协作保留精确submission和真实actor。
5. 所有实际writer（以及用户决定纳入的已done变更）已接入并通过真实身份、实际持久事实及竞争验收后，最后才开放 `enforce` 政策准入。任何先行基础包单独归档仍不关闭M1。

### 8. 实施后的必要验收形状

- 入口矩阵每行至少一条真实身份成功路径与明确拒绝路径；VCS用实际hook调用和持久状态证明，不能以mock或直接调用validator替代；协作验收/自完成/R38分别断言operation、decision、status、queue/Stop回执。
- 每个实际tx验证当前policy/人类或机器授权、issue/project版本、有效期、精确submission/spec/artifact/receipt及当前PR数据库事实。测试原观察pass但live hold/unknown、旧revision、同优先级head冲突、过期、撤权、原execution消失等拒绝。
- 并发矩阵覆盖policy CAS/off/enforce变化、issue/project迁移、PR/link变化、catalog归档、主体与credential撤销，以及两次并发消费；断言不能多完成/多消费、失败不留下部分业务写入。
- 批量必须检查前项已提交、后项拒绝/未知与未尝试事实；不把整个请求的HTTP状态当全部rollback。普通完成仍不得使其他任务或父任务自动done。
- 如新增skip/完成账本，真实迁移、并发唯一索引形状、留证删除和禁止带历史rollback需独立验收；不加FK，concurrent index逐文件。
- 每DB测试包、最终normal/race轮次各用新专属库；真实tar内容/SHA、持久评估与认证路由复用旧夹具；每次失败保留，冻结最终源码后独审，计数分轮不相加。
- 不启动历史环境，不申请或消费真实模型预算；AC43/E5/H3a/知识停止规则不变。

## Files found

45份相关源码已按完整路径、字节数、SHA-256写入 `completion-source-manifest.json`。核心定位为：`handler/issue.go`（普通及批量）、`service/issue.go`（所有普通创建）、`service/issue_status_write.go`（系统独立事务）、`service/collaboration_lifecycle.go`（精确协作完成）、`service/admin_collaboration.go`（R38独立事务）、四份 `handler/delivery_*.go`（已交付政策/观察/评估/提示）、`handler/agent_artifact.go`（现有制品权限）和对应sqlc查询。

## Related specs

- `.trellis/spec/server/delivery-completion-hints.md`
- `.trellis/spec/server/delivery-policy-evaluations.md`
- `.trellis/spec/server/delivery-evidence-observation.md`
- `.trellis/spec/server/controlled-artifact-execution.md`
- `.trellis/spec/server/collaboration-delivery.md`
- `.trellis/spec/server/collaboration-administration.md`
- `.trellis/spec/server/durable-delegation-receipts.md`
- `.trellis/spec/server/squad-delegation-receipts.md`
- `AGENTS.md`、`CLAUDE.md`、`.trellis/workflow.md`及上一hint归档中的writer/decision记录均已核对相关约束。

## External references

无；本次问题是当前仓库的事实及待决业务意图，未使用外部文档推定用户决定。

## Caveats / Not Found

- 未发现完整完成门禁、skip授予/消费、共享人类评估授权的现成实现；不将旧建议当批准。
- 本研究没有运行任何测试，不声称现有验收在本轮重新通过；历史验收真实性由独立归档核查处理。
- 几次初始检索使用不存在的猜测路径（例如 `server/queries`、`issue_iteration.go`、`collaboration_workflow_transaction.go`、`quick_create.go`、`queries/agent_artifact.sql`）被工具诊断拒绝；随后以 `rg --files` / 符号搜索定位真实文件。它们是只读检索失误，不是产品行为red或测试失败。
- manifest只绑定本轮已检查源码；后续源码变动必须重新核对受影响入口和哈希。所有建议仍需服从用户的最终业务语义。
