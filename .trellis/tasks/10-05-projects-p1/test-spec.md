# P1 验收与测试设计

状态：已实施的测试规格与验收映射；本轮 P1 实施验收完成（2026-10-06）。各域测试、迁移、性能与移动兼容已有实测，最终产品 `31534d844` 重建后的 Web/Desktop 8/8 通过（49.7秒，零重试/跳过）；首次完整检查失败与后续定向验证分开记录。最终结果见 [验收记录](verification.md) 与 [逐项审计](../10-05-projects-p1-verification/acceptance-audit.md)，本表描述必要断言，不自动代表该行全部通过。证据绑定实际commit/环境/命令，T1通过不能代替P1。源需求为`projects-prd.md:131`、`:418`，补充`:360`、`:393`、`:477`；设计见 [design.md](design.md)，响应/证据/hash/排序唯一合同见 [api-contract.md](api-contract.md)。

## 1. 测试层、实际文件与fixture

下表简码在映射表引用时，同时表示具体测试层和实际新增/扩展文件。纯规则只保留一个canonical矩阵，组件测接线/恢复，E2E测真实串联，避免重复DOM矩阵。纯解析/规则采用node；mutation、access/session lifecycle和组件测试按实际文件声明采用jsdom，不将C/R整组称为node。

| 简码 | 测试层与文件 | fixture/方法 | 负责子任务 |
| --- | --- | --- | --- |
| F | Go DB integration：新增 `server/internal/handler/project_revision_test.go`、`project_timezone_test.go`、`project_delete_concurrency_test.go` | `testutil`/`dbfx`建成员/项目/任务；`testutil.Call`；两连接锁屏障/故障注入 | foundation |
| H | Go unit：新增 `server/internal/projecthealth/health_test.go`；DB integration：`handler/project_health_test.go` | 固定时钟/日期/完整目录；真实DB全量集合、快照分页与JOIN去重 | health |
| P | Go DB integration：`server/internal/handler/project_update_test.go`、`project_update_concurrency_test.go`、`project_update_notifications_test.go` | 人类/PAT/task/cloud/legacy-agent actor，稳定request UUID、丢响应/重启投递/撤权屏障 | progress |
| C | Vitest：`packages/core/api/project-p1-{schema,client}.test.ts`、`projects/{goal-template,description-save,progress-draft-store,access-lifecycle}.test.ts`；扩展 `projects/mutations.test.tsx` | 新旧/畸形响应、跨空间identity、QueryClient/StorageAdapter、AbortSignal/晚响应 | ui |
| V | Shared views：`packages/views/projects/components/project-management.test.tsx`、`project-review-regressions.test.tsx`、`project-overview-lifecycle.test.tsx`、`project-risk-pagination.test.tsx`；扩展 `project-detail.test.tsx` | 原ContentEditor/mention格式、模板/冲突/焦点、旧过滤偏好、资源/小队数据 | ui |
| R | Vitest：扩展 `packages/core/realtime/use-realtime-sync.test.ts`、`issues/cache-coordinator.test.ts`，`projects/realtime.test.ts`、`projects/access-lifecycle.test.ts`及shared views `project-overview-lifecycle.test.tsx` | Query+内存/持久草稿，WS事件、规划日界、重连、撤权导航失败 | ui |
| MO | Mobile Vitest node：新增 `apps/mobile/data/project-p1-api.test.ts`、`data/queries/projects.test.ts`、`data/realtime/project-ws-updaters.test.ts`、`data/realtime/project-access.test.ts` | 独立ApiClient/纯parser调用、mobile扁平Project[]/detail QueryClient、mock WS/Storage/401回调；不加载RN模块/不用Web hooks | ui；verification独立执行 |
| E | Playwright：新增 `e2e/projects-p1.spec.ts`、`e2e/projects-p1-desktop.spec.ts` | `TestApiClient`建完整数据，Web生产式服务/Desktop，后台mock执行计数 | verification |
| M | DB migration/integration：`server/internal/migrations/project_p1_rollback_test.go`，辅以 `handler/project_delete_concurrency_test.go`、`project_association_concurrency_test.go`；[迁移报告](../10-05-projects-p1-verification/migration-verification.md) | 空库/既有填充库/部分失败库、catalog/ledger对照、约束扫描、up/down/恢复 | verification |
| O | [C性能报告](../10-05-projects-p1-verification/performance-c-report.md)、[原A失败报告](../10-05-projects-p1-verification/performance-report.md)及 `performance/` 实际脚本/样本 | 固定机器/版本/并发/冷暖缓存，EXPLAIN ANALYZE、P50/P95、query数、outbox积压 | verification |

统一fixture：两个互不授权工作空间；owner/admin/member/离开成员；有效/归档/跨空间/离线agent；有效/归档/leader失效squad；五项目状态；内置和自定义（含archived/未知）任务状态。正式N=10样例为6done+2cancelled+2todo；另加pending/rejected/duplicate和候选项目记录、父子任务、无/失效负责人、昨天/今天/明天/空截止日。集成测试不得被mock数据库替代。

## 2. PRJ-001—014 需求追踪

| 需求 | canonical证明与扩展断言 | 层/文件简码 | owner |
| --- | --- | --- | --- |
| PRJ-001 | 身份/五状态/属性/默认小队/资源不变，五种原任务视图可用；无第二项目实体 | F、V、E | foundation/ui/verification |
| PRJ-002 | 空/非空描述模板预览、章节选择、重复应用；原description与执行brief同源 | C、V、F | ui/foundation |
| PRJ-003 | 显式验收/依据必填/适用description版本/不可变快照，独立于100%与completed | P、F、V | progress/foundation/ui |
| PRJ-004 | N/F/C/U、空集合/全取消/未舍入排序、旧done_count兼容 | H、C | health/ui |
| PRJ-005 | 5种admission×租户×实际/候选项目；普通接受只准入；开关关闭也不混入 | H、F、E | health/foundation/verification |
| PRJ-006 | 日期/时区/DST、风险并集、有效引用与离线、archived/unknown类别、全量父子 | H | health |
| PRJ-007 | 卡片与下钻同版本一致，数据变动刷新；隔离个人筛选，incomplete不绿 | H、V、E | health/ui/verification |
| PRJ-008 | 人工发布/取消/输入恢复/真实身份/证据再查/站内成员提及，零执行 | P、C、V、E | progress/ui/verification |
| PRJ-009 | create/correct同意图重放、payload冲突、revision冲突、不可变修订、通知去重 | P、C | progress/ui |
| PRJ-010 | completed警示不阻止，原因缺省留痕，旧客户端仍可修改状态 | F、V | foundation/ui |
| PRJ-011 | 暂停/取消/完成/重开只改项目，运行执行/任务保持；七日从进入进行中计 | F、H、E | foundation/health/verification |
| PRJ-012 | 管理员原子删除与writer双顺序，无孤儿、停用关联自动化且保留历史 | F、M | foundation/verification |
| PRJ-013 | lead/资源失效、统计失败、描述并发均有可恢复提示，无静默改配/覆盖 | F、H、V | foundation/health/ui |
| PRJ-014 | Web/Desktop共享行为、移动/CLI/旧客户端能力限制、跨空间和机器actor拒绝；移动单独实测 | F、P、C、R、MO、E、M | 全部；verification汇总 |

## 3. PRJ-AC-01—27 逐条验收

| 原验收 | fixture、动作与必要断言 | 测试层/实际文件简码 | 负责子任务 |
| --- | --- | --- | --- |
| PRJ-AC-01 | 同一旧项目保留ID、五状态、属性/资源/小队；逐个打开board/list/table/swimlane/gantt，原链接/自动化入口可用 | F、V、E | ui/verification |
| PRJ-AC-02 | 空正文插模板保存，DB只有description更新；overview/editor/项目执行上下文同值同description_revision，不新增goal镜像 | F、V；扩展 `server/internal/handler/chat_project_context_test.go` | foundation/ui |
| PRJ-AC-03 | 原文含目标章节，选择缺失章节预览再追加；重复应用不自动全量插入，取消/编辑器未就绪不丢原文 | C、V | ui |
| PRJ-AC-04 | 全done/cancelled闭合100%、无验收；latest_acceptance为空，页面尚无记录，项目状态不变 | H、P、V | health/progress/ui |
| PRJ-AC-05 | v1通过，描述改v2，快照仍v1；多验收含较新failed/旧版记录，按published_at/id选择latest与current_description两摘要；旧版更正不改排序；图标不失效 | F、P、V | foundation/progress/ui |
| PRJ-AC-06 | passed/partial无证据且explanation空白，422定位依据、无新record/outbox、草稿保留；failed可无通过依据 | P、C、V | progress/ui |
| PRJ-AC-07 | 6done+2cancelled+2todo，N10/F6/C2/U2/closure .8；若展示actual .6；旧done_count8 | H、C、E | health/ui/verification |
| PRJ-AC-08 | 零正式任务（可含非正式候选），计数0、ratio null、暂无任务，不100%；项目过期仍单示 | H、V | health/ui |
| PRJ-AC-09 | 仅2cancelled，N2/F0/C2/U0/closure1，验收未通过且状态无自动变化 | H、V | health/ui |
| PRJ-AC-10 | pending/rejected/duplicate+候选project混入数据；普通五视图/overview/health结果排除，历史授权读取可用 | H、E；扩展 `triage_boundary_test.go` | health/verification |
| PRJ-AC-11 | pending拟项目有默认squad，普通接受后N只+1；重放不加第二次；task执行行和唤醒事件均0 | F、H、E；扩展 `triage_boundary_test.go` | foundation/health/verification |
| PRJ-AC-12 | 固定D下昨天/今天/空日期与done/cancelled组合，只有未结束且due<D逾期；不随查看者时区变 | H | health |
| PRJ-AC-13 | 一个blocked+逾期+无有效指派任务，B/O/A各1，union1，三个下钻返回同ID一次 | H | health |
| PRJ-AC-14 | 有效同空间agent但runtime离线，不计A，独立环境不可用；不可调用的私有agent也不误判未分配 | H | health |
| PRJ-AC-15 | 自定义key映射in_review（含archived），按类别计R；显示名改变不影响，未知key不默认为todo完成 | H | health |
| PRJ-AC-16 | 3逾期含他人/子任务/隐藏状态，个人saved view仅自己；点卡片准确3条，显示临时scope，退出原偏好保留 | H、V、E | health/ui/verification |
| PRJ-AC-17 | 同snapshot一个未知key/目录查询失败，N保留未知/U，complete=false、unknown或失败原因，health unavailable无绿色结论 | H、C、V | health/ui |
| PRJ-AC-18 | member预览→发布→刷新，DTO所有身份/revision/null字段齐全；snapshot非递归，不含验收/进展；历史作者离开仍署名、已删占位不返回email | P、C、E | progress/ui/verification |
| PRJ-AC-19 | DB已commit但HTTP响应丢失，同request/hash重试返回相同id/revision；一组outbox/inbox，无重复行 | P、C | progress/ui |
| PRJ-AC-20 | 更正必填理由及expected_revision，append-only保留原文/作者/时间、显示修订人/原因；过期验收更正不绑定新描述 | P、V | progress/ui |
| PRJ-AC-21 | U>0且无验收，UI提示剩余/取消/验收，允许completed；有/无原因分别审计；旧API无原因仍成功且任务状态不变 | F、V、E | foundation/ui/verification |
| PRJ-AC-22 | 有运行task的项目逐一paused/cancelled/completed/reopen，task/issue状态、关联和执行计数不变；真实迭代部分见§5待I1 | F、E | foundation/verification |
| PRJ-AC-23 | owner/admin删除，issue保留project_id空、执行/评论保留、专属progress/resources清理；实际迭代不存在，不能标整场景通过，见§5 | F、M | foundation/verification |
| PRJ-AC-24 | member/agent即便lead仍无权删除；owner/admin授权删除；交叉workspace和撤权并发均拒绝 | F | foundation |
| PRJ-AC-25 | 两客户端共同读v1，一方保存v2，另一旧revision409；v2不覆盖，客户端文本/比较可恢复；缺CAS真实写428 | F、C、V | foundation/ui |
| PRJ-AC-26 | outsider以另一workspace的project/update/evidence/attachmentID请求详情/统计/历史/下载；无内容/计数泄漏；已缓存后撤权立即清除 | F、P、C、R、E | 全部；verification汇总 |
| PRJ-AC-27 | 正文同时提有效member/离开member/agent，预览只有效member通知，发布agent不执行；投递前撤权不发送，无邮件/外发 | P、E | progress/verification |

## 4. 原验收表之外的 P1 边界

这些项来自PRD§14、§16、§19及共同README，不因没有单独AC编号而省略。标识P1-FR是本任务测试标签，不改原PRD编号。

| 测试标签／来源 | fixture/断言 | 层/owner |
| --- | --- | --- |
| P1-FR-01 字段；PRD:360 | Unicode code point中文/英文/emoji正文0/1/10000/10001、理由0/1/1000/1001、证据50/51；trim与无效类型一致；保留现有长描述不收紧 | P、C；progress/ui |
| P1-FR-02 日期；PRD:368 | start>due新编辑422；只改图标不拒绝历史异常日期；null清空和缺字段不变；无效日期400；不静默交换 | F、V；foundation/ui |
| P1-FR-03 时区/七日；README:119/PRD:240 | 未配置UTC明确、IANA有效/无效、member拒绝设置、旧settings覆盖不丢列；D-6/D-7、DST23/25小时、无发布从进入进行中或migration基线计、更正不重置 | F、H、R；foundation/health/ui |
| P1-FR-04 并发属性；PRD:397/405 | 两描述CAS；图标旧请求不回写旧描述；新状态/日期expected_revision冲突；旧无token非描述字段合法且审计“未填写原因” | F、C；foundation/ui |
| P1-FR-05 快照；PRD:215/253 | 双连接屏障在读取两阶段间改变任务/目录/有效指派：RR统计必须单一快照或40001整笔重试；同事务实际snapshot写入，preview统计version变化409，calculated_at独变不冲突；unknown要求快照503，历史不被改写 | H、P；health/progress |
| P1-FR-06 证据复核；PRD:372/404 | issue.revision、execution.state_version/结果digest分别变化均409；chat非creator/私有agent不满足读策略拒绝；旧snapshot等待fence时成员/来源已变触发重试，不读RR旧授权；外链无验证版本；canonical固定向量排除采集时间、纳入接收者/统计版本 | P、C、V；progress/ui |
| P1-FR-07 发布竞争；PRD:398 | 并发同UUID同hash仅一份；同UUID不同hash409；correct两版本一成一409；超时查回原结果；幂等查回仍授权，撤权不得读原内容 | P；progress |
| P1-FR-08 删除/撤权编辑；PRD:399/400 | 删除事件停提交并允许复制本人未提交文本；scope 403、404 workspace_access_denied/撤权先移除保护Query/编辑器/候选/草稿，不依赖导航成功；三类操作/来源/能力403保留仍授权的数据与本人输入；晚网络响应不得重填 | C、R、V、E；ui/verification |
| P1-FR-09 不完整/失效；PRD:401/402 | 聚合或目录失败、lead离开、资源不可访问：明确时刻/原因/重试，旧值仅标过期；无假0/绿色/静默改配；历史作者保留署名而不泄权限 | H、P、V；health/progress/ui |
| P1-FR-10 删除全部writer；PRD:406 | 显式/继承/自动化创建、单条/批量换项目、T1接受/手动CSV候选、资源、聊天、小队、进展、通知逐入口以barrier测试先关联/先删除；无活跃孤儿/锁环；候选ID与分拣审计保留但显式失效，接受拒绝失效候选；迟到幂等重放404不复活进展/通知 | F、M；foundation/verification |
| P1-FR-11 故障删除；PRD:407/408 | 每阶段错误整体回滚；非archived自动化status=paused/pause_reason=project_deleted，archived保留、trigger.enabled=false；所有project_id解除；运行/历史/共享资源保留；投递/删除两锁顺序屏障无死锁 | F、P、M；foundation/progress/verification |
| P1-FR-12 通知恢复；PRD:269 | 两worker同候选无锁扫描后按正序锁只投一次；投递/撤权/删除barrier无环；崩溃重启既有inbox不重复；5秒轮询/backoff/12次deadletter/ctx取消可用fake clock测；修订仅补新提及、已读不改业务 | P、E；progress/verification |
| P1-FR-13 混合版本；PRD:479/481 | 真实旧router合法ws子资源project-capabilities404；旧错误路径projects/capabilities确实400，不能mock替代；新路由member200/非成员404且workspace_access_denied；400/401/403/404/network/schema各分支；mobile旧字段不猜F/C，原属性写不丢P1字段，description428 | C、MO、F、E；ui/foundation/verification |
| P1-FR-14 上线回退；PRD:483 | 关闭新UI能力仍保留全部进展/验收/快照，兼容后端可只读；禁止任意旧二进制绕过CAS/丢历史入口；回退不改任务/状态 | M、C；verification/ui |
| P1-FR-15 WS/日界；README:124 | due/assignee/delete/admission/目录/member/agent/squad/runtime/timezone/跨日/重连重算；mobile自己的flat list/detail在完整事件patch、ID事件invalidate，新字段不被旧partial覆盖；scope 403或404 workspace_access_denied先清保护缓存、晚响应不得回填 | R、MO、E；ui/verification |
| P1-FR-16 权限身份；PRD:346 | owner/admin/member可发布，task_token/cloud_pat/legacy-agent及agent lead不可最终发布；当前成员读写/下载限制；管理员设置时区仅人类 | F、P；foundation/progress |
| P1-FR-17 安全呈现；PRD:379 | 富文本/链接转义、外链schema、mention伪造recipient、跨空间证据ID；只使用既有安全渲染策略，不引入新依赖或外部fetch | P、V；progress/ui |
| P1-FR-18 无执行副作用；PRD:389 | 模板进入原描述上下文但进展不无限追加；状态/健康红/成员与agent提及/项目小队变更不触发执行或重配已有任务，T1执行门槛保留 | F、P、E；foundation/progress/verification |

## 5. I1 的证据边界与交接

当前研究未发现已实现的迭代实体/结束快照，因此PRJ-AC-22与AC-23中的真实迭代分支和共同WM-AC-05/06/11不能在P1用不存在的数据表、mock列或“没有修改”搜索证明为已验收。

P1可验证部分：删除只清issue.project_id、不删除issue/task/comment，项目状态修改只写project/audit，发布快照不受实时任务变动影响。P1 design/SQL不得加入任何对未来迭代归属或历史的写操作。

I1实施时由I1负责人补真实fixture：当前迭代+结束快照中的项目任务→项目暂停/取消/重开/删除→当前归属与结束快照逐字段不变；项目改名/任务移动后历史仍原值。P1 verification报告把这两行标为“现有部分通过／迭代集成待I1”，不能将AC-01—27笼统宣称全部通过。P1本轮验收完成不表示I1真实迭代集成已经通过。

## 6. 迁移、性能与可观测性

- M：空库up、旧正式/非正式项目填充库up、索引构建失败/重试、ledger已记但对象缺失恢复；catalog确认无新增FK/cascade、每索引concurrently单语句。空新增表且writers停止的down成功；保留进展/revision/request/outbox/state audit/规划配置或版本变更时guard明确拒绝；并发写入与锁内检查两顺序不丢数据；索引部分down失败按up恢复，不能承诺恢复历史FK。
- M：回填逐字段比较标题/描述/日期/状态/lead/资源，旧done_count金样不变；进行中历史时间明确migration来源，nullable时区effective UTC；工作空间删除清所有新表与outbox。
- O：500项目/10,000正式任务按design采样P95/资源基线，HTTP 2秒固定预算已通过，跨页面5秒收敛以独立浏览器证据为准；追加0/1/10次每秒任务/目录/指派变化各10分钟、每秒翻页操作，统计refreshed比例/达到第二页成功率与P95耗时/连续重置次数；先基线记录并评审可用预算再最终判定。原A已验证不可用；按ADR-05改C后完整复测通过，三档各至少600秒、1709/1709非空续页严格前进、静态最大P95约558ms；保留原失败样本。非空后缀100%真实ID前进，terminal空后缀单列；sticky变更提示、低ID重入、成功/失败从头刷新均需验证，不把refreshed=true误计为重置。
- O：统计完整率/unknown原因、overview耗时、snapshot刷新、CAS/preview冲突、幂等重放、outbox待投递/失败重试、删除失败与孤儿扫描形成现有日志/指标；不记录正文、证据内容、token。日志只用授权范围内ID/原因/计数。
- E：键盘完成模板选择、风险下钻、发布/冲突恢复；错误有文字、焦点可找、不只靠颜色；目标信息找寻和风险定位按PRD:465/466先记录基线，不虚构90%/30秒达标。

## 7. 执行、证据格式与准出

各子任务先最窄canonical测试再扩展；Go使用dbfx/testutil，UI纯逻辑node环境，共享组件不放app测试。MO符合`apps/mobile/vitest.config.ts`仅lib/data node测试：API层mock native入口、Query层mock `@/data/api`，用真实mobile key与flat Project[]，验证signal/30秒timeout、401与403清理、旧属性write响应parser。不得新增RN测试依赖。根检查排除mobile，须独立执行`pnpm -C apps/mobile typecheck`、`lint`、`test`，具体全命令见implement §8。DB/mobile失败或skipped分开报告，规划检查不代替产品测试。

每项记录：需求/AC/FR标签、实现commit、测试文件/test名称、fixture、命令/cwd、环境与时间、通过/失败/skipped、证据路径、剩余依赖。真实agent smoke不属于P1，所有无执行断言用安全mock执行后端和DB计数。

准出必须有：原功能无损、正式集合统一、旧API不变义、描述防丢写、验收不可变、发布/通知去重、删除所有writer竞争、撤权缓存清除、Web/Desktop完整流程、移动/旧客户端明确能力限制、迁移回退报告和性能已实测预算。I1待验部分单列，不扩大P1范围、不作虚假完成声明。
