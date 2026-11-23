**Multica 智能体与 AI小队研发场景审计 · 2026-11-05**

当前角色覆盖已经较完整，下一阶段最有价值的投入是：统一阶段交接语义，把现有证据校验接成必经交付检查，收窄执行权限，并让汇报直接消费可核对的数据。

本报告审查 Multica 产品中的智能体、小队、自动化及支撑它们的研发流水线。依据当前工作区源码、规范、测试和 CI 配置；起始提交为 `b43659907f68549732ce2aa9ee701bd76e87ec8c`，分支为 `codex/cross-workspace-collaboration`，工作区已有未提交改动。结论不代表部署实例已经具备相同版本或配置。按本轮要求，未创建 Trellis 任务，也未实施修复。

优先级表示建议处理顺序，并非 CVSS 漏洞等级。下文分别注明合成输入复现、静态确认、条件性风险和能力建设，避免把优化建议写成已发生的事故。

当前注册表有 **23 个角色模板：14 个公开工作角色、9 个小队队长；9 个小队模板；10 个自动化模板**。可靠性工程师、智能体评测工程师、体验验证工程师、迁移审查员、进展报告员和安全漏洞修复小队均已存在。无需重复增加这些岗位。计数来源：[角色注册表](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/builtin_agent_templates_roster.go)、[小队注册表](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/builtin_squad_templates.go)、[自动化注册表](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/builtin_autopilot_templates_roster.go:35)。

| 领域 | 已有基础 | 优先补强方向 |
| --- | --- | --- |
| 需求与开发 | 产品、架构、实现、诊断角色；子任务、阶段、独立工作树 | 阶段交付与人工验收信号一致；任务级验收条件与资源所有权 |
| 评审与质量 | QA、代码、安全、体验、迁移、智能体评测；结构化 lifecycle 校验 | 将证据和当前提交绑定，纳入状态流转和交付门禁 |
| 安全 | 工作空间隔离、任务身份、Access、自治等级、人工批准、MCP broker | 宿主隔离、资源与动作范围、批准后受控执行、日志脱敏 |
| 非功能 | 队列/运行时延、失败、重试、Token、估算成本指标；可靠性角色 | 持续容量与故障验证、SLO、灾备演练、跨子任务预算 |
| CI/CD | TS 构建/类型/检查/测试，Go race、sqlc、govulncheck，跨平台 Host 测试 | 发布测试环境一致、拒绝零测试假绿、关键用户路径自动验证 |
| 汇报 | 日报/周报模板；范围、时间、分页、去重、关闭口径、数据缺口要求 | 服务端冻结事实快照、确定性计算、交付效果与成本归因 |
| 治理与演进 | 模板版本、保留用户定制、条件更新/回滚；知识与复盘合同 | 实例升级、真实行为评测、证据过期和知识失效管理 |

以下四项应先进入修复清单。

**1. P1：子任务交付信号与阶段推进条件不一致。静态确认。**

运行时指令要求子任务完成工作后进入 `in_review`，并声称阶段屏障与父任务通知依赖这个信号。实际通知只在进入 `done/cancelled` 类别时运行，执行结束也不会自动改变业务任务状态。证据：[运行时指令](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/daemon/execenv/runtime_config_sections.go:728)、[父任务通知条件](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/issue_child_done.go:84)、[终态定义](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/issue_child_done.go:390)、[执行完成语义](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/task.go:4398)。

具体影响条件：父任务在进行中，阶段一子任务交付评论并进入 `in_review`，阶段二仍在 `backlog`；若没有人工/PR 把子任务置为终态，也没有额外触发父任务，阶段屏障不会因此唤醒父任务。不能据此说所有小队都会卡死；如果产品要求逐阶段人工验收，等待本身合理，但当前指令仍然错误。

建议明确区分“已交付”和“已验收”。需要人工确认的阶段显示具体等待原因；允许自动交接的阶段使用独立的交付信号。不能直接把全部 `in_review` 视为 `done`。验收应贯穿实际注入指令、子任务状态、父任务唤醒、人工拒绝、重复事件与下一阶段准入。

**2. P1：发布验证缺少数据库环境。静态确认的配置缺陷。**

Release 的 `verify` 执行完整 Go 测试包装脚本，但没有配置 PostgreSQL service、`DATABASE_URL` 或迁移步骤。包装脚本会包含 router/handler；它们的 `TestMain` 在缺少 URL 时退出 2。按当前 YAML 声明的干净托管 runner，发布验证将被阻断。本轮没有实际推标签或运行远程发布。

证据：[发布验证](/Volumes/artisan/code/2026/multica-0.4.37/.github/workflows/release.yml:23)、[完整测试入口](/Volumes/artisan/code/2026/multica-0.4.37/scripts/test-go.sh:30)、[handler 前置条件](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/handler_test.go:44)、[router 前置条件](/Volumes/artisan/code/2026/multica-0.4.37/server/cmd/server/integration_test.go:43)。

建议复用主 CI 的隔离数据库、迁移和测试初始化合同。验收是在干净 runner 上完成同一标签的验证，保留实际执行的测试清单；不能通过删掉数据库测试来消除失败。

**3. P1：数据库不可连接时，整包测试可能未运行却返回成功。静态确认。**

上述两处 `TestMain` 对 URL 缺失会失败，但 URL 存在而连接失败时直接 `os.Exit(0)`，没有进入 `m.Run()`。普通包装脚本依赖退出码，没有预期测试清单检查。证据：[handler 跳过分支](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/handler_test.go:52)、[router 跳过分支](/Volumes/artisan/code/2026/multica-0.4.37/server/cmd/server/integration_test.go:51)、[包装脚本](/Volumes/artisan/code/2026/multica-0.4.37/scripts/test-go.sh:39)。这证明存在假绿路径，不表示每次 CI 都走到了该路径。

建议 CI 的数据库初始化失败一律返回非零；输出 `go test -json` 并验证预期 suite/test 确实执行，标明允许的 skip。已有 Native Host 测试清单检查可复用，见 [现有证据门禁](/Volumes/artisan/code/2026/multica-0.4.37/.github/workflows/ci.yml:640)。验收包括数据库断开、零测试执行、过滤器误匹配和被批准的可选测试跳过。

**4. P1：结构化日志中的口令存在脱敏遗漏。已用合成输入复现。**

`redact.InputMap` 递归处理 value，却不使用 key 判断是否为敏感字段。因此普通 `password`、嵌套 `api_key`、`map[string]string` 中的 `TOKEN` 可原样保留。JSON 文本中的普通口令以及符合当前格式的裸 `mat_` 任务令牌也未被遮盖。`PASSWORD=...` 正控制能被遮盖，说明是形态覆盖缺口。

证据：[脱敏实现](/Volumes/artisan/code/2026/multica-0.4.37/server/pkg/redact/redact.go:102)、[守护进程出站调用](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/daemon/daemon.go:8723)、[服务端入库/广播前调用](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/daemon.go:4895)。本轮没有读取真实凭据，也没有证明实际部署已有泄漏。

建议同时按敏感字段名、本次注入的确切凭据值及产品令牌格式脱敏，覆盖嵌套数组、工具入参和 JSON 输出；流式分片需要另做验证。验收必须检查出进程、持久化、WebSocket 三处的同一合成样本。仅继续补充厂商前缀正则不足以解决普通口令问题。

研发体系还有以下十项值得投入。优先复用现有结构，不另建平行系统。

| 编号与优先级 | 现状、影响 | 建议与验收 |
| --- | --- | --- |
| **5 · P1：执行权限与审批** | API 身份隔离真实存在；宿主文件和网络权限仍属于守护进程用户。默认 Codex/Claude 的无人值守路径不提供通用文件系统隔离；批准记录不拦截 shell。报告员为 Contributor，其“只写本次报告任务”主要由指令限制。 | 为评审、实现、安全扫描、发布配置可验证的独立运行环境及最小凭据；API 在既有工作空间边界内增加“资源 + 动作 + 本次执行”限制。高风险操作走受控执行器，批准绑定提交、环境、参数摘要、有效期和一次性执行许可。验收拒绝报告员修改业务任务、评审员写代码、过期批准执行新参数。 |
| **6 · P1：统一交付门禁** | 已有 Agent Evaluation、治理和发布观察的结构化校验；普通任务状态流转没有统一强制消费这些证据。GitHub 已同步当前 head 的 CI 状态，合并关闭任务主要依据 PR 聚合状态。 | 项目按变更风险声明必需检查：代码、测试、安全、迁移、体验、AI 行为。复用现有校验器，绑定 commit/artifact、执行身份、证据与有效期。提交变化使旧证据失效；区分代码合并、验收、部署、生产验证。测试旧 SHA 绿灯不可放行新提交，缺证据保持 unknown/hold。 |
| **7 · P1：关键路径纳入日常 CI** | 主 PR CI 未执行 Playwright；路径过滤也没有 `e2e/**` 与 `playwright.config.ts`。本地 `make check` 有 E2E，安装版桌面业务验收主要是手动工作流。 | PR 跑小而稳定的“建智能体/小队→分派→执行回执→评审→交付/报告”smoke；夜间和发布前扩展生产 Web、安装版 Desktop、升级和恢复矩阵。验收修改 E2E 文件会触发检查，核心退化能阻断合并。 |
| **8 · P1/P2：指令一致性与有效委派** | 合并门禁小队说明有六类审查，队长说明仍列三类。审查被显式要求串行。leader 的 `action` 记录未绑定某次成员派发回执，而评论 API 已返回 `trigger_outcomes`。 | 统一维护角色/适用检查/路由来源；先消费已有派发结果，区分评论已发表与成员已入队。对同一冻结提交并行只读审查，收齐必需结果再继续；写入任务保持明确工作树与文件所有权。验证权限拒绝、离线、重复派发、审查版本变化及退回次数。 |
| **9 · P2：确定性的工作汇报** | 日报周报已要求固定窗口、分页、去重、按状态历史计数、排除报告任务、缺失不等于零。但目前这些动作主要交给模型通过通用任务/评论接口完成，本轮未找到专用报告快照聚合合同。 | 服务端生成可追溯快照：范围、时区、窗口、截止水位、覆盖率、分类后的任务编号、去重数和排除原因；模型负责解释已核对的事实。按工作空间/项目/周期/时区定义幂等身份。验收分页、重开、取消、范围迁移、无权限、补跑和迟到事件，并明确滚动 24h/7d 与自然日/周的区别。 |
| **10 · P2：跨执行的成本和资源预算** | 已有 Token、估算费用、排队时间和失败指标，也有自动化运行次数额度。普通执行默认没有绝对时长上限；次数额度不等于项目或小队的总费用预算。普通重试已有次数上限，不能称无限重试。 | 在父任务/项目/小队统一归集子任务和重试，设置时间、Token/估算费用、子任务数、工具调用和并发预算。阈值触发提示、检查点或按策略暂停；未知 usage 单独显示。验收并行与重试无法绕开总额，保留长任务的合理运行能力。 |
| **11 · P2：持续非功能验证** | 可靠性角色、SLI/SLO/灾备说明与监控已存在；已查容量测试包含显式 opt-in 和 benchmark，日常工作流未运行相应负载。内建告警规则集中于离线、排队超时、执行失败。 | 为排队、首次有效输出、阶段交接、取消生效、断线恢复、DB 连接、FD/内存、数据持久性定义可观测目标。在固定环境做容量、长时间运行、故障注入、备份恢复演练；报告样本量、P95/P99、基线和失败样本。跨 Web/Desktop/Mobile 的协议、可访问性与升级兼容按风险纳入。 |
| **12 · P2：AI 行为评测与模型调度** | 评测器已有六类 case 与版本/trace 要求；case 的 `result` 同时承担行为结果和门禁含义，成本/延迟的 hold 可以表示符合预期的安全停止。脚本化流程测试不能证明模型稳定遵守规则。 | 分开 expected、observed、verdict，保留实际模型/工具/提示词版本、样本量、成本、时延及阈值。正常、工具故障、注入、越权、重复调用、上下文丢失都纳入版本化用例；用回放做 PR 门禁，真实模型用已有授权的隔离环境做定期抽样。模型选择依据这些数据，避免给所有岗位统一换成最高成本模型。 |
| **13 · P2：软件供应链** | Go 有 govulncheck。对 `.github/`、根脚本和发布配置的有界搜索未发现 JS 依赖审计、SAST/secret 扫描、显式 SBOM/证明门禁。Actions 使用可移动标签；Windows 安装包当前发布路径明确关闭签名，macOS 签名走外部手动流程。 | 按实际依赖与可达性补齐扫描；Action 固定 SHA 并维护更新；制品绑定源码 SHA、依赖清单、来源证明和适用平台签名。例外保留负责人、原因、期限。验收下载的实际制品能追溯到同一源码和检查结果。在线仓库扫描/保护设置本轮未核验，不能据此断言完全不存在。 |
| **14 · P2：模板升级、知识与协作治理** | 模板创建后复制为工作空间实例，升级不会覆盖定制；已有 hash、版本比较、CAS 和条件回滚。可靠性、复盘和知识来源合同已有基础，但新模板不等于已有实例已更新。 | 提供实例差异、保留定制的升级合并、限定实例灰度、升级前后评测和回滚。知识保留来源、适用版本、负责人、过期/撤销条件；仅将已验证结论提升为可复用规范。治理台展示实际角色版本、任务使用版本和未验证能力，不把默认关闭或尚在验收的协作能力显示为全面可用。 |

第 5 项有两个需要明确区分的补充点。其一，自治策略读取失败目前返回“非智能体”，部分调用方因此跳过门槛；`CreateSquad` 存在只有该预检、随后直接写入的静态路径。若策略查询短暂失败、后续查询恢复，可能放松该次限制。建议错误时返回可重试失败，并把旧版未声明策略单独迁移。本轮未做数据库故障注入，不能宣称已复现越权；任务更新/删除还有事务内重查，不能把预检问题直接套用到所有接口。证据：[自治策略读取](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/agent_autonomy.go:37)、[创建小队门槛](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/squad.go:266)。

其二，插件 MCP 当前按工作空间获取获批连接，适合进一步收窄到角色/本次执行所需工具及参数。已有 broker 的工具名、Schema、HTTPS/域名、撤销和调用限额校验应保留；这不是“所有 MCP 都没有授权”。证据：[连接获取](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/plugin_mcp_transport.go:147)、[broker](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/daemon/remote_mcp_broker.go:247)。

上述能力建设的关键代码依据：

| 项目 | 可复核位置 |
| --- | --- |
| 宿主与审批的真实边界 | [安全模型](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/security-model.zh.mdx:42)、[系统自治说明](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/builtin_agent_autonomy.go:10)、[报告员权限说明](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/spec/server/builtin-templates.md:73) |
| 结构化评测和治理 | [评测校验](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/lifecycle_handoff.go:245)、[治理校验](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/lifecycle_handoff.go:319)、[普通任务状态授权](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/issue.go:3581)、[PR 关闭聚合](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/github.go:1897) |
| 用户路径 CI 覆盖 | [主 CI 路径过滤](/Volumes/artisan/code/2026/multica-0.4.37/.github/workflows/ci.yml:47)、[前端构建检查](/Volumes/artisan/code/2026/multica-0.4.37/.github/workflows/ci.yml:304)、[手动 Desktop 验收](/Volumes/artisan/code/2026/multica-0.4.37/.github/workflows/desktop-smoke.yml:3) |
| 路由漂移与串行审查 | [小队的六类检查](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/builtin_squad_templates/review-gate/INSTRUCTIONS.md:9)、[队长的三类检查](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/builtin_agent_templates/review-gate-lead/INSTRUCTIONS.md:9)、[现有席位检查](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/builtin_agent_autonomy_test.go:281) |
| 委派与回执 | [队长活动记录](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/squad.go:1228)、[已有触发结果](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/comment.go:75) |
| 报告数据合同 | [进展报告 skill](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/builtin_role_skills/multica-progress-report/SKILL.md:14)、[报告流程测试边界](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/autopilot_template_progress_test.go:14) |
| 时长、额度与观测 | [运行时上限](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/daemon/config.go:25)、[运行次数额度](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/autopilot_quota.go:171)、[已有指标](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/metrics/business.go:120) |
| 容量与告警 | [opt-in 容量测试](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/triage_performance_test.go:16)、[当前告警规则](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/admin_alert.go:22)、[可靠性 skill](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/builtin_role_skills/multica-reliability-engineering/SKILL.md:16) |
| 供应链 | [Go 扫描](/Volumes/artisan/code/2026/multica-0.4.37/.github/workflows/ci.yml:585)、[Action 标签](/Volumes/artisan/code/2026/multica-0.4.37/.github/workflows/ci.yml:35)、[安装包签名现状](/Volumes/artisan/code/2026/multica-0.4.37/.github/workflows/release.yml:555) |
| 实例升级 | [复制与定制合同](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/builtin_agent_templates.go:12)、[现有条件更新/回滚](/Volumes/artisan/code/2026/multica-0.4.37/scripts/localize-agent-instructions.md:25)、[协作能力的当前边界](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/spec/server/index.md) |

建议把现有岗位组成按需流程，减少无收益的交接。下表是后续设计建议，不代表本轮已经调整配置。

| 场景 | 建议的工作路径 | 必需交付 |
| --- | --- | --- |
| 小改动、已知原因缺陷 | 实现 → QA/代码审查；只有未知原因才进入诊断 | 最小改动、有效回归、当前提交检查结果 |
| 中大型功能 | 产品/架构按需 → 有明确所有权的并行实现 → 独立审查 → 人工验收 | 验收标准、接口合同、当前产物、未解决风险 |
| 安全或依赖风险 | 现有安全修复小队分诊 → 必要诊断/迁移审查 → 最小修复 → 独立安全与回归验证 | 适用性和来源、影响范围、回归、兼容与残余风险 |
| 发布和事故 | 发布/事故队长 → 发布工程师执行获批动作 → 可靠性与 QA 验证 → 文档/复盘 | 批准、同一制品、观察窗口、回滚限制、恢复与预防证据 |
| 日报、周报与管理 | 确定性事实收集 → 进展报告员解释 → 负责人作业务决定 | 范围、时间、覆盖率、任务/PR/部署证据、阻塞负责人和下一步 |

小队可用性还应从“成员在线”扩展为“适合这次工作”。现有项目 readiness 已核对完整花名册与运行时，不需要重做；在其上加入仓库访问、必需工具、排队时间、预算和能力验证日期。相同角色会跨小队复用，默认相同运行时不等于资源互不影响。应按实际负载选择成员和运行时，而不是固定增加智能体数量。依据：[项目 readiness](/Volumes/artisan/code/2026/multica-0.4.37/packages/core/projects/execution-squad.ts:87)。

工作汇报建议保留四种用途，统一从事实快照产生，避免每次重新解释统计口径。

| 报告 | 应回答的问题 | 应附的证据 |
| --- | --- | --- |
| 日报 | 本期交付了什么、哪里阻塞、谁需要采取下一步 | 已完成/已交审/已合并分别列出；负责人、任务链接、检查状态、数据缺口 |
| 周报 | 交付速度和质量是否改善，返工/等待/成本在哪里 | 周期与排队分布、一次验收通过率、返工和线上逃逸缺陷、小队成本归因；与自身历史基线比较 |
| 发布报告 | 哪个产物进了哪个环境，实际观察如何 | commit、artifact digest、CI/评测、批准、部署结果、观察窗口、健康/业务指标、回滚状态 |
| 事故复盘 | 影响与恢复发生了什么，哪些措施能防止再发 | 事实时间线、恢复与数据损失证据、根因置信度、预防任务、负责人和验证结果 |

交付效能可参考 DORA 当前的五项指标：变更前置时间、部署频率、失败部署恢复时间、变更失败率、部署返工率。将它们用于团队/服务改进，并补充人工介入率、每项验收交付的估算成本、错误放行率；本报告没有测量这些指标的当前值。[DORA 官方说明](https://dora.dev/guides/dora-metrics/)

推进顺序建议为：第一批修复 1–4，并统一第 8 项已确认的指令漂移；第二批完成 5–7 的权限、门禁、关键路径 CI，以及有效委派回执；第三批建设 9–14 的报告快照、预算、非功能基线、模型评测、供应链与实例升级。各批都以独立验收结果结束，不以模板数量、评论数量或智能体执行次数作为成功标准。

安全设计建议与 OWASP 的工具最小权限、执行授权、高影响动作完整性、跨智能体边界和对抗测试方向一致；具体权限应结合现有工作空间/任务身份实现。[OWASP AI Agent Security](https://cheatsheetseries.owasp.org/cheatsheets/AI_Agent_Security_Cheat_Sheet.html) CI 建议将令牌权限收敛到各 job 所需，并固定 Action 提交 SHA；仓库现有 Release 工作流顶层还授予 `contents/packages: write`，适合进一步下沉到实际发布 job。[GitHub Actions 安全参考](https://docs.github.com/en/actions/reference/security/secure-use)

本轮验证记录：

- 三个独立审计方向分别核查小队开发流程、安全边界、CI 与非功能；主审复核关键缺陷的调用链和源码位置。
- 静态清点得到 14 个公开工作角色、9 个队长、9 个小队、10 个自动化。确认 Release verify 含完整 Go 测试，而不含数据库 URL/service；主 CI 未声明 Playwright test 或 E2E 路径过滤。
- 使用临时 Go 程序调用仓库真实 `pkg/redact`，全部输入均为合成值，临时目录退出后清理。结果见下方。
- 在 `server/` 执行 `go test ./pkg/redact -count=1`：`ok github.com/multica-ai/multica/server/pkg/redact 0.415s`。通过的是现有测试，不代表新发现已修复。
- 未运行完整 lint/typecheck/测试套件、数据库故障注入、真实模型或线上压测；未触发远程 CI、发布或外部通知。未核验部署实例、在线分支保护、当前依赖 CVE 全量结果、真实团队指标。报告中“未找到”的表述均限于已说明的源码/配置范围。

```text
structured_password_preserved=true
structured_api_key_preserved=true
structured_token_preserved=true
bare_40hex_task_token_preserved=true
json_password_output_preserved=true
key_value_positive_control_redacted=true
exit_code=0
```

合成复现的核心调用如下，字符串不对应真实凭据：

```go
sample := "audit-synthetic-value-12345"
out := redact.InputMap(map[string]any{
    "password": sample,
    "nested": map[string]any{"api_key": sample},
    "env": map[string]string{"TOKEN": sample},
})
fmt.Println(out["password"] == sample)
fmt.Println(out["nested"].(map[string]any)["api_key"] == sample)
fmt.Println(out["env"].(map[string]string)["TOKEN"] == sample)
bare := "mat_" + strings.Repeat("a", 40)
fmt.Println(redact.Text(bare) == bare)
jsonOutput := fmt.Sprintf("{\"password\":\"%s\"}", sample)
fmt.Println(redact.Text(jsonOutput) == jsonOutput)
fmt.Println(redact.Text("PASSWORD="+sample) != "PASSWORD="+sample)
```

本轮交付仅新增此审计报告；没有产品代码变更或代码简化。上述问题和改进均仍待后续修复/建设，不能视为已经闭环。
