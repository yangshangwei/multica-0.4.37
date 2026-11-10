# Research: v1 可豁免检查、批量结果与强制政策启用合同

- Query: 在已批准 D1–D5 与后续采用推荐方案的授权下，核对可安全 skip 的现有检查，并收敛 D6 旧客户端/批量逐项结果、D7 政策版本及回退边界。
- Scope: internal；读取源码、规格与任务决定；仅新增本研究，不修改产品、任务状态或旧证据。
- Date: 2026-11-11
- Worktree: `/Volumes/artisan/code/2026/mc-delivery-completion-enforcement-20261110`。
- Decision provenance: 当前 `research/decision-register.md:94` 记录用户「A 后续如果有选择，请默认按照推荐执行」；主会话采纳本报告的 v1 空豁免清单、逐项完成回执和非全批持久幂等边界。

## Findings

### 1. 现有固定观察器没有可安全豁免的检查

`server/internal/handler/delivery_evidence.go:347` 的完整结果分支如下。这里没有测试覆盖率、审批人数或可选质量阈值等独立检查。

| 当前结果/原因 | 源码锚点 | v1 是否可 skip | 理由 |
| --- | --- | --- | --- |
| hold / spec_digest_mismatch | `delivery_evidence.go:348` | 否 | 精确规格绑定失配 |
| hold / source_revision_mismatch | `:351`、`:378` | 否 | 声明提交、实际制品提交或数据库已观察 PR head 不一致 |
| hold / evidence_expired | `:354` | 否 | 已批准 D2 禁止豁免到期；期限也不能通过恢复延长 |
| hold / producer_task_failed | `:357` | 否 | 生产者 cancelled/failed 是原固定证据合同的失败，不是已定义的可选质量检查 |
| unknown / evidence_missing | `:213`、`:360` | 否 | 缺失执行/结果，不能制造成功交付事实 |
| unknown / execution_unknown | `:362` | 否 | 原不确定执行不可被豁免升级 |
| hold / execution_failed | `:364` | 否 | 实际受控制品失败，不能宣称成功制品 |
| hold / receipt_invalid | `:367` | 否 | 审批/执行完成事实、规范回执或精确输出身份不完整 |
| hold / artifact_digest_mismatch | `:370` | 否 | 精确制品绑定失配 |
| unknown / source_head_missing | `:373` | 否 | 缺少来源事实 |
| unknown / source_head_ambiguous | `:375` | 否 | 存在互相冲突的同优先级来源事实 |
| pass / evidence_verified | `:381` | 不适用 | 没有失败检查可豁免 |

在结果分支之前，调用者及原生产者/发起人/运行时所有人/审批人/领取人的当前权限、issue/project revision、task generation、规格、执行与审批绑定已经检查；这些错误是拒绝而非可选检查。`deliveryReceiptValid`（`:388`）还要求 succeeded、确定的 claim/finalization、正确 artifact identity、size、规范 JSON 和 SHA。单纯将 producer failed 纳入 skip 虽可能仍有 tar，也会扩张现有固定政策，而不是落实已批准的有限豁免。

**采纳建议：当前政策返回 `skip_supported:false` 和 `waivable_checks:[]`。** D2 已批准的批准人、理由、<=24 小时且不超过证据期限、指定消费者、可撤销和同事务一次消费规则保留为未来真实可豁免检查的必要合同；它不证明当前存在这种检查。禁止把任意 hold 或整个固定政策当成一个可 skip 项。

不创建永不成功的 skip 表、领取/撤销 API 或伪成功记录。不为提供 skip 人造质量检查。完成请求中出现 skip 意图时明确拒绝 `delivery_skip_not_supported`；严格请求校验不得静默忽略。未来只有引入真实且独立的可豁免检查、明确政策/合同版本并验证后，才能开放对应机制。这里的拒绝能力不能被报告为“可用 skip 已交付”，父 M1 仍保留该限制。

### 2. D6 当前真实消费者与兼容风险

| 当前代码 | 已存在行为 | 本包必须照顾的影响 |
| --- | --- | --- |
| `server/internal/handler/issue.go:4811`、`:4828` | `{issue_ids, updates}`；共享更新体，逐 issue 调用普通 writer | 共享 `updates.expected_revision` 不能代表不同 issue 的完成绑定；需逐项绑定 |
| 同文件 `:4886` | 无有效 mutation 时 200 `{updated:0}` | 保留准确的零写入响应，不伪报 N 项完成 |
| 同文件 `:4966`、`:5118`、`:5124`、`:5210` | 有些逐项错误 continue，有些直接 return；先前事务可能已提交，但错误响应丢失 prefix；末尾只有 updated | 新响应必须记录真实已提交前缀、当前拒绝/不确定、剩余未尝试 |
| 同文件 `:3582`、`:3682`、`:3916` | 随机 operationID 供 iteration 留证；仅 55P03 事务重试；最终 commit error 直接返回 | 此 ID 不是客户端幂等身份；最终 commit 失败不可当 rejected 或重试 |
| `packages/core/api/client.ts:3124` | 直接 `fetch` 并断言 `{updated:number}`，无 schema | 新返回须解析，非法/截断/未知结果不能变成功 |
| 同文件 `:642`、`:2263`、`:2333` | ApiError 保留原 JSON body；非 2xx 才 throw | 可在现有错误类型上解析逐项结果，无须新传输层；不能用 207 表示部分失败 |
| `packages/core/issues/mutations.ts:431`、`:543`、`:579` | 乐观改全部；错误回滚全部；settle 刻意不重取 list/myAll | prefix 已提交时会显示旧状态；必须按逐项结果校正并重取相关列表/详情 |
| `packages/views/issues/components/batch-action-toolbar.tsx:107` | 任意成功 Promise 都按所选 count 提示全成功 | partial 必须拒绝 Promise；成功计数取服务器 updated，不能取请求长度 |
| `packages/views/issues/surface/use-issue-surface-actions.ts:164` | 返回类型不传计数，等待 mutation 后直接结束 | 成功计数/错误必须传过 surface adapter，不能吞掉不完整结果 |
| `packages/views/modals/run-confirm.tsx:180` | 批量 Promise 成功后关弹窗，忽略结果 | 不完整返回不得自动关窗，也不得误报全完成 |
| `apps/mobile/data/api.ts:288`、`:816` | 仅找到单 issue PUT；非 2xx 保留 body，但用户文本只读取 message | 缺绑定拒绝应同时给 error/message 可读文案；无须新增移动端 batch |
| `server/cmd/multica/cmd_issue.go:1453`、`server/internal/cli/client.go:417` | CLI 使用单 issue PUT，非 2xx 为错误；没有 batch API 调用 | 缺绑定返回明确非成功；新增完成路径不应复用无幂等键的自动重发行为 |

仓库搜索未发现 mobile 或 CLI 调用 `/api/issues/batch-update`；Web/Desktop 共用 core/views。`packages/core/query-client.ts:13` 与 mobile query client 的 mutations 均为 `retry:false`。CLI 带键 POST 的 `GetBody=nil`、禁止 redirect 和不重放模式见 `server/internal/cli/client.go:367`，可借用该边界，不能把新 key 放在 client 全局。

### 3. D6 推荐并采纳的结果合同

保留现有 endpoint、`issue_ids`、`updates` 和数字 `updated`；为完成意图增加按 issue 对应的显式绑定（issue expected revision、authorization ID、原操作 key），不能传另一人的 evaluation key。结果 `results` 按输入顺序，每项含 `index`、请求中的 `issue_id`、`state` 和稳定 `code`。有权且确知提交才可附 committed revision/完成 receipt ID；拒绝或失权时不增加原观察、私有评估和未授权资源详情。

| state | 必须有的事实 | 不可推断 |
| --- | --- | --- |
| committed | 本项实际事务确定提交，完成状态与授权消费回执同事务 | 不等于全批成功，也不证明后续外部副作用完成 |
| rejected | 请求或事务确定未产生本项修改，拒绝原因已知 | 不能用于提交结果不明 |
| not_attempted | 因前项失败/请求级拒绝/无 mutation 而根本未进入本项写事务 | 不能用没有回执推断 |
| unknown | 写入/commit/响应链无法证明是否提交 | 不允许自动接管、补偿或再次消费 |

`updated` 只统计当前响应已确定 committed 的项，unknown 不计入；它是已知已提交数量，不是精确最终总量。请求无 mutation 保持 200/updated=0，可以加 no_changes 结果。新协议拒绝重复/不对应的逐项绑定，绝不让一个授权供多项使用。

按输入顺序执行，遇第一个拒绝或不确定立即停止；保留先前提交，后续标 not_attempted。这保持逐条事务，**不是全批原子**。原来静默跳过/错误丢失 prefix 是本包明确修正的异常语义，不能声称 off/observe 的所有历史错误行为完全未变。请求级语法、状态、共同项目或协作批量禁令仍在任何写入之前拒绝。

HTTP 规则：全部确定成功为 200；确定拒绝使用原有 400/403/404/409 或缺少完成绑定的 428；有已提交前缀仍返回非 2xx，并给 `code:batch_update_incomplete`、`cause_code`、`updated` 和全部逐项结果；未知提交为 503、`code:batch_update_outcome_unknown`。不用 207，因为已有 fetch 会将其当成功。缺绑定的 428 可复用已有 iteration confirmation 的约定（`iteration_confirmation.go:14`），不能回退到旧无门禁写入。所有错误同时保留 error、message 两种现有消费者可见的短文案。

示意（第 0 项已提交，第 1 项缺授权，第 2 项未尝试）：

```json
{
  "code": "batch_update_incomplete",
  "cause_code": "delivery_completion_authorization_required",
  "error": "1 item updated; 1 rejected; 1 not attempted. Refresh before continuing.",
  "message": "1 item updated; 1 rejected; 1 not attempted. Refresh before continuing.",
  "updated": 1,
  "results": [
    {"index": 0, "issue_id": "<requested UUID 1>", "state": "committed", "code": "updated"},
    {"index": 1, "issue_id": "<requested UUID 2>", "state": "rejected", "code": "delivery_completion_authorization_required"},
    {"index": 2, "issue_id": "<requested UUID 3>", "state": "not_attempted", "code": "previous_item_failed"}
  ]
}
```

若在 loop 中提前返回，已提交子任务的 parent/stage 通知必须仍按已提交集合执行一次；当前 `notifyParentsOfBatchChildDone` 位于 loop 末尾（`issue.go:5220` 附近），直接 return 会遗漏。它仍是 best-effort 后处理，不能把通知失败改写成状态未提交。

#### 恢复保证的精确范围

主会话选择**不新增整个 batch 的持久幂等/恢复引擎**。只复用新完成授权的逐项消费回执：相同原操作 key、授权及请求 digest 在完整当前权限下恢复原消费结果，不重复状态写入或副作用；改变 payload/目标/status/revision 必须冲突。已知 consumed 的恢复必须先于新的状态准入分支，防止重放在 D4a 的 done→done 分支再次写字段。仍保留当前身份/资源权限检查，消费记录不公开私有评估。

遇 unknown 或 HTTP 丢失时，调用者 GET 每个原授权/原完成回执；没有确定终态不等于可重做。没有完成授权的 legacy no-key 请求不会自动获得幂等，非完成项也没有持久 batch 回执。本包不提供 GET 全批结果、不自动从 not_attempted 续跑、不自动换新 key。读取当前 issue 状态只能显示当前事实，不能证明某个无键历史请求是否提交。恢复后继续是新的明确意图，并重新取得当前权限与版本。

#### 客户端最小必要调整

- core API 用现有 `parseWithFallback`/zod 解析新成功和 ApiError.body；无法解析的写响应返回“不确定”，fallback 不能是 `{updated:N}`。允许旧服务器 `{updated:n}` 形状，但数量不足所请求范围时禁止全成功文案并重取；不要补造逐项 committed。
- 新结果存在时验证结果索引、请求 ID 对应、数量、updated 与 committed 数一致。遇未知枚举、重复项或缺项保守不确定，不能 silently 丢弃。
- batch mutation 可先回撤乐观快照，再用已确定响应/读取校正；对已提交、unknown 和未有完整结果的批次，重取本 workspace 受影响 list/myAll/flat/table/detail/children，避免 `staleTime:Infinity` 长期显示假回滚。不以错误到达顺序覆盖更新的 WS revision。
- toolbar/surface/run-confirm 传播不完整结果；实际全部成功才给成功提示，计数取 updated。partial/unknown 保留选择与可恢复信息，不自动重交。
- 仅提供门禁拒绝与结果真实性所需的兼容消费，不等于新增 UI/CLI 自动提示、自动验收或自动代办。

### 4. D7 模式、版本、部署和回退

当前 `776_delivery_policy_evaluation.up.sql:4` 固定 `policy_version='delivery-artifact-observe-v1'`，`:5` 仅允许 off/observe；`delivery_policy.go:58` 做相同请求校验。政策查询按独立 revision 取最新（`delivery_policy.sql:34`），写入锁 project 行，包括首次无 policy 情况（`:31`）。

**版本建议：** 继续保留 `delivery-artifact-observe-v1` 为固定观察检查算法版本，不因为增加门禁而重解释原观察。新增 `mode=enforce`，为新授权/消费事实冻结独立 `completion_contract_version=delivery-completion-enforcement-v1`。政策能力可增量返回该合同版本、skip_supported 与 waivable_checks，缺少能力字段不推断旧服务器支持。

1. 默认 off、不自动启用。observe 继续只观察；显式 completion-hint 继续只是只读快照，所有成功 `enforced=false/usable_for_completion=false`。这里的 enforced 描述提示动作本身，不声称所在项目没有强制政策。
2. 当前 owner/admin 通过原政策接口、非空理由和独立 revision CAS 追加 enforce。重复设相同 mode 仍是新 revision；off→observe→enforce 或 enforce→off→enforce 的 ABA 不复用旧授权。
3. 新评估允许在当前 observe 或 enforce 的相同固定观察算法下创建，必须绑定该当前 policy revision。现有 `delivery_evaluation.go:172` 与 `:244` 只接受 observe，必须有意扩展；否则 enforce 后无法获取可用的新评估。原结果不可变，旧 revision 的历史 pass 只恢复为 stale，绝不自动晋升或延长。
4. 当前 enforce 修订下的新评估仍不是许可；明确人类验收/授权和每个实际消费事务继续独立复核。hint 读取当前 enforce 模式时同样仅返回 matching/attention，不代表已有有效授权。
5. 政策改变使尚未消费授权失效；off/observe 写入不顺便消费旧授权。已经消费的回执保留，只证明原冻结完成事件；恢复不重新准入或续期。D4a 后续非项目修改不改写历史。
6. 在所有普通/move/batch/create/comment-child/VCS/协作/R38 writer 及 D4b 项目调整/删除路径覆盖并实际验收之前，服务端禁止接受 enforce 设置。启用入口最后交付；源码、索引 readiness 不能单独证明全部部署实例均已升级。
7. 若配置开关关闭、依赖未就绪或代码不支持某个已存在 enforce 合同，相关新完成必须拒绝；不能将其解释为 off。合法 owner/admin 的明确降级政策操作与历史读取可在其各自完整 schema/权限合同下继续。
8. 必须先升级/验收所有实际 writer 进程，再允许项目启用。旧二进制完全不执行门禁，混跑旧实例或直接回退可绕过；新进程上的 feature flag 不能保护旧进程。部署前提应写入操作文档，本研究不声称已部署验证。
9. 业务回退采用 CAS 追加 off/observe、理由与 actor 留证；绝不覆盖旧政策或删除授权/消费历史。回退旧二进制前必须停止准入、确认没有仍 enforce 的项目并排空旧请求；若旧二进制不能安全保留新历史的权限/删除边界，保持新门禁代码，仅回退政策。
10. 新迁移追加，不改 776–780 历史文件；无 FK，新增索引各自 CONCURRENTLY 单文件。新增 mode/合同/表/索引的精确 readiness 单独验证，不能只看 schema_migrations 或原 `DeliveryPolicySchemaReady`。降级必须先停准入，并在任何删除索引/收窄 enum 前拒绝保留的新事实；不能删历史换取回滚成功。

## Files Found / Implementation Dependencies

- `server/internal/handler/delivery_evidence.go`：唯一现有固定观察器；要在 service writer 的原事务复用，不能复制算法或反向 import handler。
- `server/internal/handler/delivery_policy.go`、`delivery_evaluation.go`、`delivery_completion_hint.go`：mode 校验、policy response 能力、当前适用性与只读提示。
- `server/pkg/db/queries/delivery_policy.sql`、generated bindings、追加迁移、`server/cmd/migrate/main.go`：新增 enforce mode/授权回执的数据库合同、精确 readiness、并发索引注册与保留降级。
- `server/internal/handler/issue.go`：普通事务、每项 batch 结果、提前停止仍处理已提交通知、commit 结果分类。
- `server/internal/service/issue.go:249`：现有重试仅 55P03；门禁单次路径不得自动重跑不确定消费。`issue_status_write.go`、协作 lifecycle/R38 与 project 调整由各 writer 所有者接入。
- `packages/core/types/api.ts:52`、`packages/core/api/client.ts:3124`、新增相邻 schema：请求逐项绑定与结果解析。
- `packages/core/issues/mutations.ts:431`、views 的 batch toolbar/surface/run-confirm：真实 prefix/unknown 展示和缓存恢复。
- 内置 `multica-working-on-issues` 与 `multica-creating-agents` skill/source-map：已安装 agent 的旧完成操作拒绝语义、恢复限制和 skip 不支持必须同步。

## Required Validation Shape

以下是待实施验收要求，本研究没有运行这些测试。

- 真实认证普通/custom done 的缺授权 428、失权拒绝、旧评估不能自动完成；原 off/observe 单更新、batch、create 的合法成功仍通过。保留现有 `TestDeliveryCompletionHintRouterLegacyCompletionsRemainIndependent`（`delivery_completion_hint_integration_test.go:433`）的 observe 对照，不将它改成 enforce 预期。
- batch `[合法 A, 拒绝 B, 合法 C]` 实际只改 A；B 无状态/消费/iteration 写入，C 未尝试；HTTP 非成功、updated=1、结果齐全。unknown 注入分别覆盖 commit 前确定回滚与 commit 后响应不明；后者原回执可恢复一次且不重发通知。
- 缺绑定、撤权、当前版本/政策失效、自定义状态归档、项目迁移竞争在首项和中间项均覆盖；跨账号/私有资源不因结果细节泄漏。不能用静态矩阵代替真实状态与回执行。
- 逐项原 key 相同请求只恢复；改 payload、消费者或授权冲突；旧 no-key 请求明确不具有全批幂等。unknown 既不自动重试也不自动续跑 not_attempted。
- `issue_batch_test.go` 原 no-mutation、stage、实际计数与父阶段一次通知回归；`issue_status_test.go` custom/归档竞态回归；`project_association_concurrency_test.go` 的锁与删除竞态继续有效。
- core schema：旧 `{updated}`、完整结果、partial 错误、截断 JSON、缺项/重复/未知枚举、不一致 updated、401/403 私有边界；mutation 真实 QueryClient 下验证 prefix 不被当全部回滚且所有受影响 cache 重取；views 验证 partial 无全成功 toast/关窗。`e2e/core-business-workflow.spec.ts:52` 当前精确比较 `{updated:3}`，需调整为验证新完整合同与真实三个状态，而非仅放宽断言。
- policy observe↔enforce、ABA、同值追加 revision、旧 pass/current stale、新 enforce 评估、off 恢复、角色/凭据撤销、数据库时钟到期、schema 缺失/错误索引以及当前政策与各消费事务两种提交顺序。
- skip_supported=false、空 allowlist 与每个现有 hold/unknown 均拒绝 skip；没有伪成功的 skip/消费记录。
- 实际迁移升级、空库可回滚、每种保留授权/消费事实及并发写入拒绝降级；参考 `delivery_policy_test.go` 与 `server/cmd/migrate/delivery_policy_test.go` 的实际迁移/无 FK/索引形状断言。各 DB 包及最终普通/race 使用独立新库，冻结源码和具名日志，所有失败保留。

## Related Specs

`.trellis/spec/server/delivery-policy-evaluations.md`、`delivery-completion-hints.md`、`delivery-evidence-observation.md`、`controlled-artifact-execution.md`、`durable-delegation-receipts.md`、`database-test-isolation.md`；PRD 的 G2/G4/G6/G7/G9–G13 与 decision-register 中 D1–D5。

## External References

无网络资料或新依赖研究。所有结论来自本地真实源代码、测试和已记录业务选择；没有外部供应商调用。

## Caveats / Not Found

- 没有找到现有可选质量检查，因此 v1 skip 不可用；批准其通用治理规则不等于凭空存在可豁免项目。
- D6 仅逐项完成回执恢复，无全批持久账本；原无键失败后读取当前状态不能证明历史请求结果。
- 本研究不声称新门禁、迁移、客户端修改、部署或运行验收已完成。
- 研究期间几次读命令使用不存在的候选路径（`server/cmd/cli`、`apps/mobile/lib/api.ts`、`server/internal/handler/issue_test.go`、`server/cmd/multica/client.go`、`server/internal/service/project.go`、`comment_delegation_receipt.go`）及不匹配的 `artifact*` glob，命令返回读取/路径失败；随后通过 `rg --files` 确认实际路径再读取。没有执行产品、测试或数据库命令，这些不是产品 red，也没有改写旧失败材料。
- Git 操作、DB/容器操作、真实模型调用、外部消息均为 0；只新增本研究文件。
