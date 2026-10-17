# 详细设计：管理页面、查询与监控

状态：拟定契约，未实现。安全写接口以 security-design.md、终端协议以 terminal-execution-design.md 为准。

## 1. 信息架构与范围

| 首期一级菜单 | 页面路由 | 内容 |
| --- | --- | --- |
| 总览 | `/admin` | 终端、队列、执行、告警、基础用量与下钻 |
| 账号与组织 | `/admin/users`、`/admin/users/{id}`、`/admin/workspaces` | 账号状态、成员归属、注册规则只读；内部组织固定显示 |
| 终端管理 | `/admin/installations`、`/admin/installations/{id}` | 受管理实例、分组、状态轴、版本、runtime、操作记录 |
| 任务与执行 | `/admin/issues`、`/admin/tasks`、`/admin/tasks/{id}` | 业务任务与执行分开；队列是 task 状态过滤，自动化是 source 过滤 |
| 监控与告警 | `/admin/alerts`、`/admin/alerts/{id}`、`/admin/health` | 固定首批规则、告警生命周期、服务与数据新鲜度 |
| 系统管理 | `/admin/administrators`、`/admin/audit`、`/admin/settings` | 角色、审计、只读部署设置和数据保留说明 |

P2 增加 `/admin/capabilities` 和 `/admin/analytics`；版本分布首期放终端列表，升级编排属 P2。P3 才开放组织列表、组织管理员和组织选择器。无功能的未来菜单不显示为可点击空页。

注册规则在账号页集中读取；不另建注册审批、邀请码或动态覆盖环境变量的设置表。终端组首期使用简单标签，部门目录与复杂组织树不实施。

## 2. 页面交互

列表统一具有搜索、时间范围、组合筛选、服务端分页、更新时间和详情入口。筛选落 URL 方便复制，不把服务端对象放 Zustand；长字段折行或截断并有可访问完整名称。

终端详情 tab：概况 / 运行环境 / 执行 / 配置与版本 / 操作。执行详情：状态时间线 / 尝试关系 / 允许查看的日志与结果 / 用量 / 操作；内容不可见时说明权限，不以“加载失败”替代。

有权限的控制按钮由后端返回 `allowed_actions` 作为显示提示，每次写仍在服务端重验。取消显示 task ID、执行引擎、当前状态、目标实例、影响范围；停止接单明确在途执行继续。需要 reason，不让点击一颗开关直接发起敏感写入。

管理写操作等待服务端受理后显示 operation，不做乐观成功。网络超时保持原 idempotency key 并查询；冲突刷新目标状态，禁止无提示覆盖。账号授权/恢复需管理员当前密码复核；密码字段不进入通用 action payload、日志或持久操作记录。

## 3. 登录、路由、前端数据层

- `/admin` 未登录时跳 `/login?next=/admin`，复用经过 sanitizeNextUrl 的同源路径；禁止任意重定向。
- 首期仅密码模式；setup/change 会话先完成原流程。无工作空间也能进入后台；不得挂现有 DashboardGuard。
- Web 路由壳置于 apps/web/app/admin，Next 导航适配置于 apps/web/platform；页面放 packages/views/admin，逻辑放 packages/core/admin。
- API 使用现有 client、parseWithFallback 与 zod；关键授权解析失败应拒绝展示管理内容。字段未知与枚举未知分别回落 null/unknown，不能回落“允许”。
- Query key 统一 `['admin', apiScope, userId, organizationScope, resource, filters]`；apiScope 表示当前服务端/连接代次。所有时间、排序、游标和筛选进入 key。
- 入后台清除当前 workspace 镜像，管理请求不依赖全局 X-Workspace-ID。即使意外带入该 header，服务端管理查询仍只使用其独立授权范围。
- 退出/换账号/换服务器清除后台缓存；403 角色撤销只清后台并显示拒绝页，保留仍然有效的普通业务会话。401 沿用 authEpoch 机制，不让旧请求清除新登录。
- 不新增跨组织 WS；前台详情 5 秒轮询、列表/总览 15 秒，后台 tab 暂停。即使轮询间隔内前端按钮仍显示，服务端撤权立即拒绝。

## 4. 查询接口目录（拟新增）

| 接口 | 参数与主要结果 |
| --- | --- |
| `GET /api/admin/me` | 当前平台角色、允许的 actions、组织范围、支持模式，不返回私有业务对象 |
| `GET /api/admin/overview` | time_from/to、timezone；各 KPI、缺失计数、as_of、规则版本 |
| `GET /api/admin/users`、`/users/{id}` | 账号 ID/用户名/姓名、状态、工作空间数量/受控成员摘要；不返回密码字段 |
| `GET /api/admin/workspaces` | 组织归属、workspace ID/名称、成员和执行汇总，不返回私有配置 |
| `GET /api/admin/installations`、`/installations/{id}` | 三状态轴、能力、运行时摘要、版本及可用操作；字段见终端设计 |
| `GET /api/admin/issues` | 业务任务 ID、workspace、状态、创建时间、执行数；标题按原业务可见性投影 |
| `GET /api/admin/tasks`、`/tasks/{id}` | 来源、关联任务/会话 ID、runtime/安装、执行状态、尝试关系、时间和结构化错误 |
| `GET /api/admin/operations`、`/operations/{id}` | actor、目标、kind、applied/confirmation、回执时间、结果码 |
| `GET /api/admin/alerts`、`/alerts/{id}` | 规则、严重程度、目标、开始/最近发生、计数、处理与恢复状态 |
| `GET /api/admin/health` | API/DB/任务协调/上报新鲜度摘要；不返回抓取 token、连接串或完整配置 |
| `GET /api/admin/audit` | action、actor、目标、阶段、脱敏变化、结果、request_id |
| `GET /api/admin/settings` | 当前支持模式、注册规则、只读配置来源、刷新和保留说明 |

写接口均在 `/api/admin` 前缀下：账号 `/users/{id}/disable|restore|recover-password|role`；执行 `/tasks/{id}/cancel`；终端 `/installations/{id}/admission`；告警 `/alerts/{id}/acknowledge|assign|close`。严格权限、版本、目标和审计规则见对应设计。

写响应丢失时，`GET /api/admin/operations?idempotency_key=...` 只查询当前 actor 的原操作，返回 0 或 1 条；查不到时可使用原 key 重试，不自动换新 key。所有写的幂等键统一取 `Idempotency-Key` 请求头；密码复核与临时密码不进入请求摘要，恢复请求同 key 重发只返回已有结果，不再次更改密码。

所有列表使用 `limit` 默认 50、最大 100，稳定复合排序，例如 `(created_at DESC,id DESC)`；cursor 绑定筛选指纹及 as_of，避免切筛选沿用旧页。执行动态状态仍可能变化，响应声明查询快照时间，不承诺长事务数据库快照。

响应通用形状：`{items,next_cursor,as_of,scope,data_quality}`，详情增加 `allowed_actions,content_access,version`。不强制每次查询计算精确总行数；总览计数有独立有界聚合。

错误形状：`{error,code,request_id}`；400 参数或语义不合法、415 非 JSON、401 会话无效、403 权限/模式拒绝、404 不存在或不可见、409 版本/幂等冲突、410 接入 challenge 过期、429 限流、503 暂不可用。不把 DB 故障伪装成无权限。

事件窗口上限首期 31 天，审计最多 90 天单次查询，超范围请求返回明确参数错误；更大周期按 P2 报表处理。合法 UUID 仍必须通过服务端加载目标，不能从用户参数直接写入关联。

## 5. DTO 与内容边界

TaskSummary 提供 task_id、source、workspace_id、issue_id?、runtime_id?、execution_installation_id?、submitted_installation_id?、status、parent_task_id?、attempt、retry_of_task_id?、rerun_of_task_id?、各时间、failure_code?。不包含 prompt、正文、完整 error、命令参数、文件路径、原始 env。

source 的分组显示 issue/chat/autopilot/API 等来源，但保留已有真实 source 字段映射，未知来源显示 unknown。Web/API 未带有效安装证明时 submitted_installation_id=null，不按 User-Agent 或 IP 猜机器。

ExecutionDetail 可链接原资源鉴权 API 获取内容；先检查 content_access，无权时标题也显示“受限任务”。执行元数据可定位问题，原始日志只有原业务权限才读。原操作回执里的分支/工作目录仍受原权限保护，后台结果只用结构化码。

## 6. 指标口径

| 指标 | 首期定义 |
| --- | --- |
| 受管理终端 | installation_id 去重，退役单列；未关联 runtime 单列，不混加 |
| 客户端活跃 | 最新被验证的客户端上报在活跃窗口内；日活使用 user/install/day，不能代替实时值 |
| daemon 可达 / 可执行 | 分别使用终端设计的连接与准入条件；可执行不等于有空闲并发 |
| 执行成功率 | 窗口按 finished_at 收集：completed/(completed+failed)；取消和未结束单列；分母 0 显示无样本 |
| 业务完成 | 按 issue 终态/验收规则，不能从 task completed 推断；首期可只展示状态计数 |
| 排队与运行时长 | 按可用的对应生命周期时间计算；缺时间不补 0；P50/P95 与样本数一起返回 |
| 基础用量 | task_usage 汇总且包含失败执行；缺失用量次数与 provider 未计价项单列 |
| 估算成本 | 价格来源和版本固定在服务端摘要；现有前端自定义价不直接当平台账本；无统一价时首期仅展示 Token |

UTC 存储，接口使用验证的 IANA timezone 进行日边界统计；相同窗口和 scope 贯穿总览和下钻。时间窗使用 `[from,to)`，如输入 local day 先明确转 UTC。组织归属变化的历史重算政策在 P3 设计；首期禁止在线转移空间组织。

## 7. 告警生命周期与存储

P1 固定三种规则：有在途执行的已关联终端失联、queued 超过 5 分钟、单次执行进入 failed。阈值是可调整工程初值，按容量和真实运行校准；普通闲置终端离线不告警。

执行失败事件可以按 organization/runtime/failure_code 的 5 分钟窗口合并；一次 task 失败只计一次，attempt 是独立执行。失联按 installation + 故障 episode 去重，排队超时按 task 去重。定期有界扫描补偿丢事件。

`admin_alert` 拟含 id,organization_id,rule,subject_kind/id,fingerprint,severity,status,first_seen_at,last_seen_at,occurrence_count,assignee_id?,acknowledged_at?,resolved_at?,closed_at?,version,operation_id?。状态 `open→acknowledged→resolved→closed`；自动恢复可从 open 直接 resolved，确认不等于恢复。

终端恢复可达/队列任务结束或取消产生自动恢复证据；一次失败是历史事实，关闭需关联处理结论（已重试结果、已处置、无需处理及原因），不假装原 task 已恢复。再次异常产生新 episode，不反复重开已关闭行。

按 `(organization_id,fingerprint)` 控制活动事件唯一；数据库索引全部独立 concurrent migration。告警处置变更与审计同事务，expected_version 避免旧页覆盖负责人；只有 super_admin 可确认/分配/关闭，observer 只读。

P1 通知渠道为后台总览待办和告警列表。外部通知不默认启用；P2 再接已有且经管理员配置的渠道，送达状态单列，不把写入数据库当通知成功。

告警扫描最多每批 200 条，使用行锁/版本控制防双实例重复处理。数据库/心跳来源异常时暂停对应离线推断，另显示检测器不可用。服务端故障导致大量失联时聚合关联，不持续发送逐台重复通知。

## 8. 必须呈现的页面状态

loading、empty、filtered_empty、permission_denied、auth_expired、backend_unavailable、unsupported_client、unassociated、stale_data、operation_applied_unconfirmed、operation_succeeded、version_conflict。

表格空态与接口失败不同；异步状态用文本及图标，不仅靠颜色。操作回执用可访问的 live region，焦点回到发起按钮。管理员角色被撤销后不继续展示缓存详情。

## 9. 实施落点

新增 `packages/core/admin/{api,schemas,queries,types}.ts`、`packages/views/admin/` 和双语 admin namespace；新增 Web admin 路由壳和平台导航；Go 查询新增 `admin_query.go` 与 SQL 白名单投影，告警新增 `admin_alert.go` service/query。具体任务分工见 implement.md，先验证路径命名及导出约定再落代码。

## 10. 数据类型与写入约束补充

下表补齐架构中的逻辑实体。所有UUID由服务端生成，时间为UTC `timestamptz`；整数版本用 `bigint`，要求非负、递增时检查溢出；JSON响应枚举均有未知值处理。索引/主键按迁移方案独立并发创建，表内不隐式创建索引。

| 对象 | 关键类型与约束 |
| --- | --- |
| `organization` | id uuid；name text非空；state text枚举；created_at/updated_at非空；首期唯一内部组织由部署初始化指定，业务请求不能创建新组织 |
| `organization_workspace` | organization_id/workspace_id uuid非空；created_at非空；workspace_id独立唯一索引；变更需授权事务，首期不开放迁移到其他组织 |
| `platform_role_binding` | user_id uuid唯一；role text=`super_admin/platform_observer`；granted_by uuid可空（部署操作），granted_at非空；部署操作者在审计actor_kind中区分，不伪造用户 |
| `user` 新增状态 | disabled_at timestamptz可空、disabled_reason text可空；原因长度受限且不含密码；凭据版本仍用既有session_version |
| `managed_installation` | public_key bytea为Ed25519规范公钥；key_fingerprint为SHA256 hex；admission text=`accepting/stopped`、admission_version bigint；组织/状态/公钥唯一索引见终端设计 |
| `installation_challenge` | nonce_hash/request_hash、purpose、deployment/organization/user范围、auth_version、expires_at、consumed_at?、result_id?；消费采用条件写并事务化，明文nonce不入审计 |
| `admin_operation` | kind/state/result_code text受控枚举；idempotency_key为UUID；payload_hash不含秘密；version bigint；actor_id可空但actor_kind必需；target类型/ID与kind匹配，不使用任意JSON命令执行 |
| `admin_audit_event` | actor_kind/user_id?、组织范围、目标kind/id、action、phase、request_id、时间非空；before/after为白名单JSONB，单条有大小限制；不可通过管理API更新/删除 |
| `admin_alert` | fingerprint text受控构造、rule/status枚举、version bigint、计数非负；时间及target非空；仅允许已定义状态转换 |

部署身份 `deployment_id` 是稳定、非秘密UUID，由受控部署配置提供并在安装接入服务校验；与可变的访问URL及organization_id分开。多实例必须相同，缺失时关闭新安装接入，不能每次启动随机生成。部署迁移保留该值；复制成独立部署需更换并重新接入，禁止把原安装凭据带入另一部署自动信任。

角色写请求使用 `role`（移除使用null）、`expected_role`、`expected_auth_version`、`reason`和管理员密码；锁内比对当前角色及凭据版本。账号disable/restore/recover使用 `expected_auth_version`。没有另建可作为授权依据的角色版本缓存。

终端接单策略使用expected_admission_version；告警使用expected_version；取消使用expected_execution_fence。字段按动作定义，不使用模糊的全平台version。源密码会话auth_version始终来自认证context，不能用body中的expected版本替代。

私密字段只在credential存储保留必要哈希；临时密码与管理员复核密码不存到operation、audit、错误详情或请求hash。删除/退役采用明确应用事务，保留执行、操作和审计引用；不得用删除用户来实现禁用。
