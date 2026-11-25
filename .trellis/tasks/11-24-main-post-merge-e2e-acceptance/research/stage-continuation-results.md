# Stage 执行与 M1 界面续测

日期：2026-11-25。当前源码：`103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。结果继续随实际运行更新。

**M1 Web / Electron 两个原始场景通过；Stage 第一次尝试在浏览器准入前被正确拒绝，三个业务场景尚未执行。** 原始失败、旧二进制和旧编排脚本均保留。产品源码、canonical 测试、断言和超时未修改。

## M1 两端实际验收

`m1-01` 使用 unchanged `e2e/issue-completion-authorization.spec.ts`，只收集两个身份，实际 **2 PASS、0 FAIL、0 SKIP、0 retry**。原 runtime 和专用 adapter 都退出 0。

复用 `.trellis/tasks/11-12-agent-squad-followup-roadmap/research/m1-renderer-fixture-20261113.mjs` 的专用 renderer producer，保留原文件不改，在本任务 private 目录保存完整原字节和副本。唯一准备行为变更是把直接 SQL 填写 onboarding 替换成实际 `PATCH /api/me/onboarding`、`POST /api/me/onboarding/complete`，校验返回的 actor ID 和 `onboarded_at`；diff 保留。其余 synthetic producer 记录、两份本地产物暂存和真实 HTTP 审批/packaging 逻辑保持原样。

首次临时 API 仅用于账号、工作区、项目、任务和产物准备，执行前明确验证本任务新数据库。staging 封为 0500 后停止该 API，以同一个新库和已封闭的输入根启动最终生产 API，再通过真实 artifact-package / approval / execution 路由打包。两步 producer 均退出 0，最终 API 的 health PID、源码 SHA、production Web runtime URL 和实际监听归属由原 runtime 验证。

两条真实界面链路分别通过键盘选择精确 approval、观察 preview、填写人类授权原因、提交一次授权和一次完成，验证 consumed authorization、版本和 receipt，并保留宽/窄 Web 与 Electron renderer 截图。桌面是当前 renderer/preload 与 fixture Main，测试确认 daemonStarts 为 0；不能提升为完整生产 Main 验收。

最终 API/Web 监听关闭，spec 自己创建的 M1 renderer 端口 `65345` 也由外层单独验证关闭；所有数据库客户端为 0，后代进程无残留且无需 fallback signals；source/tool/build 绑定未变，gate 原值保留。失败和成功数据库都未销毁。

## Stage 第一次准入失败及修正准备

`stage-01` 编译并实际启动当前 `TestCollaborationStageExecutionHarness`。专用 AF_UNIX bridge 完成 ready 握手，验证 harness PID 32753 与 health PID 相同；捕获两个自身监听端口 64955、65043。原 runtime 随后以 **`Actual API candidate or listener PID differs`** 拒绝继续，未启动 Web/browser，也没有运行三个 Stage 业务测试。不能把 Go harness 在关闭时的 PASS 当作三个业务场景通过。

原因定位在 task-private 编译命令：`go test -c` 的测试主体不是最终的 `main` package，原 `-X main.commit=<candidate>` 没有绑定真正的提交变量。对原二进制执行 `go tool nm`，实际存在的变量是：

```text
github.com/multica-ai/multica/server/cmd/server.commit
```

原二进制缺少该变量的 `.commit.str` 注入符号。`compile02` 改用完整包路径 `-X github.com/multica-ai/multica/server/cmd/server.commit=<candidate>`，输出全新 `stage-router02.test`，编译退出 0，`nm` 同时观察到变量及 `.commit.str`，保留 build info 和 source-after 清单。当前实际 health 的提交校验继续保持原样，不能仅凭编译成功算通过。

原失败启动正常通过 private shutdown 请求结束，harness 自身退出 0；两个端口关闭、socket 移除、全 cluster 客户端 0、gate booleans 与 epoch 均未变化、无残余进程，源/工具/构建校验通过。没有放宽 health 来源检查。新的 `stage-02` 只能在独立复核新编译产物及输入后，使用全新数据库、artifact slots、socket 和输出目录重新进入。

## 来源保留与证据

以下相对于 campaign QA 根：

- `stage-acceptance/m1-01/`：实际 staging/packaging 命令、bootstrap/final API 归属、M1 renderer 分配、完成及独立资源审计。
- `runtime/m1-01/`：两个 canonical 测试 collection、Playwright JSON、命名对账、截图、原运行和清理证据。
- `stage-acceptance/stage-01/` 与 `runtime/stage-01/`：原始准入失败、ready/listener/shutdown/exit 及资源恢复；零 Stage 业务执行。
- `stage-acceptance/compile01.json`、`compile01-symbol-diagnosis.json`、`compile02.json`：错误与正确链接符号的实际命令、工具、二进制和 source 清单。
- `stage-acceptance/producer-plan.json`、`m1-producer-onboarding.diff`：旧 producer 与仅 setup 修正的边界。
- `stage-acceptance/compile02-original-adapter-retention.json`：证明 `run_stage.before-compile02.py` 字节 SHA256 `bcb62c58aa7f65455bd4c926ffc1a0efabfd14c5446aed1d2358d8704cf96783` 与 stage-01、m1-01 的各自受审输入完全相同。原成功运行以这些保留字节绑定，不用之后变更过的 wrapper 冒充。

私有 adapter 离线 Python 检查 6/6 PASS、Node syntax PASS；真正的 AF_UNIX bridge 用显式新建 fake harness 验证请求身份、原回复传输、shutdown 及 socket 退出，四项 PASS。没有在离线检查中运行真实 SQL、模型或 daemon。新的 wrapper 只调整 Stage binary/receipt pin，并额外保留进入来源守卫前的实际 health JSON；尚待实际 `stage-02` 的结果。

## 2026-11-25 stage-02 complete replay

The independently reviewed compile02 adapter ran unchanged: 3 collected/observed/PASS, 0 fail/skip/retry/missing/unexpected/global errors, runtime and adapter exit0. The actual health commit matched103e2bc32. All API/Web/renderer and harness ports closed, control socket removed, no owned processes remain, cluster other clients0, gates restored with epoch2→8, and final source/tool/artifact bindings passed. Evidence: `stage-acceptance/stage-02/{completion,cleanup}.json` and `runtime/stage-02/{named-inventory,completion,cleanup,source-end}.json`. Original stage-01 remains failed before browser admission.
