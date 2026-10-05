# I1 技术设计

状态：拟实现 v1；本次文档不是产品上线证明。

## 1. 证据与选择

现有入口：`server/internal/handler/issue.go:3240` 独立事务更新，`:3915` 删除，`:4073` 批量更新；`triage_actions.go:105` 接受、`:345` 目录锁；`project.go:662` 项目删除；`server/pkg/db/queries/issue.sql` SQL 写入；`packages/core/issues/cache-coordinator.ts` 与 `realtime/use-realtime-sync.ts` 缓存协调；`apps/desktop/src/renderer/src/routes.tsx` 桌面路由。实施前刷新行号，不能只改单一 HTTP 入口。

原则：服务器约束全部入口；管理动作不影响执行；历史事实可重现；所有结束副作用原子且幂等；共享时区只有一个来源。

方案 A（采用）：Issue 保存唯一当前归属，独立参与表/事件/快照；同步事务结束，outbox 异步通知。优点是普通任务筛选简单、结束明确；代价是必须收齐全部任务状态写入和协调锁。
方案 B：完全事件溯源实时重建。优点是统一回放；代价是改写现有任务系统、增加运维和读模型成本，首期无必要。方案 C：仅标签/保存视图，无可靠基线和结束幂等，无法满足 PRD。关键驱动为既有客户端兼容、历史稳定和 1,000 任务规模。

## 2. 数据与迁移

所有关系无 FK / REFERENCES / CASCADE。所有查询包含 workspace_id。新列默认保留既有行为，已有任务 current_iteration_id=NULL，rollover_count=0，不虚构历史。

| 对象 | 拟定字段及不变量 |
| --- | --- |
| workspace | 复用 P1 的 planning_timezone text NULL；NULL 有效 UTC；不放可整体覆盖的 settings JSON |
| workspace_iteration_settings | workspace_id, enabled false, revision bigint=1；每空间一行，I1 mode 恒 manual |
| iteration | id, workspace_id, name, description, coordinator_user_id NULL, timezone, start_date, end_date, status planned/active/completed/cancelled, revision, scope_revision, created_by, started_by NULL, started_at NULL, logical_ended_at NULL, processed_at NULL, end_reason NULL；日期纯 DATE，mode=manual |
| issue | current_iteration_id UUID NULL, iteration_rollover_count int NOT NULL DEFAULT 0；通用更新缺字段保留原值；真实归属/计数变化递增现有 revision |
| iteration_participation | workspace_id,iteration_id,issue_id,first_joined_at,current_joined_at NULL,has_started_current_participation boolean,last_left_at NULL,in_original boolean,original_facts jsonb NULL；一次身份一个记录，移出不删除；原始事实只在 start 写一次 |
| iteration_event | id,workspace_id,iteration_id,sequence,operation_id,issue_id NULL,kind,actor,occurred_at,sampled_at,before_facts,after_facts,reason；包括计划活动、开始、join/leave/reenter/delete/cancel/reopen/status/rollover/end/date_edit；计划活动不算期中范围变化 |
| iteration_snapshot | iteration_id,workspace_id,schema_version=1,operation_id,body jsonb,created_at；每迭代最多一个，completed/cancelled 活动期冻结，禁止 UPDATE |
| iteration_operation | id（server生成稳定operation_id）,workspace_id,actor_user_id,request_id,operation,payload_hash,result jsonb,created_at；持久结果与业务同事务；用于 create/edit/start/move/end/cancel/disable/delete，不依赖短 TTL |
| iteration_notification | id,workspace_id,iteration_id NULL,operation_id,recipient_user_id,kind,local_date NULL,status,attempts,next_attempt_at,last_error_code；outbox 与业务同事务，id 作为 inbox 稳定 ID |

索引逐个独立单语句迁移 `CREATE [UNIQUE] INDEX CONCURRENTLY`；新表不写内联 PK/UNIQUE 导致普通索引。ID 唯一索引完成后如需 PK 用后续 ADD CONSTRAINT ... USING INDEX。清单：settings(workspace_id) 唯一；iteration(id) 唯一及 (workspace_id) WHERE status='active' 唯一、(workspace_id,status,start_date,id)；issue(workspace_id,current_iteration_id) partial 非 NULL；participation(workspace_id,iteration_id,issue_id) 唯一及 (workspace_id,issue_id,iteration_id)；event(id) 唯一、(iteration_id,sequence) 唯一；snapshot(iteration_id) 唯一；operation(id) 唯一、(workspace_id,actor_user_id,request_id) 唯一；notification(id) 唯一、(workspace_id,operation_id,recipient_user_id,kind) 唯一及 pending(next_attempt_at)。每日提醒另唯一 (iteration_id,recipient_user_id,local_date,kind)，需相应 iteration_id 列。所有唯一冲突转换为业务冲突或重放，不吞错。

迁移编号在实施时分配，不能与 P1 并行抢号。先落地一方拥有 planning_timezone 迁移、handler、测试，另一方复用，不各自 IF NOT EXISTS 掩盖不同合同。文件部分执行/无效索引由 catalog + pg_index.indisvalid 检查并前向修复；schema_migrations 不能单独证明结构。

## 3. 写入收口与一致性

新增 `server/internal/iteration/` 保存纯统计/时间/规范化；事务业务放 `server/internal/service/iteration.go`（新），handler 接收鉴权/DTO。handler/service/daemon/父子自动状态更新必须在原业务事务内调用同一 `RecordIssueChange(tx,before,after,actor,operation)`，不得从 WS 事后补日志。按真实状态类别（含自定义状态目录）记录，未知类别阻止开始/结束并给出可修复错误，不能当 todo 伪造快照。

为避免遗漏，foundation 提交 writer inventory：从 SQL 的 UPDATE issue、DeleteIssue、状态目录归档/分类变更、服务端任务完成回写、批量、移动工作空间及创建各调用点反向追踪到入口，每项标出事务/锁/测试。凡影响快照标题、项目、负责人、状态、归属或删除的入口都要同事务递增 scope_revision 并写事实；comments/附件无统计变更不制造范围变化。跨空间移动含任意参与历史的任务在 I1 明确 409 iteration_history_move_unsupported，避免旧历史泄露和身份错配；没有参与历史仍按现有移动流程。

锁顺序必须与 T1/P1 兼容：workspace shared → actor member fence/active member → T1 settings（仅 T1 流程）→ status catalog shared → workspace iteration advisory transaction fence → iteration settings → iterations ID 排序 → attachments/issue ID 排序 → participation/event。跨资源 project/member/agent 引用沿用现有 FOR SHARE NOWAIT，冲突回滚而不等待形成反向锁环。对无迭代关联的普通任务写也需先取得 iteration fence 再锁 issue，以防同时 join 导致漏事件；该轻量 workspace 级互斥会串行相关写，必须测吞吐，后续优化必须保留同样的线性化证明。不在新服务中获取已经被调用者以反序持有的锁，先改调用者入口。

预览 REPEATABLE READ 同一事务授权并读取；第一条 SQL 设置隔离，成员使用 locking read，不能以快照前授权代替当前权限。提交 READ COMMITTED，取得以上 fence 后重新加载全量受影响实体并重算 preview hash。所有相关 writer 遵守 fence，保证集合、引用与状态不会在提交中漂移；目录锁阻止类别变更。成员撤权复用 LockSubscriberWrites/LockActiveMember；共享 workspace 锁阻止空间删除，引用 NOWAIT 防并发删除。只有全部校验通过才写基线/快照/去向/操作结果/outbox，一次 commit。全部相关 fence/实体锁取得后、首次业务写前，以 clock_timestamp()（测试注入时钟）采样一次 business_at，供本次 occurred_at/started_at/logical_ended_at 使用；禁止用 PG now()/transaction_timestamp() 的事务开始时间。sequence 是权威业务顺序；时钟回拨时事件时刻不小于该迭代前一事件，记录原始 sampled_at 供诊断。processed_at 是最终写结果前的锁内时间，不冒称数据库不可知的真实 commit timestamp。

SQLSTATE 40001/40P01/55P03 整事务最多三次（25ms、75ms 加 0—25ms jitter，受 context 限制）；业务 409 不自动换预览。unique request 竞争回滚后在新事务重新授权并查询结果，hash 相同才重放。commit 前不广播、不发通知、不调执行。

## 4. 生命周期与原子流程

- Start：预览日期、全部计划任务（包括先前加入后变终态者）及保留/移出选择；过期计划拒绝，提前开始 today 以保存时区今天改开始日并保持日历时长。事务确认单一 active；冻结 O/原始事实；已终态保留有标识；空 O 合法。
- Move：全部选定任务校验 expected_revision、原归属、权限、正式准入与目标状态；已 cancelled 不能新加，done 仅显式 allow_completed=true 加 active；planned 不接新终态。整体成功或回滚；同目标为 no-op 不重复事件/计数。移出理由必填，跨期同时写旧 leave 与新 join/reenter，不改变累计结转次数。
- End/active cancel：预览包含全部 S、未完成 R、执行指示和逐项目标。提交资格/版本完全一致才冻结 S/O/events/指标/图表/任务当时摘要及去向，再释放全部源指针，R 选择 planned 目标的 +1 结转；done/cancelled/移出均 NULL。不终止或重启执行。取消 active 同一协议但快照类型 cancelled；不进入正常完成样本。
- End-and-start：next_iteration_id 显式给出，全部结转目标可以多个 planned，但只指定一个作为下一 active；旧关闭、全部去向和 next 的既有计划任务+流入任务形成新 O 同一事务。新计划终态保留/移出、提前开始日期等和 Start 相同预览规则。任何冲突全部回滚。
- planned cancel 释放成员，保留参与/活动/原因；无原始承诺，不伪造正常完成快照。Delete 仅从未开始且无参与历史/当前成员的空计划；名称活动不阻止。幂等操作结果独立保存可重放已删除计划结果，但仍需空间授权。
- Disable：管理员预览该空间所有 active/planned 及其完整集合；同事务 active 按功能禁用 ended+全部移出、planned 取消+释放、最后 enabled=false。不允许分页分批提交；失败保持 enabled 与全体原状。再次启用不复活旧计划。
- 日期重叠提示，不创建第二 active；active start_date/started_at 锁定，end_date 修改要原因；历史 name/description 更正只留活动记录，绝不修改 snapshot 原事实。

## 5. 统计、燃起图与快照

定义严格继承 PRD §10：S 当前正式成员；C cancelled；E=S−C；D=S 中 done；O start 固定；OD=O∩D；R=E−D。比率分母 0 返回 null。O 不因删除/取消/移出缩小；完成后移出不再贡献 D/OD。父子独立 ID 计数，不 JOIN 放大。新增 added_unique 是实际开始后至少进入一次且不在 O 的唯一 issue_id 数；第一次期中加入计新增，之后移出再入只增加 reentry_events，不增加新增。原始成员的再入也只计重入。计划期关联但在 start 前移出的非 O 成员，start 后首次进入仍计新增，不使用 lifetime first_joined_at 判断。removed_events/reentry_events 是实际变动次数，重复请求和同目标 no-op 不计；满足 |S|=|O|+added_unique+reentry_events−removed_events。当前取消集合数与累计取消/重开事件分开。

开始建立基线，所有影响状态类别/成员的事务写带 before/after 的事件与单调 sequence。图表在 iteration.timezone 每个本地日末采样 E/D，O 参考固定；日内实时末点保留 calculated_at，延期延伸至实际 end。历史 snapshot 保存已解析图表点和完整事件，无需回连实时 issue 重算。删除任务先写 delete/leave 事实、保留 original 和参与身份后删除实体。快照每个成员保存 issue UUID/identifier/title、project id/name、assignee type/id/display、status key/category、was_completed_at_start、rollover count 与目标；null 引用如实记录，不能因实时项目删除清理快照。

## 6. 时间、通知和跨端

复用 P1 `/api/workspaces/{ws}/planning-timezone`，owner/admin 人类写，null 回 UTC；创建显式确认 effective_timezone，写入迭代后不可修改。仅认可 IANA 和 UTC，拒绝 Local/数值偏移。Date 用 YYYY-MM-DD，不用 JS Date 转 UTC 渲染。Go 时区数据库读取失败拒绝保存；date.AddDate 处理日历时长，午夜 gap 取首个有效时刻、fold 取首次（为边界 helper 写真实 tz 测试），不得直接假定 time.Date 对异常午夜的规范化正确。I1 只提醒逾期，绝无自动结束/自动开始。

开始/结束/取消/日期修改通知协调人与受影响任务负责人，去重同一 user；agent/squad 不猜测邮箱或额外人类接收者，仅复用已授权现有身份映射，无法路由留审计。每日逾期按保存时区日期去重，协调人为空给 started_by，失权停止个人提醒。outbox 发送再次成员 fence/权限校验，stable inbox ID+同事务 delivered 标记，崩溃重试不重复；撤权清理保护内容，重试不能重建。首期无外部邮件/聊天。

Web/Desktop：新 `packages/core/iterations/` query keys `[wsId,iterations,...]`、API schemas、mutations；`packages/views/iterations/` 共享页和预览；两端薄路由。Query 唯一服务器事实来源；草稿/选择如需持久放 core Zustand，成功等服务端再导航。WS 只身份/version，task/status/project/member 变化失效相关 query；重连/恢复焦点重新读取，未收到事件不能依赖本地已结束状态。分页列表和全量预览选择分离。Mobile 仅 types/pure helpers；本期验证旧请求缺字段不清归属，无法确认端阻止结束/结转并提示支持版本。

## 7. 发布与回退

先 additive schema 和 writer/快照契约，全部 writer 覆盖和兼容测试通过前能力=false；再 API/客户端，默认 disabled。P1 无需启用，但共享时区实现必须单一落地；项目删除后历史保护作为两模块联合验收。没有 P1 实现时对既有 DeleteProject 先做 I1 保护测试，不声称未来 P1 集成已经验证。

已产生任何 iteration/参与/事件/操作/outbox 或非 NULL 指针/非零计数/规划时区，禁止破坏性 down；前向修复或功能只读。空库 down 在隔离维护环境停止全部 writer，表锁 NOWAIT 后二次 guard，独立并发索引撤销需整个维护窗口无 writer。P1 已使用 planning_timezone 时 I1 回退不可删共享列。旧服务端不具备事件捕获不得回滚到使用 I1 数据的数据库；UI 可回退但兼容服务端保留。

## 8. 风险和 ADR

Decision：指针+参与/事件/冻结快照，同步原子提交+持久幂等/outbox。Drivers：兼容、历史可信、闭环可测。Alternatives：全事件溯源改造成本高；标签无法承诺；异步分块结束破坏整体成功，拒绝。Consequences：空间写锁吞吐和 writer 收口是最大成本；先基线，不超时后假成功。Follow-ups：I2 边界历史恢复不能直接用 I1 当前事实冒充过去；容量另行评审。

Pre-mortem：遗漏 daemon writer 导致图表丢事件→writer inventory+真实并发测试；结束响应丢失→持久 request/hash/result+稳定 inbox ID；P1 删除/时区独立落地漂移→共享所有者和联合 fixture。实施中发现旧端无法在保留原行为同时满足锁序，先更新本设计再扩大改动，不以 UI 隐藏绕过。

## 9. 空间删除与读取授权

沿用 `server/internal/handler/workspace.go:1070` 删除事务，在 DeleteWorkspaceIssueRoots 前清本空间 notification/inbox保护载荷、operation、snapshot、event、participation、iteration/settings；同一 workspace 排他锁与事务，不留下 orphan，不靠FK级联。普通任务/项目/成员删除不删历史摘要，只有正式工作空间删除按其既有保留政策清理。每次读取快照、事件、操作结果与通知先验证当前空间权限，若未来任务权限比空间更细，必须逐项重新授权并隐藏无权摘要及派生计数，不能把历史快照当免鉴权存储。

## 10. 补充统计和事件采集

Statistics 同时给 initial_effective、net_effective_change=effective−initial_effective、net_effective_change_ratio（initial_effective=0时null）、started、cancel_events/reopen_events。started 仅计 E 中本次参与曾达到 in_progress/in_review/done 或产生执行启动记录者，blocked 单独不算。每次当前 join 初始化 has_started_current_participation（初态满足类别即true），后续类别转换或真实执行启动同事务置true，leave清当前参与标志，reenter重新计算新参与；累计参与历史保留。execution-start writer 即使不改issue状态也必须在inventory中，取得同一fence并记活动事实，不能凭agent是否在线推断。快照冻结这些字段。图表不填实际开始前/未来点、缺失标unknown，日界[next00:00]属于次日。
