# 跨工作区协作测试报告

**结论：已测核心流程通过，保留 1 项 P2 体验问题。** 跨账号、独立工作区之间的任务流转与访问隔离有通过证据；并发旧版本请求被安全拒绝，之后既能刷新恢复，也能保留输入在原页继续并实际完成。本轮未发现已测场景中的越权读取、重复创建或任务身份丢失。真实 AI 执行、Linux 运行隔离及完整上线验收仍有缺口。

记录日期：2026-10-19；报告生成时间：2026-10-19T02:42:50-07:00（America/Los_Angeles）。原始执行日志使用 UTC。Trellis 子任务：`cross-workspace-qa-report`，关联仍在进行中的父任务 `cross-workspace-collaboration`。本轮是测试与报告交付，未修复产品问题。

**环境与来源**

| 项目 | 本轮记录 |
| --- | --- |
| 分支/提交 | `codex/cross-workspace-collaboration` / `8fc94420221ddde15a02aa8feffea18f55b54f88` |
| Web | Next.js production，端口 13800；构建 ID `JMCCYRdssyg2GZrPFR8ku` |
| API | 本提交的 Go 服务，端口 18880；测试专用 PostgreSQL、API Redis 与 Go 测试 Redis |
| Desktop | macOS arm64 上实际 Electron + 受控 native-service 夹具，renderer 端口 5974 |
| 测试身份 | 合成 A/B/C 账号；A/B 各有独立工作区且彼此不是对方成员，C 无本单权限 |
| 开关 | 仅任务专用数据库开启创建；共享运行时/真实 provider 执行未开启 |
| 代码状态 | 基线与当前 HEAD 相同；`apps/web/next-env.d.ts` 的生成变化在续测开始前已存在，本轮未改产品源码 |

来源核对：[当前 provenance](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/provenance-current.json)；[原始基线](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/baseline.json)。没有将历史其他提交的测试计入本轮。

**测试结果**

| 范围 | 结果 | 证据 |
| --- | --- | --- |
| 标准 Web 客户端 | 6/6 通过，0 自动重试/跳过 | [Playwright JSON](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/web/results.json) |
| macOS Electron | 9/9 通过，0 自动重试/跳过 | [Playwright JSON](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/desktop/results.json) |
| 独立工作区 A/B/C 完整流程 | 1 次最终通过；部分生命周期操作前刷新 | [完整旅程](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/web/human-stable-results/human-workflow-A-and-B-kee-eba6d-eptance-and-live-revocation/journey.json) |
| 强制并发旧版本请求 | 1 次通过；验证拒绝、原请求回执及刷新后控件恢复 | [原始结果](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/web/race.json) |
| 刷新后真正重新提交 | 1 次通过；新请求键、HTTP 200、同单状态 done | [完成证据](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/web/recovery-followup-final-results/recovery-followup-a-reject-4d1bc-h-completion-really-commits/confirmed-recovery.json) |
| 原页保留输入后真正重新提交 | 1 次通过；无整页刷新，新请求键、HTTP 200、状态 done | [完成证据](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/web/recovery-inline-results/recovery-inline-a-rejected-3738f-ommit-without-a-page-reload/confirmed-recovery.json) |
| Core / Views 单元测试 | 71 个实际文件、901 条测试记录通过（752 + 149），0 失败/跳过 | [回归审计](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/evidence-audit-regression.md) |
| Go 相关回归 | 13 包退出 0；581 个顶层测试通过、7 个显式跳过；另有 900 个子测试通过 | [逐包结果](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/backend/package-outcomes.json) |
| Go 并发与静态分析 | 13 包均启用 `-race -count=1`，无 race 诊断；`go vet` 退出 0 | [回归审计](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/evidence-audit-regression.md) |
| Lint / TypeCheck | 3 条 lint、4 条 typecheck 命令退出 0；所选范围，非全仓全平台检查 | [精确命令与退出码](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/evidence-audit-regression.json) |

标准客户端 15 项，加 4 次补充测试执行通过；另保留 3 次失败尝试。全部 9 份客户端执行 JSON 合计 **22 次执行，19 次通过、3 次失败、0 自动重试**。这些不是 22 个独立需求，也不能与父子 Go 测试相加计算覆盖率。

**业务覆盖与观察**

| 场景 | 已观察结果 |
| --- | --- |
| 跨工作区派单与待办 | 外部处理人从账号工作台看到同一单据；没有因此成为来源工作区成员 |
| 普通评论 | 不改变处理责任、不触发 AI；完整旅程各检查点 run 数均为 0 |
| 补充信息与交回 | 责任从 B 转 A 补充，再明确交回 B；普通评论不会代替交回 |
| 转交与验收 | 转交后保留参与入口；提交验收将责任交发起人，状态为评审中，未自动完成；发起人可完成/重开 |
| 在线撤权与无关账号 | C 的详情访问返回 404；移除当前处理人后 B 已打开页面失去详情，责任回到发起人 |
| 账号请求范围 | 完整旅程记录 242 次账号范围请求，未夹带工作区 ID/slug 请求头，0 pageerror |
| 来源页、字段与讨论 | 来源与共享详情保持同一单据；字段编辑权限、评论、回复/历史、编辑中撤权及来源成员空 WS 刷新场景通过 |
| 私有文件与丢响应 | 上传/创建丢响应后恢复、明确拒绝后的重新提交、预览在线撤权场景通过 |
| 资源与配置 | 离线目录下资源拥有者授权/撤回、参与者管理、挂起后的配置切换场景通过；不代表真实 AI 执行 |
| Desktop 导航与守卫 | 工作区间账号标签页、独立窗口/重启、零工作区、工作区列表失败、账号/服务切换和登录守卫通过 |
| 屏幕布局 | 已人工查看 Web 窄屏、宽屏和 Electron 200% 编辑截图；不是完整无障碍或多浏览器审计 |

[完整客户端案例清单与早期失败审计](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/evidence-audit-browser.md)列出了 6 个 Web、9 个 Electron 场景名称及夹具限制。该审计早于最后两次补测；恢复后的真正成功提交以本报告的新证据为准。

**问题 CWC-QA-001：并发拒绝的初始提示不准确（P2，未修复）**

另一账号刚变更处理责任后，在未及时取得新版本的页面立即“完成任务”，会收到明确的 `409 stale_revision`。界面此时先显示 “Could not load collaboration. Try again.” 和 “The result is not confirmed”，并暂时禁用操作。任务实际仍可访问，该次操作已经被拒绝；这组提示会让用户误以为加载失败或操作可能已执行。

复现步骤：

1. A/B 分别拥有独立工作区，A 创建协作单并交给 B，双方打开该单。
2. B 提交验收后，A 立即完成；原始真实流程记录客户端版本 5、服务器版本 6。也可在 A 完成请求发出前，让 B 先真实转交该单，稳定触发同类冲突。
3. A 的完成请求返回 `409 stale_revision`，查看上述初始提示。
4. 点击“Check original result”，原请求回执返回 200，其业务结果仍是 `stale_revision`；此时才明确显示操作没有应用。
5. 等最新单据到达后，“Keep input and use current version”可在原页保留输入继续。该路径与整页刷新后重新打开表单的路径均已实际成功提交。

期望：明确区分已知的并发拒绝与网络结果未知，保留输入并清楚引导使用最新版本。当前恢复方式已验证有效，本问题没有数据损坏、永久卡死或必须整页刷新的证据。正常旅程的一次冲突与多次受控竞态均有记录；未测发生概率。

证据：[初始错误提示](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/web/recovery-followup-final-results/recovery-followup-a-reject-4d1bc-h-completion-really-commits/11-explicit-rejection-shown-as-unconfirmed.png) → [原请求已明确拒绝/保留输入控件](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/web/recovery-followup-final-results/recovery-followup-a-reject-4d1bc-h-completion-really-commits/12-original-rejection-recovered.png) → [原页恢复后实际完成](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/web/recovery-inline-results/recovery-inline-a-rejected-3738f-ommit-without-a-page-reload/16-inline-completion-committed.png)。原始自然冲突 trace 见 [human-verified trace](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/web/human-verified-results/human-workflow-A-and-B-kee-eba6d-eptance-and-live-revocation/trace.zip)。

**失败历史与测试过程**

| 时间 UTC | 记录 | 分类与处理 |
| --- | --- | --- |
| 08:28 起 | 准备专用服务、基线、前后端与生产 Web 检查 | 原始计划与命令 metadata 保留 |
| 08:32 / 08:34 | 标准 Web 6 项 / Electron 9 项 | 全部通过 |
| 08:34:58 | human-initial 失败 | 脚本将原生 Fetch 的数字属性 status 当作函数；修正一次性脚本，404 断言保持 |
| 08:35:51 | human-verified 失败 | 正常验收后立即完成遇到真实 stale_revision；保留为产品体验问题证据 |
| 08:39 | human-stable / forced-race 通过 | 前者在部分动作前刷新；不能据此称产品问题已修复；后者起初只证明恢复就绪 |
| 09:33 | followup-initial 失败 | 只设 ENV_FILE 未给夹具注入专用 DATABASE_URL，登录前置读取了另一数据库，未取到验证码；切回既有专用 runner |
| 09:34 | followup-final 通过 | 刷新后以新键重新提交，实际 done |
| 09:39 | recovery-inline 通过 | 原页保留输入并更新版本，以新键重新提交，实际 done；无需整页刷新 |
| 收尾 | 独立证据复核、Trellis 记录、报告和环境清理 | 原始失败、截图、数据均保留 |

[测试脚本/环境修正说明](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/web/harness-corrections.md)保留原因。首轮默认环境发现使用了过期登记且 psql 未在 PATH；随后使用已就绪的任务专用环境，未将默认启动日志计为产品测试结果。临时 browse 观察中的一次定位歧义及快速导航导致的 auth fetch 日志也不计作稳定产品缺陷；主流程及两条新恢复路径的 pageerror 断言均为 0。单测仍有 pnpm 配置、React act/DOM 和 i18next 告警，未将其直接定性为生产错误。

**覆盖限制与后续验收**

- 7 个 Linux 场景显式跳过，另 4 个已选 Linux 测试名称在本次 Darwin 执行中没有事件，合计 11 个选中顶层测试缺少通过证据；详细名称见回归审计。没有伪记为通过。
- 共享执行/真实 AI provider 未开启。没有验证真实推理、个人供应商额度、完整 transcript/用量、长任务和真实终止证明；资源表单与离线队列通过不能替代这些验收。
- Electron 用例使用实际进程与受控 native-service 夹具，覆盖所用路由/IPC/preload/renderer；未覆盖完整安装包 Main 启动、daemon 整体、安装升级、Windows/Linux Desktop。
- 未覆盖 Safari/Firefox、iOS App、全量父任务验收、压力/耐久、长时间断网、生产 S3/Redis 分布式故障、大附件和完整安全/无障碍审计。Web 窄屏截图不代表 Mobile 已交付。
- 本轮未做产品修复。后续产品工作需单独处理 CWC-QA-001，并补齐真实运行与平台验收；父任务保持进行中。不用本报告签署全量上线通过，也不生成缺乏覆盖依据的全站健康分。

**交付与保留**

报告、结构化结果、截图和原始日志保存在本报告目录；另保存 `.gstack/qa-reports/qa-report-cross-workspace-2026-10-19.md` 副本与 Trellis 测试记录。仅新增报告、记录和一次性 QA 夹具，无新增依赖、产品改动、部署或推送。清理结果见 [cleanup.json](/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/cwc-qa-20261019T082840Z/cleanup.json)：测试专用 API/Web/renderer 与容器已停止，数据库、上传文件和证据保留；只处理经身份核对的本轮服务。
