# 核心功能与近期合入代码测试计划

## 本轮目标与固定范围

对 `codex/cross-workspace-collaboration` 的核心业务、整条功能分支与近期修复开展可复现的功能回归。首个候选为 `7af9ab9905c98c77bb091223ebe687c639475bb4`，主干比较基线为本地 `main` / `origin/main` 的共同 merge-base `dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`。本轮未 fetch；若开始执行时 HEAD 改变，重新登记候选，不能混用结果。

整条分支有 446 个提交、7,697 条变更路径，其中大量是验收材料；旧验收 `822c15785` 后有 128 个提交、1,016 条变更路径。按产品领域和行为建立覆盖，不把文件数量当作功能覆盖率。

2026-11-02 已成功收集当前 Playwright 清单：**439 项 / 148 个文件**。默认项目仅 Chromium，收集成功不代表测试通过。证据：`research/playwright-inventory.txt`、`research/playwright-inventory-run.json`。

## 测试批次与顺序

| 批次 | 内容 | 优先级与完成条件 | 依赖 |
| --- | --- | --- | --- |
| B0 | 固定提交、静态清单、隔离环境与连接预算 | P0：确认 API/Web/DB/构件属于本任务，记录所有不满足前置 | 已完成只读预检和 E2E 收集；执行环境待建立 |
| B1 | lint、typecheck、TS 全量、脚本、Go build/vet/race、独立 Mobile 检查 | P0：保存每段退出码、实际通过/失败/跳过与原始日志 | 规划审阅通过；Go DB 必须按包隔离 |
| B2 | 生产 Web 下的核心业务及 legacy 完整回归 | P0：当前 439 项各有明确归属；条件项另列，不计为通过 | B0；生产 Web；测试专属账号/数据 |
| B3 | password/admin/device、跨账号协作、Host、真实 Electron、知识开关两态 | P0/P1：补齐 B2 的条件用例，重点验证近期修复与旧失败 | 各自独立配置/数据库；与 B2 串行使用共享 PG 预算 |
| B4 | 安装升级、平台、兼容与人工探索 | P1/P2：可执行项完成；缺平台/构件/账号项逐项 BLOCKED | 对应平台、同源构件与授权；不能用交叉编译替代 |
| B5 | 独立复核、覆盖对账、结论及会话交接 | 全部结果能追溯版本与证据；剩余项有恢复步骤 | 前述结果齐备或明确记录阻塞 |

B1 通过后执行 B2/B3；某段失败时保留现场并继续不依赖该段的安全检查。不能把串行总入口在早期退出后未运行的后续阶段算作通过。B2/B3 的服务只能串行启动，原生窗口测试也按串行执行。

## 功能覆盖矩阵

每行均验证正常路径、拒绝/异常路径、刷新/重启后的持久化，以及相邻视图的一致性。测试身份以完整标题和参数为准，详细清单见已收集库存。

| ID / 优先级 | 核心领域 | 关键可观察结果 | 现有主要入口 |
| --- | --- | --- | --- |
| F01 / P0 | 登录、会话、账号 | 验证码/密码/设备模式正确；退出和撤销后拒绝旧会话；改密、绑定、切换服务器不串号 | `auth*.spec.ts`、`password*.spec.ts`、`missing-user-session.spec.ts` |
| F02 / P0 | 工作空间、成员与隔离 | 创建/切换/邀请接受拒绝撤销；角色限制；移除后实时撤权；外部任务 ID 不能越过 workspace 过滤；删除失败保留原数据 | `workspace-access-branches.spec.ts`、`workspace*.spec.ts`、`global-scope-navigation.spec.ts` |
| F03 / P0 | 任务生命周期 | 创建、字段编辑、负责人、状态/优先级、标签/日期/附件/评论、批量操作和确认删除持久；失败回滚/草稿保留/原键重试不重复 | `task-lifecycle-qa.spec.ts`、`issue-mutation-branches.spec.ts`、`issue-table.spec.ts`、`issues.spec.ts` |
| F04 / P0 | 项目与迭代 | 创建、范围、归档/完成；进度和风险口径一致；验收关联正确目标版本；并发修正保留作者和原文；历史分页完整 | `projects-p1-*.spec.ts`、`iterations-*.spec.ts`、`progress-reporting.spec.ts` |
| F05 / P1 | 分诊、搜索与导航 | 候选转正式任务/去重/恢复；隐藏字段和过滤保留；搜索、链接、历史与返回正确；窄屏可操作 | `triage*.spec.ts`、`navigation.spec.ts`、`global-scope-navigation.spec.ts`、`iteration-scope-business*.spec.ts` |
| F06 / P0 | 跨账号/跨工作空间协作 | 创建者/处理者/参与者各自权限；精确收件人查询；接受、转交、补充、交付、验收、退出；私有内容不泄露；同键重试只产生一次结果 | `collaboration-workbench-web.spec.ts`、`collaboration-recipient-picker-web.spec.ts`、`collaboration-delivery-web.spec.ts`、`collaboration-leave-web.spec.ts`、`collaboration-source-surface.spec.ts` |
| F07 / P0 | 智能体、小队与执行 | 创建/分配/发现、权限与容量；A/B 独立执行域；FIFO/合并输入；停止/重启后保持暂停、显式恢复；撤权与开关关闭不启动新执行；结果准确回写 | `agent*.spec.ts`、`builtin-agent-concurrency.spec.ts`、`collaboration-cross-client-execution.spec.ts`、`collaboration-host-runtime.spec.ts`、`collaboration-lifecycle-controls.spec.ts` |
| F08 / P0 | 交接、协作上下文、召回 | 启动版本与当前版本分别展示；并发编辑/版本恢复；截断内容和旧评论可召回；来源撤权立即拒绝；冻结来源不伪称 live；同键丢响应恢复 | `collaboration-handoff-chain-*.spec.ts`、`collaboration-runtime-handoff-*.spec.ts`、`collaboration-memory-*.spec.ts`、`collaboration-preparation-records-*.spec.ts` |
| F09 / P0 | 知识审批与使用 | 来源授权；人工批准精确版本；纠错替换下一次真实 Host 输入；撤回/清除/到期不能继续使用；临时失败重排；开关关闭/开启都符合契约 | `collaboration-knowledge.spec.ts`；Go `internal/service`、`internal/daemon`、`pkg/agent` 对应专项；历史 RKI6 证据仅作风险索引 |
| F10 / P0 | 通知、实时与多会话 | 离线通知、去重/已读、深链、重连补偿；撤权后不得发送敏感内容；多 Web/Electron 会话状态一致且不串工作空间 | `collaboration-mention-journeys-*.spec.ts`、`collaboration-offline-*.spec.ts`、`collaboration-records-web.spec.ts`、`collaboration-account-desktop.spec.ts` |
| F11 / P1 | 聊天、文件与故障恢复 | 正常发送/上传；重复快捷键只发一次；失败草稿/待发送保留并可重试；附件权限、原始字节、删除/保留语义一致 | `chat-*.spec.ts`、`collaboration-retained-attachment.spec.ts`、`collaboration-selection-boundary.spec.ts` |
| F12 / P1 | skills、MCP、能力市场与资源 | 发现/导入/发布/授权/撤回；配置验证、密钥不回显；原生与 Web 可消费；复用现有能力但不误启动实际供应商 | `capability-market.spec.ts`、`agent-mcp.spec.ts`、`mcp-*.spec.ts`、`admin-resource-publishing.spec.ts`、`collaboration-resource*.spec.ts` |
| F13 / P0 | 自动化与管理 | 模板配置、触发与停用；管理员身份/重新认证；执行筛选；创建/执行/Host 三开关及 epoch；更新不破坏历史 | `autopilot-*.spec.ts`、`admin-collaboration-switches.spec.ts`、`platform-admin-controls.spec.ts`、`collaboration-administration-*.spec.ts`、管理单元测试 |
| F14 / P1 | 客户端与兼容 | Web/真实 Electron 同语义；账号页无工作空间仍可进入；深链/弹出窗口/重启/服务器切换；旧字段/未知枚举防御解析；中文和窄屏主流程 | `collaboration-account-desktop.spec.ts`、`password-server-switch.spec.ts`、`mcp-desktop.spec.ts`、core schema 与 desktop main 测试；Mobile 单独入口 |

矩阵中的文件族是领域入口；当前可执行文件名由 `research/playwright-inventory.txt` 与 `coverage.csv` 核实。新发现但无自动化的功能补为人工用例，标记 NOT_RUN 后执行，不能只列文件就判定已覆盖。

## 近期提交的必复验项

| 风险项 | 近期变更依据 | 本轮必须证明 |
| --- | --- | --- |
| Transfer 当前收件人契约 | `d39d54a07` / 合流 `4085e5b38` | 工作台两条完整旅程；正常转交及旧完成意图并发拒绝保留原断言 |
| 嵌套 runtime 清理泄漏开关 | `5dec88f98` / 合流 `020d18e62` | memory 用例后同库、同 API、不重置开关继续 Leave；原门布尔恢复、epoch 不倒退 |
| 知识真正输入与部署开关 | `057bc9616`、`e6e7fe07d`、`5bd1ecf13`、`639bf6ef8` | 纠错只进入下一次物理输入；部署开关 OFF/ON 与 Host 实际输入一致；确定性 Provider 不等于商业模型使用 |
| 初始 memory 归因 | `1eb0cb33c`、`58d486656` | DB 启动 base=0，后续保存 v1，启动标签仍指向启动快照；Web 与原生保持一致 |
| bundle 来源可信边界 | `91113cbeb` / 合流 `231a08063` | 合法 fixture 满足真实可信来源条件；非法路径仍拒绝；不放宽生产规则 |
| 原生 renderer/API 来源 | `b8035617d` | 启动前准确配置 CORS/WS origin；真实 Electron 与实际 API 连通；未配置时保持 Web 原条件 |
| 日期与知识时钟边界 | `9c056ad68`、`464540191`、`d4bc37a86` | 固定测试时钟；审批/到期使用数据库时间；不存在审批行不一律判过期 |
| 私有 Go adapter-v5 | 既有 `adapter-v5-local-verification.json` 尚待独立复核 | 未经独立复核不启动旧 launcher；若新环境使用 adapter，必须审阅实际文件及契约；不能写假 clearance |
| PG 连接与原生焦点 | 第三次完整检查 47 失败/7 未运行 | 新运行保留连接预算；原始原生用例不加超时/不删 focus 断言；保留历史失败，单次成功不追认历史 |

## 环境、账号与数据

- macOS arm64；已核对 Node `22.23.2`、pnpm `10.28.2`、Go `1.27.0`；记录与 CI Go 1.26 的版本差异。测试不自动下载新工具或安装新依赖。
- `psql` 在 `/opt/homebrew/opt/libpq/bin/psql`，执行时显式补 PATH。Helm/Redis 当前 PATH 未找到，继续检查本机缓存/容器条件，缺条件则记录。
- 当前默认登记 API 显示 `mismatch`，不能复用或停止该未知服务。创建任务专属 registry、数据库、端口、配置副本；不读取/打印账号密钥，不改默认库。
- Go DB、legacy Web、password/admin/device、知识开关 ON 各自独立，执行前核对 `current_database()`、API `/health`、`/api/config`、进程所属与提交。
- 整套 Go 的 DB 包使用各自新建并迁移的数据库；只允许拥有该数据库的协调者负责创建/清理。共享 PostgreSQL 的连接预算涵盖所有 API 池和测试直连；任务 API 建议 `DATABASE_MAX_CONNS=10` / `DATABASE_MIN_CONNS=1`，仍须先核对实际变量支持及 cluster 余量。
- 生产 Web 必须 `next build` + `next start`；同工作树仅一套 build/server；构建临时目录置于 Git 树外、任务自有目录。记录 build ID、API/WS 地址、renderer/preload/CLI 哈希。
- 三类合成身份：创建者/接收者/第三方；每类至少两个隔离工作空间；另有独立管理员。测试数据由现有 `TestApiClient` / `testutil` 创建。覆盖空数据、长中文、多页历史、并发、撤权、丢响应、失败回滚。
- 仅运行任务自造的 fake Provider/CLI；默认 Agent CLI guard 保持开启。商业真实 Provider、供应商账号、额度消耗另列 BLOCKED，不能套用历史调用额度。

## 执行与证据要求

确切命令见 `implement.md`。每个 run 保存 `candidate`、源码/测试指纹、配置非密摘要、开始结束时间、命令、退出码、统计、原始日志；浏览器保存 JSON reporter、失败截图/trace。所有 E2E `workers=1`、`retries=0`。

状态只使用 PASS、FAIL、BLOCKED、NOT_RUN；原始框架 skipped/interrupted 单列。按完整用例键去重统计功能数量，按 run 编号另计重跑次数。后续成功不覆盖原失败。

问题记录包含：版本、严重度、前置、复现步骤、预期/实际、证据、产品/测试/环境/不稳定性分类、下一步。仅确认可复现现象后再定性；没有证据的“可能问题”不当作已证实缺陷。

## 退出标准与报告

本地验收通过须：所有 P0 和可执行 P1 通过；无未解释的跳过、setup 失败或未运行项；静态/单元/集成门禁通过；测试开关恢复且进程清理有证据；独立复核没有阻断问题。

只要关键平台/安装/真实账号门缺少证据，整体结论保留为“本地已覆盖范围的结果，完整验收尚未完成”。跨平台编译、fixture Provider 成功和旧版本历史成功均不能代替实际平台或商业账号验收。

最终交付：本计划、`coverage.csv`、逐次执行日志/摘要、`defects.md`、`report.md`、`progress.md` 与 `session-prompt.md`。产品代码保持当前版本；若需修复则另建有测试证据的修复范围，修复后建立新候选。
