# Research: D6 批量更新公开协议冻结

- Query: 固定 P2 后端与 core/views 可独立实现的 wire 合同，不依赖授权账本内部结构。
- Scope: internal；只读源码/配置，仅写本文件。
- Date: 2026-11-11
- Worktree: `/Volumes/artisan/code/2026/mc-delivery-completion-enforcement-20261110`。
- Decision: 主会话已明确批准本文请求字段名称、结果形状、无操作例外及旧计数解析规则；已同步 `verify_completion_sources` 授权合同研究。

## Findings

### 1. 固定请求形状

复用 `/api/issues/batch-update`、既有 `issue_ids` 和 `updates`；单项更新的新增字段也固定如下。所有字段是可选扩展，旧的合法 off/observe 更新无需增加字段。

```go
type DeliveryCompletionBinding struct {
    AuthorizationID       string `json:"authorization_id"`
    OperationID           string `json:"operation_id"`
    ExpectedIssueRevision int64  `json:"expected_issue_revision"`
}
type BatchDeliveryCompletionBinding struct {
    IssueID string `json:"issue_id"`
    DeliveryCompletionBinding
}
// Append to the existing UpdateIssueRequest; do not replace its other fields.
// Completion *DeliveryCompletionBinding `json:"completion,omitempty"`
// ProjectChangeReason *string `json:"project_change_reason,omitempty"`
type BatchUpdateIssuesRequest struct {
    IssueIDs           []string                         `json:"issue_ids"`
    Updates            UpdateIssueRequest               `json:"updates"`
    CompletionBindings []BatchDeliveryCompletionBinding `json:"completion_bindings,omitempty"`
}
```

```ts
export interface DeliveryCompletionBinding {
  authorization_id: string;
  operation_id: string;
  expected_issue_revision: number;
}
export interface BatchDeliveryCompletionBinding extends DeliveryCompletionBinding {
  issue_id: string;
}
// Add to UpdateIssueRequest:
// completion?: DeliveryCompletionBinding;
// project_change_reason?: string;
export interface BatchUpdateIssuesOptions {
  completion_bindings?: BatchDeliveryCompletionBinding[];
}
```

- completion 与每个 completion_bindings 元素必须严格验证：规范非零 UUID、非负整数 revision、恰好已定义字段，不接受 null、重复/大小写别名字段或部分对象。Go 解码需区分 revision 缺失与合法 0，不能用零值默认为已提供。JS 必须 safe integer。
- batch 禁止 `updates.completion`，防止一个授权被全批共享；每个 binding 的 issue_id 必须对应请求中的唯一项，不可重复或额外出现。只需为需要完成授权的项提供 binding，不能要求所有 off/observe/非完成项都有。
- `completion.expected_issue_revision` 是该授权对应任务版本；不能用共享 `updates.expected_revision` 替代。单项请求如同时携带既有 expected_revision，两者必须一致；batch 不从共享 expected_revision 补造逐项版本。
- project_change_reason 放在 updates，与单项/move 使用相同字段；有效时去首尾空白、非空、有效 UTF-8、最多 1,000 个字符。它是控制/留证字段，不是 issue 字段或独立 mutation，不能乐观合并进缓存。是否需要理由仍在每项实际来源/目标政策事务里判断；无实际项目变化不生成调整记录。
- completion 与 project_change_reason 不能单独使 `hasMutation=true`。无实际更新字段仍为 no_changes，不能因带授权重复消费。原 consumed 授权的同请求恢复由 writer 在普通 done→done 写路径前识别，不由 batch 提前改写字段。
- 不新增 batch Idempotency-Key、全批 request_id 或全批恢复 endpoint。每项 authorization_id + operation_id 指向其原授权/消费回执，消费者可 GET 原记录恢复；前端不得自动生成替代 key 或重发。

### 2. 固定结果与错误形状

```go
type BatchUpdateIssueResult struct {
    Index    int    `json:"index"`
    IssueID  string `json:"issue_id"`
    State    string `json:"state"`
    Code     string `json:"code"`
    Revision *int64 `json:"revision,omitempty"`
}
type BatchUpdateIssuesResponse struct {
    Updated int                      `json:"updated"`
    Results []BatchUpdateIssueResult  `json:"results"`
    Code    string                   `json:"code,omitempty"`
    CauseCode string                 `json:"cause_code,omitempty"`
    Error   string                   `json:"error,omitempty"`
    Message string                   `json:"message,omitempty"`
}
```

```ts
export type BatchUpdateIssueState = "committed" | "rejected" | "not_attempted" | "unknown";
export interface BatchUpdateIssueResult {
  index: number;
  issue_id: string;
  state: BatchUpdateIssueState;
  code: string;
  revision?: number;
}
export interface BatchUpdateIssuesResponse {
  updated: number;
  results?: BatchUpdateIssueResult[]; // absent only for a valid legacy response
}
export interface BatchUpdateIssuesErrorBody extends BatchUpdateIssuesResponse {
  code: "batch_update_incomplete" | "batch_update_outcome_unknown";
  cause_code: string;
  error: string;
  message: string;
}
```

新后端在已解析、有权且有 issue_ids 的 batch 结果中总返回 results。每项位置与请求相同，`index===数组下标`、issue_id 与该输入匹配；不按 UUID 排序。updated 等于已确定 committed 项数，unknown 不计入；它不声称总计只提交了 updated 项。revision 仅在本项 committed 且允许披露时出现，代表该提交记录的版本，不保证仍是当前最新版本。无需公共 receipt_id、grant row shape 或原 observation。

对同一原完成请求恢复得到的 committed 表示该原操作已经提交；不声称本次重新写入。updated 统计该响应可确定的 committed，重放不再次运行发布/派发/父通知。

| state | 语义 | 本包输出 code |
| --- | --- | --- |
| committed | 事务确定提交，或合法恢复原完成提交 | `updated` |
| not_attempted | 后续未开始写入 | `previous_item_failed` |
| not_attempted | 请求级失败，未开始任何项 | `request_rejected` |
| not_attempted | 请求没有有效 mutation | `no_changes` |
| rejected | 本项确定未提交 | 下表有限原因 |
| unknown | 本项最终提交结果无法确认 | `commit_unknown` |

v1 的 rejected/cause_code 使用以下有限的公开类别；内部错误映射到这些类别，不暴露私有评估/授权存在性细节：

| code | HTTP（该原因导致停止时） | 用途 |
| --- | ---: | --- |
| `invalid_batch_update_request` | 400 | body/完成 binding 形状、重复/额外绑定等请求级问题 |
| `invalid_issue_id` | 400 | 本项 ID 非法 |
| `issue_not_found` | 404 | 本项不可见或不存在，合并外部可观察行为 |
| `invalid_issue_update` | 400 | 日期、stage、父关系、自引用/环、assignee 参数等确定输入错误 |
| `access_denied` | 403 | 当前动作主体/成员/autonomy/assignee 权限拒绝 |
| `triage_review_required` | 409 | 既有 intake/admission 拒绝 |
| `status_conflict` | 409 | 目标状态在事务前归档或失效 |
| `revision_conflict` | 409 | 当前任务版本变化 |
| `project_conflict` | 409 | 实际项目关联/删除或政策锁竞争 |
| `delivery_completion_authorization_required` | 428 | enforce 新完成缺逐项完整绑定 |
| `delivery_completion_denied` | 403/404 | 消费者或验收人当前权限不成立；保持原鉴权不可区分原则 |
| `delivery_completion_conflict` | 409 | 授权/政策/版本/意图/期限/证据不再有效，不能消费 |
| `delivery_project_change_denied` | 403 | enforce 移出不是当前 owner/admin 本人 |
| `delivery_project_change_reason_required` | 428 | 应提供项目变更理由 |
| `delivery_project_change_conflict` | 409 | done 未重开、同次迁移完成、来源/目标政策版本冲突 |
| `delivery_skip_not_supported` | 409 | v1 无可安全豁免检查 |
| `dependency_unavailable` | 503 | 在确定未提交前的 schema/数据库故障 |
| `commit_unknown` | 503 | commit 已尝试且结果不确定 |

这些是批量层公开归类，不要求重命名 ledger/service 内部错误。客户端把 code 当字符串解析，以 state 和 HTTP 决定成功/失败；未来未知 code 使用通用文案，不能转为成功。未知 state 或错误的状态组合则为无法确认的协议结果。

HTTP 200 只允许：全部项 committed，或明确无 mutation 时所有项 not_attempted/no_changes、updated=0。任何拒绝使用非 2xx、`code=batch_update_incomplete`；任一 unknown 使用 503、`code=batch_update_outcome_unknown`。cause_code 是首个停止原因。已有提交前缀保留，后续 not_attempted；不用 207，也不返回 200 partial。error 与 message 为同一短文案，分别兼容 core 和 mobile 的既有读取方式。

Middleware 401、无法解码 body、缺少 issue_ids 或请求级旧 helper 提前拒绝，可继续返回既有普通错误体而没有 results；这些都发生在 loop 前。客户端不能因某个旧错误体缺 results 就推断在未知服务器版本上全批未提交。保持原 HTTP 状态，重取请求涉及的资源。

### 3. 旧成功和错误的解析合同

新 `ApiClient.batchUpdateIssues(ids, updates, options?)` 返回 `Promise<BatchUpdateIssuesResponse>`。options 仅序列化可选 completion_bindings；既有双参数调用体不变。实现使用 `fetch<unknown>`、既有 `parseWithFallback`/zod，fallback 取 null 后显式抛错，不能 fallback 为一个成功计数。

1. 旧 `{updated:n}` 仅在 n 为非负 safe integer 且不大于请求项数时可解析；保留 `results` 缺失，不补造逐项 committed。`n===请求数` 可走旧合法全部成功。`n<请求数` 抛携带已知计数的 typed incomplete；原响应无法定位成功项，因此重取全部请求资源。
2. 无有效 mutation 的 `{updated:0}` 保留 no-op 成功。使用同一明确的 mutation-field 白名单判断，控制字段 completion/project_change_reason/suppress_run/handoff_note/expected_revision 不算 mutation。不要把修改字段是否真与 DB 相同交给客户端推测。
3. 新 results 必须长度准确、index/issue_id 一一对应、没有额外/重复项、revision 为非负 safe integer、updated 与 committed 一致。成功必须满足上一节 200 状态组合；有失败不能仅凭 HTTP 200 成功。
4. 对 ApiError.body 作相同解析。结构正确的已提交前缀、拒绝、未尝试和 unknown 通过类型化错误供 mutation 读取；原 ApiError 的 status/body/认证过期语义保留。不解析的错误、网络失败、截断 JSON、未知 state 都触发重新读取，不能当 definite rollback。禁止任何写重试。
5. 不需要实现新的 transport/client 或把 recovery endpoint 拼接到 UI：逐项恢复 UI 不属于本包；这里只保证拒绝诚实、缓存与实际提交一致。

建议一个最小 `BatchUpdateError extends ApiError`，仅附 `outcome?: BatchUpdateIssuesResponse` 与 `outcomeUnknown:boolean`；或等价的 schema+type guard，在既有 ApiError 上读取同样信息。不要依赖 throw message 的字符串匹配。缺 results 的旧错误体仍保留原错误类型并进入保守重取。type 名可由客户端 lane 决定，wire 名称不可变。

### 4. Surface/缓存接口及最小 UI 变化

`IssueSurfaceActions.batchUpdate`（`packages/views/issues/surface/actions-context.tsx:29`）由 `Promise<void>` 改为 `Promise<BatchUpdateIssuesResponse>`；`use-issue-surface-actions.ts:164` 返回 mutation 的结果。API 与 mutation 参数增加可选 completion_bindings，不改变旧双参数或 `{ids,updates}` 调用。surface adapter 不需要 ledger 内部类型。

toolbar（`batch-action-toolbar.tsx:107`）取直接 mutation 或 surface 的结果，成功提示使用 `result.updated`；updated=0 不显示“已更新全部所选”。partial/unknown 保持选择、显示已有失败反馈，不发成功 toast。run-confirm 只在该 Promise 真正成功时关闭。不得将 mock 中的 undefined 当成旧真实 API 成功；相应测试 mock 应返回 `{updated:n}`。

`useBatchUpdateIssues`（`packages/core/issues/mutations.ts:431`）在乐观 patch 前剔除 completion/project_change_reason。保留合法全部成功时已有避免整板 refetch 的行为；不完整/未知错误则撤回本次乐观 patch并刷新当前 workspace 的请求相关 detail/list/myAll/flat/table/children 与衍生视图。不要只回滚不刷新，因为旧代码 `:579` 的 settle 故意跳过 list/myAll，配合 staleTime=Infinity 可长期显示错误。不能回写较旧 revision 覆盖较新 WS；可以依靠失效/读取解决而不从请求内容伪造 committed issue。

### 5. 整个后端 loop 的出口核对

| 当前锚点（`server/internal/handler/issue.go`） | 当前分支 | 新结果处理 |
| --- | --- | --- |
| `:4830`–`:4963` | body、iteration confirmation、IDs为空、认证、无 mutation、priority、autonomy、workspace、协作、共享状态/项目预检 | loop 前拒绝，updated=0；解析/认证不足时旧错误体可保留。no mutation 输出 no_changes |
| `:4972` | invalid UUID 或 GetIssueInWorkspace error 后 continue | 本项 rejected 并停止；数据库读取错误不混成 not found；后续 not_attempted |
| `:5010`–`:5109` | assignee/date/parent/stage/作用域错误 continue | 确定输入/权限错误归类 rejected 并停止；适合共享语法的检查可前移，但项目/主体等仍逐项当前复核 |
| `:5054` | 父链读取失败会 break traversal | 读取失败应明确拒绝，不把没有检查完环当检查通过 |
| `:5112` | issueAdmissionMutation 的 write helper 直接写响应 return | 先取 domain error，交统一 batch 收尾，不能直接丢弃 committed prefix |
| `:5118` | updateIssueAtomically 执行真实事务 | 成功返回立即加入 committed；在 publish/dispatch 等后处理之前记录确定提交事实 |
| `:5124` | access/admission/project/status race helper 直接 return；其他 error continue | 改统一记录本项 rejected/unknown 后退出 loop；不能由写 HTTP 的 helper 提前结束 |
| `:5139`–`:5207` | publish、可能 enqueue、汇总 childDone | 仅首次实际 committed 执行；恢复原回执不重做。后处理失败不推翻已提交状态 |
| `:5210`、`:5219` | updated++ 和 loop 后聚合父通知 | 统计所有已确定 committed；早停也要聚合实际已提交 child 集合一次，再统一写响应 |

`updateIssueAtomicallyOnce` 的 commit 在 `:3916`，返回普通 error，必须带上“是否已经尝试 commit/是否确定 rollback”的类型事实。SQL/授权/schema 错误且没有 commit 可为 rejected；网络/commit 确认丢失为 unknown。现有 55P03 重试（`service/issue.go:249`）仅适用于明确回滚的旧项目关联事务；门禁授权消费路径按单次合同拒绝，不能将 unknown 传回重试器。

不调整已经确认的全局 preflight 协作禁令；不使逐项原子变为全批原子。此次将旧静默跳过/丢失 prefix 改为显式失败，是有意修正，不能宣称旧异常行为完全未变。合法 off/observe 批量日期、stage、assignee、null 清空、项目调整、custom done、suppression/parent 通知都需回归。

## Canonical Tests / Commands

本文没有执行以下命令。执行时由主会话为每个 DB 包及 normal/race 分配新专属数据库并保存原始日志。

| 所有者 | Canonical 文件/测试 | 证明内容 |
| --- | --- | --- |
| 后端 handler | 新 `issue_batch_delivery_test.go`；`TestBatchUpdateDeliveryOutcomeMatrix`、`TestBatchUpdateDeliveryCommitRecovery`、`TestBatchUpdateDeliveryCommittedParentNotifications` | 真实 `[committed,rejected,not_attempted]` 与 unknown，状态/回执/队列/父通知不伪造或重跑 |
| 后端 handler 回归 | `issue_batch_test.go` 的 `TestBatchUpdateNoMutationReturnsZero`、`TestBatchUpdateValidUpdatesPersistAndCount`、`TestBatchUpdateStageOnly`、`TestBatchChildDone*`；`issue_status_test.go`、`issue_update_project_scope_test.go` | 合法 off/observe 保持，no-op/custom/原字段语义有效 |
| 真实认证 router | 新 `delivery_completion_enforcement_integration_test.go` 中 `TestDeliveryCompletionEnforcementRouterBatchOutcomes` | 真实身份、实际 tar/SHA 和中间项拒绝/提交后不确定/原授权恢复；旧客户端无绑定428 |
| core API（node） | 新 `api/issue-batch-update.test.ts` | 唯一协议矩阵：请求序列化、旧新成功、partial、计数/索引/ID/未知/截断、无自动重发；文件首行 node environment |
| core mutation | 现有 `issues/mutations.test.tsx` batch describe | 真实 QueryClient 的 prefix/unknown 重取、合法全成功不 blanket refetch、控制字段不进缓存、较新 WS 不回退 |
| views | `batch-action-toolbar.confirm.test.tsx`、`use-issue-surface-controller.test.tsx`、`run-confirm.test.tsx` | 只测计数/错误传递和不假成功，不重复协议矩阵 |

已核对 package.json：pnpm 10.28.2，core/views 均 `test:vitest run`、`typecheck:tsc --noEmit`、`lint:eslint .`。catalog Vitest ^4.1.0、TypeScript ^5.9.3、zod ^4.1.5；无新依赖。core 默认 node，views 配置 jsdom/react plugin。纯协议矩阵保持 node；mutation 现有 TSX 使用其原 jsdom 声明。Turborepo 是含注释 JSONC，不用严格 JSON parser 解释。

```sh
pnpm --filter @multica/core exec vitest run api/issue-batch-update.test.ts issues/mutations.test.tsx
pnpm --filter @multica/views exec vitest run issues/components/batch-action-toolbar.confirm.test.tsx issues/surface/use-issue-surface-controller.test.tsx issues/surface/use-issue-surface-actions.collaboration.test.tsx modals/run-confirm.test.tsx
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/core lint
pnpm --filter @multica/views lint
```

```sh
# DATABASE_URL must identify this package's newly allocated database.
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -json -p 1 ./internal/handler -run '^(TestBatchUpdate|TestBatchChildDone|TestBatchCustomTerminalStatusEntersStageBarrier|TestWritesRejectAnArchivedStatus|TestArchiveCommitsInsideTheWriteRaceWindow|TestProjectAssociation)' -count=1 -timeout=10m
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -json -p 1 ./cmd/server -run '^TestDeliveryCompletionEnforcementRouterBatchOutcomes$' -count=1 -timeout=10m
```

最终 race 将 `-race` 加在 `test` 后，另建数据库，不复用上述普通轮。新测试名必须实际存在，库存核对防止 regex 零选中冒充通过。前端测试同样核对库存：core 配置 passWithNoTests=true，所以 exit 0 本身不证明选中测试。

已有 `e2e/core-business-workflow.spec.ts:52` 精确比较 `{updated:3}`；如运行相关 production Web 流程，改为验证新的全部逐项 committed 和实际三项状态，保留未选择 control；不能只改成宽松匹配后宣称 partial 已测。此处不启动浏览器/部署环境。

## Related Specs / References

- `research/policy-compatibility-design-20261111.md`：D2 空 skip 清单、D6 不提供全批幂等和 D7 模式/回退。
- `research/transaction-design-20261111.md`、`design.md`：借用 writer 原事务、真实主体、原授权恢复分流和 Prepare/Finish。
- `.trellis/spec/server/database-test-isolation.md`：独立 DB/外部状态目录/失败留存。
- `CLAUDE.md`：API zod/parseWithFallback、package boundaries、真实行为 canonical test 位置。
- `packages/core/api/client.ts:642`/`:2263`/`:3124`、`packages/core/issues/mutations.ts:431`、views 上述三处：现有消费者。

## Caveats / Not Found

- 未实现产品、执行测试或访问数据库/容器；无 Git、模型或外部消息操作。
- 无 mobile/CLI batch 调用点；它们只需继续对单项完成拒绝正确返回非成功，不新增批量 UX。
- v1 没有全批持久账本，旧无键请求不会自动获得恢复/幂等。丢失响应时只能按原逐项授权读取，不能推断未提交项并自动续跑。
- 研究读取时一条不匹配的 `server/internal/delivery*` glob 返回无匹配；另一条严格 JSON 读取 turbo.json 因其合法注释失败，已改直接读原文。均为研究命令问题，无产品运行或历史证据改写。
