# main 合并后完整端到端测试计划

日期：2026-11-24。基线：`main@103e2bc32`。任务：`11-24-main-post-merge-e2e-acceptance`。

## 测试目标与原则

验证这次合并后的实际用户链路，找出回归、权限或一致性问题，并让每个结论都能追溯到本次运行。先检查已有测试和环境，再按本计划执行；产品源码与现有断言保持为基线，必要的测试编排和诊断只放在本任务私有目录。

覆盖按三条轴组织：业务领域、场景类型（正常/拒绝/边界/并发/恢复）、运行环境（认证模式/功能开关/客户端）。P0 是数据隔离、安全、核心生命周期与执行控制；P1 是主要用户功能与近期合并回归；P2 是兼容性、显示与运营辅助。P0/P1 先于大规模压力和安装包扩展。

当前盘点有 187 个顶层 Playwright spec、465 个静态测试声明、122 个条件跳过调用点。本次只读 collector 已展开为 538 个用例（468 expected-pass、70 声明跳过、0 收集错误），没有运行测试体。运行时仍可能增加条件跳过；每次实际 lane 保存自己的 collection，以具体测试身份对账，复跑不增加覆盖量。

## 业务与风险覆盖矩阵

每行的各类场景均进入结果账本；已有用例优先，缺少自动化的场景使用任务私有探针或明确记为缺口。

| ID / 优先级 | 领域 | 正常链路与预期结果 | 拒绝、边界、并发与恢复 | 主要验证入口 |
| --- | --- | --- | --- | --- |
| T01 / P0 | 启动、配置、迁移 | 新库迁移、启动、health/config 与实际源码/监听进程匹配；重复迁移幂等 | 错误配置明确拒绝；旧 ledger 升级、条件迁移和 concurrent index 中断/恢复；不连接默认库 | migration/启动 Go 测试、script regressions、实际 API readiness |
| T02 / P0 | 认证与账号 | 实际邮箱验证码、密码注册/登录、设备认证、问卷/跳过、刷新/退出、账号绑定；正确CSRF cookie/token可写 | 无效/过期验证码、错误密码、限流/cooldown、会话过期、旧 401、退出失败、需要改密/完成资料、跨 Server 身份清理；缺少/错误CSRF和跨Origin HTTP/WS拒绝且无副作用 | `auth*`、`password-*`、`onboarding-smoke`、`main-supplemental-qa`、auth Go/TS |
| T03 / P0 | 工作区、组织、成员 | 新建第二工作区、切换、邀请/加入、角色变化、退出/删除和默认工作区恢复 | 非成员和跨租户拒绝；重复 slug、空工作区、邀请失效、撤权时打开页面、最后管理员、删除失败不提前导航 | `workspace-*`、`global-scope-navigation`、membership/organization handler |
| T04 / P0 | 任务生命周期 | 创建、编辑、状态/优先级/负责人、父子关系、批处理、看板拖拽、删除；列表与详情一致 | 无权限修改、失效 ID/版本、失败 rollback、部分批处理、重复点击、长 Unicode、千行分页；完成要求和证据门 | `core-g1-*`、`issue-*`、`main-supplemental-qa`、delivery completion Go |
| T05 / P1 | 项目与小队默认 | 项目新建/目标/概览/成员/默认执行小队、任务链接、进展发布/订正、健康与风险 | 项目权限、被撤成员、默认小队失效、并发订正、失败重试、不同工作区同名项目 | `projects-p1-*`、`project-squad-*`、`project-progress-*` |
| T06 / P1 | 迭代 | 创建、计划/活动视图、开始、冻结进度、关闭、carryover、历史与范围审计 | 日期边界、UTC/Shanghai、旧 revision、重复关闭、冻结后修改、部分失败、无权限；跨端计数一致 | `iteration-*`、`iterations` Go、mobile iteration tests |
| T07 / P1 | 筛选、搜索、分页、导航 | 表格/看板/详情切换、筛选/排序/查询、浏览器返回、深链和桌面 tab/history | 空结果、长文本、多页、过期引用、跨工作区路由、账号无工作区、刷新/恢复滚动、查询字段缺失 | `issue-table`、`sidebar-*`、`global-scope-*`、router/schema tests |
| T08 / P0 | 评论、附件、引用 | 人/Agent 评论、表情、编辑/删除、上传/下载、任务/知识引用和通知 | 机器凭据不得借管理员身份改他人评论；恶意内容、下载撤权、重复上传、过大/空文件、失败重试、记录保留 | comment/attachment Go、shared views、相关 browser specs |
| T09 / P1 | Inbox、Triage | 通知未读/已读/归档、分组 snooze/wakeup、审阅接受/拒绝/重复/重开与批处理 | UTC snooze receipt、撤权通知隐私、过期/critical 条目、重复 intent、旧版本、跨设备同步、失败 rollback | `inbox-snooze`、`triage-*`、`core-g2-*`、inbox/notification Go |
| T10 / P1 | Agent、角色、小队 | 创建/模板/能力/负责人/运行时/自主级别、小队组建/发现、项目默认 | 不可用 runtime、失效权限/资源、非人类授权边界、空目录/长列表、模板语言和 provenance、取消 AI assist | `agent-*`、`squad-*`、`autopilot-*`、`actor-*` |
| T11 / P0 | 协作创建与信息 | 账号工作台、来源选择、协作者、草稿、说明/文件、资源、输入历史、分页和恢复 | 跨来源权限、原始 key 丢响应、过期 cursor、字段漂移、撤权后读取/编辑、旧客户端兼容；不得泄露私有内容 | `collaboration-*`、information/content/source Go/TS |
| T12 / P0 | 协作审批与交付 | 人工审批、阶段转换、交付提交/退回/重新接受、自完成策略、子任务/唤醒 | 只有正确当前签署人/发起人可决策；scope/版本/epoch 过期拒绝、并发决策/重复请求、撤权、不可逆结果保留 | collaboration stage/delivery/records/source browser 与 Go race |
| T13 / P0 | 协作执行与控制 | namespace 与 host 分开；受控真实 daemon/provider/CLI 接收、执行、结果/评论/附件写回 | Stop/取消/超时/断线重连、重启恢复、FIFO/容量、晚到输出、原 attempt 归属、错误/无结果、进程清理、自动知识启动 | `collaboration-host-*`、stage/cross-client/runtime-output，受控 fixture binaries |
| T14 / P0 | 知识全生命周期 | 项目知识/协作候选、新建/检索/来源、正式审核、派生/订正/撤回/清理、策略/leases | source/target 独立授权、版本与有效期、撤权/删除、重复审批、original key 恢复、无效引用/文件、launch context 隔离 | `collaboration-knowledge*`、`project-knowledge*`、knowledge handler/service |
| T15 / P0 | 全局权限与隔离 | 同一资源在人/管理员/普通成员/非成员/PAT/Agent/daemon 下符合权限矩阵；合法Origin与cookie+CSRF或机器凭据路径分别成功 | 合法身份的 403/404 与状态无副作用；不能以 malformed 400或fixture缺CSRF证明产品权限拒绝；撤权打开编辑器、跨租户 WS/paging/下载、非法Origin HTTP/WS | credential/capability Go 矩阵＋两账号 live API/浏览器 |
| T16 / P0 | 实时与一致性 | 两个实际客户端看到状态、评论、未读、项目/迭代与协作变化；工作区切换不串数据 | 掉线/缺事件/重连、重复/乱序 revision、缓存与乐观 rollback、账号切换、被撤订阅、停止后迟到写回 | WS Go、`core-g4-*`、collaboration recovery/cross-client、shared query tests |
| T17 / P1 | 设置与资源 | 偏好、语言/主题、技能/MCP 市场与模板、PAT/资源发布、运行时连接和 welcome/recovery | 无权限/失效资源、无匹配、秘密只展示一次、旧枚举/坏响应、取消无写入、Server 切换/重启恢复 | `settings-*`、`mcp-*`、`skill-*`、`runtime-*`、schema tests |
| T18 / P0 | 管理、安装与观测 | 合成管理员账号、资源发布、managed enrollment、runtime control、feature switches、metrics | 非 admin/机器凭据拒绝、会话再认证、失效 enrollment、原 key 恢复、默认关闭、切换并发、隐私投影 | `platform-admin*`、`admin-collaboration-*`、managed/observability Go |
| T19 / P1 | 桌面原生与持久化 | 当前 renderer/preload 的 tab、IPC、登录/账号页、Server 切换、窗口/详情、文件与重启恢复 | 不可达服务器、过期身份、stale tab、关闭/daemon 控制、外部 URL/renderer 文件访问与拖拽区 | native desktop browser specs、desktop main Vitest；生产 Main 单独验证 |
| T20 / P1 | 移动端 | 明确运行独立 typecheck/lint/Vitest，验证任务/项目/迭代/Inbox/知识数据语义与 WS | schema 漂移、缺字段、错误权限、修订顺序、附件重复、时区和计数；原生 UI/前后台/键盘/网络切换需要原生设备 | `apps/mobile` gates；iOS native 条件未满足时保留 BLOCKED |
| T21 / P1 | CLI、daemon、安装/升级 | 当前编译 CLI、fake Agent 协议、daemon 注册/执行/控制、原生 OS 进程边界；当前安装资产来源一致 | 断线/重启/父进程结束、环境污染、路径含空格、权限/配置失效、旧客户端/离线升级；不使用旧包冒充当前源码 | four-path critical smoke、Go agent/daemon、installed-host/Main specs、installer scripts |
| T22 / P2 | 多语言、主题、可访问性 | English/简体中文、明暗、键盘/焦点、窄屏/普通桌面/200% zoom、长内容滚动 | Modal/portal 焦点、禁用/错误/空状态可理解、hover 不掩盖 active、菜单与筛选不误操作 | 已有 visual/layout/browser 场景＋重点链路 captures |
| T23 / P1 | 文档、插件、自托管 | docs 生产构建与主要中英入口、内置指南、plugin iframe/scroll、self-host/offline 配置 | iframe 消毒/跨域/redirect、路径/锚点、配置错误、upgrade preservation、秘密不进日志 | docs tests/build、`plugin-*`、iframe config、self-host/offline regressions |
| T24 / P1 | 性能、规模、清理 | 现有规定的分页/千行、100用户传播/独立写回指标；环境停止后无自有孤儿 | 不能丢失败样本/放宽阈值；连接/磁盘/线程预算；过期资源处理；只停止自有 PID，退出码反映 cleanup 失败 | canonical performance phases、原始 samples、DB/进程/文件归属终审 |
| T25 / P1 | Chat 会话与消息 | 新建/继续会话、选择Actor、消息pending→成功、附件、详情/会话导航、归档/删除；草稿与窗口一致 | 发送失败保留pending并可retry；401/离线/过期Actor、上传失败、取消实际任务、切换账号/工作区/Server时草稿不串、乐观状态与规范server消息对账 | `chat-actor-picker`、`chat-attachments`、`chat-recovery-branches`、core/chat与handler chat tests |

## 运行环境矩阵

| Lane | 实际配置与客户端 | 必须验证 | 不可替代的范围 |
| --- | --- | --- | --- |
| L0 | 当前源码，静态/TS/Go 单包新库，UTC 与本地时区 | 全仓 lint/typecheck/TS、script、Go normal/race/vet、关键时间回归 | 不等同实际用户 E2E |
| L1 | legacy；当前 Go API＋生产 Web＋当前 Electron renderer/preload | 全量默认 collection、核心 G1–G8、人工协作/知识、桌面和普通设置 | SimulatedDaemon 与 fixture Main 单独标记 |
| L2 | password；device/Cloud 关闭；limiter=single；独立账号 | 真密码表单/注册/资料/绑定、密码协作链路和 Server 切换 | 只选兼容 specs，不把要求 legacy 的 beforeAll 放入此 lane |
| L3 | password＋platform admin/managed；合成 admin 与 enrollment | 管理、发布/观测、开关、runtime 控制和知识 context | 与 namespace/host 的 gate 组合分别记录 |
| L4 | 单独 device-auth 部署 | 设备身份/登录与恢复 | password 启动不兼容 device 模式 |
| L5 | 明确 namespace-only allocation；deterministic fixture binaries | 协作 namespace/stage/多账号与原 intent 恢复 | host_execution_enabled 必须 false |
| L6 | 单独 host-enabled password allocation；真实本地受控进程 | daemon/provider/CLI、Stop/超时/重连/结果保留、自动知识 | 证明编排与写回，不代表商用模型推理质量 |
| L7 | 当前生产 Main/打包或安装客户端；私有 profile/home/feed | 完整桌面启动/IPC/daemon生命周期、兼容和受控升级 | 单独构建并绑定；renderer 测试不能替代 |
| L8 | 已安装其他浏览器；聚焦核心流程 | Firefox/WebKit 的 auth/CRUD/附件/导航 smoke | 无对应浏览器时明确 NOT_RUN；不临时添加依赖 |
| L9 | mobile 独立 gates；原生 iOS 条件 | 数据语义与原生移动流程分别记录 | 本机无 Xcode/模拟器，手机宽度 Web 不是 native mobile |
| L10 | 性能专用 allocation；100个合成账号和固定拓扑 | prepare → propagation → writeback、完整采样与清理 | 不混入普通并发环境，不复用旧样本 |

## 执行顺序

1. 固定来源，保存实际 collection、工具版本、已有运行进程、磁盘/PG连接预算；审查本次环境和执行脚本。
2. 先静态和测试编排回归，再 TS/mobile、Go 单包新数据库/race/vet。Go 的默认测试不访问个人已安装 Agent CLI。
3. 构建与启动本次 API、生产 Web、Desktop renderer，检查当前 listener/commit/config 和服务 URL；先 four-path critical smoke，再 L1 全量默认 suite。
4. 根据 L1 的实际跳过清单执行 L2/L3/L4；每种模式配置/账号和数据库明确归属，保持密码限流有效。
5. 分开执行 L5 namespace、L6 host/knowledge/output 和 L7 production Main/installed；原始完整场景零重试，不改断言、不提高超时以掩盖失败。
6. 补充两账号授权、跨客户端恢复、UTC时间和重点可访问性/浏览器覆盖；执行 docs/self-host/installer 的本地可运行入口。
7. 具备规范前置条件时执行 L10 性能，保留所有样本和原阈值；不满足时明确登记阻塞而不是运行不完整样本。
8. 核对 collection 与所有结果身份、最终源码/工具/资产和资源归属，停止自有进程，保留失败数据库与证据，出具发布建议。

## 证据与通过标准

- 每次运行保存：run ID、main SHA/源码状态、实际命令和开始/结束时间、工具版本、认证与开关模式、数据库名和服务/进程归属、构建/资产指纹、完整退出码和命名用例账本。
- 浏览器保存 JSON/HTML 结果和失败 trace/screenshot/相关日志；API/权限记录合法调用者、响应与状态变化；Go 保存原始 JSON events，构建事件与命名 test events 分开解析。
- 结果只允许 PASS / FAIL / SKIP / BLOCKED / NOT_RUN；退出 0 但没有对应断言执行不算 PASS。诊断通过不能覆盖原先失败的完整场景。
- 产品问题、测试代码问题、环境/来源问题分开分类，均保留原证据。修复工作需要单独明确范围；本次以验证和缺陷报告为主。
- 有未解决 P0/P1 失败、来源不匹配、必需链路缺失或资源清理失败时，不给这些范围绿色验收。报告完整性与产品是否通过是两个不同结论。

## 已知前置限制与应对

- 默认 `make check` 的 legacy/开关关闭限制通过独立 lanes 补足，不将条件 skip 视为绿灯。
- `collaboration-runtime-output-allocation.mjs` 当前只允许历史 `11-20-full-e2e-regression-20261120` 的真实 active/archive task 身份。相关专门 subrun 可使用它的实际归档路径创建全新的当前源码/资源 allocation，并关联本任务；不伪造 metadata、不修改历史 evidence。若无法满足全部守卫，则该链路 BLOCKED。
- 本机缺完整 Xcode/iOS模拟器；原生移动端保留缺口。Windows安装、签名/升级与真实商用模型也必须分别有实际运行依据，不能继承历史报告。
- `psql` 已安装于 `/opt/homebrew/opt/libpq/bin`，仅为本次子进程补充 PATH，无需安装新依赖。
- Go包/浏览器数据库分离；HOME/TMP/GOTMPDIR 使用 Git 外可信祖先链，API pool/publisher配置不进入普通 Go 测试；先读连接预算再安排并发，不调整共享数据库参数。
- macOS完整production Main会注册系统 `multica://` 协议；HOME/userData并不能隔离LaunchServices。L7启动前保存NSWorkspace原默认app的精确URL及同bundle既有注册URL集合，只使用task-owned稳定安装路径；结束后只注销本次新路径并恢复/复核原默认与原集合。未能证明可恢复或来源归属时，不启动该native subrun，记为BLOCKED。

## 结果交付

`report.md` 给出结论、按 lane 的实际用例计数、T01–T25覆盖状态、缺陷严重度与证据、条件 skip/未运行清单、变更文件和自有资源清理情况。独立产物位于本次新建 QA 目录；研究依据位于本任务 `research/`。
