# S3 v2 / r05 wire 独立规划检查

**结论：PLANNING PASS，开放规划 finding 为 0。** 原 S3P-C01 在 v2 合同层正式处置；S3P-C02 在主会话选定的技术方案及本次新 wire 文档修正后关闭。此结论不是产品修复、测试通过、完整 S3 或父 24 例验收。S3 仍为 planning，当前 S2 会话 pointer 未切换。

## Findings (fixed)

仅修改 research/recovery-followup-wire-supplement-r05.md 的技术边界，原文先保存在私有证据。未修改产品/helper/test/global spec、原 v1/v2 研究或原 review。

| 文档问题 | 修正及实际源码依据 |
| --- | --- |
| 缺失旧 event linkage 却按 event sequence 排页 | 按主会话选择改为真实 handoff (created_at,id) / upper tuple；linked、legacy_unlinked 显式 null、unavailable 闭合 provenance；旧行仍在 immutable affected denominator 内。activity.go:181–183 明确保留无精确 handoff link 的旧事件，issue_handoff 不存 event UUID |
| 将现有 content reader 当作预算/sealer | comments.go:103–120 只提供 RR/READ ONLY。新读取显式 5 秒、完整编码 JSON 加 newline ≤65,536 bytes、limit1..50、encoded cursor≤2048；首个完整 item 超限503，不丢正文/行/假next=null。只复用 recall.go:105–118 的实际 JSON size 方法，不复用会反转/omit 的 comment-page envelope |
| 人类 history 没有可直接借用的 sealer | run handoff cursor 是 base64；T4 MAC 的 key 来自 frozen run salt。按主会话选择 history-only HMAC domain，复用 auth.JWTSecret():21 的实际缓存 key，并用 ValidateJWTSecret():53 验证它；公开开发默认 key 不可用。sync.Once 要求真实重启轮换，不能用新 env 值配旧 cached key |
| nullable / count / return provenance 不明确 | 钉住真实 actor/from/to/status/resolution、request handoff 与 operation key 的区别、int64 decimal denominator、旧 event null 和不可用 stub；正常 return 的状态/fallback 从原事务事实取得，不从当前 handler/catalog/updated_at 推断 |
| 创建读上下文与真正有效 project 容易混淆 | 六字段 not_selected/known presence、source/project 相关性与 Query key；无政策为支持的 revision0/off；GET-only observation 不进 POST/hash。issue.go:442–491 的 parent inheritance 决定真正有效 project；新 root-only guard 消费该身份，旧 actor-owned key 在新 admission 之前恢复 |
| repair replay / empty default / pending fallback 接线遗漏 | repair 进入 directed receipt replay 类；新写才查 Transfer+ChangeStatus。repair-only default_status空串必须新 closed variant，旧 schema 的 nonempty/default-in-options 不放宽。保留 pending-safe 正常嵌套、实际 raw pointer，source workspace departure 不误判 creator unavailable。managed blocked 创建用现有409 status_unavailable及既有 own-result enum |

修正遵循主会话的技术选择，没有新增用户持有的产品决定、runtime-v2、run-local output-upload capability、配置、依赖或授权令牌。

## Findings (not fixed)

无待决规划问题。下列为激活依赖，不是已经实现或验收的 finding 修复：最终 S2 source/lock contract 接受及绑定；通过后的设计/实施/jsonl 同步；实际 S3 owner 实施、迁移/retention readiness、normal/race/Core/Views/API/Web/Desktop 验证；真实有效 signing key 在执行前配置/验证。

当前 S2 intermediate-product-r02/source 的15路径 overlay manifest SHA 22421f02d6a7ca2688eeb95c1c056e04d72086cfac07f7d2ed076977e01ee24e、state=intermediate_not_final，只是明确未接受的依赖。guard/mode 实施证据不构成其最终独立 acceptance，不能用它固定 S3 的最终锁序。

## S3P-C01 正式处置

| 原序列 | v2 的合法路径 | 未来证据 |
| --- | --- | --- |
| A creator 替换他人的当前请求 | blocked 下直接无父 B2 拒绝；当前有权人用精确历史源做一个 R，真实提交 blocked→S 与 superseded links；之后另一个显式新 K/B2 保存真实 S 并正常 return | V2-01 |
| B 移除当前 recipient | 原 security depart 必须成功，旧 supersession 不归因 R；R 观察原 pointer=null、无 pending 时显式选工作状态；之后独立 B2 | V2-02 |
| C 旧 blocked / 不完整链后来 managed | 当前 project/policy 决定管理，不需要 stage-task 行；显式 human-selected R，不造旧状态/ETA/condition；B2 保存新 S | V2-03 |
| D 完整 B2 嵌在不可用旧 parent | 不能证明完整合法退出、cycle/missing/partial 均拒普通嵌套；无 pending 的 R 可以 supersede 真实旧行，null event provenance 不造 ID 或漏 denominator | V2-04 |
| E 原 requester 退出/停用 | 原 return_target_unavailable 不 reenroll；当前有 Transfer+ChangeStatus 的 Q 可显式 self-target R，from=to 如实，后续 B2 requester 是 Q | V2-05 |
| F 正常完整嵌套控制 | B2 return blocked→blocked 重新激活其精确完整 parent，parent 再恢复合法状态；无 R，pending-safe 出口仍须验证 | V2-06 |
| G 真正 pending/in_review | exact submission/event/current eligible creator 优先；不得工作状态、别的验收人、造 submission、改原 stage/M1 绑定或盲用 archived→in_progress fallback。后续正常接受/退回独立 | V2-07 |
| H required creator 真正不可用 | 正常 repair 拒绝；不新增 R38 repair/proxy；独立合法 cancel/reopen/successor 产生新 lifecycle，并依 S2 重确认。仅失去 source membership 不等于 creator 不可用 | V2-08 |

实际落点：handoff.go:153/206、workflow.go:233/277/375/559、access.go:86–132。handoff 的 PreviousStatus 始终是 before.Status；同一内部 primitive 可支持 repair status+handler 的一次事务和真实 self-target，但不能用普通 Transfer HTTP 再 ChangeStatus HTTP。

原 operation 检索在新 mutate callback 前；repair 需与 directed 类一致以允许旧原键在处理人变化后恢复，新 callback 再核新写权。现有 Supersede… 只 exec、没有 R link/count；必须增加窄原事务 UPDATE/精确 affected count、旧行不可变 link 和同 commit event/receipt，不建立平行 ledger。所有必需写失败全回滚，未知 commit 保留原 K；R 后不自动 B2/Resume/dispatch/消耗旧批准。

## S3P-C02 正式处置

| 边界 | 选定合同与结论 |
| --- | --- |
| presence / 旧 digest | raw field 区分 absent/null/invalid；合法老 omitted 字段用 nil/omitempty 保留。新 managed 要求及 ETA 时钟在原键恢复之后；不回填旧请求 |
| finite text / ETA | reason/condition 各4096 decoded UTF8 bytes、非空无NUL；显式 timezone、0–6 fractional digits、原输入进 digest、持久 UTC microsecond instant 对 final DB clock。旧 ETA 过时不抹失败/重算旧回执/调度 |
| current vs history | complete metadata ≠ actionable；legacy/unavailable/旧服务端unknown分别表达；实际 pointer null 与 dangling pointer不同，current authority/source access epoch保护完整响应 |
| runtime-v1 | protocol:217/256/348 对旧 typed fields/version严格；snapshot source:128返回原 note。仅新live/new snapshot可用既有 missing string；旧bytes/hash/source不改，不增加 structured runtime metadata/version/capability |
| selected project | GET可选project_id；not_selected managed:null保留choices；known精确source/project/revision0或当前修订/boolean。缺schema503；真正Create parent inheritance与当前policy锁独立，Source-only options不能作创建许可 |
| 原 creation result | 使用现有409 status_unavailable及五字段 own rejected result，savepoint回滚后才提交；原键旧成功/拒绝不重新 admission。该code已在server/Core allowlist |
| history provenance | 真实handoff tuple/upper/revisionkeyset，旧event linkage为null并入页。historical_information basis仍只认精确真实origin event；legacy不能借读获得 repair权限 |
| denominator / paging | exact full repair relation count为非负int64 decimal，0/51+与legacy均保留。任何一条读错误/超限不伪装完整；opaque cursor不是receipt/权限 |
| HMAC/current authority | history-only domain绑定viewer/root/projection/revision/lifecycle/authority/upper/last/repair ID。实际cached JWT key先验证，公开dev默认503；source-only participant/account nullable且用真实member身份防remove/rejoin恢复，密钥轮换按进程 |
| encoded budget | 新读显式5秒、encoded JSON含metadata/escaping/newline≤65536；limit1..50/cursor≤2048；真实next cursor先预留，首条完整 item超限503。comments budgeter的omit/reversal/envelope不能直接套 |
| normal return audit | actual return actor/time/restore/fallback取原row/event，旧不可验证则null/unavailable。pending正常blocked nesting可保留，但最终非blocked出口要in_review/currentcreator或显式pendingrepair |
| repair options/replay | 新action明示加入status allowlist/closed parser；只repair valid context接受empty default，旧默认契约保持；新写权限在旧receipt recovery之后重查 |

## 验证

- S3 最新 check.jsonl **18/18** 引用及 prd/design/implement 已加载；复用规范均核对原hash。原S2 hook不决定此slice的ActiveTask。
- 41 个实际ROOT源码文件捕获/复核无字节漂移；这是静态源码证据，不是完整/编译/accepted final source graph。
- 文档JSON示例、closed创建group、tuple/provenance/预算/key约束、markdown空白、引用与SHA检查通过。原v1/v2、旧review及档案报告前后hash未变。
- 产品 Lint / TypeCheck / tests：**NOT_RUN**；数据库/API/browser/Provider/账户/迁移/产品执行 **0**。未写镜像测试，未Git修改/提交/分支，未task.py start或改变当前S2 pointer。
- S3状态仍planning；父24例不变，本报告不核销业务验收或原NOT_RUN。

## 主会话同步及激活清单

1. design 纳入通过后的 v2 + r05 wire（本次 final SHA d9934020f24751ee722bb0bf115a0e587a37a4ee6bb0465cf12e26fee90e76c5）与本 review，明示一步R/独立B2、pending优先、legacy/provenance/keyset/HMAC/65536/5秒/旧hash与runtime-v1边界。
2. implement 将最终accepted S2 manifest/lock helpers作为先决条件，单一server owner接续shared handoff/workflow/SQL/handler；精确载明root-only effective-project创建guard、source-only nullable authority、key实际配置时点和原事务status/affected-count审计。不能以intermediate15覆盖作final引用。
3. implement/check jsonl 保留当前18真实引用与原失败/timeout范围；加入本 review、最终S2 acceptance/manifest与auth jwt/main、creation/options/actualIssueCreate、handoff/workflow、recall JSON sizing、human/runtime schema等确切参考。此review引用不替代实现/后续独立验证。
4. 实施/检查沿用V2-01–18并补新wire的project0/on/off/继承/旧键、0/51+legacy affected、nullable source-only authority、secret rotation、oversize503、原事务fallback正例和closed schema/real Web/Desktop。不得把同矩阵重复套DOM、source capture或研究PASS当产品验收。

私有证据：/Users/artisan/.multica/acceptance/pay241-20261115-continuation/resume-r05/s3-plan-check。before/after inputs、原wire与新wire、结构检查及本报告JSON保存精确来源/hash/每项disposition。
