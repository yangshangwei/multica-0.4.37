# 重开暂停、逐条复查与知识 v2 兼容设计

2026-11-14，规划研究。用户已确认重开立即暂停、重新验收后逐条复查、相同恢复、变化修订、冲突待审，人工撤回不可自动逆转。本文件仅提出设计；未修改产品、执行测试或调用模型。

## 1. 源生命周期实现立即暂停

- 现有重开会修改 session、暂停执行目标，并推进 `issue_collaboration.lifecycle_epoch`：`server/internal/service/collaboration_lifecycle.go:528`、`:579`。完成时记录确切 accepted 决策及 completed 事件：同文件 `:496`、`:519`、`:632`。
- 当前 `knowledgeEligible` 只看 approved、目标、有效期、主体、受众与来源，不看重开：`server/internal/service/collaboration_knowledge_resolve.go:363`。因此单独发送通知不能实现暂停。
- 建议每个自动候选新增独立的有效性头记录，指向不可变评估记录；评估保存 `candidate_id/version/content_hash`、`source_issue_id`、`validated_close_epoch`、精确 accepted event/SubmissionID、目标 project、规则版本与检查结果。字段名称为设计建议。
- 当前有效条件：候选仍 approved、头记录指向通过的评估、来源仍为同项目、来源处于正式验收后的关闭状态，且 `current_lifecycle_epoch == validated_close_epoch`。来源 epoch 与 accepted 决策内 epoch 不是同一个时刻，必须保存关闭后的 epoch，不能猜测 `+1` 重建。
- 重开只需现有 source/session 写与一条 source invalidation outbox，无 target/candidate 查询、更新或扇出。epoch 一变，全部相关自动候选在消费时自然不满足条件，approved 历史不变。
- 应在普通/协作 prepare、原请求 replay、manifest revalidation、launch authorization、普通续跑历史检查统一调用有效性判断；不能只改初次列表筛选。
- 已有读取锁可承担边界：`server/internal/service/collaboration_knowledge_locks.go:219` 在同事务锁 source issue、collaboration、session 后锁候选；对应 `server/pkg/db/queries/collaboration_knowledge_service.sql:88` 使用 `FOR SHARE NOWAIT`，会与重开 UPDATE 串行。
- 头记录和评估身份应纳入当前锁提示/验证集合，继续遵守 target→source→candidate 的读取顺序。新增源码状态读取不能放在持有这些锁之前形成唯一授权依据。
- “立即”定义为重开提交后，新准备/新启动授权/新续跑检查不能通过旧评估。重开前已合法完成授权或已披露给模型的字节无法收回，不声称零延迟杀停所有消费者。
- UI 用独立 `effective_eligibility` 显示 active/paused/pending_revalidation 等效果，保留 `state=approved` 的历史事实；权限 `availability` 仍表达 available/source_unavailable/restricted，不混用。
- 当前 source invalidation 只是 target revision/hint，不改候选状态：`server/internal/service/collaboration_knowledge_invalidation.go:20`、`:223`。重开新增一条 scan 即可；消费不能依赖扫描或 websocket 已送达，页面允许既有短租约刷新延迟。

## 2. 重新验收后逐候选恢复

| 复查结果 | 最小变更 |
| --- | --- |
| 相同正文、范围及语义，来源仍有效 | candidate ID/version/原提议/原批准记录不改；追加指向新 accepted event/SubmissionID 的评估，CAS 更新有效性头到新 close epoch，再推进目标 revision/hint。 |
| 内容实质改变且新规则通过 | 沿同 lineage 创建新候选版本，来源绑定新验收；规则采纳新版本并原子 supersede 前版。禁止修改旧正文、旧 sources 或旧批准。 |
| 冲突、依据不足、检查错误 | 保留旧版本暂停；保存新候选/检查结果待处理，不仅因任务重新 done 就恢复。 |
| 人工 withdrawn/rejected、被人工修订替代 | 后台不恢复、不以新 candidate ID 绕过人工决定；回写前对 candidate state/state_revision 与人工覆盖标记检查。 |

- 新评估唯一身份至少包含 candidate、accepted event 与评估目的；同一原始工作重试恢复已有结果。新的重开与重新验收拥有新的事件身份，旧任务不能覆盖新头记录。
- 同一提交不能仅靠模型重写是否逐字相等判断语义未变：明确的正文/适用范围相同可走恢复；模型认为“类似”但字节改变时走修订或待审。
- 新验收可能提供了新的佐证，但原 approved candidate sources 是不可变的：`server/migrations/747_collaboration_knowledge.up.sql:94`。将本次佐证放入新评估，与原来源并列读取；原来源已 purge 不因新评估而恢复泄露旧材料。
- 复查完成时再次校验来源生命周期、目标 project、最新策略、候选状态和项目预算；与人类 withdraw 使用同一候选行/状态 revision 防止晚到恢复。
- 未撤回的暂停 approved 候选继续按既有预算保守计数；不可悄悄释放容量并在恢复时突破 16 条。修订仍按现有同 lineage 替换计算预算，见 `server/internal/service/collaboration_knowledge.go:601`。
- 建议 runtime v2 manifest entry 加 `eligibility_revision` 或不可变 `validation_id`（人工候选明确 null），将实际提供的有效性证明绑定进 manifest hash。复查后旧 manifest 不得被换成新证明；旧会话继续遵守精确历史复查，不能自动回退新会话重放业务输入。

## 3. v1 不能直接扩字段的证据

| 层 | 当前限制及锚点 |
| --- | --- |
| 人类 API 请求 | `server/internal/handler/collaboration_knowledge.go:49` 列表 query 仅允许 limit/cursor；`:92` 等请求由严格 DTO 解析，没有版本协商头。 |
| 来源与贡献 | `server/pkg/protocol/collaboration_knowledge.go:191` 与 `:218` 仅允许 memory/comment/run_record；`:398` contribution 严格解码并构造 protocol_version=1 proposal 验证。新增 delivery 来源在旧 runtime 也会失败。 |
| Web/Desktop | `packages/core/collaboration/knowledge-schema.ts:77` 强制提议者 UUID、人类 decision UUID；`:91` strictObject；`:96` 固定 protocol_version=1。`:173` 声明协议却不符合时抛 ProtocolError，不能当空列表。 |
| 前端请求层 | `packages/core/api/client.ts:1394` 当前知识请求不发协商头；`:1419`、`:1428`、`:1435` 分别解析 source/detail/target，三个入口都需更新，不能只改协作页。 |
| 普通 runtime | `server/pkg/protocol/task_knowledge.go:13` 固定 TaskKnowledgeVersion=1；`:81` manifest entry 无有效性版本，字段顺序定义 hash。 |
| 普通持久历史 | `server/internal/service/task_knowledge.go:67` 要求 DB knowledge_context_version=1、JSON version=1 且 DisallowUnknownFields；`:104`、`:112` 又检查 manifest/authorization=1。`server/migrations/747_collaboration_knowledge.up.sql:86` DB CHECK 也只允许1。 |
| 协作 runtime | `server/pkg/protocol/collaboration_knowledge_runtime.go:10` 版本1；`:66` bundle manifest 也固定1。`server/internal/handler/collaboration_runtime.go:171` 只解析 knowledge_version=1。 |
| 协作持久历史 | `server/internal/service/collaboration_knowledge_runtime.go:21` 使用固定 producer key `knowledge:claim:v1` / `knowledge:prepare:v1`；`:31` 严格解析保留 event payload；`:52` 首先寻找旧 claim。换新键会让旧读者找不到历史。 |
| 人工身份模型 | `server/migrations/747_collaboration_knowledge.up.sql:18` 与 `:37` 把提议、批准约束为非空用户 UUID；`packages/views/collaboration/knowledge-review.tsx:46` 显示这些字段为人类操作。 |

## 4. 建议采用的明确版本策略

1. **人工接口 v2 显式协商。** 新客户端在所有 source/detail/target/decision/recovery 请求发送 `X-Multica-Knowledge-Version: 2`；正文/响应 protocol_version=2。header 不存在时仍按 v1；header 与正文不一致拒绝。新客户端兼容读取纯 v1 服务端结果，但不向旧服务端发系统采纳请求。
2. **双版本 DTO。** v1 类型冻结；v2 增加 proposal/decision origin、系统规则/工作身份、effective eligibility 与评估版本，并新增准确 `delivery` 来源（SubmissionID、accepted event、hash）。新来源及评估必须在锁与授权验证范围内；不得在原 v1 union 中无版本地插入。
3. **v1 表示边界。** v1 只允许完整、纯 v1 可表达的对象/集合。涉及系统提议/采纳、delivery 来源、暂停或新有效性语义时，返回无正文的明确 unsupported（例如409及固定错误码）；不得伪造人类用户、漏掉新条目仍称全量、或把暂停 approved 描绘为可用。应在完整列表/历史边界决定表示能力，不只检查当前页。
4. **恢复不依赖最新投影。** 已提交的 v1 人工 operation 原回执保持 v1，可按原 actor/request 恢复，即使候选如今有新状态；新操作不能借旧 v1 DTO 修改不能表示的对象。客户端存储的未确认请求身份也须保存协议版本，重试原始字节而非升级成v2。
5. **runtime 协商升级至2。** 普通 `TaskKnowledgeInvocation/claim/prepare/authorize/observations` 与协作 `knowledge_version` 协商均支持1/2；新 daemon 请求2，新协议响应、贡献、manifest 全链一致。旧 v1 daemon 仅可处理完全可表达且历史允许的 v1集合；不能删除 delivery/eligibility 字段后降级提供自动知识。
6. **新存储版本2，旧存储严格读取。** 普通 `knowledge_context_version` / JSON delivery version 新写为2，允许1/2并按版本选择完整严格读写器。读取旧行不能用v2默认值改写原 hash/receipt；读取v2不能丢弃新字段后回写v1。跨会话历史仍扫描全部暴露，不按当前版本过滤掉旧记录。
7. **协作固定slot不能失踪。** 保留已有 claim/prepare producer key 作为稳定查找槽（名称中的v1只是历史键）；payload 加明确版本并分支。若选择新键，必须先增加所有读者必经的版本屏障，否则旧版本可能把该run判为无知识历史。不得同时制造两份互相矛盾的claim。
8. **渐进部署顺序。** 先部署能严格读取1/2的服务端与消费暂停检查，再更新 daemon/客户端，最后启用系统来源及v2写入；仅关闭自动沉淀开关不会使旧服务器变安全。产生v2后禁止回滚到仅读1的服务端；保留观察、终止、旧回执与历史读取。

## 5. 隐藏风险与必须覆盖的检查

- v1 source/detail/target 任一入口仍暴露自动行，会导致空UUID、错误人类归属或整页 schema失败；不能仅靠新前端修复。
- 候选来源 JSON、评估来源、runtime贡献、manifest、保存的普通 delivery、协作producer事件属于不同版本层；只提升一个常量不完整。
- 旧 v1 session 曾收到知识后，关闭开关/遇到v2/旧manifest不满足均不得静默走无知识或新会话 fallback。沿现有拒绝和清理路径保留已发生事实。
- 新评估只改变有效性，不改变旧 approved 历史；UI 展示“已采纳·暂停使用”而非把历史改成 pending。人工撤回与评估恢复应有双提交顺序回归。
- 需覆盖：重开提交前后 prepare/authorize；重新验收与迟到复查；人工withdraw先后；同正文新证明；变更修订；冲突；项目移走；v1/v2 source/detail/target分页和原回执；普通所有claim通道、会话历史；协作旧key读取；所有新源/字段的严格解码。
- 本方案的兼容失败是明确不可用，不是系统已自动升级所有安装器；实际 Web/Desktop/daemon 版本矩阵仍需执行验证。
