# I1 API / 数据合同 v1

`GET W/iteration-operations/{request_id}`、capability/settings 读取及默认关闭发布开关后的 settings enable 已实现。生命周期 LG 与历史 HG 的 API 已完成本轮集成验收；结束/交接/禁用已通过 CG 集成验收，见 cg-verification.md。planning-timezone 已由 P1 实现，直接复用。其余路径复用已有鉴权路由组。JSON snake_case，UUID 标准文本、time UTC RFC3339Nano、date YYYY-MM-DD；所有 revision 正安全整数，计数非负，nullable 明示 null。

## 端点

`W=/api/workspaces/{ws_id}`，`I=W/iterations/{iteration_id}`。授权先校验空间成员，写逐项校验任务权限；配置仅 owner/admin 人类，机器身份仅现有明确授权不额外升级。

| 请求 | 输入/输出 |
| --- | --- |
| GET W/iteration-capabilities | `{workspace_id,schema_version:1,supported:boolean,enabled:boolean,manual:true,atomic_handoff:boolean}` |
| GET W/iteration-settings | `{workspace_id,enabled,revision,planning_timezone,effective_timezone,timezone_configured}` |
| POST W/iteration-settings/enable | WriteEnvelope + `{expected_revision,confirmed_timezone}`；不建周期；初次尚无 settings 以 revision=1 默认行锁后创建 |
| GET/PUT W/planning-timezone | 完全复用 P1 合同，不增加第二条设置字段写路径 |
| GET W/iterations | status/search/from/to/issue_id/limit/cursor → `{workspace_id,items:Iteration[],next_cursor}`，日期按交叠过滤 |
| POST W/iterations | WriteEnvelope + `{name,description,coordinator_user_id,start_date,end_date,confirmed_timezone}` → WriteResult |
| GET I | `{workspace_id,iteration:Iteration,statistics:Statistics,snapshot:Snapshot|null}` |
| PUT I | WriteEnvelope + `{expected_revision,fields:{name?,description?,coordinator_user_id?,start_date?,end_date?},reason}` |
| GET I/issues | scope/project_id/assignee_type/assignee_id/status_category/status/priority/label_id/search/limit/cursor → `{workspace_id,iteration_id,scope_revision,items:HistoricalIssue[],total,next_cursor}`；历史默认 snapshot 成员 |
| GET I/events | after_sequence/limit/cursor → `{workspace_id,iteration_id,items:Event[],next_cursor}` |
| POST W/iteration-previews | Draft → Preview；start/move/end/cancel/disable/delete/handoff 全部同入口 |
| POST W/iteration-operations | `{request_id,draft,preview_hash}` → WriteResult |
| GET W/iteration-operations/{request_id} | 当前 actor 自己的已提交 WriteResult；未见返回 404 operation_not_found（不代表其他并发请求一定失败） |

WriteEnvelope 为 `{request_id:UUID}`，普通 create/edit/enable 也有持久 hash，重放规则同下；初次 create 201，其他成功200，重放200。删除走 operation 而非物理 DELETE，结果保留身份和 deleted=true。

当前 `iterations_i1` 服务端发布开关默认关闭，且未发布到通用前端 flags。`supported` 表示发布开关状态，capability 的 `enabled` 为发布开关与持久 settings.enabled 的合取；settings 读取保留真实持久值。`atomic_handoff` 在 CG 验收后随 supported 返回；发布开关关闭时仍为 false。发布开关必须等完整 I1 交付后才能启用。已经提交的同意图请求在开关关闭后仍可由当前有权主体回放，不重新改变设置。

## DTO

```text
Iteration={id,workspace_id,name,description:null|string,coordinator_user_id:null|UUID,
 status:planned|active|completed|cancelled,mode:manual,start_date,end_date,timezone,
 revision,scope_revision,started_at:null|time,logical_ended_at:null|time,processed_at:null|time}
Statistics={original,current,cancelled,effective,completed,original_completed,remaining,
 added_unique,removed_events,reentry_events,cancel_events,reopen_events,started,initial_effective,
 net_effective_change,net_effective_change_ratio:null|number,effective_ratio:null|number,original_ratio:null|number,
 chart:[{date,effective,completed,original}],calculated_at}
Snapshot={schema_version:1,workspace_id,iteration_id,operation_id,end_type:completed|cancelled,
 reason,logical_ended_at,processed_at,original:[HistoricalIssue],scope:[HistoricalIssue],
 events:[Event],statistics:Statistics,destinations:[{issue_id,target_iteration_id:null|UUID,
 rollover_count_before,rollover_count_after}]}
HistoricalIssue={issue_id,identifier,title,project_id:null|UUID,project_name:null|string,
 assignee_type:null|string,assignee_id:null|UUID,assignee_name:null|string,status_key,status_category,
 was_completed_at_start:boolean,rollover_count,priority?:null|string,labels?:null|HistoricalLabel[]}
HistoricalLabel={id:UUID,name:string}
Event={id,sequence,iteration_id,issue_id:null|UUID,operation_id,kind,actor,occurred_at,sampled_at,
 before_facts,after_facts,reason:null|string}
Draft={operation:start|move|end|cancel|disable|delete|handoff,iteration_id:null|UUID,
 expected_iteration_revision:null|integer,expected_scope_revision:null|integer,
 expected_settings_revision:integer,reason:null|string,
 moves:[{issue_id,expected_issue_revision,expected_source_id:null|UUID,target_id:null|UUID,
 allow_completed:boolean}],start:{target_id,mode:scheduled|today,
 terminal_choices:[{issue_id,retain:boolean}]}|null}
Preview={workspace_id,actor_user_id,draft,preview_hash,previewed_at,
 start_preview:{reference_date,effective_start_date,effective_end_date,timezone}|null,
 iterations:Iteration[],issues:[{issue_id,identifier,revision,source_id,status_category,project,assignee,
 title,running_execution_count,rollover_count}],statistics,recipients:[UUID],
 invalid_items:[{issue_id:null|UUID,code}],total_affected,complete:boolean}
WriteResult={workspace_id,request_id,operation_id,operation,replayed:boolean,iteration_ids:[UUID],
 result:{snapshot_id:null|UUID,deleted:boolean,settings_revision,issue_count},committed_at}
```

Draft moves 语义：move 为用户明确选定全集；end/cancel/handoff 必须恰好覆盖预览中的全部 R，无重复/遗漏；终态无显式目标隐式释放；disable 强制全部移出，不接受客户端自定去向；start.terminal_choices 恰覆盖新基线已有终态项，不给选择就不可提交。start scheduled 仅 start_date<=当地今天<=end_date；未来计划必须 today，过期必须先改日期。end/handoff 的 target 只能 planned；start.target_id 在 handoff 是新 active，普通 end 必须 null。完成任务加入 active 需 allow_completed=true。未知/不适用 fields 拒绝，不静默忽略。

## 历史字段、过滤与归属事件

`HistoricalIssue.priority`、`labels` 是 schema_version=1 的增量字段。新捕获记录总是保存实际优先级（含 `none`）和标签数组；`labels:[]` 表示当时确定没有标签。旧快照缺字段或为 null 均表示未知，不等价于 `priority:"none"` 或 `labels:[]`，不得从实时任务、标签目录或之后的参与记录补齐。当前 Go 响应会把缺失旧字段解码后输出为 null；客户端仍须兼容旧服务器省略字段。开始时捕获的 original 和结束时捕获的 scope 各自冻结；后续优先级变化、标签改名/删除和任务删除都不改变保存值。标签只保存 `{id,name}`，不承诺冻结颜色或其他目录字段。

`GET I/issues` 的 `scope` 默认为 `current`，可选 `original`；已关闭时 current 使用 snapshot.scope，original 使用 snapshot.original。`priority` 可为 urgent/high/medium/low/none，省略或空串表示不筛选，其他值返回400 invalid_request。`label_id` 必须是非零 UUID，显式空串、null、畸形或重复参数返回400 invalid_request；只按捕获的标签 ID 匹配，不要求实时目录还存在。未知旧字段不匹配具体 priority/label_id。所有事项页过滤条件仅改变 items/total，不重算顶部全迭代 statistics；未知或重复事项过滤参数拒绝。

`GET W/iterations?issue_id=UUID` 在当前授权工作空间内按历史参与表 EXISTS 过滤，继续返回相同 Iteration 列表及 keyset 分页。它独立于实时 issue 与 original/current 集合，因此期中加入后移出、开始前移出和已删除任务的参与仍可查到。省略 issue_id 表示不加该条件；显式空串、null、非 UUID、零 UUID 或重复 issue_id 返回400 invalid_request。合法但未参与或属于其他工作空间的 ID 返回本空间空集合，不返回其他空间的信息，也不要求先读取实时任务。可组合 status/search/from/to 等既有过滤；要只列已结束参与需明确提供相应 status。

新加入、移出、重入、结转与删除产生的归属事件，在非 null 的 IssueFacts 对象上附加两个字段：

```text
MembershipIssueFacts=IssueFacts & {
 source_iteration_id:UUID|null,
 target_iteration_id:UUID|null
}
```

来源侧 leave/delete 的 before_facts 同时保存这两个键，after_facts 仍是字面 null；目标侧 join/reenter 的 after_facts 保存相同来源/目标，before_facts 仍是字面 null。未开始迭代对应的 planned_activity 使用同一规则。新事件中显式 null 表示已知无归属，旧事件缺少键表示未知；不可把整个 facts 为 null 当作缺失侧的迭代身份，也不可把旧缺字段推断为无归属。这些键不出现在事件顶层，不改变 IssueFacts 统计字段或统计 reducer。actor、occurred_at、sampled_at、reason 仍为既有事件顶层字段；来源/目标 ID 冻结，展示名称可单独读取实时目录，但不能反写历史 ID。关闭操作生成的 leave 保留在追加审计流中；既有关闭前 snapshot.events 截点与 snapshot.destinations 不变。

## 预览与重放

Preview 来自同一事务全量集合，客户端页大小不能决定 total_affected。列表默认50最大100，以 `(start_date,id)` 或 issue_id/sequence 稳定 keyset；周期列表 cursor 绑定工作空间、全部过滤条件及集合版本（迭代 revision/scope_revision 指纹）；事项/事件 cursor 绑定工作空间、迭代 ID、过滤条件、revision 和 scope_revision。事项 cursor 另外绑定同一一致性读取所得完整 original/scope 展示投影的 SHA256 摘要，标签改名或优先级变化即使未递增 scope_revision 也会使旧页返回409 cursor_stale，须从第一页重读；摘要是内部游标内容，不是新增响应字段。已关闭投影只来自快照，不因实时标签或任务变化漂移。预览首期全量支持至少1,000条；测量后记录部署上限，超过上限返回413 iteration_operation_too_large 并保持原状，绝不截断。禁用也遵守整体上限，不能用分批部分禁用绕开。

规范化：字符串 CRLF→LF/trim、UUID 小写、对象键排序、显式 null、moves/terminal_choices 按 issue_id 排序并拒绝重复、日历日期严格合法、数组语义固定。服务端 canonical SHA256；hash 不是授权凭据。preview_hash 包含 contract_version/ws/actor/规范 draft/settings revision/迭代 revisions及日期时区/所有受影响成员身份和 issue revision/状态目录解析事实/引用显示与可用性/捕获的历史展示字段（包括 priority 和 labels 的 ID/name）/recipient IDs/全部目标和 handoff 原计划成员；包含 start_preview 的 reference_date/effective_start_date/effective_end_date/timezone（start/handoff），不含 previewed_at。跨本地午夜重新计算派生日期，任何变化409 iteration_preview_stale，不可无确认换成新日期。运行执行只作说明，若执行状态变化导致任务 revision/状态变化则冲突；纯执行进度变化不影响管理结果。结束逻辑时刻采用 design §3 的锁后 business_at，不冒充预览时刻或 transaction_timestamp。

写在锁内重新生成全部比较事实并匹配；不同则409 iteration_preview_stale，仅返回授权 changed_fields 和可刷新信息，保留用户输入。业务意图改变用新 request_id；网络未知沿用旧 ID 与原 payload。request hash 包含 actor/ws/operation/规范输入/preview_hash，先鉴权后查操作；同 ID 同 hash 回放原结果，即使现态改变也不再次写；不同 hash409 idempotency_conflict。GET operation_not_found 或未分类404时继续以原 ID 幂等提交，不新造 ID；workspace_access_denied404必须清除保护缓存并停止重发。明确 iteration_not_found 的写入404为已拒绝，保留输入供修改而非无限恢复。历史对象 deleted 后仍可回放同空间已授权本人操作。

## 兼容、T1 与错误

Issue 新响应字段 `current_iteration_id?:UUID|null, iteration_rollover_count?:integer`；旧对象省略表示不支持/未知，不能显示计数0伪装已知。通用更新不带字段保留；带迭代字段必须走同一服务/CAS，不接受不带 expected_revision 的迭代写（428 iteration_confirmation_required）；通用批量含迭代走完整 preview 操作协议，否则428。新任务创建的非空 current_iteration_id 需同时提供 expected_iteration_revision（目标版本），在原创建事务校验并加入；保留原有重复任务 409 行为，不另建 iteration_operation。单任务 PUT 的迭代分支仅接受 current_iteration_id、expected_revision、iteration_reason、allow_completed；与其他字段混合明确返回 400，移出/换期须填 iteration_reason。iteration_rollover_count 始终由服务器维护。要求分拣入口拒绝正式归属但可展示候选意向，CSV 报告未采用。

普通创建的 `allow_completed` 为可选布尔字段，仅非空 `current_iteration_id` 时适用；创建 done 任务并加入 active 必须显式为 true，默认 false。planned 不接纳新终态任务，cancelled 任务不能以此绕过限制。该字段不改变普通创建原有的负责人执行入队行为，也不额外记录 execution_started。

T1 `TriageActionInput.fields.current_iteration_id` 仅 accept/accept_and_execute 支持，accept 的 request_id 仍为唯一操作身份，先按目标正式 status 校验、同事务接受+安排；执行显式意图沿用 T1 outbox，不生成第二个执行。T1 能力响应增加可选 iteration_assignment，服务端全部接好后才 true；不支持时 UI 不提交新字段，手工调用未知字段明确拒绝。

能力专用路径在已确认可访问工作空间用合法 UUID 探测；旧服务器404才为不支持，401/403 及明确携带 workspace_access_denied 的404为认证/撤权（不得降级为旧服务），400为请求错误，网络/畸形为失败，不统一当 disabled。新写 DTO 用 parseWithFallback + zod 严格身份/关键字段验证；无法确定写成功必须 protocol error，不能 fallback 成空成功。所有枚举有 default，unknown 状态只读提示。

错误统一 `{error,code,field_errors?,current?,retryable?}`：400 invalid_request；401 unauthenticated；403 forbidden；404 iteration_not_found/operation_not_found；409 iteration_preview_stale/iteration_active_conflict/iteration_revision_conflict/iteration_history_move_unsupported/idempotency_conflict；422 iteration_validation_failed/iteration_disabled；428 iteration_confirmation_required；413 iteration_operation_too_large；503 iteration_retry_exhausted（写冲突预算耗尽）或 iteration_unavailable（结果读取/存储不可用），Retry-After:1。T1 原有409 triage_review_required保持原合同，新增迭代入口准入拒绝也用409同码，不能以新422更改旧入口。current 绝不带无权任务标题/计数。

operation_id 由服务端为首次成功写生成，保存在 iteration_operation.id；同请求重放返回原值。event/snapshot/notification 使用该稳定ID，不能仅以不同actor可重复的request_id关联。T1接受中的迭代事件沿用其既有稳定 action identity，并在事件actor/type中可辨来源，不伪造另一份执行或迭代操作。

## LG/HG 当前接口边界

- 周期维护操作目前要求当前人类工作空间成员；设置启用仍限 owner/admin，不从任务指派推导新的机器维护权限。普通任务的确认归属写入沿用现有 Contributor/任务授权。
- 所有新生命周期请求在类型解码前拒绝重复对象字段（含 JSON 解码会视为同一字段的大小写别名），保留原始数字验证，拒绝尾随 JSON 与过深嵌套。
- 周期列表使用 `(start_date,id)` keyset，游标绑定工作空间、过滤条件和集合版本；集合/范围变化返回 cursor_stale。单期事项/事件页绑定对应 revision/scope_revision；事项页另绑定完整展示投影摘要。
- 已关闭详情和事项集合只读冻结 snapshot；事件接口还可读取追加的元数据更正审计，但不重算或覆写 snapshot.events/statistics。
- 原始承诺保存完整 HistoricalIssue 与本次参与的 started 事实。空旧前缀使用现有冻结回退规则；预览 identifier 纳入确认哈希，前缀或旧空间名称变化须重新预览。
- 标签关联增删及标签改名/删除在同一事务复用 W01 锁顺序：workspace KEY SHARE → subscriber fence → 当前 member SHARE → status catalog shared → I1 workspace fence；关联编辑随后锁已解析的 iteration/issue，重新读取同空间标签与 resource_type。机器调用者的 agent/task 引用在等待后锁定并重验身份，授权持有至提交。已撤权/归档/换绑返回403，消失或移出空间的 issue/label 返回404，不吞成500；沿用标签原有权限策略，不引入额外维护权限或任务删除的 Contributor 门槛。创建和 T1 接受的标签写入沿用其既有 I1 事务 fence，空间删除由 workspace 排他锁隔离。
- 历史标签按受影响任务集合批量捕获；写入捕获同时 SHARE NOWAIT 锁住已有标签和关联，I1 fence 排除新关联幻读。标签本身不制造范围统计事件；`label:updated` / `label:deleted` / `issue_labels:changed` 现有事件应触发客户端实时迭代展示缓存失效。普通任务热写路径未增加标签查询。
- snapshot.destinations 必须完整覆盖冻结 scope，终态/移出显式 null 且计数不变，合法结转为 +1；不得省略字段、重复任务或结转回本期。真实目标状态/权限与原子结束由 CG 验证。
