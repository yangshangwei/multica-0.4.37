# S2 执行计划

## 2026-11-16 r09 与首次实际运行

最新证据见 `research/progress-20261116.md`。以下旧时点记录保留，不能替代本轮来源绑定。

- P2 已独立接受为 `P2_CODE_ACCEPTED_R09_SCOPED`，阻挡项 0。三条已批准 ROOT 路径完成窄整合；入口可显式启用，项目政策仍默认关闭。原 HTTP 503、K06 两次错误测试假设与 READ ONLY recorder 失败均保留。
- 当前 5,038 文件服务层图已独立接受为 `JOINT_SERVICE_ACCEPTED_R09_SCOPED`；37+79 两段 normal/race 各 116 顶层、321 命名、263 叶 PASS，无失败或跳过。该结论限于绑定服务范围，完整 S2 仍须最终跨层证据复核。
- 生产前端 3,358 输入/1,961 链接已核对；5 项类型、2 项 lint、4 项测试命令、2 项构建均成功，619 个参数用例来自 36 个文件。匹配 server/CLI/router harness/Host/受控 provider 已构建，运行库实际 853 条迁移账本与源一致，865 索引有效；真实本地管理员初始化、改密、重新登录与 `/api/admin/me` 权限核验通过。
- 首次正式四场景 `four-scenarios-r01` 以 exit1 停止：E01 在参与者邀请准备收到 `400 invalid_request`，尚未打开 Web/Desktop 页或派发 Host；E02–E04 为 `NOT_RUN`。实际 0 PASS、1 FAIL、3 NOT_RUN，原 4 分母、请求、回执及清理记录保留，未自动重试。
- 原因是 E2E `workflowAuthority` 复用 Host 输入时带上工作流合同不接受的 `execution_view_version: 6`。仅修 `e2e/fixtures/collaboration-multi-account-closure.ts` 的字段选择，先新私有副本与独立检查，再绑定修正版重跑。服务端 strict decoder、原断言和超时保留。
- 知识任务已完成完整 ROOT 输入封存，允许本条 fixture 与 PAY 任务 MD/meta 更新；新的 fixture 属于封存后独立差分。五项 peer mechanical 改动仅登记为新副本输入，当前冻结运行源和旧报告不改写。

下一门槛：失败运行 after/资源核对 → 单点 fixture 修正及独立接受 → 修正版四场景 → A01–A12 与完整 writer/事务证据复核。父矩阵仍为 4 PARTIAL、20 NOT_RUN；S3 完整恢复、S4 真实接入、S5 最终候选验收仍开放。

## 2026-11-15 续接推进

本轮用户要求列出推进计划并继续现有 PAY-241；已重新绑定此 `in_progress` 子任务，不另建任务。完整依赖、验收门槛及分工见 `docs/plans/2026-11-15-pay-241-continuation.md`。本轮不执行 Git 提交或生产操作，保留既有授权与安全边界。

- 唯一服务端实施代理接续仓库外保留的 2d 草稿，按 checked 2c 到 2d 的最小差分整合；不重写客户端或 E2E 已通过的编写增量，不覆盖他任务共享文件。
- 发现保留的 2d DDL r02 将候选摘要约束修正为已有阶段候选使用的 bare hex64；旧测试库已经应用前一版同名 861/864，不能靠再次 `up` 证明新约束。旧库和失败证据保留，未改账本冒充匹配。
- 主会话从已独立检查的 2c 加两层明确 DDL 覆盖重建 4,949 文件冻结，所有哈希与文件清单匹配。重新构建 migrator，在专用空库完整迁移至 835 条账本、最后 864；结构与精确 ready 账本均成立，零新外键。
- 新建并核对 10 个隔离克隆，分离实施 service/handler/router normal/race、主会话 harness 与独立检查库；配置和 HOME/TMP 在仓库外，仅按明确包使用。当前 DDL 通过七项空库 down/up、半回退、不可变及保留历史拒绝控制；合成行不代表正式交付或部署。
- 已重新核对前端 3,127 输入文件、1,961 链接、1,586 构建产物及 23 外部追踪文件，零差异。该构建复用证明不等于产品联调；四个实际场景仍未执行，等待最终 2d 及匹配 CLI/Host 构建。
- S3 v2 恢复合同及 S4 外部输入作为当前 S2 后续依赖并行研究；不会提前激活或实现其他任务。总矩阵保持 4 PARTIAL、20 NOT_RUN。

私有证据根：`/Users/artisan/.multica/acceptance/pay241-20261115-continuation`。下一门槛：2d 全入口/事务测试与独立审查 → 匹配产品构建 → 四场景与 A01–A12。

状态：in_progress（2026-11-14激活）；产品规则已确认，最终技术规划复核通过（research/plan-review-final.md，阻挡项0）。用户已授权实现、本地提交与归档。

最新交接（2026-11-14 09:53Z）：用户转到新会话；完整状态见`research/new-session-handoff-20261114.md`。服务端与E2E检查代理因Agent Router资源引用错误停止，无本任务在途测试进程；六个S2包数据库连接审计为0。2b已有冻结实现与检查报告，但**S2B-C01/P1：T4/QA可选报告引用遗漏制品级读取ACL**尚未修复，是续接首要事项。2c合同已冻结，仅有DTO/序列化草稿，未进入执行账本深度实现。E2E第二批编写和独立局部修正已完成，实际场景执行0。不要把旧“实施中”摘要或代理报错本身当作最新产品完成/失败结论。

续接核对（2026-11-14 10:00Z）：新会话 `01a1d97d-7768-7f30-871f-87c43c99fb71` 已绑定现有 S2，无新建任务。HEAD 仍为 `8e824b303`，623 项未提交状态，index 为空；旧 PAY-241 主会话最后事件为交接完成，当前原生代理树无旧实施代理。六个包数据库连接均为 0，证据为仓库外 `new-session-db-audit-2.json`；初始工作区文件清单为 `resume2-workspace-manifest.json`（派发后、owner 加载上下文期间采样，不能替代各 owner 的编辑前字节）。唯一服务端 owner `/root/s2_server_resume` 先修 S2B-C01、冻结并独立复查后续接 2c→2d；客户端 `/root/s2_client_resume` 按已冻结 §8.10 并行完成 3c。实码另有旧会话未验证的制品 ACL 测试草稿与 runtime fixture setup 草稿，先保留原始字节再验证，不继承冻结 2b 的通过结论。联合 E2E 实际执行仍为 0。

最新增量（2026-11-14 10:34Z）：**S2B-C01/P1 已通过独立复查并关闭**，见 `research/server-increment-2b-acl-fix-results.{md,json}` 和 `research/server-increment-2b-acl-fix-check.{md,json}`。原始五项错误放行与夹具失败均保留；14 文件显式覆盖重建为 4,894 文件，独立 service23、ACL race13、handler44 及 vet/build/gofmt 通过。主会话核对实施/检查报告及原始日志哈希，并将历史制品权限、空页与 RR 权限锁合同写入 stage 规范。原服务端 owner 已接续 2c，先交 DDL/sqlc/可编译冻结再由主会话迁移。客户端 3c 的 16 文件、14 对日志/退出码已冻结并经主会话核对，Core400/Views54、类型/lint 通过；`/root/s2_client_check_resume` 正在独立检查。`/root/s2_e2e_resume` 接续缺失发布/回滚/完成门禁场景编写，实际运行仍等 2d 及客户端稳定。政策启用保持 false，父矩阵不变。

后续状态（2026-11-14，本次会话持续推进）：

- 客户端 3c 独立通过，C3C-C01（重复 intent/approval 的矛盾 fact）已修复；最终 Core403/Views54。3d 已完成 29 文件、2,532 文件隔离源码与 152 链接冻结，Core544/Views110、类型/lint 通过，独立检查正在修复已复现的 C3D-C01/C02（旧路由无 code 的 404/405 不应清来源权限；候选分页必须核请求 limit/cursor）。所有原始失败保留。
- 服务端 2c 已实现并冻结纯 S2 的 63 文件覆盖/4,938 文件完整源码，六包 normal/race 及静态检查通过；新增原 issuer 制品读取权限回归已红→绿。独立检查保留相同绿灯，并实证两项 P2：S2C-C01 兜底收尾重新起算 5 秒预算，S2C-C02 release 的传递批准读取状态未跟随原 reviewer 撤权。原执行消费已正确拒绝且零调用；唯一 server owner 正做两项窄修复，之后独立复查，尚未进入 2d 实施。见 `research/server-increment-2c-results.*`、`server-increment-2c-check.*`。
- 2c 迁移已由主会话应用到 8 个自有数据库，831 个账本版本至 843、13 新索引及实际 schema readiness 验证通过。专用迁移库空 down/up、843 及直接 829 的保留历史拒绝均通过，保留一条明确的合成迁移探针 intent。原八个 F2 清理注册失败不变。见 `research/server-increment-2c-migration-results.*`。
- 已冻结 F06 协作/R38候选与 completion-preview、安全 activity 合同：`research/server-increment-2d-client-contract.md`、wire13.18；接口实现仍随 2d，客户端可以据此并行。E2E3 主体独立通过，E3-C01 USTAR 数字字段夹具问题已修，11 纯测试/静态检查通过。原 E2E owner 在增量 4 改接真实共享 M1 表单、跨 Web/Desktop 的同原 grant/operation-ID 消费与恢复；实际四个场景仍均未运行。
- 私有 JSONL/socket 控制桥经独立 26 项纯测试检查通过，未运行实际 harness。root 后续 shutdown ACK 疑点被额外纯探针否证，桥代码未再改动。已在 `/Users/artisan/.multica/acceptance/pay241-s2-20261114-r01/allocation.json` 分配短私有 socket、固定 0500 staging 根及八个 0700 槽位、0700 output；未启动 API/浏览器。
- 新的并行任务 `11-14-project-knowledge-automation` 占用 844–859 及部分共享 server/Core/Views 文件。保留其代码与混合图失败；S2 基于已检查源加明确 owned overlay/hunk 隔离重建，不运行其迁移，不将其差异混入 S2 提交。2d 新迁移编号须再次查看最新占用。HEAD 仍以实际核对为准，主会话未提交。

最新运行级状态持续写在仓库外 `pay241-20261114-resume/resume2-coordinator-state.json`。这个状态记录不替代各增量冻结报告；2b 通过不关闭 S2，2c/2d、联合 A01–A12、S3/S4/S5仍按依赖继续。

续接更新（2026-11-14 14:28Z，以上各时点记录保留）：

- 服务端 2c 两项 P2 已由同一 owner 修复并独立关闭，见 `research/server-increment-2c-followup-check.{md,json}`。最终基线为 4,939 文件；独立 normal15/race15、vet/gofmt 通过。首次 followup race 的 `55P03 github_installation` 仍按未知原因保留；后续单例、完整49项及独立重跑通过不抹去原失败。
- 客户端 3d 已独立通过，C3D-C01/C02 修复保留；最终 Core550/Views112、类型/lint 通过。见 `research/client-increment-3d-check.{md,json}`。跨 source issuer/completion consumer 使用既有授权 API 与消费端引用表单；本增量没有新建独立 issuer 工作台。
- E2E4 最终编写独立通过，13项纯夹具、隔离类型检查、lint及4场景发现通过；2,552源码文件/638类型依赖核对无漂移。实际 API/Web/Desktop 场景仍为0。见 `research/joint-e2e-increment-4-check.{md,json}`。A09还须服务端 self/R38 成功、跨issuer/consumer、三种M1模式和最终时钟的真实writer证据。
- 服务端 owner `/root/s2_server_resume` 开始2d，先做提交候选持久绑定及同事务最终门禁，再项目逃逸/重开、F06/activity/readiness；政策启用仍false。并行任务迁移已占844–860，后续号码在创建前复核。
- root已核对HEAD未变、index为空，平台Web/Desktop及构建配置无本地差异；以HEAD平台文件加checked E2E4/Core/Views构建3,127文件的隔离前端副本，1,961依赖链接仅指向隔离workspace或现有pnpm包。生产Web构建进行中，尚无构建通过结论；Desktop CLI等2d冻结后构建。私有绑定在 `pay241-s2-20261114-r01/frontend-build-r01/input-binding.json`。
- `/root/s3_recovery_prepare` 只研究现有S3后续恢复与阻塞四字段，不修改共享writer；`/root/s2_final_matrix` 只核对最终A01–A12完整分母和真实入口证据，不提前运行或关闭验收。

## 1. 事实与规划

- [x] 加载 AGENTS/CLAUDE、Trellis 和父任务全部输入。
- [x] 记录初始 HEAD、149项 tracked/226项 untracked 差异，限定本阶段文件。
- [x] 原生研究代理核查服务端、客户端、测试环境和邻接任务责任。
- [x] 创建 S2 子任务，起草 PRD/design/implement，并集中询问关键产品规则。
- [x] 收到两次规则答复，收敛PRD与wire/data合同；保留人工手建，禁止自评且QA/发布分离。
- [x] 独立 `trellis-check` 做规划评审，处理阻挡项；配置真实 implement/check jsonl。
- [x] `task.py start` 激活实施；服务端首增量（wire-contract + 协议/策略/角色/建议家族/child准入）已派发原生trellis-implement。

## 2. 基线与环境

- [x] 新建 `/Users/artisan/.multica/acceptance/pay241-20261113-01`，与代码仓库分离。
- [x] 创建 `multica_pay241_20261113_01_s2_baseline_service` / `_handler`，实际数据库身份校验与迁移；配置仅留0600私有JSON。
- [x] 运行原协作交付、M1、当前授权、原键恢复基线；service 50/handler 31 named通过，无跳过，见research/baseline-results.json。
- [x] 为新用例及race分别分配service/handler四个独立数据库，记录允许的package/test；主会话已迁移至828，子代理不得切库、迁移或访问旧业务DB。
- [x] 本次续接另建 `s2_2c_router_r01`、`s2_2c_router_race_r01`、`s2_2c_harness_r01`、`s2_2c_migrate_r01` 四个独立配置/数据库，分别用于 router normal、router race、主会话控制的实际 router harness、迁移验证。使用已核对的 2b 冻结 DDL/二进制从空库迁移，816 个账本版本完整匹配至 828，17 个 S2 索引 valid/ready/live，未复用运行中 API 或包数据库。证据在仓库外 `s2-2c-verification-allocation.json`、`s2-inc2c-baseline-migrations.json`、`s2-inc2c-baseline-all-stage-index-audit.json`。当前仅完成基线准备，后续 2c DDL 仍须冻结后由主会话迁移；不代表已执行 router/harness 验收。

## 3. 实施文件归属与顺序

主会话只负责计划/规范、隔离环境、集成、证据、本地提交和归档。产品与测试由原生 `trellis-implement` 执行；其他会话的未提交改动不得回退。

| Lane | 独占文件/职责 | 依赖 |
| --- | --- | --- |
| 服务端 | 新 `server/internal/collaboration` 固定阶段类型/规则；新 `service/collaboration_stage*`；SQL/迁移/sqlc生成；`collaboration_workflow.go`、`collaboration_handoff.go`、`collaboration_lifecycle.go`、`delivery_completion*.go`、`issue.go` 必要事务接线；新 handler 与 router 接线；同层测试 | 最终阶段/角色与协议设计；共享事务文件由同一代理持有 |
| 客户端 | 新 `packages/core/collaboration/stage-*`；`api/client.ts`、`collaboration/index.ts`；新 shared Views 面板/表单与 detail 挂载；双语文案与同层测试 | 服务端 DTO 已冻结；不得改服务端或平台路由 |
| 联合验收 | stage canonical E2E、已有 closure 必要接线；fixture 环境与证据结构 | 后端/客户端集成可编译；不与前两lane共享产品文件 |

每一lane按行为红→绿并保存首个失败。报告CLI已按 `research/test-report-contract.md` 确定新增subject/submit/receipt，由服务端owner同步完成protocol、runtime/claim、HTTP/router、run-local guard/帮助及builtin说明。已有API/CLI完成路径必须被同一服务端门禁拒绝，不为证明拒绝复制一套完成工作流。

服务端首增量先交付wire-contract：逐动作列出design中的判别请求/最小确认、方法/路径、当前真人权限、版本/签字到期边界、成功与拒绝/进行中/404；项目政策单列原键修订恢复，issue动作复用原workflow操作。客户端以此冻结合同开始，不能提前按全部字段必填的统一回执编码。此接口交付由同一实现owner执行，不增加用户确认步骤；未完成所有真实writer门禁前不开放政策启用。

## 4. 检查顺序

1. 服务端定向单测、HTTP入口与事务回归；按design末尾实际入口表覆盖所有done与项目脱离/删除、人工child控制组、未批准机器拆解、角色撤权、同键/换键重放。包括child语义版本和正常状态变化分离、完成before→after、root重开/配置变化与enqueue/Start两个顺序。
2. 阶段服务/handler关键竞争用例 `-race`；迁移真实up/down/保留历史与独立并发索引检查。
3. Core协议/恢复、Views接线；`pnpm typecheck --concurrency=1`、`pnpm lint --concurrency=1`、相关 tests，`go vet` 和编译静态检查。
4. 冻结源码副本与依赖边；构建当前API/CLI、production Web、Desktop renderer/preload和受控Provider。浏览器库独立。
5. 三个人类测试会话验证可用/拒绝/版本变化/恢复；补管理员/独立code reviewer/release会话以覆盖角色分离。模拟人签字需注明。
6. 宿主fixture使用host gate；namespace fixture使用host=false；串行切换并按本次分配原值恢复，epochs保持单调。
7. `trellis-check` 独立完整差异/证据审阅。发现实质问题返回对应lane；只在新变化或未决问题要求时重跑扩测。

## 5. 汇总与提交

- [ ] A01–A12逐条记录结果、命令、来源版本、环境、actor、原始证据及限制。
- [ ] 更新父矩阵/roles-and-states/缺口；保留原24例和旧历史列。S2局部断言不冒充整条PASS。
- [ ] 更新确有变化的spec，避开已有dirty规范或仅提交本阶段独占新文件。
- [ ] 实际核验 API/Web/Desktop 与源码哈希，停止本阶段拥有的进程，保留DB/失败证据并恢复gate。
- [ ] 仅用明确文件清单暂存，Lore格式本地提交，独立检查通过后归档S2。
- [ ] 继续S3→S4→S5；父任务维持开放，不能因S2关闭而结项。

回滚点：规划未完成不进入实施；实现红灯不关闭阶段；数据库保留审计，不用删除历史或重置他人文件回滚。真实资金、生产发布、真实模型账号均不在当前授权执行范围。

## 独立审阅的实施增量与必改接线

1. 协议/策略/角色/建议家族及全部child准入。未齐全部门禁的中间代码不可提供可启用政策入口。
2. typed证据/签字人代际、stage-only final guard、submit/done/project detach/delete接线；M1 off/observe/enforce矩阵与当前DB时间到期竞争。
3. Core/Views阶段操作、M1协作typed完成消费端（明确本包承接）、GET-only stage恢复与activity/通知投影；两端真实合成合同验证。

后两项已加入明确归属：服务端lane同时拥有completion preview/真实writer final guard与activity投影；客户端lane同时拥有普通/协作区分的M1 schema和stage_confirmation恢复、Query失效与双语通知。既有普通M1行为继续回归保护。

F01–F06的拟定技术收敛已写design，须独立复核；用户角色规则已确认；最后技术复核通过即激活S2实施。S3独立受控timeout测试子增量已经通过独立规划审查并实施，不代表S2通过。


## 2026-11-14 接管与推进决定

- 原协调会话 -25 的 S2 服务端/客户端 lane 于 00:48Z 因模型错误中止，未写 1b 报告。-45 于 01:53Z 接手，重派的两个 lane 在首个工具调用前遇到 429，未改动文件，随后 -45 退出。02:05Z 起由会话 -3a 负责 S2；起点 HEAD `fb9f4c070`，合同 sha256 `ced0cd66…`。
- 用户指令：服务端在合同第 13 节补上 A5 `code` 后放开 A5，然后跑 A01–A12。核对结果：第 13 节第 3–5 条已记录 `code`，但第 2 批（A6–A9、R4、T1–T4、10.2 完成/移出门禁、P2 启用）尚未提供服务，A03–A07、A09、A11 无法验收。用户选择**先实现第 2、3 批，再一次性跑完整 A01–A12**，不做中间状态盘点。
- 执行顺序：服务端 1b 收尾 ∥ 客户端放开 A5 → 服务端第 2 批（按 2a 证据/A6/R4/代码评审/A8、2b 测试报告与 QA、2c 发布/回滚与执行事实、2d 完成/移出门禁、P2 启用、activity 投影的顺序串行，由单一 owner 负责）∥ 客户端第 3 批（按冻结的第 8、9、10.2 节）→ 联合 E2E 与生产 Web/Desktop → A01–A12 → 独立 trellis-check。

本阶段实现owner划分：服务端一个trellis-implement持有全部server文件与迁移/sqlc/报告CLI/事务接线，先输出wire-contract供客户端并行；客户端一个trellis-implement持有core/views/locales共享接线与测试，不碰server。E2E在接口稳定后单独交接或由客户端owner增加；主会话负责DB分配、构建和实测。实际文件列表在agent实施结果中记录，任何超出已确认业务范围的必要变更先报告。

### 本次续接占用核对（2026-11-14 02:30Z）

- Codex会话 `01a1d7db-fa02-7b61-87a7-f9d4d4a22338` 绑定现有S2任务，未新建任务。起点HEAD `fb9f4c070`，同分支488项未提交改动；创建页面、文档等其他任务差异不纳入S2提交。
- 原协调 `dad148bc` 的1b/A5代理于02:21Z因额度错误停止，主会话随后被用户中断；两个新results文件均只是草稿。确认这一状态后才由原生 `trellis-implement` `/root/s2_server`、`/root/s2_client` 续接。服务端保持唯一owner，先完成1b报告与针对性验证，再顺序2a–2d；客户端先完成A5再第3批。
- S5原协调 `22c65976` 于02:23Z也因额度错误停止，未发现心跳补丁。r01恢复报告及原始样本保留，S5另行续接，不重复已有运行。
- 本次状态/源码采样记录位于仓库外 `/Users/artisan/.multica/acceptance/pay241-20261114-resume`。这次采样发生在派发之后，不替代此前 `research/initial-worktree.json` 的原始基线。
- 并行创建页会话随后独立提交 `482cd7f683739aca72050b6b09fdfe724742a898`，明确未含stage/Core/locales路径；本阶段源码仍是该HEAD加S2未提交覆盖，不能把它当S2已提交。
- 客户端A5收尾已完成并验证：`research/client-a5-release-results.{md,json}`，14个变更文件与9份命令日志哈希均由主会话复核；Core232/Views102通过、类型检查与stage lint通过。A5暂停解除，接续客户端3a。真实API/Web/Desktop及完整S2独立检查仍未进行。

### 2026-11-14 05:55Z 增量核对与接续

- HEAD为`8e824b303`，S5工具提交不包含S2产品代码；工作区仍有555项并行差异，当前index为空。当前任务指针仍为S2。
- 服务端1b、2a均有正式results。2a的45个文件、34份命令日志/退出码、4份主会话迁移记录已核对哈希，两项静态检查由主会话重跑通过。冻结副本与核对记录位于仓库外`pay241-20261114-resume/s2-2a-source`及`s2-2a-coordinator-hash-check.json`。独立`/root/s2_server_check`只读审阅此副本，不占用实施文件。
- 唯一服务端owner`/root/s2_server`接续2b；先冻结T4及create-only评论test_subject供客户端接入，再完成报告CLI、实际command/首次claim绑定及QA审批。所有迁移仍由主会话执行，政策启用保持false。
- 客户端3a独立复查关闭C01–C08；Core292、Views127及C08受影响30项通过，见`research/client-increment-3a-recheck.{md,json}`。`/root/s2_client`正在3b1，限于现有完成/管理/项目退出的阶段错误反馈；T4尚未冻结，不猜接口。
- 主会话另分配`multica_pay241_20261114_s2_ghsnapshot_r01`与`multica_pay241_20261114_s2_migrate_r01`，分别只供ghsnapshot、cmd/migrate验证；均已从空库迁移至825。新外部HOME/TMP通过Git-root拒绝探针。数据库测试与实际降级保护的结果另行记录，当前不宣称通过。
- A01–A12及production Web/匹配Desktop联合验收仍未执行。完整父矩阵状态保持4 PARTIAL、20 NOT_RUN；局部测试数不作为验收完成比例。
- 2a补充验证完成：ghsnapshot全包race为24个顶层/42个named通过；实际821–825迁移空库down/up通过，留存代码候选时在825拒绝且索引/账本不变。现有迁移注册检查失败，发现16个S2索引缺少重试cleanup注册，另8个F2缺项独立保留。见`research/server-increment-2a-supplemental-results.{md,json}`。
- 2a独立检查完成但尚未通过：S2A-C01(P1)迁移重试注册、S2A-C02(P2)私有child描述摘要字典推断均已交原server owner修复；V01为最终消费前审批到期的待复现竞争窗口，不能直接当第三个已证实缺陷。原2a报告保留，修复需新证据。见`research/server-increment-2a-check.{md,json}`。
- `/root/s2_e2e`仅新增阶段验收spec/fixture并做静态检查，复用现有Web/Electron测试设施；不启动API/浏览器，不提前运行A01–A12，不修改server/Core/Views。完整运行仍等待2d及客户端合同稳定。
- 服务端已冻结wire §§8.8/8.9及13.14–15（SHA256 `a71d71570cf1f337d0f8c299e77e307e6d37516dd516eaa6e0882548c5669520`）：T4分页/详情/用例/日志、create-only typed评论test_subject、source-only artifact_digest:null，以及source-denied child digest:null。客户端据此开始privacy+3b2。
- 客户端3b1报告完成：Core365/Views136通过、类型检查通过、lint零错误及项目页两条既有warning。主会话核对冻结副本482个文件、24个增量文件、19份日志/退出码及4个未改保护文件，均匹配；原生独立检查待空闲检查lane接续。见`research/client-increment-3b1-results.{md,json}`及仓库外`s2-client3b1-coordinator-hash-check.json`。
- V01后续已由实际service测试中的最终DB-time seam复现：原批准/原签字人到期两个分支仍推进，调用者到期控制组正确拒绝。首红日志`s2-server-inc2b-followups-red-1.log`保留；服务端owner继续修复及normal/race检查，不能据此修改旧2a检查时点的结论。

### 2026-11-14 迁移与E2E编写完成后的状态

- 2a三项修复已有独立的`research/server-increment-2a-followup-results.{md,json}`：阶段service90、关键race5、HTTP10、S2清理注册1项通过，无skip。主会话核对11个文件与6份日志/退出码，冻结到仓库外`s2-2a-followup-source`；原生独立复查仍待执行，不改旧2a报告。
- 2b的826–828迁移与sqlc已生成，全后端编译通过；主会话核对六份DDL并从各自825基线顺序应用到四个数据库，唯一command索引结构、valid/ready/live/unique及无FK均核实。证据：仓库外`s2-inc2b-ddl-before.json`、`s2-inc2b-migrations.json`。新的迁移binary为`s2-migrate-inc2b-828`；其执行只使用冻结的826–828文件目录。
- 客户端privacy+3b2进行中。测试评论使用既有受保护mention-targets的`can_submit_commands`和comment/v6/当前stage/review事实；不放开全局直接命令runtime门禁。`/root/s2_client_check`已对3b1冻结副本开始只读独立检查，缺失的纯依赖如需补充，必须单独记录来源/哈希，不能把当前3b2全包当历史3b1重跑。
- 首批E2E编写完成，两个新文件及11份检查日志由主会话核对；类型/lint/发现检查通过，测试体执行0。见`research/joint-e2e-increment-1-results.{md,json}`。3b2已改变其部分UI依赖，实跑必须重新绑定最终源码；不得沿用静态检查时的依赖哈希宣称联合验收。
- E2E仅为本次公开注册的精确新账号补合成`@example.invalid`邮箱（原邮箱必须NULL，匹配ID/用户名，事务内校验自有DB），其余邀请/移除/重入仍走真实API。主会话启动验收API时必须明确清空`RESEND_API_KEY`与`SMTP_HOST`，核对邮件服务选择本地日志分支；这不是公开邮箱设置流程的验收。

### 2a复查收尾、3b2检查与A05编写

- 2a后续复查补齐三份原先漏列的nullable-digest测试文件，保留首次独立重建失败并添加`server-increment-2a-followup-fixture-supplement.json`；重新构建/vet通过。复查随后发现A5只判断来源成员快照但未锁成员行，真实第二连接DELETE与确认可同时提交。服务端通过既有source-member动作allowlist修复，验证scope-first、revoke-first及empty external控制组，normal/race与独立复查均通过。最新结论在`research/server-increment-2a-source-fence-check.{md,json}`，旧报告不回写。
- 3b1独立检查通过（另跑173项冻结schema测试）。3b2独立检查修复B01：当前R1拒绝必须同步清除/隔离旧R4/T4，禁止旧私有证据维持C1/QA可用；保留草稿和原键。最终Core70/Views64及类型/lint通过；原477/63不与重跑相加。主会话核对496快照文件、34个最终范围文件、24份实现与10份检查日志，零差异。见`research/client-increment-3b2-check.{md,json}`。
- `/root/s2_e2e`编写A05第二批；授权最小扩展现有`collaboration-host-runtime.ts`和`collaboration-host-provider.go`及其测试，增加可选Git/test-report动作与默认成功不变的失败控制。同一服务端owner负责`collaboration_host_router_test.go`及相邻边界测试中的可选Git输入，使用既有HostConfig.Git，在capability注册前验证，ready仅输出安全摘要。所有原超时保持不变，普通fixture行为保持回归；不使用synthetic preparation rows证明正路径。
- 主会话已提供不带DB/账号环境的纯测试launcher：仓库外`run-pure-s2.mjs`用于protocol/CLI/command三包；另一个`run-pure-s2-host.mjs`仅允许`./internal/daemon -run ^TestCollaborationHostHarness`与`./internal/daemon/collaborationrun -run ^TestHostBrief`。各包独立外部HOME/TMP；HostBrief测试会启动显式测试Provider，不能称为只有静态检查。旧launcher字节保留，避免改变既有CLI证据。
- 2c技术准备已写`research/server-increment-2c-contract-preparation.{md,json}`，正在独立评审，尚未冻结或实施。主会话选择保留当前有效批准门禁：迟到事实可存历史，但不自动续期、重新调用或推进；不增加既有部署重新背书语义。实际router测试harness与生产默认无适配器的区别须保留。
- 已通过trellis-update-spec新增`.trellis/spec/server/collaboration-stage-approvals.md`并加server/Core索引链接，记录成员锁、最终DB时间、R1→私有缓存、迁移重试及证据清单要求。新规范结构/链接/空白检查通过；server索引含原有他任务差异，提交须只选本阶段新增hunk。
