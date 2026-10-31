# 协作上下文交接、召回与沉淀

状态：in_progress（父任务统筹）。首轮修复的确定性验收和正式 E0 已完成；当前主推进对象为“可靠上下文交接与召回”的集成与验收。真实模型实验保留独立账本，知识沉淀在交接契约稳定后实施。

## 已确认决定与当前基线

- 用户已在会话 `01a18a36-e482-7840-9900-3ff8174b4ce7` 的用户轮次 14 确认：“发起人或当前处理人可提出；项目负责人/管理员确认项目知识；workspace owner/admin 确认空间知识；技能层不纳入本轮”，并要求按照计划推进。原文档的“待确认”未及时更新，本次纠正，不重新索取同一授权。
- 真实模型实验沿用该会话用户轮次 0、14 的授权，保持串行、记录所有无效尝试，上游连续失败后停止重试。E1–E6 仍须正式 E0 通过。
- 首轮产品验收基线为 `4c833e737efb6c997e5cb56c74c43e9237c91956`，已合入 8 个修复。生产 Web/原生 Desktop、迁移升级/回退/重跑、sqlc 和 Go/前端检查已有带来源的记录，详见修复闭环子任务 `verification-2026-10-31.md`，不覆盖新的 runtime 改动。
- 最新真实实验记录为修复闭环子任务 `verification-e2-resume-2026-10-31.md`：E1 三个槽位已消耗，1 个有效样本事实正确但未回写；E2a trial 1 在 4 次 429 后超时。下一次从 E2a trial 2 接续，剩余最多 23 次调用，原始失败不覆盖。
- 用户已批准六步推进：固定 runtime 集成快照与台账 → 接线及启动注入集成 → 确定性与 Web/Desktop 验收 → 环境恢复后独立续跑真实实验 → 交接契约稳定后实现知识沉淀 → 父任务整体验收。新建依赖、技能层和生产发布仍不在本轮默认范围。
- 当前集成 worktree：`/Volumes/artisan/code/2026/mc-runtime-integration-20261031`；首轮实验保留原隔离源码。runtime 与启动注入负责人完成各自切片后交接，仅增量合入，避免覆盖并行实现。父任务记录源需求和跨子任务验收，产品实现落在对应子任务。

## Goal

让任何合法接手的人或 Agent 能准确回答：当前权威状态是什么；人要求我接手什么；哪些发现、决定、理由和被否决方案已经确认；下一步是什么；每一项从哪里来、由谁写、属于哪个版本和授权范围、经过什么验证。信息在停止、撤权、移除作者、换机器、换 Agent、并行小队和跨平台接力后仍能按权限保留并可审计召回。

## 规划时证据（2026-10-30，保留历史边界）

- 8 个修复提交已在集成 HEAD 4b0cbfde7，但集成日志没有记录 HEAD，最终验收未完成。
- 规划开始时主工作区有 52 项未提交改动；规划期间并行流程将 HEAD 推进/amend 到 85b98c3bb，当前仍有 47 行状态变化。规划代理使用独立 worktree /Volumes/artisan/code/2026/mc-context-plan-20261030，没有改写主工作区；该变化必须在实施前重新建立基线和归属。
- A/B/C 的独立复核已写入 research/review-*.md。A 找到附件预算的实际实现缺口和实验框架证据缺口；B 找到重复迁移前缀和恢复文案范围偏差；C 确认角色/步骤、smoke 白名单、补充请求展示的窄修复，但成员正文、队长结果、接收方真实界面仍未闭合。
- 协作上下文已有单份 Markdown 活文档、不可变版本、作者/run 来源、基线版本、并发冲突、AI 脱敏和人类清除能力。停止/撤权前尚未定期把 provider 记录分段入库；最终 outcome 或当前授权拒收时，过程发现可能只留在 daemon ledger/provider 本机。
- RuntimeContext 已返回冻结输入、roster、dispatch comment 和最新 200 comments；没有成员完成结果、已读输入 reader IDs、队长续作触发原因、前序 Agent final answer/current handler/handoff note 的统一 envelope。
- 普通任务已有 workspace/project context 注入和权限/修订模式；source context 明确拒绝协作单 capture。协作结论不应直接写入生效上下文或技能内容。

## Requirements

### R0 — 首轮交付闭环

1. 将 8 个修复提交、A/B/C 审查、附件预算修复、迁移修复和最终验证组织为一个可回退的交付链。
2. 最终 HEAD 必须能证明迁移、sqlc、Go、前端、lint、typecheck、相关 integration/e2e 已执行，日志带 HEAD、命令、数据库/环境、测试名和退出码。
3. E0–E6 严格沿用预登记语义：E0 用正式 smoke；E1/E2a/E2b/E4/E5/E6 先硬化记录框架后串行运行；环境故障与产品/模型失败分开归档。不得用 E0b、旧 /tmp 日志或 Go exit 0 代替真实行为证据。
4. 保留问题→修复提交→验证证据→剩余限制映射；任何“通过”只对已运行的代码版本、测试层和样本成立。

### R1 — 可靠交接权威包

后续每一次 claim/接手都能得到一个版本化、按权限过滤的 handoff envelope，至少包含：

- 单据 ID、状态、当前处理人及生效时间/交接序号；
- 人的原始意图、当前转交说明、请求者和目标；
- 当前协作上下文版本及内容指针；
- 前序 Agent 最终答复或其“无最终答复”事实；
- 每个成员/队长轮次的 run、角色、步骤、父子关系、完成结果和结束事实；
- 每条输入的作者、原始快照、是否已被哪些 run 读取、是否仍待处理；
- 决定、理由、发现、被否决方案及其出处、作者、版本、授权范围、验证证据；
- 被截断、缺失、权限过滤或来源不可信时的明确标记。

### R2 — 中断、撤权和作者移除保留

- run 过程记录按有限大小分段 checkpoint；已获授权且已脱敏的片段在业务记录中可见，后续授权失效不能抹掉既有事实。
- 失去业务授权后，不接受业务结果、附件或新发现；仅允许窄的终止/封口元数据通道，并把“未上传的本地剩余内容”明确标记为缺失。
- 移除/退出指令作者后，原输入、作者身份、交接说明、已写版本和已发生的 run 保留；不把 daemon/资源拥有者冒充作者，不因撤权回滚历史。
- ledger/provider 本机副本和服务端历史的清除范围必须显式列出；首轮不扩大清除能力。

### R3 — 成员、队长、并行通道接力

- 成员输入中直接带 role、step、parent run、delegation comment 引用和“只执行本步骤”边界；服务端结构化提供派活正文，不依赖模型猜测。
- 队长续作获得本次唤醒的成员结果、失败/停止原因、前序 final answer、已处理/未处理输入和所有合并触发来源；合并仍最多一个续作，但不丢来源。
- 不同 Agent/机器/Provider 接力只依赖可见 handoff envelope 和已授权业务记录，不依赖私人 provider session 或工作目录延续。

### R4 — 评论与共享上下文召回

- 评论 API 支持 before/稳定游标、limit、字节预算；响应返回 total、returned、truncated、next cursor 和作者显示名/角色/本单 AI 指令权。
- prompt 中的评论、协作上下文和转交材料带来源/可信度/权限字段；它们是待分析数据，不能改变系统授予的工具/业务权限。
- 不把来源空间普通 Comment 权变成 SubmitCommand/ControlAI；非指令权文本可见但明确标记为数据，只有服务端受理的 input/delegation 才能驱动执行。
- 旧客户端可读取已有字段；新字段经 zod/strict schema 和 malformed-response test，未理解的字段不授予权限。

### R5 — 最小逐层沉淀

- 协作单结论只能生成“知识候选”，候选保存正文、摘要、来源 issue/run/memory version/comment、作者、时间、适用范围、授权范围、验证证据、状态和 supersedes/retract 链。
- 候选与生效上下文分离；人确认后才可写入项目或 workspace context。推荐：发起人/当前处理人可提出，目标项目负责人或 workspace owner/admin 按目标权限确认。
- 项目/空间生效内容需有版本、出处、纠错、过期和撤回；修订不覆盖原候选。撤权、删除来源或撤回候选会阻止后续注入并保留审计。
- 首轮不自动写入技能层；技能层只有在另一个任务中证明必要性、执行权限影响、审查、版本和撤回后再规划。

## Acceptance criteria

| ID | 可观察验收 |
| --- | --- |
| AC0 | 最终提交链包含 8 个修复及必要最小修复；迁移 lint、fresh/upgrade/reapply、sqlc 无漂移、Go/TS/lint/typecheck/定向 e2e/integration 结果均带 HEAD 和日志；主工作区未提交改动未被触碰。 |
| AC1 | 33+ 历史、实际 FileCount/PerFile/Total/space 小于 32、可选附件超预算、引用附件超预算、引用丢失、小队不同 runtime 上限的矩阵符合 R0/R1 规则。 |
| AC2 | 正式 TestHostProviderAccountSmoke 在修复后走真实 authorizer、实际 provider/CLI，记录 launch/termination/proof/context-read/outcome；E0b 不计作 E0。 |
| AC3 | E1→E6 按预登记顺序、单场景单 trial 串行；每次记录 source commit、framework hash、provider/model/CLI、完整 prompt/brief、工具请求/结果、run/outcome/proof、评分；环境失败不计产品成功。 |
| AC4 | 触发停止、撤权、作者移除或 provider 失联后，接收方可看到已持久化 checkpoint、缺失范围、当前 handler/handoff 和最后权威 memory version；撤权后的业务结果写入被拒。 |
| AC5 | 成员和队长续作的 prompt/context 含 role、step、delegation、member result、final answer、reader IDs、wakeup causes；两个并行 writer 的版本冲突可解释且合并后两边事实保留。 |
| AC6 | 评论分页和字节预算返回 total/truncated/cursor；任意截断都提示 Agent；作者角色/SubmitCommand 标记可见且不改变工具权限；长评论和最旧/最新事实均可按页召回。 |
| AC7 | 非指令权作者的注入性评论作为数据而非指令；两个自控账号真实模型 E5 结果按预登记判定；普通 task/plugin/IM token 不能读写私有协作内容。 |
| AC8 | 知识候选未经人确认不会改变 project/workspace/skill；确认后生效版本带来源和适用范围；纠错、过期、撤回和来源权限变化可追溯并停止后续注入。 |
| AC9 | 任何接手验收记录都能从 envelope 追到代码、评论、memory version、run event、commit 和测试/实验记录，并指出“未验证/缺失/被拒”的范围。 |

## Out of scope

- 原规划轮次不修改产品代码、不运行正式真实模型；该轮已结束，后续实施和验证按三个子任务边界执行。
- 不新增通用知识库、向量检索、服务端 LLM 总结、跨 Server/外部 IM、Mobile 协作界面。
- 不让协作 run 继承来源 workspace/project 的全部权限，不把普通 Comment 权提升为 AI 控制权。
- 不把完整文件、补丁、凭据或 provider 私人会话塞进 handoff envelope。
- 不自动把候选写入技能层；不以“模型大概率会读提示”作为交接保证。

## 已确认权限边界

已确认：先完成首轮修复交付闭环；真实实验沿用授权且串行；协作结论到项目/空间必须经过人确认；技能层后置；改动小、可回退、不加依赖。

提出者限定为发起人或当前处理人；项目候选由目标项目负责人/管理员确认；workspace 候选由目标 workspace owner/admin 确认；技能候选不在本轮。确认者必须是人且同时具有来源可见权与目标权限。不得把普通参与者的评论权视为提出或确认权限，也不得把 Agent 项目负责人身份当作人工确认。该决定已获用户确认，依据见本文开头。
