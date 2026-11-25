# Cookie、CORS 与 WebSocket 补充验收

候选仍为 `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。**security-boundary-02 已实际完成：一个私有 Playwright 测试、11个边界见证全部PASS，运行与清理退出0。** 首轮仪器失败完整保留；本结果只关闭明确的cookie/CORS/WS子范围，不关闭已确认的评论机器权限缺陷。

## 发现的覆盖缺口

既有完整 Go 运行包含 `TestPlatformAdminRouterCookieWritesRequireBoundCSRF`、`TestAdminResourceRoutesEnforceRolesAndStoreCapability`、`TestPasswordRejectsCrossOriginSimpleRequests` 和 `TestCheckOrigin`。它们分别验证 cookie CSRF 绑定、管理员资源拒绝、简单表单认证请求拒绝及 WebSocket Origin 判定；不等同部署上的完整浏览器网络边界。

现有浏览器用例证明合法 cookie/CSRF 路径与正向 CORS。`password-branches.spec.ts` 在真实允许来源上保留 CORS 响应头并验证 Retry-After；密码协作 fixtures 验证两个真实登录 cookies 和请求认证形状。现有本轮评论权限探针使用 bearer 凭据，没有补齐 cookie CSRF 负向路径。未找到现成的完整 live 缺失、伪造、其他会话 CSRF 加恶意 HTTP/WS Origin 矩阵。

## 最小探针契约

使用普通密码账号的 `PATCH /api/me`，正文只修改合成名称。`passwordAuthenticate` 在 cookie 请求进入业务 handler 前调用 `ValidateCSRF`；正确 JSON 与真实已完成 onboarding 账号避免把 400 或账号准备拒绝当作权限证据。无需管理员权限、工作区、Agent 或模型。

私有测试复用 `TestApiClient` 注册两个合成账号，调用公开 onboarding 路由，再从真实 `/auth/login` 获得各自 cookie jar。所有凭据只驻留内存；报告只保存认证形状和合成账号的公开 profile。计划共有 11 个独立命名见证：

1. 正确 cookie 与绑定 CSRF 的第一次写入成功。
2. 缺失 CSRF 返回 403，公开 profile 不变。
3. 格式正确但伪造签名的 CSRF 返回 403，profile 不变。
4. 另一账号会话的真实 CSRF 返回 403，profile 不变。
5. 同一合法 cookie 与 CSRF 在拒绝矩阵后仍能写入。
6. 允许来源的真实 OPTIONS 返回对应 ACAO、credentials 和 CSRF header 许可。
7. 非允许来源 OPTIONS 无 ACAO；HTTP 200/204 本身不是 CORS 通过。
8. 真实 Chromium 从允许来源发出 cookie-only PATCH，观察真实 Cookie、CSRF 且无 Authorization，profile 更新。
9. Chromium 从非允许来源收到 CORS 拒绝，保留真实预检、无已传输 PATCH 的网络见证，profile 不变。
10. 允许来源 WebSocket 握手 101，真实第一帧凭据得到 `auth_ack`，随后关闭。
11. 非允许来源 WebSocket 握手 403，无打开、无凭据帧，profile 不变。

两个浏览器文档都是 localhost 上的无业务静态 fixture，通过精确文档路由提供。API、OPTIONS、fetch 和 WebSocket 响应不拦截、不模拟。证据仅证明这一普通 profile 写入和 WS 握手契约，没有声称所有数据库、审计或通知表完全不变，也没有将文档 fixture 宣称为产品 UI 旅程。

## 文件与执行接口

代码在 `.omx/private/qa-work/main-post-merge-e2e-20261124-080504/security-boundary-probe/`：

- `run_probe.py`：供受审 runtime 的 `--probe` 调用，复用已受审的上下文/路径/数据库输入验证，核对自身与依赖审查摘要，执行固定 Node/Playwright 并再次核对来源。
- `security-boundary.spec.ts`：一个原始 Playwright 测试，包含上述 11 个具名见证。
- `playwright.config.ts`：单 worker、零 retry、90 秒单测试、120 秒全局限制，禁用 trace/video/screenshot 防止凭据进入调试产物。
- `probe_selftest.py`：离线计数和报告路径反例；不启动服务、数据库或浏览器。
- `tsconfig.json`、`eslint.config.mjs`：私有代码的严格类型与仓库 ESLint 检查。
- `run_probe.py.review-template.json`：明确为 `REVIEW_REQUIRED`，只能由独立父会话审查后生成 accepted sidecar。

运行负责人使用全新 password lane 与 `--probe-only --probe <绝对 run_probe.py 路径>`。原 runtime 的历史 scope 字符串写着 HTTP-only；本探针实际包含 Chromium，必须以专门 `security-boundary-receipt.json` 和 `probe-result.json` 的显式 scope 为准。为避免影响其他已固定的 native/performance 来源，本补测没有修改原 runtime。

`probe-result.json` 保留 11 项完整状态、失败/未运行身份、前后 profile 和清理结果；`security-boundary-playwright.json` 保留一条真实具名终端；`security-boundary-receipt.json` 绑定原始报告与探针/依赖/源码摘要。外部运行器仍负责 API、数据库和整个 lane 的来源/资源收尾。

## 已完成的离线验证

证据位于 `artifacts/qa/main-post-merge-e2e-20261124-080504/security-boundary-probe/`。

- `preparation-02.json`：严格 TypeScript（含 unused 检查）和实际作用到两个 TS 文件的仓库 ESLint 均 exit 0；只读 collector 为一个 expected-pass 身份、零测试体、零收集错误。
- `preparation-03.json`：9 个离线反例 PASS，包含重复/缺失见证、重试、非零命令、全局/终端错误、清理未完成及继承外部 reporter 路径。
- 初次 collector 的 ESM `__dirname` 失败原样保留；配置改为 `import.meta.url` 后只读收集通过，没有运行产品测试。
- 独立审查发现 Playwright 的 `PLAYWRIGHT_JSON_OUTPUT_FILE` 优先于 `OUTPUT_NAME`。修正后通过实际已安装 Playwright 的只读 collector 证明新嵌套文件落点正确，预置旧报告 SHA 未变。旧 wrapper 在 QA 中保留。
- `preparation-04.json`：失败前先保留实际状态、profile 和网络布尔见证，失败不会丢掉测量事实；9 个离线反例、严格类型与 scoped ESLint 再次通过。旧 spec 与旧 accepted review 保留。父会话已独立接受这一最终准备版本，仍须等待实际 lane。

## security-boundary-01：已执行，保留仪器缺陷

首个真实 lane 已完成，原始结果是 **11/11 见证，9 PASS、2 FAIL**，整套退出1。两个失败名为 `browser-cors-allowed-write` 和 `ws-hostile-origin`。两个账号、Chromium 及数据库连接均关闭/清理；外部 runtime 的 API和来源收尾也有独立回执。

**不能把原始9个 PASS 都当成有效的真实 API 覆盖。** 已安装 Playwright 的 `crNetworkManager.js:270` 会在请求路由启用时自动为 OPTIONS preflight 合成204。首版虽然只打算提供静态文档、对 API fallback，启用的 interception 仍触发了这条行为。实际 API 日志只记录两个直接 HTTP OPTIONS200，没有两个浏览器 OPTIONS；因此允许/恶意浏览器 CORS 两项均缺少真实预检证据，原恶意来源的表面 PASS 也保留为仪器污染，不能升级为安全通过。

WebSocket 原始负向请求实际上已被 API 拒绝：同一时间的 server log 明确记录 `ws: rejected origin`、`request origin not allowed by Upgrader.CheckOrigin` 和 `/ws status=403`。浏览器也记录未打开、未认证、error、close；但 Chromium 没有发出所期待的 handshake response CDP event，数值状态见证为空。该项失败是观测器缺口，未显示产品允许了恶意连接。

原始可独立使用的通过证据有8项：5个 cookie CSRF、2个直接 HTTP OPTIONS、1个真实 WebSocket 第一帧认证。完整探针仍然 FAIL；上述子项不替代修正后的完整重跑。

## security-boundary-02 的窄范围修正

准备已去掉所有 Playwright request routing，改用两个真实本地静态 HTTP listeners：一个绑定本 lane 预分配但尚未启动的 Web 端口，另一个只分配新的 loopback端口。不采纳未知 listener，不停止已有进程。每个 listener 的实际地址/端口/owner PID及关闭后的 ECONNREFUSED都进入报告并成为通过条件。

浏览器 CORS 请求现在直接经过网络到真实 API，记录预检 status 与公开 ACAO/credentials字段。WebSocket 负向仍要求浏览器未打开/未认证，另发同一恶意 Origin 的标准 RFC6455 Upgrade并严格要求403；这两种观测明确分开描述，没有把任意 error当作403。

`security01-source/` 保留完整旧代码和旧 accepted review。`preparation-05.json` 记录修正后9个离线反例、严格类型和 scoped ESLint通过。父会话已独立接受新sidecar，然后由runtime负责人在新数据库执行第二次完整探针；没有改产品、现有测试或原失败回执。

## security-boundary-02：实际通过

11个命名见证均执行并通过，没有失败、跳过、重试或未记录项。nested Playwright为1个测试，不能把11个见证加到默认538的canonical用例数。

- 合法cookie+CSRF前后两次HTTP写入200。缺失、伪造和其他账号会话的CSRF均403，响应是`CSRF validation failed`，完整公开profile前后相同；所有写入均无Authorization。
- 允许来源真实Chromium发出OPTIONS200，响应允许正确origin与credentials，后续真实cookie PATCH200并更新profile。非允许来源的真实OPTIONS200没有ACAO，浏览器TypeError、无已传输PATCH、profile不变。实际API日志记录了两次直接HTTP与两次浏览器OPTIONS，消除了第一轮工具合成204的歧义。
- 允许来源的实际WS101后得到auth_ack并关闭。恶意来源浏览器未打开/未认证，独立同Origin标准RFC6455 Upgrade403；API日志分别记录这两个拒绝请求与精确恶意Origin。Chrome仍不报告失败的握手response事件，报告保留该事实，没有伪造CDP403。
- 两个文档listeners确实由probe拥有，位于localhost IPv6 loopback端口64451和64686；二者关闭后连接探测均为ECONNREFUSED。Chromium exit0，两个合成账号删除，数据库clients关闭，外部API端口和原gate恢复、source/build绑定均通过。

实际证据：`artifacts/qa/main-post-merge-e2e-20261124-080504/runtime/security-boundary-02/{probe-result.json,security-boundary-playwright.json,security-boundary-receipt.json,probe.receipt.json,api.log,cleanup.json,completion.json}`。独立覆盖审计`snapshot-08/security-probe-audit.json`重新核对outer completion与nested原始摘要，结果PASS。

该子范围的风险是证明边界：只观察特定`PATCH /api/me`公开profile与同一部署网络策略，没有声称所有业务写入、全库审计或通知无副作用；cookie CSRF、HTTP浏览器CORS、WS Origin判定仍是三种不同机制。
