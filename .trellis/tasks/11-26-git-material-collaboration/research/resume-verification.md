# 续作验证

本轮从已有实现继续。验收只使用任务测试库、本机 HTTPS GitLab 和 HTTP provider 测试；未对真实仓库写入，也没有调用真实模型。

| 已执行 | 观察到的结果 |
| --- | --- |
| 已核对数据库身份后，`go -C server test -race ./internal/service -run '^TestMaterial' -count=1 -v` | 首轮三个服务测试通过，覆盖响应丢失、手动等待、撤销权限和门禁；这是后续加固前的结果。 |
| 独立 handler 库从零迁移至 907，`go -C server test ./internal/handler -run '^Test(TaskCapability\|RequireHumanActor\|PluginAction)' -count=1` | 通过。该轮仅证明已有凭据、人工入口、插件安装边界未回归。 |
| `go -C server test ./cmd/multica ./internal/material -run Material -count=1` | 通过。新增 CLI 用例先证明代理错误正文会进入诊断，再验证修复后仅输出 HTTP 状态及可恢复提示。 |
| 插件读基线、面板桥接与 SDK | 宿主从已挂载 issue 取作用域，服务端检验当前授权、安装版本、项目、基线摘要；不接受插件代作审批。候选输出走已有任务 API/CLI。 |
| `node --test e2e/fixtures/material-gitlab.node-test.mjs` 和生产 Web / Electron 人工启动旅程 | 本地 GitLab 测试通过；两个客户端均核对了合并后的 PRD 字节、刷新后零派发、人工启动一次，以及方案/拆解合并后不启动编码。Electron 使用测试 Main 与生产 renderer/preload，不代表安装包验收。 |

首轮浏览器日志与截图位于测试机私有目录 `/private/tmp/multica-material-20261126/material-run-BWojS0`。可复现的入口及环境要求见 `e2e/fixtures/material-run.md`。测试数据库凭据及操作令牌不进入仓库。
