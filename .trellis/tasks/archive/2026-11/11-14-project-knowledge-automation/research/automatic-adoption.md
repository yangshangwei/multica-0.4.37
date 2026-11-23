# 自动采纳、验收来源与现有知识契约

2026-11-14；只读源码研究，未运行测试、模型或产品写入。仅本任务研究文件由本研究者维护。以下“建议”是规划选项，不代表已经实施或获得产品决策确认。

## 结论

自动采纳需要新增显式的系统决策身份、规则记录和后台工作记录，不能把验收人的 UUID 填入现有人工确认接口。现有版本、目标计数、权限解析和失效通知可以复用。正式交付尚不是知识允许引用的来源类型；自动沉淀必须补齐这一契约，或者明确只接受现有三种来源，不能把交付正文伪造成一条用户评论。

## 现有实现证据

| 领域 | 事实与源码位置 | 对规划的影响 |
| --- | --- | --- |
| 人工主体 | `server/internal/collaboration/knowledge.go:28` 要求 `p.Human()`；提议者需来源可读且为发起人/当前处理人；确认者需来源可读、目标成员且为 owner/admin 或人类项目负责人。`server/internal/service/collaboration_knowledge.go:151` 再次拒绝非人工主体。 | 不能给后台伪造人类 Principal；应新增只接受受控工作/策略身份的内部命令，公共人工 API 保持原权限。 |
| 数据主体 | `server/migrations/747_collaboration_knowledge.up.sql:18` 强制 `proposed_by_user_id NOT NULL`；`:37` 要求已发布内容同时有 `approved_by_user_id`；`:40` 要求非 pending 决策有 `last_decided_by_user_id`。 | 需要迁移决策/提议来源模型；“系统执行者”和“授权策略的人”分别保存，不能把后者显示为逐条人工确认者。 |
| 不可变历史 | 同迁移 `:94` 禁止删除候选，禁止修改正文、来源、目标、原确认事实；只允许 pending→approved/rejected/withdrawn、approved→withdrawn/superseded，并递增 revision。 | 自动生成、规则结论及来源需冻结；修订沿现有 lineage 创建新版本。增加字段也要纳入保留契约。 |
| 准确版本 | `server/internal/service/collaboration_knowledge.go:548` 按 state revision、version、content hash 决定；`:583` 使用 DB 时钟；`:594` 再查来源；`:625` 同事务推进 target revision，`:668` 写 target hint。 | 内部规则采纳也需要这些检查；复用领域原语，不在后台裸写 approved。 |
| 现有幂等 | `server/internal/service/collaboration_knowledge.go:191` 通过 issue/actor/request 命名空间恢复原结果；变更 payload 拒绝；`server/pkg/db/queries/collaboration_knowledge.sql:70` 检查同 lineage 的精确前驱。 | 系统工作需要自己的持久身份与原结果恢复；现有人类幂等键不能代表系统工作。 |
| 来源类型 | `server/internal/service/collaboration_knowledge_sources.go:21` 先确认来源是协作任务；`:54` 仅解析 memory/comment/run_record，匹配正文 SHA256。`packages/core/collaboration/knowledge-schema.ts:25` 同样只接受这三种。 | 普通任务不是现有知识来源；业务交付及验收快照也尚未成为来源。当前功能起点应明确为协作任务。 |
| 验证声明 | `server/pkg/protocol/collaboration_knowledge.go:60` 明确 verification 是提议者声明；commit/test reference 是惰性文本，不打开 URL/路径，不代表测试已经被服务端核实。 | 不能把旧字段 `verified` 当机器可核验的自动采纳证明。需要单独的证据检查记录与结果。 |
| 正式验收 | `server/internal/service/collaboration_lifecycle.go:476` 精确匹配当前 SubmissionID；`:496` 写 `accepted` 决策；自验收在 `:515` 先生成 self delivery，再写 accepted。`:632` 将带决策的 completed 事件写入事务。 | 可作为可靠触发事实；单次运行 success、普通状态字符串、时间邻近评论都不足以触发。 |
| 决策来源校验 | `server/internal/service/collaboration_delivery.go:257` 联结确切交付事件、handoff、作者、来源空间；`:308` 验证唯一决策、发起人和 lifecycle，且 `completed + accepted` 才为验收。 | 用该类领域读取组装验收快照，避免自行从最新评论重建结果。 |
| 完成与重开 | `server/internal/service/collaboration_lifecycle.go:549` 关闭原会话，`:553` 取消等待指令，`:564` 请求停止目标；重开 `:532` 后仍暂停目标；`:579` 推进 lifecycle。 | 自动整理必须独立于原业务队列，不通过重开、恢复目标或普通评论提及启动。触发时需记录验收所属及关闭后的 lifecycle，不能简单把 accepted 内旧 epoch 与当前 epoch 作等值比较。 |
| 当前重开语义 | `server/internal/service/collaboration_knowledge_resolve.go:363` 只检查 approved、目标、有效期、使用主体、受众和来源可用性，不检查来源任务是否重开。 | 当前重开不会自动使旧知识停用；复查/暂停自动贡献是新增产品语义，需要明确。 |
| 版本与去重 | `server/migrations/749_collaboration_knowledge_lineage_version.up.sql:1` 仅唯一 lineage/version；`750_collaboration_knowledge_approved_lineage.up.sql:1` 每 lineage 最多一个 approved。`server/pkg/db/queries/collaboration_knowledge.sql:71` 允许独立 lineage 保存相同正文。 | 目前没有目标内内容去重、语义去重或矛盾检测。需新增工作级去重及项目内比较规则。 |
| 容量 | `server/pkg/protocol/collaboration_knowledge.go:24` 每目标最多 16 条、128 KiB；`:26` 每次运行最多 32 条、256 KiB；`server/internal/service/collaboration_knowledge.go:601` 检查完整目标集合。 | 每单生成数条会很快触及容量；第一期不能默默淘汰旧知识或截断正文，至少需要容量满转待处理的明确行为。 |
| 失效机制 | `server/internal/service/collaboration_knowledge_invalidation.go:20` 在来源写事务只追加扫描；`:63` 按 cause/target 去重推进 revision/hint；`:223` 扫描不改变候选状态，只发目标变化。memory `purge` 在 `collaboration_memory.go:199` 入队。 | 现有 outbox 适合通知和当前权限重查，不会替代“知识结论已失效”的状态/复查判定。 |
| UI/协议 | `packages/core/collaboration/knowledge-schema.ts:77` metadata 强制人类提议 UUID，decision 也强制 actor_user_id；`:91` 使用 strictObject。`packages/views/collaboration/knowledge-review.tsx:46` 直接按人类账号显示提议、确认、决定人。 | 自动审计需扩展 schema、API、UI 和中英文文案；先保证旧客户端不会把系统采纳误呈现为人类确认。 |

## 最小设计接缝建议

### 1. 验收事实与异步工作分离

- 已提交结果可以预生成草案，但只有 `completed` 事件中经过域校验的 `accepted` 决策允许进入规则采纳。
- 建议在正式验收事务中只保存轻量后台工作/事件投递记录，不运行模型、不锁定外部 AI、不等待整理结果。数据库事务失败本身不能声称验收成功；模型失败在事务提交后处理。
- 以确切验收事件 ID 为生成工作的唯一来源身份；保存 issue、SubmissionID、关闭 lifecycle、目标 project、冻结的策略 revision。重复投递恢复同一个工作，不因为策略 revision 改变就重做历史任务。
- 原始抽取结果一旦落库，传输重试复用它；不要因采纳回执丢失再次调用模型并创造新候选。
- 无 project 在入队前跳过；采纳前再次校验当前项目与冻结项目一致。项目移走、删除或失权时不静默改投工作空间/其他项目。
- 重开、返回新一轮验收、策略关闭、资源撤权与采纳并发时，由领域事务核对确切来源/策略/生命周期，过期工作不得发布。

### 2. 系统采纳身份与规则证据

- 保持 `ProposeKnowledge/ApproveKnowledge` 为人工操作入口；新增内部的生成/规则决定入口，只接受数据库可验证的工作身份和策略快照，不接受模型自带“批准人”。
- 建议显式记录 `proposal_origin`、`decision_origin`（human/automation）、automation job、policy identity/revision、accepted submission/event、生成器身份/版本，以及各规则结果。字段名称需在设计收敛时定稿。
- 公共历史保留原人工语义；系统记录不能伪填 creator/handler/owner UUID。策略授权者是授权依据，不是该候选的人工 confirmer。
- 规则结果至少区分 pass / fail / indeterminate；基础设施错误、来源失效和模型格式错误不能解释为通过。
- 复用精确候选 revision/hash、DB 时间、target 全量预算、发布 revision 与 hint 的事务行为。与人工修订并发时检查同一项目知识集合版本，避免比较完旧集合后发布冲突内容。

### 3. 正式交付作为知识来源

- 推荐新增明确的 delivery/acceptance 来源，包含精确 SubmissionID、交付及验收事件引用和正文 hash；由现有交付领域读取验证当前访问和保留快照。
- 原有三种来源继续支持；交付 hash 不能只取“最新上下文”，也不能由 AI 自报版本。
- 新来源需要 Go 协议、服务端 resolver、Core schema、来源链接及运行时携带结构一起更新。严格解码和已安装旧客户端兼容需要列入方案；不能直接往 v1 结构塞字段并假定旧读者容忍。
- 交付说明里的“测试通过”仍只是文本声明。第一期若使用实际测试作为自动规则，必须定义可信记录的身份、执行结束事实、测试范围和被验证版本；缺少这些时转待处理。

### 4. 去重、冲突与纠错

- 先做可以确定的完全重复检测：同目标、规范化正文/适用范围及有效版本；模型语义比较只作为额外审查信号，不能把“未发现冲突”承诺为数学证明。
- 涉及替代已有结论、范围扩大、来源冲突或无法判断新旧时，第一期转人工；不由后台自动改写既有 approved 正文。
- 重开复查应有明确的消费侧效果。如果目标是立即停止使用受质疑的自动贡献，需要显式可逆的暂停/复查状态或单独有效性记录；现有 withdrawn 不可恢复，若直接撤回则重新启用必须创建新版本。
- 只对自动贡献施加新的自动复查策略；不要因重开一条任务将人工确认历史自动重写为另一种身份或无条件撤回。
- 容量达到 16 条时建议保留可见的待处理结果，不自动删旧条目；容量扩展、压缩或检索化应作为独立决定。

## 必须在设计收敛前明确的产品选择

1. 自动采纳策略由谁授权、是项目默认启用还是负责人配置一次、授权者离任后策略是否仍有效。当前代码没有系统代行知识决策的产品契约。
2. “有依据”的自动化标准：只允许对已验收明确结论做有引用的提炼，还是接受有测试证据支撑的新推论；两者需要不同的规则与验证。
3. 重开任务时旧自动知识立即暂停，还是只标记复查继续使用；两者对后续任务影响不同。
4. 超过目标 16 条预算后的体验与处理职责，以及每次整理条数/重试/消耗的上限。

普通任务若同步纳入，需新增与其正式完成契约绑定的知识来源和触发点。仅因 ordinary issue.status=done 就复用 collaboration accepted 语义会扩大本次范围；当前 PRD 以协作任务为起点更符合已有能力。

## 建议验证矩阵

- 源验收：正式他人交付/自验收正例；run success、待审、退回、取消、legacy/malformed delivery 反例。
- 幂等与竞态：重放事件、两个 worker、回执丢失、采纳与重开/项目变更/失权/策略撤销/人工修订的双提交顺序。
- 内容规则：零候选、完全重复、疑似语义重复、冲突、引用错误、声明性测试、真实可核对证据、超预算和模型输出损坏。
- 审计与使用：human/automation 明确区分；来源被清除后所有投影隐藏；自动采纳后真实输入包含精确候选；撤回/暂停后新运行不再收到。
- 无项目、manual、无资源、资源离线：不偷偷启动原执行队列，业务验收已提交仍有效。

研究范围仅静态契约与建议；这些条目未执行验证，也不代表现有代码已经支持自动采纳。
