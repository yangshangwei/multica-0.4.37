# P1 开发拆分与交付顺序

本文件是实施计划，未执行的产品项保持未勾选。用户本轮只授权规划。设计依据 [design.md](design.md)，线协议唯一来源 [api-contract.md](api-contract.md)，验收依据 [test-spec.md](test-spec.md)，现状证据见 [backend-map](research/backend-map.md) 与 [frontend-map](research/frontend-map.md)。

## 1. 五个子任务与依赖

| 子任务 slug | 交付范围／唯一责任 | 测试准备门槛 → 产品实现启动门槛 → 完成集成门槛 |
| --- | --- | --- |
| `projects-p1-foundation` | schema/迁移、时区、属性/描述CAS、审计、锁/删除；持续共享SQL/router/sqlc整合 | 已评审计划 → 实施授权 → FG、各域提交的共享改动已顺序整合且全回归通过；不得在FG时归档 |
| `projects-p1-health` | 全量统计、有效指派、快照/下钻、完整性、旧计数兼容 | API合同已评审（可先写fixtures） → FG → HG及相关共享生成文件已由foundation整合 |
| `projects-p1-progress` | 手动记录/验收/证据/修订、幂等/通知worker | API合同已评审 → FG（与health并行，无统计路径先做） → HG+PG，附统计发布与共享worker/router整合通过 |
| `projects-p1-ui` | core/views/Web/Desktop、移动读取兼容 | API合同已评审（先mock契约） → FG（可在API开发时做UI） → HG+PG+UG，真实接口和全部平台验证通过 |
| `projects-p1-verification` | fixtures/基线、集成/迁移/回归、文档与证据 | 已评审计划（第一波准备基线） → FG用于迁移/服务集成，各API就绪可递增验证 → FG/HG/PG/UG且前四任务完成、最终证据齐全 |

里程碑：FG=foundation基础schema/迁移、锁/事务/DTO脚手架及其局部测试可用；HG=health真实API+事务采集函数及canonical测试就绪；PG=progress真实API/outbox就绪；UG=各平台真实接口集成完成。每项在verification.md记录commit/命令/结果后才能解锁，不能只口头宣布。

元数据唯一含义：`meta.depends_on`仅列“必须已完成整个子任务才能启动实现”的任务ID；本方案五项均为[]，因为启动依赖是阶段里程碑而非完整任务，不能把foundation完整结束写成health/progress前置。另用`meta.preparation_gates`、`meta.start_gates`、`meta.integration_gates`逐字表达本表，父代理同步五子任务；不支持里程碑的调度器由父代理核对证据后启动，禁止忽略gates。verification“准备”不等于“完成验收”；foundation共享整合owner持续到所有下游文件变更归并结束，不形成彼此等待完成的循环。

父任务负责跨子任务决策、共享合并、Architect/Critic复核与完整性审计。父子树只表达组织关系；本轮均保持planning，规划就绪不表示产品完成。

## 2. 文件 ownership 与共享文件规则

下表“拟新增”路径是实施落点，不声称文件已经存在。引用现有文件行号来自 research；开发时若文件重排重新定位。

| owner | 独占文件／模块 |
| --- | --- |
| foundation | `server/migrations/*project*`、规划时区及进展新表迁移；`server/pkg/db/queries/project.sql`、`workspace.sql`；拟新增 `project_update.sql`、`project_health.sql` 的最终SQL整合；`server/cmd/server/{router,main}.go`（含progress worker注册）；全部sqlc生成文件；`handler/project.go`、拟新增 `project_timezone.go`、`project_write_fence.go`；`handler/workspace.go`；`handler/issue.go`、`triage_actions.go`、`autopilot.go`、资源/聊天/小队配置入口与`service/issue.go`必要锁改动 |
| health | 拟新增 `server/internal/projecthealth/{snapshot,health}.go` 与本域测试，`handler/project_health.go`/`project_health_test.go`；负责聚合 SQL 内容，提交给 foundation 应用到共享 SQL；与 foundation 共同评审 ProjectResponse 分项字段，由 foundation 修改 `project.go` |
| progress | 拟新增 `handler/project_update.go`、`project_update_notifications.go`、`project_update_test.go`、`project_update_notifications_test.go`；负责进展 SQL 内容交给 foundation；不写任务 comment/agent 执行链，不改 T1 语义 |
| ui | `packages/core/types/project.ts`、`api/{client,schemas}.ts`、`projects/`、`drafts/register-all-drafts.ts`、`realtime/use-realtime-sync.ts`、`issues/cache-coordinator.ts`；`packages/views/projects/`、`views/issues/surface/`必要临时范围接口、`views/locales/`；现有 Web/Desktop 项目页面/paths；`apps/mobile/data/`和项目显示兼容所需最小修改 |
| verification | 拟新增 `e2e/projects-p1.spec.ts`、`e2e/projects-p1-desktop.spec.ts`、`server/internal/handler/project_p1_integration_test.go`、迁移演练夹具/报告、`apps/docs/content/docs/projects*.mdx`、内置项目 skill 与 source-map 文档、本任务 `verification.md`；避免与各实现者同写其 canonical unit 测试 |

`router.go`、所有 queries/生成 db 文件由 foundation 单 owner 修改和 `make sqlc`。health/progress 提交明确 query/request/response 变更提案；foundation 顺序整合后再运行生成器，禁止多 agent 同时覆盖生成结果。core 全局 schemas/client/realtime 由 ui 单 owner；其他子任务用契约和测试数据沟通，不直接修改。

共享 existing writer 行号与风险：`project.go:680`删除、`issue.go:3523`单条换项目、`:4166`批量、`service/issue.go:345`创建/父继承、`triage_actions.go:43`NOWAIT、`autopilot.go:1270`绑定；完整名单见 design §8。只修与 P1 删除正确性相关边界，不扩展成全仓库重构。

## 3. Foundation：数据、写入与删除

- [ ] 实施前查 live schema/catalog、迁移 ledger 与最新编号，建立空库/升级库两种基线。读取 `CLAUDE.md:110` 和 server 层 spec；不得新增外键/级联。
- [ ] 创建 design §3 增列/新表、单文件并发索引、up/down、回填标记；历史 description 不改字节，历史进行中只从迁移起计时。同步 workspace 显式清理所有新表。
- [ ] 写失败用例后实现 rawFields 锁内合并、租户 WHERE、description CAS/428、全属性 revision、五状态审计和原因缺省；旧非描述写仍合法，新日期编辑校验最终日期组合。
- [ ] 新增规划时区GET/PUT及`/api/workspaces/{validWorkspaceId}/project-capabilities`成员子路由，settings整体替换不覆盖新列；保留真实旧router400/新子资源404测试，400/401/403不降级。
- [ ] 将所有 project 关联 writer 纳入共享锁/多项目排序；T1或已持 issue 锁路径保持 NOWAIT、整事务失败重试。新增项目删除影响接口，管理员删除整笔解除任务关联/停用自动化/清专属数据。
- [ ] 逐阶段故障注入证明全部回滚；project→issue 与 issue→project 两种顺序证明无悬空/死锁；既有运行执行、历史、共享资源保留。
- [ ] 锁定并交接 health/progress 所需查询/DTO/通知身份接口；为共享文件顺序整合出完整 sqlc 生成 diff。

完成证据：description 并发与旧客户端矩阵、时区权限/回填、全 writer 删除竞争、迁移往返、原资源/小队/T1 fence 回归。此阶段不得凭 schema 完成就宣称健康/进展可用。

## 4. Health：同快照聚合与下钻

- [ ] 先固定 PRJ-AC-07/08/09/10 的 gold fixtures；正式白名单不读取分拣开关，按稳定 ID/父子任务计数。
- [ ] 类别目录包含 archived；无法解析 key 和读取失败显式 incomplete；成员/agent/squad引用有效性与 runtime 可用性分离，不能调用 canInvokeAgent 当分配判断。
- [ ] 同只读事务得到 N/F/C/U、B/O/A/R、去重风险、项目逾期、最近进展/七日提醒；日期纯DATE，UTC未配置与IANA/DST测试齐备。
- [ ] 生成规范 snapshot_version；overview/drilldown共享聚合，分页版本变化重置第一页/明确 refreshed。故障时不返回假绿色与假零，未知状态不当完成。
- [ ] 与 foundation 整合 list/get/search/create/update/squad响应；保持旧 done_count=F+C，新分项缺失能区分于零。
- [ ] 提供接受qtx的系统快照函数，读取/preview/发布统一RR与授权locking read，不另开事务；双连接屏障证明目录/任务/指派变化后来自单一快照或整事务重试。DTO/错误/刷新fixtures统一引用api-contract。

完成证据：同版本卡片/下钻100%一致、跨版本刷新、完整性矩阵、10,000条全量聚合正确性；性能只在已固定环境下报告。

## 5. Progress：人工记录、版本与站内通知

- [ ] 按api-contract做read/list/history/preview/create/correct；RR事务第一条SQL设隔离，actor/成员locking read与项目/source锁组合验证，40001/40P01/55P03最多3次整事务重试。
- [ ] 先实现 request_id/规范payload hash重放、结果查询、同键不同载荷409；更正CAS、理由和append-only历史。
- [ ] 验收锁住适用 description_revision，passed/partial依据校验；旧版更正不能冒充新版验收，图标变化不令验收过期。
- [ ] 发布前证据/可见性/描述/接收者重新核对；变更返回差异并要求预览；snapshot由服务端采集，不完整时提供无快照发布选择。
- [ ] 提及解析复用格式、发送链独立；outbox无锁候选扫描后逐条正序锁，worker在main.go由foundation注册，5秒轮询/退避/12次deadletter；屏障证明投递/删除/撤权无环，重复worker/崩溃无重复通知。
- [ ] 确认保存/更正/验收/通知均无任务状态、执行或外发副作用；最近进展依首发时间，更正不抹去七日提示。

完成证据：真实数据库并发重放/撤权/描述竞争、不可变历史、outbox恢复、人工发布权限矩阵及零执行/零外发断言。

## 6. UI：共享页面与混合版本

- [ ] core严格解析新DTO，捕获wsId/项目身份，更新query keys/abort；能力404与403/network/malformed分开，新字段缺失不能从旧done_count猜实际完成。
- [ ] 把描述版本更新/完成/发布/删除改为server-first安全路径；保留普通可预测属性的局部乐观编辑。修现有delete在服务端前移除缓存的问题。
- [ ] 复用ContentEditor/RevisionConflictCompare；模板预览选择章节追加、不重复、不覆盖；串行autosave携adopted revision，晚响应不丢新正文。
- [ ] 概览独立显示目标、系统健康、人工判断、完成/取消、状态和验收；空态、未知/失效/加载失败、旧版本验收、资源/负责人问题均给恢复入口。
- [ ] 风险结果用显式temporary context绕过activeView和个人过滤，以列表/表格保全父子和无日期任务；清除/返回不改用户原偏好。原项目五视图、资源、默认小队/自动化继续可用。
- [ ] 项目进展草稿按连接/workspace/project/操作分离并登记清理；稳定request_id重试，失败标尚未发布，取消无请求，迟到成功不清新草稿。发布预览收件者与证据差异。
- [ ] 扩展WS/Query依赖至due/assignee/delete/admission/目录/member/agent/squad/runtime/timezone；日界和重连刷新；撤权先遮蔽/清缓存草稿再导航，self-event单 responder。
- [ ] 同步shared path参数与Web/Desktop深链、返回、刷新、新页；移动端只做独立读取/能力限制、原CRUD兼容。已读`apps/mobile/CLAUDE.md`及真实package scripts；按test-spec MO落点在mobile/data/node测试API、Query、WS、撤权/旧字段保留，不导入Web hooks/store或引入RN renderer。
- [ ] 中英文本、键盘流程、焦点/错误、长正文、缩放与窄屏验证；视觉迭代按项目规则保留visual-verdict证据。

完成证据：canonical core矩阵 + shared views组件交互 + Web/Desktop浏览器路径，移动混合版本/拒绝绕过测试。不声称移动新增编辑完成。

## 7. Verification：基线、集成与发布准备

- [ ] 第一波固定fixture/环境及基线；等新查询落地后对比EXPLAIN/延迟/资源，按PRD建议锁定可复现门槛，记录未达项和调整依据。
- [ ] 对照test-spec逐项收集PRJ-001—014、AC-01—27与FR-only证据；I1未实施的真实迭代场景显式标外部依赖，仅验证当前“不改其他字段/执行历史”部分。
- [ ] 跑新后端集成、迁移空库/升级/中断恢复/索引失败重跑、角色/机器凭据/跨空间矩阵、旧新客户端双向契约、回退只读历史。
- [ ] 浏览器使用本工作树归属环境、生产式Web服务与Desktop流程；现有T1/项目小队/资源/任务五视图回归。测试只用mock执行，不访问真实agent账户。
- [ ] 更新项目中英文文档、内置项目skill/source-map、新字段/428/时区/删除影响说明；记录迁移/回退和能力开关；逐项记录真实命令、日期、commit、结果与未验范围。
- [ ] 最终由非实现者检查统计谓词、权限/锁顺序、CAS、无执行副作用及测试覆盖；发现问题回归所属owner处理，验证完成再提产品上线审批/发布任务。

## 8. 验证命令与执行门槛

开发者开始前核对 `Makefile`、各 `package.json` 与pnpm工作空间为当前命令真源（`CLAUDE.md:74`）；新增文件路径以实际落地为准。

```bash
make sqlc
go test ./internal/projecthealth ./internal/handler ./internal/service
go vet ./...
pnpm typecheck
pnpm lint
pnpm test
make test
pnpm exec playwright test e2e/projects-p1.spec.ts
pnpm exec playwright test e2e/projects-p1-desktop.spec.ts
pnpm -C apps/mobile typecheck
pnpm -C apps/mobile lint
pnpm -C apps/mobile test
make check
```

Go命令在server/下运行，其余在根目录。根pnpm typecheck/lint/test和make check排除mobile（根package.json、scripts/check.sh:117），不能代验；三条`pnpm -C apps/mobile`分别执行tsc、expo lint、vitest+ios-run shell测试（mobile/package.json:22）。mobile针对性验证可用`pnpm -C apps/mobile exec vitest run data/project-p1-api.test.ts data/queries/projects.test.ts data/realtime/project-ws-updaters.test.ts`。DB不可用或mobile既有失败必须列清，不能把skipped记passed。此规划轮未执行产品测试。

规划交接门槛：Architect审阅后Critic复核、父子范围/ownership/deps一致、引用/JSONL有效、无TBD核心契约。产品交接门槛：逐项证据齐全、lint/typecheck/unit/integration/e2e/static analysis通过，已知历史环境失败单独列明，不能沿用T1结论充当P1证据。

## 9. 回退点与交接人员

波次一结束后可停在新增schema未开放UI；波次二可关闭P1能力保留原项目；上线后UI可回退但历史数据与含CAS/只读历史的后端必须保留。破坏性数据库down不是常规回退步骤。

建议角色：foundation/health/progress/ui用executor，各域复杂事务取高推理；verification用test-engineer或verifier；跨域先architect再critic。可用角色还包括explorer（代码定位）和designer（共享界面审查）。单人依本文件顺序执行亦可，不强制OMX运行时；需要并行时仅在对应依赖已满足后委派明确ownership，不递归改写全局计划。
