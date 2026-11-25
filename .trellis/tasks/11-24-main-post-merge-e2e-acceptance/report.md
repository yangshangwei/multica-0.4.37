# main 合并后端到端验收报告

基线：`103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。运行：`main-post-merge-e2e-20261124-080504`。2026-11-25 验收执行与证据交付完成，产品结论 FAIL。

## 当前结论

**产品验收不通过，不建议发布。** 评论机器凭据越权已在 legacy/password 真实写入中复现；Go race 整包场景失败与安全扫描14项可达发现仍未闭合。macOS 与 Linux 打包 Main 均已实际启动并登录，但在旧标题断言失败，后续控制链路未验证。旧 CLI 兼容性场景在原240秒预算超时，且保留一次兜底卷清理失败。

本任务保持产品与既有测试源码不变，验证当前 main 并保留缺陷证据。后续修复不能用本报告中的旧通过数代替修复后回归。

## 最终覆盖与续接成果

最终覆盖快照 `coverage-audit/snapshot-24/` 已对默认138个跳过身份全部给出完整补测结果：最近执行 **117 PASS、21 FAIL、0 NOT_RUN**；保留全部历史失败的保守口径为 **114 PASS、24 FAIL**。默认完整运行仍为400 PASS /138 SKIP，不改写原始结果。按默认538个身份合并其最近补测结果为517 PASS /21 FAIL；这不包含额外私有探针、配置重复执行或后端测试。每个领域/配置的真实层次与未验证步骤见快照中的 T01–T25、L0–L10 矩阵；领域计数重叠，不能相加。

本次补齐 Stage 原三项、knowledge-deployed 开/关各一项、业务场景三项和 Desktop 信息恢复完整六子场景。业务初次运行虽有3个原始PASS终端，因丢失原退出码继续保留 INTERRUPTED；新 `business-scenarios-02` 以完整3PASS回执补齐。独立 Desktop 新实例避免原36次注册挤入30次预算，保留原失败，不放宽限流。

真实自然响应暂停探针通过并经独立审查后，`recovery-extras-delayed-02` 的 Web 全三子场景通过；Desktop 在 source-only membership loss 的拒绝提示匹配6个元素而失败，后续子场景未运行。原缺少暂停准入标记的 Web 失败也保留。续接队列总共25次执行为12 PASS /13 FAIL，其中有一项同身份环境复验，不能算25个新功能。

新增失败按实际边界分类：两项 host 用例在启动许可前失败，Provider 未启动，API许可503/慢请求与连接池压力存在，但取消先后及根因 UNKNOWN；九项 namespace 浏览器用例在 daemon ready 能力探针处拒绝，底层原因未捕获；导航/提示定位器失配的后续业务断言仍未验证。详情与后续处理见 [defects.md](defects.md) 及 `research/host-visit-failure-review.md`。成功的 `order-export-leave-01` 不抵消另一导出场景失败。

## 已核对的执行结果

| 检查 | 实际结果 | 证据及解释 |
| --- | --- | --- |
| 前端静态与类型 | PASS；15/15 项执行、0 缓存；UI exports PASS | `research/frontend-results.md`；30 个既有 lint warnings、0 errors |
| TypeScript/Vitest | 1,190 文件、15,255 命名测试 PASS | `frontend/named-inventory-final.json`；含移动 196，不重复计桌面脚本子集 |
| Go normal | 86/86 包完成；25,580 命名 PASS、24 SKIP | `backend/normal-summary.json`；每包独立数据库 |
| Go race | 86/86 包完成；25,578 命名 PASS、24 SKIP；1 场景 FAIL | 两个失败终端是同一父测试及 `wrong_consumer` 子测试；不是两处独立缺陷 |
| Go 构建、vet、wrapper | PASS | 原静态 receipts；无须为恢复上下文重跑 |
| Go 安全扫描 | FAIL，exit 3，14 项可达发现 | 本机 Go 1.27.0 与依赖版本需分别归类；不是已证明 14 种线上可利用攻击 |
| UTC 专项 | 3 包、41 顶层 / 93 命名 / 76 叶用例 PASS | `research/utc-regression-results.md`；与 Shanghai 全量结果分开 |
| Linux 后端条件补测 | 2 项进程测试、1 项 root 前置子测试 PASS；7 项 namespace 用例 BLOCKED | `backend-linux-acceptance/result.json`；默认 Docker 策略下普通/root clone 探针均拒绝，未改变安全策略；仅 CGO-disabled normal 模式 |
| 默认 Chromium / Web / Electron renderer | 538/538 身份完整；400 PASS、138 SKIP、0 FAIL | `runtime/legacy-broad-02/named-inventory.json`；生产 Web，worker 1、retry 0 |
| 额外密码注册/分支/切换/MCP | 16 PASS | 三个已完成的 `runtime/password-*-01` lane；不重复执行 |
| 评论权限真实 HTTP 探针 | FAIL；legacy 26 PASS / 8 FAIL / 3 风险观察，password 13 PASS / 4 FAIL / 1 风险观察 | 合法 API 颁发的 `mat_` 任务凭据、真实写入前后状态；未调用模型 |
| 迭代与 Triage 性能补测 | 原三个条件 SKIP 全部 PASS | 原样本数与 2,000ms 门槛；迭代最慢 P95 536.588ms，Triage 最慢 161.068ms；不等同 L10 百人协作采样 |
| 四条核心 Agent 链路 | 4/4 首次 PASS，已包含在 400 中 | `critical-business-cases-reconciliation-20261125.json`；是默认整套内的业务证据，不宣称独立 smoke launcher 或 CI 已运行 |
| 脚本与平台契约 | 53 入口 PASS，4 NOT_RUN | Helm 1、PowerShell 3 缺运行条件；`research/script-platform-results.md` |
| 五个原脚本集成 SKIP | 已补齐：5 顶层 / 23 命名 PASS | `scripts-integration/completion-ledger.json`；真实 Compose/Docker/PostgreSQL，未执行安装发布 |
| 生产构建 | Web、Desktop、docs 均 exit 0；Server/CLI 编译通过 | `builds/commands.json` 与 `runtime/binary-builds.json` |
| 独立设备认证 | 1/1 PASS | `runtime/device-01/`；独立 device-auth 配置、来源与清理通过 |
| AI 辅助与执行人推荐 | 12/12 PASS | `runtime/assist-01/`、`runtime/recommendations-01/`；本地确定性 Provider，非商用模型质量验证 |
| 平台管理员基础场景 | 7/7 PASS | `runtime/admin-{access,accounts,controls,executions,installations,observability,switches}-01/`；每组独立 API/数据库/合成管理员 |
| 管理资源发布 | 2 PASS、1 FAIL | `runtime/resource-publishing-01/`；真实上传流程到 nativeConsumer 点击 Skills 链接超时，外部私有报告/存储及清理通过 |
| 项目知识自动化 | Web/Desktop 2/2 PASS | `knowledge-acceptance/knowledge-02/`；当前生产 daemon、实际知识输入与重新接收，本地确定性模型/Provider，非完整生产 Main |
| CSRF 与 HTTP/WS Origin | 1 个私有浏览器测试、11/11 明确边界见证 PASS | `runtime/security-boundary-02/`；实际 cookie/真实本地文档服务、零页面路由拦截，拒绝后资料未变，来源与全部清理通过 |
| 百人协作性能 L10 | 3/3 阶段 PASS；独立算术复核 PASS | `performance/performance-01/`；100 账号/连接，10,000 个传播正式样本，业务写回和收尾各 100 个正式样本，均无缺失 |
| 当前 macOS arm64 `.app` | 组装 PASS；完整 Main 场景 FAIL | `native-acceptance/installed-main-02/`；真实密码表单/HTTP 登录后等待 Collaboration 标题超时，尚未进入 Stop/重启/Resume；不等同安装升级或正式签名发布 |
| Linux arm64 打包 Main | 实际启动/登录后原标题断言 FAIL | `linux-main-acceptance/linux-main-04/`；Electron39.8.7、当前包/CLI、file renderer；旧linux03私有权限拒绝保留，Stop/重启/Resume等后续未运行 |
| 旧 v0.6.7 CLI | 原240秒超时 FAIL；一次兜底清理 FAIL | `installed-cli-acceptance/installed-cli-03/`；复制权限已修正并实际9次relay启动，但不构成完整10次写入/拒绝矩阵；20容器10卷最终均不存在 |
| Stage 与补充恢复 | Stage3PASS、业务3PASS、Desktop信息恢复1PASS | 精确原身份/原子场景与退出码保留；知识部署开/关分别1PASS；其余失败见覆盖矩阵 |

以上 QA 路径均相对于 `artifacts/qa/main-post-merge-e2e-20261124-080504/`。同一测试在 normal/race、不同配置或诊断重跑中分别记录，不相加宣称独立业务覆盖量。

L10 在单个本地 API（pool25/min5、无 Redis、多副本关闭）上使用当前真实 HTTP/WS 与确定性受控 Provider。注册按原 30 次/300 秒窗口进行，准备、传播和写回分别耗时 707.28、144.01、234.98 秒。独立重算结果：传播 pooled P95 **665.18ms**，每事件最慢观察者 P95 **701.03ms**（门槛均 2,000ms）；业务结果 P95 **107.35ms**，最终收尾 P95 **409.20ms**（门槛均 5,000ms）。500 个传播暖身样本与 2 个写回暖身样本完整保留并排除，100 个不同观察者与 socket 身份成立。三个原始命令退出 0、source/tool/manifest 校验和清理均通过，原 gate booleans 恢复且 epoch 2→4，API 端口关闭、cluster 排除审计连接后的客户端为 0。其证据只证明该本机单进程受控负载，不代表商用模型延迟或跨区域部署；API 描述符上限未采集。

L10 的进一步只读数据库核对确认含暖身的 101 次 run 与 101 条 Agent 评论/关联均匹配，实际状态均 settled/success 且 `host_exit_observed` 成立，每次有一个授权且已观察的 launch。API 原始日志仍有 **9 次 HTTP503**（8 次 runtime capabilities、1 次 stop-intent），后续收敛与指标通过不等于每次请求都成功；14 次 heartbeat 均为 HTTP200。传播保存了 100 个独立 socket 身份，写回则记录每次 100 条连接、零断连并在测试运行中比较连接身份，但其逐次 socket 身份基线只保留在内存，没有序列化，无法事后独立重建该明细。完整边界见 `research/performance-continuation-results.md`。

## 已确认的阻塞与边界

1. **P1，评论管理越权。** legacy 的 owner/admin 任务凭据与 password 的 owner 任务凭据，对其他人类作者的评论执行编辑、删除、清空附件和带伪造 actor headers 的编辑，分别得到 HTTP 200/204，并观察到实际内容、版本、附件集变化或删除。合法的自身 CRUD、人类管理员 JWT/PAT 对照通过；普通成员编辑/删除收到 403，跨工作区访问收到 404，取消后的任务凭据收到 401。缺陷定位于普通评论角色分支未限定 human principal；对应已有 `11-20-comment-admin-human-authority` 规划任务，本次没有修改它或产品源码。证据为 `runtime/comment-authority-{legacy-02,password-01}/probe-result.json`。generic `comment_id` 上传另有四项风险观察，需要确认其专门权限契约，未擅自当作同一缺陷。
2. **P1，Go race 整包场景失败。** `TestStageCompletionIndependentConsumerAuthorityAndMinimalReceipt/wrong_consumer` 在 M1 授权准备阶段收到 `delivery_completion_conflict`，还未进入预期权限拒绝断言。保留的 PostgreSQL 日志在失败前 1ms 明确记录 `github_pull_request` 的 `SHARE MODE NOWAIT` 表锁申请被拒绝，对应 `server/internal/service/delivery_evidence.go:132`。竞争持锁者未知。窄范围 normal/race 各 5 个命名测试通过，但没有关闭原始整包失败。原流没有 `WARNING: DATA RACE`，不将此失败定性为 Go 数据竞争或权限绕过。
3. **P1，安全扫描失败。** 14 项发现涵盖本机 Go 1.27.0 标准库及 `gorilla/websocket` v1.5.3；报告也列出 `x/net` v0.57.0 的相关映射。包括 Windows 专属条目，不能将其宣称为本机 macOS 利用复现。完整 ID、影响版本和修复版本在 `backend/continuation-result-01.json`。CI 声明 Go 1.26，本机结果不证明 CI 或发布制品所用版本。
4. **P2，外部公共资源不可用。** GitHub 公共 skills 集成补测 `skills-public-01` 已实际失败：远端返回 HTTP 403。缺少响应头证据，不能断言原因就是限流；未借用用户 token，原失败保留。
5. 原生 iOS 缺 Xcode/模拟器，Windows 缺对应原生 runner，Firefox/WebKit 缺已安装浏览器。真实商业模型、正式签名/发布与外部生产操作不属于已授权的本地验收。

本次 Linux 补测补齐了 `TestCodexExecuteRetriesAfterSignaledProcessIsReaped`、`TestPiExecuteAttachesStdinPipe` 以及 `TestClaudeRootSudoPreflight/root_bypass_without_sandbox_errors` 的真实容器执行。父/子测试不重复计为两个业务断言。另七个 Linux router/daemon 场景的 namespace 前置在 UID1000 和 UID0、默认 seccomp、全部 capability dropped 条件下均收到系统拒绝，记为当前获准环境 BLOCKED；此结论不代表所有 Linux 环境不支持，也不覆盖原 Go race 失败。静态二进制、源码/依赖/工具摘要及所有原始退出码已保存，结束时自有容器为零。

安全边界首轮 `security-boundary-01` 的 9 个名义通过中仅 8 个可作为实际证据：Playwright routing 自动回复浏览器预检，导致两条浏览器 CORS 观察无效；另一个 WS 失败来自未采集到 CDP 状态，而实际服务记录 Origin 拒绝。第二轮改用真实本地页面服务器，不启用 routing，保留原 11 项预期和资料前后断言，独立 RFC6455 Upgrade403 与浏览器拒绝分别记录，最终完整通过。第一轮结果和源码仍保留，不以其无效观察宣称安全或产品漏洞。

## 来源与资源恢复

已保留最初 32,724 个跟踪文件的来源清单，所有已核对执行仍对应同一基线。六个既有 PNG/Markdown 支持产物另有当前观察绑定，未被当作最初历史清单的组成部分。

2026-11-25 续接时 Docker 已停止；现有 Docker Desktop 恢复后，只恢复本任务已有的标记容器与数据卷。动态端口改变会保存旧分配记录并重新绑定，不重建覆盖旧库。每个实际子场景使用新数据库/私有 HOME，失败数据库保留。

桌面 current05 的打包失败保留。current06 使用任务私有 Electron 副本，将 14 处经内容相等证明的展开条目恢复为 framework 内部相对链接；原缓存及产品源码未改动。续接发现 current06 在未运行 Main 的准备阶段已出现在系统协议注册中；其加入时间与调用者未知。仅注销这一本任务新增路径后，独立复核恢复了原 9 个注册 URL 与 `file:///Applications/Multica.app/` 默认值。随后 `installed-main-02` 实际运行失败，但其进程、来源/包字节及原协议状态恢复均通过；原 `installed-main-01` 保留为未执行准备记录。

后端 normal/race/UTC/诊断专用 PostgreSQL 与 Redis 已停止。runtime/performance 共用的确切 PostgreSQL 容器也经原 cleanup 命令退出0并停止，停止前其他客户端0，数据卷与所有失败数据库保留。最终 `runtime/global-final-cleanup-receipt.json` 复核182个服务端口均关闭、自有进程和锁为空、无额外 campaign 容器、后端PG/Redis仍停止；32,724个跟踪文件、工具、构建资产和依赖绑定保持原候选，协议默认值与9个注册路径精确一致。

两条原始历史审计均退出1，保留产品失败、原中断和不同adapter证据结构的记录，不能把资源清理PASS改写成历史全绿。CLI03曾使用一次兜底卷删除，其清理FAIL继续保留；最终状态确认其20容器10卷均不存在。最终资源PASS只表示当前自有资源已收尾。

## 本轮文件变更

变化限于本任务计划/报告/缺陷清单、私有验证脚本和运行证据。复用原运行器与测试，删除无效的Python父进程Docker拦截，改为实际Node子进程入口的受限wrapper；没有新增产品依赖，没有修改产品/canonical断言、重试或超时。私有适配均有独立审查与针对性离线验证；源码未变，因此保留原全仓lint/typecheck/TS/Go证据，不重复宣称新执行。

交付入口：`test-plan.md`、`defects.md`、`research/coverage-continuation-review.md`、`coverage-audit/snapshot-24/{summary.json,coverage-matrix.md,original-skip-dispositions.json,final-ac-review.md}` 与 `runtime/global-final-cleanup-receipt.json`。独立终审确认AC01–AC06满足如实交付FAIL报告的条件，绿色发布条件不满足。T/L矩阵是具名用例及实际层次的覆盖，不代表所有可能交互均已验证；docs采用既有测试及生产构建，未宣称独立live站点浏览；四条Agent关键链路来自默认整套，未宣称单独launcher/CI运行。规范经验保留于 `research/spec-sync-review.md`，未改动固定候选的规范文件。任务已原地标记completed，以保留绝对证据引用。

用户随后要求提交并推送。本次提交包含任务文档、覆盖审计脚本、最终覆盖账本和关键结果/清理回执；文件清单及摘要见 QA 根目录的 `publication-manifest.json`。原始日志、trace、截图、私有编排、凭据和完整构建产物留在本机原路径，因此历史文件引用及摘要不代表所有原始文件都随 Git 发布。该提交只交付验收资料，不改变被测候选或FAIL结论。
