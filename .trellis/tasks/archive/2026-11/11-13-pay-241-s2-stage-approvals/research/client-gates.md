# Research: S2 共享客户端、审批复用与 Git 物料边界

- Query: 查明 S2 可以复用的共享 Web/Desktop 客户端边界，识别审批、阶段和最终完成的缺口，提出最小接线范围。
- Scope: internal，静态源码与规范研究；未改产品代码、未运行产品测试或环境。
- Date: 2026-11-13
- Source: 本次父会话的当前工作树；S1 报告仅用于定位既有合同，不视为当前版本 S2 通过证据。

## Findings

### 1. 协作详情已经是 Web/Desktop 的共同入口

`packages/views/collaboration/collaboration-detail.tsx:23` 是账户协作详情；`source-bridge.tsx:36` 将来源工作空间的普通 Issue 入口导向同一组件。`packages/views/issues/components/issue-detail.tsx:1189` 不会为协作 Issue 再走普通 Issue 详情。因而 S2 的任务内阶段面板放在此共享详情即可覆盖两种平台与两种入口，不必新建顶层 route。

当前布局在 `collaboration-detail.tsx:95` 挂载 `CollaborationWorkflow`，在 `:107` 展示正式交付结果。现有规范要求需求 → 交付/决定 → 讨论，权限辅助项进入保留挂载状态的 drawer；新阶段摘要应在主任务内容中可发现，不能只出现在来源工作空间的设置页面，使外部合法参与者无法审核。

`packages/core/collaboration/presentation.ts:17` 的 `canAccept` 同时要求当前用户是 creator 和 handler、当前 `in_review`、可读且尚未决定的精确 submission、当前 lifecycle；`:20` 是发起人自己完成的独立分支。`:25`–`:27` 把已有 submission 表达为 `pending_acceptance` 或 `legacy_review`。这些是交付验收合同，不是代码评审。`action.ts:17`–`:39` 的动作集合没有代码评审、QA 或发布阶段。

### 2. 现有几个“审批”对象不能互相当作授权

| 现有对象 | 可复用的机制 | S2 不可直接继承的结论 |
| --- | --- | --- |
| 协作正式交付/接受 | 当前 creator/handler、精确 submission、生命周期和原请求回执；见 `presentation.ts:17`、`operation.ts:15` | 发起人接受不等于代码评审/QA/发布签字；`in_review` 不可改为代码评审 |
| M1 普通 Issue 完成 | 证据候选 → 当前预览 → 人工授权 → 另行提交；持久原意图、撤销、终端回执；见 `issues/completion-queries.ts:106`、`:133`、`:159` | 不能把 `pass` 提示当授权；当前 TS `completion-schema.ts:16`–`:33` 只允许 ordinary 目标、人类 consumer 且 evidence issue 等于 target issue，不能直接拿来签协作的普通子任务证据 |
| 知识候选 | `candidate_id/state_revision/version/content_sha256` 精确审批、拒绝、撤回、有限当前权限投影和原键恢复；见 `knowledge-schema.ts:44`–`:68`、`knowledge-actions.ts:78` | 目标是项目/工作空间知识发布，不是需求计划、子任务生成、代码评审或发布；不可用 `knowledge.state=approved` 解锁任何 S2 阶段 |
| Triage | pending 与工作流 status 分离，接受时修改字段、人工决策、revision/request_id 与不可执行边界；见 `types/triage.ts:4`、`:75`、`views/triage/triage-action-dialog.tsx:197` | 当前有权的人类成员皆可审核；`reviewer_id` 是分配/通知，不是职业许可。接收的是一个已有 intake Issue，不是一个建议版本原子生成多个子任务 |
| 通用 AgentApproval | 人类明确确认、风险类、请求者和执行回报的界面；见 `types/agent-template.ts:101`、`:123`、`agent-approval-card.tsx:82` | 只有 `id/decision/note`，未绑定候选/PR head/CI/制品版本。`production_release` 是风险分类，`executed` 是此对象的回报状态，不能据此判定真实部署成功 |
| 受控 artifact-package | 后端已有 `expected_spec_digest` 的 typed 审批与单次执行合同；见 `.trellis/spec/server/controlled-artifact-execution.md:17`–`:20` | 当前通用审批 card 不发送 digest，不能当作该 typed 合同的完整 UI，也不是部署适配器；不要重复建设受控制品执行 |

Triage 当前真实权限合同见 `.trellis/spec/server/triage.md:30`–`:43`：人类 owner/admin 配置、当前人类 member 审核，同时检查机器凭证和 `resolveActor`。普通接受不派发，`accept_and_execute` 是另一个人工意图（`:60`–`:95`）。因此可以沿用服务端 admission 的复用思路，但把 QA 名称填到 reviewer 字段不能满足 S2 越权要求。

通用审批队列仅使用 workspace agent list 显示名称（`views/agents/approvals/agent-approvals-page.tsx:49`）；它不是跨工作空间协作审批入口。`core/agent-approvals/queries.ts` 明确目前没有 approval realtime event，依赖 focus 与 mutation 失效。S2 不能把这个队列的刷新行为当作阶段实时同步已经覆盖。

### 3. 当前系统身份和职业名称没有隐式对应

`packages/core/collaboration/schema.ts:251`–`:264` 的能力是 read/comment/invite/transfer/edit/complete 等实际服务端权限，没有业务职业值或 release/QA 动作。`:234`–`:242` 的 creator、handler、participant/account epochs 应保留。固定项目经理责任人与可转交 handler 是两个语义，不能用显示名或增加一个只读 badge 冒充新权限。

推荐服务端给出每任务当前被配置的责任/签字用户 UUID、能力与角色配置版本；客户端只显示这些事实。所有准入由服务器重查当前人类身份和作用域。客户端 `can_* === true` 只决定是否展示动作，不替代后端拒绝。对于账号停用/退出/撤权，应显示历史签字的真实身份但标出当前不适用；不可让角色名称或保留的旧批准恢复权限。

### 4. 最小客户端方案建议（需由主会话与服务端设计收敛）

1. 增加任务内独立业务阶段投影，不扩张 `issue.status_category`，不改 `in_review` 或旧 `deriveCollaborationDetailState` 的含义。阶段面板区分当前候选、证据版本、已发生的决定、当前有效性、缺项和下一步；批准、已部署、已验证是不同事实。
2. 服务端提供受保护的阶段读模型和明确版本绑定的变更命令。新读写使用已有 `CollaborationScope`、`collaborationKeys`、`protectCollaborationRead` 和 `fetchCollaboration`。缺少可选旧服务端接口应显示“不可用/未启用”，不能视为已经满足门禁；结构化对象拒绝继续执行当前数据清除规则。
3. UI 同时呈现候选内容及不可变来源引用。接受、修改后接受、拒绝必须作用于屏幕上明确的 candidate/version/evidence snapshot；修改后接受应让用户看到修改后的候选并由新版本批准，不在提交时悄悄重绑定最新版本。已拒绝/失败/旧批准保留在历史中。
4. 采用知识审批的固定意图/原键回执模式，或扩展现有协作 action 机制。`knowledge-actions.ts:74`、`:99` 冻结请求；`:113` 的 GET-only 原操作者恢复与不可读正文分离。`operation.ts:26`–`:86` 在保存意图后写入、验证精确结果、失效并读取权威状态。不要引入新的 Zustand 服务器数据镜像。
5. 最终完成仍经过现有 M1 事务门禁和协作发起人精确验收。当前 `action.ts:27`–`:32` 及 `ApiClient.changeCollaborationLifecycle` 的完成输入没有客户端授权选择消费闭环；若 S2 承接该 UI，需添加协作目标的 typed 绑定与受保护候选选择，不能把现有 ordinary binding 改 `kind` 后强转。是否由本包补充协作 M1 消费端应在实施清单中明确，并与其既有任务归属对齐。
6. 发布/回滚界面应把建议、版本绑定人工批准、执行请求和实际结果分开。S2 可建立审批规则和显示可信结果的接口，S4 提供实际集成观测；没有真实结果时不渲染“已发布/已回滚”。

建议表单继续使用共享 Dialog，保留输入、未知原请求和恢复入口；服务端冲突展示当前缺项/版本差异并要求明确新意图。授权读取丢失后清除受保护候选、附件和正文；历史最小回执不能带回已不可读材料。现有 `knowledge-review.tsx:93`–`:107` 已采用 `can_approve === true` 与显式“已审阅”确认模式，可参考交互，不要直接拷贝知识业务组件。

### 5. 建议文件归属

| 责任 | 预期最小文件范围 | 原因 |
| --- | --- | --- |
| Core 阶段协议/查询/原意图 | `packages/core/collaboration/stage-approval-schema.ts`、`stage-approval-queries.ts`、必要的 action/recovery helper 及同名 canonical 测试；名称由最终协议决定 | 新业务合同独立于知识和 delivery，使用现成访问保护 |
| Core 接线 | `packages/core/api/client.ts`、`packages/core/collaboration/index.ts`；只有若向 issue 加摘要才改 `schema.ts` | API 私有传输已保护 session/epoch；此文件必须明确单一修改代理 |
| 共享 UI | `packages/views/collaboration/stage-approval-panel.tsx` 与有需要的表单子组件、对应 DOM 测试；`collaboration-detail.tsx` 挂载 | Web/Desktop、账户/来源视图共用；不复制平台页面 |
| 现有完成 UI 适配（若明确承接） | `core/collaboration/action.ts`、`operation.ts`、`views/collaboration/workflow.tsx`，并有选择地复用 `core/issues/completion-*` 合同 | 接既有事务完成授权，保持原键/精确 submission；不要整套重做 |
| 文案 | `packages/views/locales/en/collaboration.json`、`zh-Hans/collaboration.json` | 业务角色/阶段/缺项/已批准与已执行区分；先读命名与中文约定 |
| 联合测试 | 现有 `e2e/collaboration-multi-account-closure.spec.ts` 或同目录明确的阶段 canonical suite | 扩展真实多账号入口验证，不把模拟批准当正式业务签字 |

以上是建议范围，不是授权扩大到整个 collaboration 或 completion 模块。通常无需改 `apps/web/app/` 或 Desktop route：共享详情已被两端复用。若最终设计要求项目级配置 UI，则另行列明确入口和文件，不临时塞入通用 AgentApproval 队列。

### 6. 验证矩阵与现有基线

- `core/collaboration/operation.test.ts`、`result-operation.test.ts`、`delivery.test.ts`、`delivery-result.test.ts`、`presentation.test.ts`：原意图、精确 submission、当前生命周期与发起人动作。
- `core/collaboration/knowledge-schema.test.ts`、`knowledge-actions.test.ts`、`knowledge-cache.test.ts`、`knowledge-lease.test.ts`：精确候选、未知请求、当前权限擦除和分页投影。仅复用测试形态，不将其知识结果计入阶段用例。
- `core/issues/completion.test.ts:23`–`:111`：丢失创建响应后原键恢复、未知完成不再次 PUT、历史 receipt 与 reopened current issue 分开、保存先于网络、替换身份后迟到结果隔离。
- `views/modals/issue-completion-authorization.test.tsx:93`–`:175`：证据选择、授权/提交两个动作、过期证据、恢复、unmount、撤销原因。
- `views/collaboration/workflow.test.tsx:114`–`:156`、`:334`、`:450`：审阅精确结果、退回与理由、真实冲突反馈、未知结果保留。`source-bridge` 和 detail suites 验证新面板两入口、拒绝缓存清理与 focus。
- 新 canonical 协议测试必须覆盖：缺省/畸形/未来字段、错误 issue/candidate/evidence 绑定；人类/Agent 能力区分；旧批准与新版候选；授权后撤权；首次请求响应丢失的原键恢复；重复点击无重复动作；无 PR/CI/QA/发布缺项。
- UI 测试只验证显示/接线、键盘与窄窗口，不重复跑整个权限矩阵。服务器绕过与真实发布必须由各自后端/集成证据证明。性能不是本研究或数次 refetch 的结论。

### 7. 产品问题仍需集中明确

现有代码没有回答父任务 `roles-and-states.md` 中的五个产品决定。建议主会话结合服务端研究提交一组紧凑方案：

- 任务级明确指定签字人，职业名称仅说明职责；是否允许一人兼任、实现者能否自评审必须明确。建议至少禁止实现者自批代码评审，并把 QA/发布互相替代明确禁止；具体一人多职限制是产品取舍，不能由前端推断。
- 最终验收沿用 creator 及现有明确继任机制，避免新增任意产品经理代理完成；固定责任人独立于 handler。是否允许正常在职 creator 主动移交验收权仍需明确，不能把仅支持停用场景的管理继任扩大。
- 默认关闭新的阶段策略，由明确有权的人配置/启用；已启用任务后续变更角色/证据使相应未消费批准失效。项目与任务配置谁优先、谁可改仍需最终合同，不能套用 Git 任务 D8 的“方案自动启动默认开启”。
- TC07 已明确 CI 缺失不能进入代码评审；选择这一严格时机不必再次弱化为“进入待测试才查 CI”。新提交/新证据应产生新候选，旧决定作为历史，不能静默沿用。

### 8. Git 工程物料任务的独立责任

`.trellis/tasks/11-10-git-engineering-material-workflow/prd.md:23`–`:32` 已确认 D1–D10：Git 管物料定义/获批版本，Multica 管讨论分派/实时进度；需求自动送审到分支 PR/MR，Multica 的人类业务批准与平台原生门禁分别存在；授权集成核对后自动合并的是需求物料，不能推广成代码自动合并。产品人无需个人 Git 身份（D6）。

该任务 `design.md:76`–`:84` 明确项目级 D8 自动启动仅用于正式需求基线核验之后的方案生成和拆解；关闭时同一任务进入 todo 等待人工，不自动派发；编码仍等方案与任务计划审核。`:82` 要求实际受影响版本重新审阅，不能因为仓库任意其他提交就重做全部材料。

S2 应消费明确来源版本和人类决定的公共合同，不实现或改写以下工作：Git 物料文件格式/正式 PRD 权威、GitLab/GitHub 写入和自动合并、D8 自动编排、缺陷 Git 入库审核策略、整体/模块物料归档。S4 对接 PAY 场景的 PR/CI/部署证据也不等于该规划任务已经交付。D8 默认开启不是本 S2 强制阶段策略默认开启的授权。

## Files found

- `packages/core/collaboration/{schema,action,operation,presentation,knowledge-schema,knowledge-actions,knowledge-queries}.ts`：账户协作模型、精确工作流和知识版本审批/恢复模式。
- `packages/core/issues/completion-{schema,queries,draft}.ts`：M1 ordinary typed 完成消费端；存在明确的 ordinary-only 约束。
- `packages/core/api/client.ts`：共享受保护 transport、协作生命周期 API 与 M1 API 接线。
- `packages/core/types/{triage,agent-template}.ts`：intake 与通用审批对象合同。
- `packages/views/collaboration/{collaboration-detail,source-bridge,workflow,knowledge-review}.tsx`：两平台详情和当前精确交付/知识 UI。
- `packages/views/modals/issue-completion-authorization.tsx`：普通任务版本绑定完成人工授权 UI。
- `packages/views/triage/triage-action-dialog.tsx`：字段修改后接受、拒绝、revision 冲突和单次提交。
- `packages/views/agents/approvals/{agent-approvals-page,agent-approval-card}.tsx`：风险行动审批展示，缺少 S2 职业/证据版本语义。

## Related specs

- `.trellis/spec/core/frontend/collaboration-workbench.md`
- `.trellis/spec/views/frontend/collaboration-detail.md`
- `.trellis/spec/server/delivery-completion-enforcement.md`
- `.trellis/spec/server/delivery-completion-hints.md`
- `.trellis/spec/server/triage.md`
- `.trellis/spec/server/task-api-capabilities.md`
- `.trellis/spec/server/controlled-artifact-execution.md`

## External references

无外部 SDK 或 API 新实现。本次依据仓库规范与固定文件行号；真实 Git 平台及支付/部署能力应由 S4 对目标实例另行核验。

## Caveats / Not Found

- 在所查共享客户端未找到版本绑定的职业阶段/QA/发布/回滚 UI；`production_release` 字符串和 Agent 模板标题不能推导这些能力已实现。
- 本研究没有验证全部服务端写入口；其精确完成门禁、审批事务与绕过边界由并行服务端研究负责。
- 未执行任何测试或网络/Provider/数据库操作。本文件是设计依据，不能计为父任务原 24 条用例通过证据。
