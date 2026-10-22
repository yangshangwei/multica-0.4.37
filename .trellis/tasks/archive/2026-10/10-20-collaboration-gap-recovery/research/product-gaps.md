# Research: 原合同中遗漏的资源入口、@all 与取消指令历史

- Query: 对照父任务原始合同和最新待办，确认资源授权入口、@all、取消指令历史的实现缺口及独立验收范围。
- Scope: internal；只读父任务、规范、core/views、handler/service、SQL 和现有测试；唯一写入为本文件。
- Date: 2026-10-20
- Parent: `.trellis/tasks/10-10-cross-workspace-collaboration`。
- Evidence: 当前源码静态核对；历史报告中的“已验证”保留其原范围，本研究未运行产品测试、浏览器、运行端或真实 provider。

## Findings

### 1. 合同和当前待办的关系

父 PRD 保留 R01–R58、AC01–AC97，明确 M03/M05 已决定资源组织和拥有者授权分工，不能因增量待办遗漏而重新视为未定产品选择（`prd.md:5`、`:35`、`:77`、`:94`）。父 `implement.md:16` 明确基础切片不代替完整双端交付；`:196` 要求选择器含 @all，`:201` 要求取消后保留记录。当前 `research/remaining-product-work-2026-10-18.md:55` 将资源登记/授权/Switch 表单列为已验证，`:63` 的剩余清单却只列兼容矩阵、运行端、P7 和全量验收，未显式追踪下面三个产品缺口。

以下状态划分依据具体实现：资源管理 UI 和取消写入已存在；@all 服务端已存在；缺少的是独立空间资源进入组织流程的公共路径、@all 选择入口和取消输入的可见历史。不要把已存在基础重做，也不要把本报告当作全部 AC 已验收。

### 2. 资源入口：已有表单，缺 creator/handler 与私人资源 owner 分离的公共路径

**原合同。** R01/R13/R22/R55，M03/M05；主验收 AC15/16/96，联动 AC01/31/37–40/49/50/86。只有发起人/当前处理人组织新增 typed agent/squad；每个实际 owner 必须先经 R12/R46 合法成为本单完整参与者，再确认自己覆盖的目标、运行环境和用途。组织不自动邀请，邀请不产生授权，普通 owner 参与者不能直接组织目标。授权齐备后当前和以后合法参与者可使用，不逐人授权。小队资源授权不自动生成独立成员 @agent 候选。`prd.md:35`、`:68`、`:77`；`acceptance.md:23`、`:24`、`:108`；`mention-redesign-2026-10-17.md:19`、`:21`、`:23`；`discussion.md:1087`、`:1135`。

**已有公开链路。**

- `packages/views/collaboration/execution-panel.tsx:71` 挂载资源管理区；`resource-panel.tsx:29`、`:36` 有 Manage/Add；`:47`、`:48` 仅本人资源显示 consent/revoke。
- `resource-form.tsx:21` 提供工作空间/目标选择；`:31` 读取 workspace；`:32` 读取候选；`:34` 拒绝 unavailable 选择；`:70` 的 owner consent 表单通过本人 identity reader 形成确认。
- `packages/core/collaboration/resource.ts:19` 限定当前 creator/handler、能力、非终态和有效 write context；`:29` 捕获原观察版本；`:66` 原键写入/GET 恢复；`:97` 的工作空间选项仅来自**组织者的** `listWorkspaces()`。
- `server/internal/handler/collaboration_resources.go:51` 是候选 GET；`:66` 复用已有 agent 可见规则以及 runtime 可用/空间 admin 条件。`service/collaboration_resource_options.go:39` 要求 invite+submit；`:47` 要求请求者在明确 execution workspace 的 membership；`:78` 再用 inventory predicate；`:99` 要求每个 owner 已参与；`:133`、`:149` 要求小队全部实际资源对组织者可见。
- `handler/collaboration_authority.go:56`、`:87` 已接登记/consent POST。`service/collaboration_targets.go:76`、`:86`、`:100`、`:109` 将组织者角色、owner 参与和实际配置重验分开；此登记 primitive **没有以组织者加入 execution workspace 替代 owner 授权**。`service/collaboration_authority.go:526` 只允许实际资源 owner consent，`:572` 固定 owner/participant/config/binding 代次，`:582` 保持 hold。
- `service/collaboration_authority.go:450`、`:462` 只从本单登记目标筛完整有效授权，`:466` 移除失效候选；`:473`–`:482` 将暂停/忙碌/离线表达为等待。这些规则应复用。

**可确定的缺口场景。** P 在来源空间派给 D，D 的机器/智能体在 D 私人空间。D 已是合法本单参与者，但 P 不在 D 空间。P 的现有 Add 表单没有 D 空间，直接候选 GET 也被 membership fence 拒绝；D 若只是普通参与者无组织权，不能通过同一 Add 流程提供其资源。即使 D 是当前 handler，可以登记自己的资源，原 handler 转交后就失去组织权；不能以这种偶然角色重叠作为跨空间资源入口的完整交付。

需要补一个公共、明确且仅涉及所选资源的组织链路，使 creator/handler 能组织 D 明确提供的具体资源，再由实际 owner consent。原合同固定了权限和结果，**未固定该链路的交互/DTO**：可在后续技术设计选择精确资源引用或本单内受限资源提供记录；本报告没有把某一种机制新增为产品要求。不得以放开 @个人目录、加入整个资源空间、让普通参与者直接 Register、自动 grant 或管理员代签来补缺口。

**历史证据边界。** 父 `research/resource-forms-2026-10-18.md:10`、`:20` 本身限定“资源空间已有成员权限”；`:56`–`:60` 描述合成、无执行资源旅程。实际 `e2e/fixtures/collaboration-resources.ts:38`–`:42` 取 creator 为 owner，`:62`–`:71` 在该 creator 空间建资源；`e2e/collaboration-resources-web.spec.ts:15`–`:24` 同账号完成组织。它证明现有 owner==organizer UI、恢复和撤回，未证明 P 不在 D 空间时的资源提供/组织/授权路径。`handler/collaboration_resources_test.go:22` 和 `:43` 的未知空间拒绝属于原目录保护，不能简单删掉来修复产品缺口。

### 3. @all：已有人员通知语义，core 和双端选择入口缺失

**原合同。** R52/R55，AC86/90/91/97（P3 主验证人员通知，P6 双端接线）；`prd.md:74`、`:77`，`acceptance.md:95`、`:102`–`:103`、`:109`，`design.md:212`、`:216`。@all 指**本单当前有效人类参与者**，不指来源全部成员、全部 agent、小队或 owner 所有资源；新评论对每个接收人一条，排除操作者，提及改变该条显示。与显式 typed AI 提及独立处理，不能吞掉 @agent/@squad 或开启隐式 AI 执行。编辑只通知新增有效人员，旧 @all 后来新增参与者不因改周边文字被重复通知。

**已有服务端和测试。**

- `service/collaboration_mentions.go:31` 识别 `[@all](mention://all/all)`，`:39`–`:46` 分离人类与 typed agent/squad，分别去重。
- `service/collaboration_notifications.go:29` 获取有效参与者；`:45`–`:49` @all 展开、排除 actor；`:66`–`:80` 用同一锁内集合比较前后新增提及，旧 @all 不重新展开。
- `service/collaboration_comments_create.go:48`、`:89` AI 逐目标处理，`:119`–`:124` 人类通知独立；`collaboration_mentions_test.go:16`、`collaboration_comments_test.go:302`、`:308`、`:314` 已有 @all 与旧编辑语义回归。

**缺口。** `packages/core/collaboration/comment.ts:52` helper 只接受 member/agent/squad，`:54` 只接受 UUID，无法生成特殊 all/all；`packages/views/collaboration/comment-composer.tsx:70`–`:75` 只循环人和 AI，无 @all 控件，也未形成可辨识的人员/AI 分组。详情编辑 `workflow.tsx:160` 仍是 raw Textarea；若本轮覆盖正文提及入口，也应沿同一受限提及能力，而非导入普通空间目录选择器。用户手写规范 Markdown 能触发服务端，不等于 AC86 的选择器交付。

应增加显式 @all 生成/选择路径及共享中英文名称，复用现有 preview/发送/通知；不要把 all 变成 z.uuid 伪账号或 typed AI target，也不要新建全站目录。人员和 AI 显示分组属于同一 AC86 缺口，不另扩成全新设计。

### 4. 取消指令：写入和留存已有，未提供可读的不可变指令/取消历史

**原合同。** R05/R15/R35/R43/R45，M07/M11；AC19/20/33/57/64/74/76/95，主验证 P5、P6 验证可见记录。`prd.md:37`、`:57`、`:65`、`:67`；`acceptance.md:27`、`:28`、`:65`、`:72`、`:85`。取消仍 waiting 的单条输入保留原文、原作者、原因以及取消者/时间，不修改剩余 FIFO、停止当前 run 或恢复两个控制层。已进入当前执行不可自动升级为 Stop。退出/移除和整单结束自动取消等待项，重新加入/重开不复活。

**必须区分的状态。**

- `suppressed`：发送前取消本次 AI 目标，文字不删除，也不创建对应 command。`service/collaboration_mentions.go:181`–`:182`；`collaboration_comments_create.go:89`–`:92` 只为 waiting 结果 append。它不是 R45 的已入队取消。
- `waiting` 普通/未读取小队并入输入：可按当前作者/creator/handler 取消。`collaboration_queue.go:152`–`:174`；`collaboration_squad_join.go:12`、`:121`–`:137`。`group_id != null` 不等于已被执行消费。
- `in_flight`/claimed：已有输入消费或当前执行占位，R45 返回 `command_not_waiting`；`collaboration_queue.go:157`。小队 continuation 的 `consumeJoinedInputs` 在 claim 同事务消费并冻结 prompt（`collaboration_squad_join.go:164`–`:174`）。历史展示不能给 claimed 输入保留 waiting cancel 按钮。
- `cancelled`：指令墓碑/取消结果；不能拿它替换停止请求受理、run 真实 proof 或 suppressed。M11 未读并入在停止/结算时可回 waiting；作者失效/整单 closed 则取消（`collaboration_squad_join.go:140`–`:158`）。

**已有留存。** `server/pkg/db/queries/collaboration.sql:175` 保留 command 原文和附件快照；`:187` 仅改 state/reason。`service/collaboration_queue.go:181`–`:197` 保存带 command/channel/batch identity 的 `command_cancelled` event 与真实 actor，event 时间是数据库事实（SQL `:65`）。`service/collaboration_workflow.go:271` 自动取消离开者等待输入；`collaboration_lifecycle.go:373` 取消整单剩余；`collaboration_account_reconciliation.go:226` 处理旧作者代次。`server/pkg/db/queries/collaboration_targets.sql:14` 返回所有 command 状态，未从库删除 cancelled。既有 queue/lifecycle/workflow/squad、execution-history 回归保护这部分行为。

**显示/读取缺口。**

- `service/collaboration_execution_view.go:52`–`:61` 与 `packages/core/collaboration/execution-schema.ts:35`–`:45` 的 command DTO 仅含身份/作者 ID/state，无不可变 content、附件、原因、取消 actor/time 或完整事件链接。
- `packages/views/collaboration/execution-panel.tsx:89` 只渲染 waiting，取消后该条消失；`:67` 的即时 “Waiting input cancelled” 成功提示不能在刷新/换端后恢复历史。`:84`–`:87` 的 group/run 只覆盖已有执行组，不能代替从未消费的 cancelled 输入。
- `service/collaboration_activity.go:70`–`:78` 将 `command_cancelled`、资源事件和账号失效事件映射为 `other`；`:99` 不解析其详情，`:126` 清详情，`packages/core/collaboration/activity-schema.ts:7` 也无 command cancel 结构。UI `activity.tsx:22`–`:26` 只能显示泛型事件及详情不可用，无法确认哪条输入取消、原文/原因。
- `service/collaboration_records.go:139` 是 **run** reader，`:145` 先查真实 run；未消费/未 claim 的取消输入没有 run，不能靠已验证的运行记录页补齐。普通评论可被 R43 编辑/删除，所以当前评论正文也不能替代 immutable command 内容。不可凭相近 timestamp 猜 comment、command、取消或自动失效关系。

需要当前本单授权下的最小不可变输入/取消投影和共享历史呈现，覆盖普通等待、未读并入、自动取消、终态后历史，以及作者评论后来编辑/删除的场景。可复用命令留存、精确 input-event identity 和操作/event 审计；若自动批量事件尚不能精确关联每条命令，应明确保留其真实已有依据，在技术设计中补准确关联，不伪造操作者或取消时间。

### 5. 可拆分交付/验收矩阵

| Requirement | Current state | Missing deliverable | Acceptance | Files / ownership | Suitable tests (planned only) |
|---|---|---|---|---|---|
| R01/R13/R22/R55；M03/M05 | 公共登记/owner consent/revoke/Switch、受限候选和同账号旅程已存在 | creator/handler 不在 owner 私人空间时，明确所选资源进入本单组织/授权流程 | P/D 两空间；D 先完整参与；P 仅组织具体资源，D 同意自己的资源；未齐不得 @；普通参与者不能 register，组织者不能代签；无其他目录/凭据外泄；授权不恢复 | 服务端 resource options/authority/targets 和 router 串行 owner；core resource schema/helpers；shared resource panel/form；双端 wiring | DB-backed 角色×owner×资源可见性/参与失效矩阵；core correlation/原 key；views 真实分角色接线；Web+native Electron owner!=organizer、组织者无 execution membership 旅程 |
| R52/R55；AC86/90/91/97 | all/all 服务端解析、去重和编辑差集已有 | core @all 生成、人员/AI 可辨识选择入口及双端中文/英文使用 | 预览无副作用；@all+显式 A/S 发送，当前人类排除 actor 各一通知；不触发额外 AI/邀请；重试去重；旧 @all 编辑不通知新加入者；非参与者不收私密内容 | core comment helper；shared composer/必要正文入口；两份 locale。服务端既有语义按需回归，不另造 all endpoint | core 非 UUID 特例生成与约束；views all 点击和分组接线；既有 service all/edit 回归；生产 Web/native Electron 混合提及与定位旅程 |
| R15/R35/R45；AC19/20/57/64/76/95 | R45 写入、取消 actor event、reason、command 原文留存、unknown GET 恢复已有 | 授权的 immutable command/cancellation read 和刷新后共享历史显示 | waiting 取消保留原文/作者/目标/取消人/时间/原因；其余 FIFO及暂停不变；claim 竞态拒且不自动 Stop；取消后刷新换端可读；自动取消/重开不复活；评论修改删除不改输入；失权不经历史绕过 | 服务端 execution/activity/专用受保护 input reader（选择一个规范权威）；core 纯 schema/query；shared execution/history；两份 locale | DB 精确 event/command 对应和作者修改/删除/移除/close；canonical cancel↔promote/claim、M11 unread matrix；core schema malformed/redacted/epoch；views 取消后记录仍在与正确按钮；Web/native refresh/重开/撤权 |

依赖：资源和 @all 可独立规划；取消历史要先确定最小投影再接 UI。三者共享 API barrel、router、locale 与核心详情 schema，应分块单一 owner 整合，避免同时大改共享文件。运行端 provider/文件准备、P7 管理员代管和全量 P8 由其他研究/子任务跟踪；本报告没有降低它们的验收要求。

## Files Found / Related Specs

- 父 `prd.md`、`acceptance.md`、`design.md`、`implement.md`、`mention-redesign-2026-10-17.md`：原 R/AC/M 合同和阶段归属；正文引用的父相对文件均位于 `.trellis/tasks/10-10-cross-workspace-collaboration/`。
- 父 `research/remaining-product-work-2026-10-18.md`、`research/resource-forms-2026-10-18.md`：最新待办及资源表单历史证据的限定范围。
- `.trellis/spec/core/frontend/collaboration-workbench.md`：当前授权、React Query、原键恢复、R45 不依赖 grant/gate、历史精确投影、资源 E2E 边界。
- `.trellis/spec/server/collaboration-targets.md`：owner 完整参与、目标 hold、M07/M11 消费/墓碑、非复活、运行协议与读取隔离。
- `.trellis/spec/server/collaboration-content.md`：immutable command 附件引用和当前对象授权；运行端文件准备单独验收。
- `.trellis/spec/server/security-boundaries.md`、root `CLAUDE.md`：机器身份不借 owner 人类权限、事务/对象授权、默认测试不启动真实 agent、无新依赖和 package 边界。
- Core/views/server 指定索引和 `.trellis/workflow.md` 已读取；research 不读取 implement/check JSONL，也不启动任何 task。
- External references: none needed；本题以仓库原合同与当前源码为依据，未引入 SDK/API 版本判断。

## Caveats / Not Found

- 未运行新测试或声称这些缺口已修复；现有测试引用是源码/历史证据，不是本轮执行结果。
- 未发现现有公共 UI 能使非组织者的私人资源 owner 向无其空间权限的 creator/handler 提供明确目标；组织者精确 POST primitive 存在，受限公共选择机制仍需技术设计。
- 没有新增产品选择：@all 是人类、本单授权列表、creator/handler 组织、actual owner 确认、取消后留存均为原已接受语义。
- 取消留存不等于任意状态都可取消；run records 不等于未执行输入历史；suppressed、waiting unread、claimed、cancelled 和真实 stopped proof 必须保持不同。
- 本报告只追踪已得到原合同和当前代码双重证据的缺口，不把完整 P8 平台覆盖或已列出的 provider/管理员代管泛化成另一个新功能。
