# S3 恢复设计

## 2026-11-19 普通 UI numeric revision 合同

按用户已确认的下一切片实施，具体路径和 RED/GREEN 顺序见 [普通 UI 计划](../../../../../docs/plans/2026-11-19-pay-241-ordinary-ui-cas.md) 及 `research/ordinary-ui-cas-intake-20261119.md/json`。智能体输出增量已独立接受并定向提交 `89740ed14`；不新建 Trellis 任务。

复用现有 `expected_revision?: number`、Core 发送/乐观字段排除/回滚和 Shared Views 写入入口。纯观察 helper 只从为本次界面提供数据的 Issue 取得已知、正数、安全整数 revision；已显式携带的 token 保持，协作来源数据不制造普通版本。负责人和优先级在 detail/menu、board、table、SubIssueRow 各入口捕获这个观察。当前 cache 仍负责分类/权威，不用于把旧意图换成更新 revision。

现有 run-confirm `issueRevision` 接收并发送调用者的原 token；确认或 suppress/handoff 不续期它。批量仅在全部已选 rendered Issue 具有同一个已知正数 revision 时使用现有单 token；缺失、不同 revision 保持兼容。旧无版本 API、status/completion/drag/drop 和 partial/unknown selection 语义保留，不增加协议、依赖、迁移或 fan-out。

选择直接在已有字段请求携带观察；通过 surface options 再建版本字段会重复现有请求字段，cache 自动填版本会改变旧观察，服务端强制全部请求携版本会破坏已授权的兼容边界。真实双端各验证两提交顺序：两个 genuine UI 请求先保持同 R，再有一次 200/一次精确 409，败方字段与界面冲突明确，fresh R+1 的显式续接完成剩余字段。旧无版本 API 控制与此 UI 分母分别记录；原字段 R06 的省略版本历史不改写。

## 2026-11-19 Agent 输出附件最小合同

本节接续已确认的 R02 要求。实际 runtime 目前只有输入附件 GET；`CreateRuntimeComment` 与受管 CLI 评论入口均不接受文件，outcome 也没有输出文件引用。人工草稿上传不能承担 Agent 输出。新候选从已接受 `server-green-r02` 导出到 `research/agent-output-allocation-20261119-r01.json` 所指私有副本；5076 个源码文件已绑定，ROOT 的六处测试差异保留，产品文件一致。

采用一个 run-bound、单文件 multipart 评论写入：原评论 UUID 仍是 `comment:<uuid>` runtime event 身份，正文、parent、文件显示名、实际 sniffed type、长度及 SHA-256 构成不可变输入。run、Agent、issue、当前 attempt 来自实际 mcb capability 与锁内运行事实，不由上传者指定，也不把资源所有人记作人工 uploader。现有 JSON 评论、普通人工上传及旧无版本 API 保持各自合同。

文件使用现有私密密文 Storage 和 `collaboration_attachment`。仅增加表达真实 Agent/run/attempt 归属所需的 nullable 列与互斥约束；人工行保持原 uploader/draft 规则，Agent 行已经交付到当前 issue、没有人工 draft/uploader。不新增重试表、恢复账本、对象存储系统或依赖。迁移不含 FK/cascade；任何必要索引均单独 CONCURRENTLY，downgrade 有保留输出时拒绝。

写入先核当前运行权威，再准备密文，提交前在同一 runtime 事务重新核对 mcb、attempt、来源/参与权及 grant。文件行、issue/comment 引用、真实 Agent 评论、原 event/receipt、原通知和既有 squad 副作用必须同事务成功。复用实际评论 writer，不能复制另一套派发或通知逻辑。原 event input 必须区分无文件 JSON 与带完整指纹/原 attempt 的 multipart，不能只比较旧三字段而别名回放。同身份同字节重投返回原回执，不写第二文件/对象/评论/引用/通知；不同字节、正文、parent 或元数据为冲突，胜者保持。run/current attempt 在行锁内限制最多 32 个已提交输出，不借用人工 draft 计数，也不能截断结果引用。确定失败只删除本次对象；只有既有 `collaborationRuntimeCommitError` 表达的未知 commit 才保留可能插入对象与原意图，内部事务 callback 重试分别重置对象归属，不能宣布未执行或换键。

当前权威检查也适用于原上传重放及 run-bound 文件读取；撤权或旧 attempt 不因原回执而恢复写/读权限。当前人类 issue reader 可按既有权限读取交付文件，普通/public 附件入口仍拒绝私密路径。输入 reader 的原命令引用继续有效；新增 runtime output reader 只允许本 run/attempt 的输出或已有冻结输入引用。

结果引用由服务端在原 outcome 提交事务内冻结：收集同 run/current attempt 已提交的输出 ID，保存为原 outcome payload 的 server-owned `output_attachment_ids` 扩展，原请求仍使用未改变的 `CollaborationRunOutcome` wire。共用 stored-outcome decoder 分离原请求与引用，覆盖 ACK、checkpoint/retention/seal、records、测试报告和 handoff 的所有存储读取；原 ACK 只比较完整原请求，旧无扩展结果保持原合同。records 使用既有 `attachment_ids` 槽位呈现冻结引用，不在读取时从后来的文件列表推断历史。Core attachment schema 兼容真实 Agent 行的 nullable 人工身份，result schema 接受附件 ID；Views 复用现有文件 renderer 显示评论/结果输出并保持当前读取权。新的输出在 outcome 已记录后拒绝，原无内容 outcome ACK、改变结果冲突与迟到拒绝仍分别保留。这个选择避免新增 Provider/Runner/ledger 协议字段。

受管 CLI 为原 `issue comment add` 仅增加显式 `--attachment`，仍必须使用原 `--request-id` 和绑定 issue；单个文件以有界 regular-file 读取，解析在该 CLI 进程的当前工作目录内，拒绝 URL、多文件、越界/非法路径及 external-file override。现有五字段 run context 没有不可变工作目录锚点，不声称更强的 run-root 保证。CLI 验证原评论与文件身份的完整 typed receipt，不仅接受 comment_id。CLI 不创建 journal、不自动换键、不回退人工 profile。真实受控 Provider 必须通过此 CLI 产生输出，不能由人工 API 上传替代。

验证分母分成同层合同与真实运行：O01 原 Agent/run/file/comment/result 身份与加密引用；O02 已提交响应未知后的原键恢复；O03 同键同字节并发收敛；O04 同键异字节/元数据竞争与胜者保持；O05 必需写回滚、确定对象清理及未知 commit 保留；O06 慢存储期间撤权与原凭据读写拒绝；O07 终结/旧 attempt/迟到重投及旧失败保持、零恢复启动；O08 冻结 outcome 引用、原 ACK 和旧无扩展兼容；O09 CLI 路径/身份/响应边界。各 owning layer 记录实际顶层与命名集合，normal/race 专库分离。另用新来源/配置/专库在 production Web 与匹配 Desktop 各执行完整受控输出场景，核对实际 Provider/CLI、原文件/密文对象/回执、结果引用、当前权限和 gate/PID/连接。独立检查完成前不核销 R02 或归档 S3。

## 2026-11-19 普通评论通知切口

普通 `issue_subscriber`/`inbox_item` 与协作参与人/运行结果通知分别验收。当前普通评论 listener 在每次投递生成随机 inbox ID；先用真实原评论事件取得RED，再仅为有效非零评论UUID派生域隔离的持久交付ID。完整元组为原评论、源workspace、issue、member收件人和`new_comment`类型，采用既有主键冲突约束及专用sqlc插入查询；无新表、迁移或依赖。匹配原事件元组的既有随机ID记录，包括read/archived，保持其原ID、正文、时间和状态；无插入时不发`inbox:new`。

普通通知继续按当前有效订阅者投递，不采用协作terminal结果的冻结收件人规则。稳定ID保证当前writer之间的同事件/收件人去重；旧/new server混合写入、已有重复行清理、泛化bus事件身份与普通composer自动幂等不纳入此增量。具体文件、RED/GREEN和双端验收见[本轮计划](../../../../../docs/plans/2026-11-19-pay-241-continuation.md)及 `research/ordinary-subscription-intake-20261119.md`。

2026-11-18 最终前置已补齐：S2 R33 的完整代码与合成 A01–A12 已独立接受，原四旅程 4 PASS、零重试。`research/s3-source-binding-20261118.md` 绑定最终 5,056 文件源码、锁 helpers、报告及本轮独立副本；15 个已有相关 ROOT 路径字节一致。首增量沿用既有 priority/transfer 和普通 child writer，补字段、原回执与真实提交顺序验证；未证明新的产品缺陷，暂不改生产实现。此前 intermediate/r03 依赖标签属于历史，不再作为当前激活阻挡项。

2026-11-15 规划补充已独立通过：`research/recovery-followup-v2-wire-review-r05.md/json` 为 PLANNING PASS，C01/C02 开放规划问题为零。阻塞恢复采用 `research/recovery-followup-block-contract-v2-20261114.md` 与已修正的 `research/recovery-followup-wire-supplement-r05.md`（SHA256 `d9934020f24751ee722bb0bf115a0e587a37a4ee6bb0465cf12e26fee90e76c5`）。当时尚需最终 S2 来源；该前置已于2026-11-18补齐。规划通过不是已实现接口或 S3 验收。

## 已通过的阻塞与历史合同

- 单个 `information_repair` 请求在既有 workflow 事务内提交真实 blocked→非 blocked 状态、当前合法处理人、精确 superseded 关系/计数、事件和原键回执。后续 B2 是人另发的新请求；修复不自动创建 B2、Resume、派发或消费完成批准。
- 正常完整嵌套与 return 保留其精确链路。pending submission 存在时只允许精确提交/事件、当前可用 creator 和 in_review 出口；不可用则拒绝。正常返回的状态/fallback 来自原事务事实，不能改写为当前 catalog/handler 推断。
- 新受管 information request 的 reason、recipient、unblock_condition、estimated_resolution_at 必须完整。原 omitted 字段保留旧请求 hash；显式 null 与缺失分开。ETA 接受带时区的零至六位小数，持久化 UTC 微秒 instant，按最终 DB clock 校验新写；旧 ETA 过时不改变历史或调度执行。
- current actionability 与 complete/legacy/unavailable 历史记录分别表达。source-only authority 的 nullable participant/account epoch 使用真实 member 身份补充 fingerprint，不能制造 epoch。runtime v1、旧 snapshot/source bytes 保持其原合同。
- creation-options 可按显式 source/project 投影 not_selected/known policy observation；GET 观察不进入 creation POST/hash。真实创建事务解析 parent inheritance 的有效 project，再核当前政策；受管初始 blocked 继续用现有 status_unavailable 拒绝。
- 人类 history 使用真实 handoff `(created_at,id)` keyset、upper tuple、当前 revision/authority 和闭合 provenance。旧 event linkage 缺失明确为 null，仍入页及 repair 的完整 decimal denominator；不造 event ID。historical repair basis 仍须精确可验证原 event。
- history-only HMAC 使用实际缓存并通过验证的 server JWT signing key，拒绝公开开发默认值；真实重启轮换使旧 cursor 不再有效。新读事务显式五秒，编码 JSON 含 metadata/escaping/newline 不超过 65,536 字节，limit 为 1–50、cursor 不超过 2,048 字节；首个完整 item 超限返回 unavailable，不能漏行或伪造完整页。

复用既有workflow operation、原run/attempt/command、Ledger、Stop/proof、outbox与Query恢复。不会建立平行重试、通知或补读系统。具体源码与切口见research/recovery-gaps.md。

## 独立增量

- timeout-proof：真实受控执行deadline、API已提交后的ACK丢失、同目录异进程原账本恢复、明确新运行及旧失败留存；Web/Electron真实呈现。该子任务已完成实测并通过独立检查，2026-11-14结项归档。
- 并发/补读/通知：分别补priority与handler同revision两顺序、普通child显式expected_revision，55条跨页离线评论及实际WS重认证，人员/Agent混合提及与冻结接收人原事件去重。复用既有fixture，不把optional ordinary CAS称为全客户端强制版本。
- 阻塞结构：S2合同稳定后扩展既有information handoff，保留reason/recipient/previous status，增加解除条件与预计解除时间。受管任务进入blocked须完整四项，解除沿用原request身份、实际人/时间和已保存恢复状态；稳定业务责任人与本次阻塞处理人分别表达。旧未启用任务的兼容规则明确记录，不改成通用工作流引擎。

## 证据边界

Runtime outcome不等于评论或附件；各自计数和ID/hash独立校验。无权读取后仅允许已定义的content-free原回执，不能回放私有正文。WS是失效提示，补读完整性通过权威分页ID与版本证明，重复帧不是重复业务事件。真实模型、支付及100人性能分别由对应任务验证，不从本包推断。

DB按package/normal/race分离，浏览器另库，源码匹配API/production Web/Desktop。每次fixture使用正确host或namespace gate，并最终恢复原标志（epoch单调）。每个实现切口独立检查；所有S3验收满足才关闭S3。
