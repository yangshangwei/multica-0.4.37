# P1 后端现状与实施边界

日期：2026-10-05。只读代码研究；下文“建议”是待实施设计，不表示代码已实现或测试已通过。路径均相对仓库根目录，行号按本次工作树读取。范围对应 `projects-prd.md` 的 PRJ-001—014，不包含 P2/P3。

## 1. 关键结论

1. 沿用 `project`、描述正文和现有五状态。当前没有描述版本、项目状态历史或项目进展实体；这些是 P1 实际新增项。
2. `done_count` 的现有含义是 **done + cancelled**。必须保留此字段兼容旧桌面端，并新增明确的完成、取消、未结束分项；不能直接改义。
3. T1 已在项目统计 SQL 使用正式任务白名单。健康聚合、下钻和发布时快照必须使用相同准入谓词，不能依赖当前分拣开关。
4. 当前统计读取会吞错误并返回零、状态目录失败会降级成内置终态。该行为可继续保障旧响应，但不能成为 P1“暂无风险”的依据；新概览需要明确 `complete`/错误原因。
5. 工作空间暂无显式规划时区。`user.timezone` 是查看时区、自动化时区属于触发器，均不能冒充工作空间规划基准。
6. 进展发布必须同时校验机器凭据类别与实际 actor；仅凭 `X-User-ID` 或 `RequireHumanActor` 中间件不足以排除 legacy agent 请求。
7. 项目删除已有事务和排他项目锁，但只显式清理聊天、项目视图；历史迁移的 issue/resource/autopilot FK 承担其他清理。P1 必须在应用事务补齐清理，并补上关联写入与删除的锁协议。

## 2. 现有入口、响应和描述

| 现状／证据 | P1 落点 |
| --- | --- |
| `server/cmd/server/router.go:2132`：GET list/search/detail、POST create、PUT update、DELETE、resources、execution-squad(s) 已齐备 | 保留路径与五状态，新增 overview / updates 子资源，不新增“专项”实体 |
| `server/internal/handler/project.go:24`：`ProjectResponse` 有 description、状态、lead、日期、issue_count、done_count、资源及默认小队 | 新增兼容字段同步到 list/get/search/create/update/configure 响应；不要仅补详情接口 |
| `server/internal/handler/project.go:153`：Update 请求没有 revision、完成原因；`:529`—`:658` 先读取旧 project，然后更新，无状态审计 | 在现有更新事务中锁住最新项目，基于 rawFields 合并，记录真实状态转换和操作者；旧请求缺少原因记为未填写 |
| `server/pkg/db/queries/project.sql:46`：nullable 字段由完整 params 覆盖、仅 WHERE id；handler `project.go:564` 拷贝先前读取值 | 加租户 SQL 条件并在锁内读取当前值，避免并发图标编辑把新描述回写旧值；新客户端可用 expected revision，旧客户端不新增必填门槛 |
| `server/internal/handler/project.go:862`：搜索有独立手写 SELECT 和扫描路径 | 新版本/分项字段要同步搜索，不能只重新生成 sqlc |
| `server/internal/service/builtin_skills/multica-projects-and-resources/SKILL.md:27`：description 是注入 agent brief 的长期上下文 | 模板只编辑原 description；验收保存不可变快照，不增加可独立编辑的 goal/scope 镜像字段 |

建议最小存储：`project.description_revision`（只在 description 实际变化时递增）；`project.in_progress_since`（进入进行中时更新，重开重新计时）；独立状态变更审计；进展及不可变修订记录。验收修订保存当时 description 文本与 revision。图标、优先级、资源或小队变化不得让验收过期。已有进行中项目无法从 updated_at 可靠推断最近进入时间，需明确迁移基线（例如迁移时开始计时并标记为迁移基线），不能伪称历史事实。

`project.go:624` 使用纯日期、区分缺失键与 null 清空；当前只校验日期格式，未在该更新路径校验 start <= due。新编辑路径应检查最终组合，同时允许历史异常数据被显示和修正。`server/migrations/166_project_dates.up.sql:4` 定义 DATE 语义；不得改成 timestamp。

## 3. 统计、状态类别与正式任务

- `server/pkg/db/queries/project.sql:69` 的 `GetProjectIssueStats`：按 `workspace_id`、`project_ids`、`admission_status IN ('not_required','accepted')` 分组计数；`done_count` 使用传入 terminal status keys。`CountIssuesByProject` (`:65`) 同样过滤 admission，但没有 workspace 参数，新接口优先使用前者的租户限定。
- `server/internal/handler/issue_status.go:46` 的 `terminalIssueStatusKeys` 使用 `issuestatus.ExpandCategories(... done,cancelled)`，明确证明 done_count 包含取消。
- `server/internal/admission/admission.go:13` 的 `Formal` 是白名单；`:28` 的 `Check` 重新读取持久状态。`.trellis/spec/server/triage.md:10` 明确未知值不放行，普通集合/聚合排除非正式任务，不能从状态、项目关联或 UI 判断准入。
- `server/internal/handler/project.go:80` 的 `loadProjectIssueStats` 在查询错误或无结果时返回 0/0；`:96` 的 `projectTerminalIssueStatusKeys` 在目录失败时降级为 done/cancelled；列表 (`:203`) 也忽略聚合错误。新健康 DTO 必须区分空集合、目录失败和查询失败。
- `server/internal/issuestatus/issuestatus.go:283` 的 `Effective` 对内置值直接返回，对无法读取/未知值原样返回。`:438` 的 `Resolver` 可批量读取，但失败同样隐藏；P1 完整性检测应明确加载目录并收集未知 key，不能把原样返回误认为有效类别。
- `server/pkg/db/queries/issue_status.sql:89` 明确 archived 自定义状态仅阻止未来指派，历史 issue 仍继承原 category。读取统计目录必须 include archived，不能只用 UI 的 active status 列表。

建议一个后端聚合核心（SQL CTE 或查询构造器，避免每条 issue 查目录/负责人）：

```sql
-- Shared admission and tenant boundary for overview, drill-down and publish snapshot.
WHERE i.workspace_id = $workspace
  AND i.project_id = $project
  AND i.admission_status IN ('not_required', 'accepted')
```

在同一快照内解析类别、有效指派与基准日，得到 N/F/C/U、blocked/overdue/unassigned/in_review、四风险并集去重数、unknown_count。未知状态保留在 N 和 U，计数显示不完整，不可推成完成。父子按各自 issue ID 各计一次，JOIN 必须避免 member/squad 产生行倍增。

新概览建议携带 `calculated_at`、`reference_date`、`timezone`、`complete`、`incomplete_reasons`、统计项和下钻描述。只读事务用同一数据库快照（例如 REPEATABLE READ）读取项目、目录、时区、负责人和任务。下钻以同一服务端条件重算，不承诺 HTTP 请求间永久保持原集合；响应标识/指纹变化时客户端提示已刷新。发布进展的快照必须由服务端在发布事务生成，拒绝客户端覆盖自动计数。

兼容建议：旧 `issue_count=N`、`done_count=F+C` 保持；新增如 `completed_issue_count=F`、`cancelled_issue_count=C`、`open_issue_count=U`，字段最终命名由 API 契约统一。N=0 的比例为 null/不适用，而非 100%。

## 4. 负责人、运行环境和项目 lead

`server/internal/handler/issue.go:3727` 的 `validateAssigneePair` 可复用写入校验：类型/ID 成对；member ID 是 **user UUID** (`:3742`)，agent 必须同空间、未归档 (`:3750`)，squad 同空间、未归档且 leader 存在未归档 (`:3775`)。调用权限另经 `canInvokeAgent` 检查。

健康“有效引用”不能直接调用此函数：它是依赖当前调用者权限的写入/执行授权，私人 agent 对某查看者不可调用，不等于任务未分配。健康查询按存储身份和所属空间/归档状态判定；是否可调用、运行时是否在线作为独立信号。现存 agent 运行时离线仍已分配。小队 leader 失效应明确作为不可执行原因，健康有效性应采用与现有小队指派生命周期一致的规则，不能仅看 squad 行存在。

`server/internal/handler/project.go:606`—`:622` 只解析 lead_type/lead_id；历史 schema `server/migrations/034_projects.up.sql:10` 仅允许 member/agent 类型，没有存在性 FK。故健康必须识别历史悬空 lead，不能假设已被创建/编辑校验排除。lead 与默认小队绝不代替 issue.assignee。

`.trellis/spec/server/project-execution-squad.md:5` 指出 configured 是配置完成，不是在线证明；`:25` 要检查实际所有成员的机器绑定；`:37` 明确 lead 与 execution squads 独立。P1 保留这些边界，不因模板、状态、健康或进展修改自动执行。

## 5. 规划时区

`server/pkg/db/queries/workspace.sql:1` 与 `:47` 只有通用 settings，没有独立 planning timezone；`server/internal/handler/workspace.go:104` 响应 settings 为任意 JSON，`:412` 会整体替换。`server/migrations/100_user_timezone.up.sql:28` 定义用户查看时区；`104_drop_runtime_timezone.up.sql:1` 说明运行时物理时区已退出架构；自动化使用触发器自己的 scheduling timezone。

建议新增专用 `workspace.planning_timezone` nullable 字段或独立类型化配置表，明确 owner/admin 人类可写。优先专用列，避免旧客户端整体覆盖 settings 抹掉新规划配置。读空值返回 effective UTC 并附 `configured=false`；写入用 `time.LoadLocation` 校验 IANA 名称，禁止从浏览器/用户查看时区静默初始化。新配置更新只影响后续快照；历史记录持有旧 timezone/reference_date。七日规则按该时区日历日期差，不是简单 168 小时，覆盖 DST。

## 6. 手动进展、修订、身份和通知

需求完整范围：`projects-prd.md:169` 验收有结论、描述版本、范围、证据/说明；`:253` 进展有作者、发布时刻、类型、正文、修订、可选快照；`:259` 起更正保留原文与更正人、时间、原因；`:269` 仅显式提及当前工作空间成员站内通知，agent 提及不得执行。

可复用模式与边界：

| 模式证据 | P1 应用 |
| --- | --- |
| `server/internal/handler/actor_guards.go:96`/`:110` 检查 task_token/cloud_pat；`server/internal/handler/triage.go:175` 再检查 resolveActor 为 member | 新进展发布、更正、验收和规划时区写同时校验两者。不能将机器凭据承载的 owning human UUID 当人工批准；读接口继续既有授权 |
| `server/internal/handler/triage.go:227`—`:245` workspace 共享锁 → revocation advisory fence → active member 锁 | P1 写事务复用共享锁/成员撤销 fence 的函数，不调用带 triage settings 的整套 beginTriageWrite；最终提交前重新确认成员仍有效 |
| `server/internal/handler/triage_actions.go:275`—`:318` request UUID、workspace+actor+request identity、payload hash、不一致 409；`:458`—`:474` 业务审计+通知身份同事务 | 进展创建与更正都需稳定 request_id；更正还需要 expected revision；重放返回原结果且不重复修订/通知 |
| `server/pkg/db/queries/comment.sql:510`/`:521` expected revision 条件更新、实际变化才增加 revision | 可借鉴 CAS；不复用 Comment 模型/评论事件，防止 mentions 和自动化触发任务 |
| `server/internal/handler/triage_notifications.go:22`/`:46` 持久 event/recipient 身份；`:194`、`:223`、`:230` 交付重查成员与撤销 fence；`:238` 用稳定 inbox ID 去重，`:254` 提交后发事件 | 采用同类项目进展 outbox/唯一键，避免直接 Bus publish 充当可靠通知。无需把 project update 塞进 triage 表或改造全局通知框架 |

建议数据边界：一个 `project_update` 稳定身份 + append-only `project_update_revision`，修订内保存正文/类型/人工判断/证据/验收适用描述快照/系统统计快照；请求身份与 payload hash 独立保留。取消编辑无写入。已发布进展不提供硬删除按钮；更正保留旧版及原因。引用读取需逐次验证当前工作空间，失效实体显示占位，不能因为历史快照而暴露后来失去权限的私有执行/资料内容。

通知唯一键建议 `(workspace_id, update_id, recipient_user_id)` 表达一次发布提及通知；更正仅对新提及且从未通知的成员补发，正文/格式重试不重发，最后规则须在 API 契约固定。保存 recipients 前从正文解析显式 member 提及并验证，不信任任意客户端收件人数组。不要订阅 EventCommentCreated；进展事件只负责 Query 刷新，Inbox 事件按现有独立投递方式发出。

## 7. 删除与并发写入

现有事实：

- `server/internal/handler/project.go:680` 要求 owner/admin，`:685` 开事务，`:693` `LockProjectForDelete` 排他锁，`:704` 清聊天，`:713` 删项目 scope 的视图，`:720` 删 project，`:727` 提交；无显式 issue/resource/autopilot 清理。
- `server/pkg/db/queries/project.sql:24` 的 `LockProjectForChatSessionCreate` 是 FOR KEY SHARE，与删除 FOR UPDATE 组成已存在的锁协议；execution-squad 使用 `:12` 的 FOR UPDATE。
- 历史迁移 `034_projects.up.sql:19` 给 issue.project_id 设置 ON DELETE SET NULL；`065_project_resources.up.sql:7` 给资源 ON DELETE CASCADE；`097_autopilot_project_id.up.sql:2` 给 autopilot ON DELETE SET NULL。这里是源码迁移证据，未对部署库 pg_constraint 作现场核验，不将其当生产现状认证。
- `server/internal/service/issue.go:345`—`:375` 在创建事务验证最终 project（包含从父任务继承），但用普通 GetProjectInWorkspace 而非显式共享锁。`server/internal/handler/issue.go:3523`、`:4166` 的单条/批量换项目校验在前置阶段。
- T1 接受的 `server/internal/handler/triage_actions.go:43` 已 `FOR SHARE NOWAIT` 锁项目并校验租户；其 NOWAIT/回滚重试协议不能被普通阻塞锁替换。
- `server/internal/handler/autopilot.go:1270` 的 parseAutopilotProjectID 只做绑定校验；项目删除仅清空 autopilot.project_id 会把未来定时任务转为无项目任务，这是 P1 删除影响中需要明确处理的真实入口。

建议交付边界：删除在 workspace / 当前操作者撤销 fence / 项目排他锁之后，显式解除同空间 issue.project_id（保留 issue 与执行）、清空自动化的项目软引用并**停用受影响自动化与触发器**、清理项目资源/视图/聊天上下文、新进展与待投递通知，再删 project。不能删除共享 squad/agent/仓库或停止已有 task；项目专属物理目录更不能删除。若选择保留项目审计 tombstone，应只保留最小身份供重试/追溯并禁止复活实体。

并发协议覆盖创建 issue（显式/父继承/自动化）、单条与批量换项目、T1 接受、聊天、资源、自动化绑定、发布/更正进展。所有新关联在自己的最终事务取得同一个项目共享锁；删除持排他锁后枚举与清理。已有 issue 先锁 issue 再请求 project 的路径须 NOWAIT + 整笔回滚重试，不能与删除的 project→issue 阻塞等待形成环。多项目批量按 UUID 固定顺序。工作空间删除的显式 owned-row 清理也必须加入新表（`server/pkg/db/queries/workspace.sql:93` 说明应用清理责任），不能只写 DeleteProject。

此处自动化停用为针对 PRD 删除安全边界的设计建议，不是已实现行为；实施任务必须说明用户删除影响提示并有并发测试，不能静默接受解绑后自动化继续执行。

## 8. 可复用测试与必须新增的证明

| 现有测试文件／入口 | 可复用范围 / 缺口 |
| --- | --- |
| `server/internal/handler/triage_boundary_test.go:39` | FormalCollectionsAndExplicitSearch 已校验正式项目统计；扩展五种 admission × category × 项目/租户矩阵 |
| `server/internal/handler/issue_status_test.go:531` | 自定义状态 project stats；扩展 done/cancelled 拆分、archived 类别和未知状态不完整 |
| `server/internal/handler/project_issue_stats_test.go:13` | 旧接口目录错误降级；新增概览同错误必须 incomplete、不能显示零风险 |
| `server/internal/handler/project_dates_test.go:28`/`:127` | 日期生命周期和无效格式；新增 final date order、UTC 缺省、IANA/DST、今日不逾期、七日边界 |
| `server/internal/handler/project_validation_test.go:96`/`:118` | 删除角色权限；新增撤销并发、删除/关联写入双顺序、issue/task 保留、自动化停用、workspace 清理 |
| `server/internal/handler/chat_project_context_test.go:250` | 删除后聊天脱离；作为清理现有行为的回归 |
| `server/internal/handler/project_resource_test.go:507`/`:667` | resource count 及创建原子回滚；确保新 DTO 与事务不丢原字段 |
| `server/internal/handler/project_execution_squad_test.go:692` | 完整响应含统计/资源/小队；新增分项后保持配置响应一致 |
| `server/internal/service/triage_project_resource_fence_test.go:15` | 项目资源与执行快照串行化，锁协议改变时强制保留 |
| `server/internal/handler/triage_test.go`、`actor_guards_test.go` | 人类/PAT/legacy agent/task/cloud actor 及重放/通知模式；新进展独立测试不能借其通过证明 |

新增核心证明：描述并发更改与验收发布不可错绑；图标更新不使验收过期；状态改变从不启动/停止/重分配 issue；返回丢失后的 create/correct replay 只产生一个修订/通知；更正历史不可变；通知投递失败、进程重启、成员退出不重复或越权；计数与下钻在同快照一致、变动有刷新标识；全风险重叠只计一次；离线 agent 仍已分配；删除并发各入口没有孤儿引用且 T1 pending/rejected/duplicate 从不进入正式集合。

## 9. 建议开发切分

1. **数据与写入契约**：描述 revision、状态审计/进行中时间、规划时区、进展修订/request/outbox schema；SQL 租户边界和应用清理；新索引每个单语句 CONCURRENTLY 迁移，无新 FK。
2. **统一聚合与下钻**：正式集合、目录/负责人解析、日期规则、完整性和健康解释；旧 DTO 保留 done_count、各响应补齐新字段。
3. **人工发布与验收**：人类身份、成员撤销 fence、描述锁/CAS、修订/快照/证据、幂等通知；与任务执行总线分离。
4. **删除与兼容收口**：共享项目关联锁覆盖所有入口、自动化解绑停用、工作空间删除、新旧 API/CLI/built-in skill 文档和回归。

验收可随各切片落地；数据模型/锁顺序和 DTO 应先统一，以免聚合、进展、删除分别定义一套口径。此研究没有运行 Go 测试或数据库迁移，也没有修改产品代码。
