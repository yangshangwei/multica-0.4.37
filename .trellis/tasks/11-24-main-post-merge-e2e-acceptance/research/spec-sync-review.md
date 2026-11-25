# 验收流程的可复用经验

本次仍在固定候选 `103e2bc32` 上执行完整验收，暂不改已跟踪的 `.trellis/spec/`。以下内容供执行结束后由主会话按现有规范段落增补；都是本次代码与实际失败证据，不是新产品要求。

## 嵌套 Playwright 报告必须覆盖 OUTPUT_FILE

Playwright 1.58.2 的 `lib/reporters/base.js:567` 先解析 `PLAYWRIGHT_<REPORTER>_OUTPUT_FILE`，其次才使用 config 的 outputFile，再处理 OUTPUT_DIR/OUTPUT_NAME。子探针只改 OUTPUT_NAME 时，外部 runner 的 OUTPUT_FILE仍会接管落点，可能覆盖外层原始报告并让子探针缺证据。

建议增补到 `web/frontend/e2e-run-environment.md` 的 recorder/私有child段：嵌套报告使用新的绝对 OUTPUT_FILE，清除冲突的NAME/DIR；测试同时证明父环境与父文件不变。证据为 `qa/security-boundary-probe/preparation-03.json`：实际已安装 Playwright只读collector收集1个身份、0测试体，将报告写入新文件，并核对预置父报告SHA完全未变。不要用fake recorder代替这个优先级见证。

## CORS验收不得启用请求路由

已安装 `playwright-core/lib/server/chromium/crNetworkManager.js:270` 对被intercept的OPTIONS preflight直接`Fetch.fulfillRequest`为204，并自动加入来源与credentials响应头。即使测试自己的handler仅提供文档、对API调用fallback，也不能据此称其browser preflight来自真实API。

建议增补到现有“Branch and fault-injection runs”：验证真实CORS时使用任务自有loopback静态文档服务且不启用Playwright routing；对照实际API OPTIONS日志和安全响应头，再判断浏览器结果。`runtime/security-boundary-01`原始9PASS/2FAIL中，恶意browser CORS表面PASS也因该机制无效，只剩8项独立真实见证。修正后`security-boundary-02`使用真实listeners，API记录4个OPTIONS200（2直接、2浏览器），允许来源PATCH200；恶意来源无ACAO、无PATCH和无状态改变。原失败保留。

Chromium对被拒绝的WebSocket可能只给error/close而没有握手response CDP事件。可以保留浏览器未打开/未认证见证，另用同Origin的真实RFC6455 Upgrade取得403，并清楚写明两者是独立观测；不能把任意浏览器error等价于403。本次API还记录了两个准确Origin的403拒绝。

## 最终来源核对必须复用同一边界

现有 `server/verification-environment.md` 已要求开始与结束分别绑定源码、工具、workspace links及构建；本次实践无需重复新增抽象规范。私有扩展只追加自己的真实fixture/config/receipt摘要，并在finally复用相同`verify_bindings`与原始阈值。knowledge02在现有固定runner之外新增确定性模型和假provider，也分别保存其源码、26个绝对provider路径、私有capture/profile/config摘要与最终所有后代进程、4端口、gate、连接收尾。准备PASS和来源一致都不能覆盖用例FAIL。

另一个计数注意点：原runtime的`inventory_complete=false`包含业务断言失败，不总表示身份丢失。建议对报告约定补一句“身份集合/命名终端完整性”和“业务通过”分开保存。完整1FAIL/5serialSKIP仍是完整的失败账本；不能因为该字段false就误报测试漏记，也不能因为身份齐全就给业务绿灯。

## macOS协议注册须早于Main准入检查

现有 `web/frontend/e2e-run-environment.md` 已记载 `app.setAsDefaultProtocolClient` 在单实例锁前执行。私有HOME只能隔离文件，不能隔离LaunchServices。此次current06准备后续接时发现本任务新路径已进入注册集，完整Main01实际尚未启动；因此“测试没launch”不是无系统注册变化的证明。

建议在现有协议段追加：包准备后和每次实际Main前都重新读NSWorkspace精确default URL及同bundle注册集；若发现变化，先按已证明的任务新增路径恢复，保存原状态与操作回执，再开始新编号运行。不得删除预先存在注册或按bundleID猜默认。`native-acceptance/continuation02-protocol-recovery*.json`、`installed-main-02/protocol-restoration.json`证明仅注销current06新增路径，恢复`file:///Applications/Multica.app/`与原9个注册URL。真实Main用例后来因locator失败，协议与进程恢复仍独立PASS；两个结论分别保留。

所有`qa/`相对路径指向 `artifacts/qa/main-post-merge-e2e-20261124-080504/`。本文件仅记录候选spec增补，未修改规范、产品或原始测试。

## 本次续接的规范审查结论

继续遵守固定候选与任务限定写入范围：本任务没有产品接口、数据库或依赖契约变更，不在执行中改动已绑定的 `.trellis/spec/`。既有规范已要求来源绑定、私有状态、真实退出码与逐场景清理。以下是补充的工具边界事实，随本研究记录交付，供后续规范维护采用。

- `docker cp` 在本次实际容器中保留了 relay 的 UID501/GID20。容器丢弃所有 capabilities 后，`0700` 副本不能执行；同 SHA 的 `0755` 新副本可进入原程序的空 stdin 拒绝。必须核对实际复制后 UID/mode 和执行回执；主机文件可执行不证明容器进程可执行。保留原文件和失败，再为副本重做准入。
- Python 父进程替换 `subprocess.Popen` 不会截获 Node 子进程自己的 `spawn("docker", ...)`。实际控制边界是选定子环境中的私有 PATH wrapper；应以真实 Node spawn 对照 argv、stdin、退出码和无副作用拒绝。
- 整个 checkout 的只读挂载仍会暴露 `.env`、`.omx/private` 等文件。Linux Main 使用相同字节的三份 canonical 控制器和三个固定 Playwright 包构造最小冻结视图；真实容器验证仅有 `e2e`、`node_modules`，且私有路径不可见。
- 本机固定 Docker 返回小写 `error: no such object: <name>`。不存在对象检查须接受实际接口大小写，并精确拒绝连接失败、权限错误等其他诊断；以大写假 recorder 单独测试不够。
- `PAY241_S3_RESPONSE_STAGE_HOLD_ACCEPTED` 同时用于 Web 和 Desktop 的共享暂停读取 fixture。只在真实自然响应探针通过、独立结果审查与所有摘要匹配后，为精确场景环境传入该标记；离线准备不能授予标记。
- 中断运行即使命名结果全部 PASS，缺少原命令退出码和最终来源/清理也不能改写为完整 PASS。恢复审计单独保存，完整重跑用新标签，旧记录保留 INTERRUPTED。
- 同一失败链上的父/子测试、同一身份的环境重跑、正常/race 两种模式必须分别保留。实际身份完整不等于业务通过；更新后的覆盖审计已有16项离线反例验证这些边界。
