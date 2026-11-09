# 测试设施只读研究（2026-11-09）

本文件记录规划事实，不是测试报告。研究未启动服务、连接数据库、执行测试或修改产品代码。

## 现有覆盖

| 文件 | 可复用事实 | 缺口 |
|---|---|---|
| `e2e/collaboration-memory-runtime.spec.ts:45` | 同任务、两个独立拥有者的实际进程/CLI、上下文 v1→v2→v3 | 无交付/验收；生成的 fixture 账号数量不能冒充实际执行拥有者数量 |
| `e2e/collaboration-detail-journey.spec.ts:37` | 人工请求信息、交付、退回、重新提交和验收 | manual、无 AI、legacy auth |
| `e2e/collaboration-host-squad.spec.ts:13` | 实际 Leader/成员/继续执行、指令合并和退出记录 | 两个 Agent 同一拥有者/运行环境，无人工验收 |
| `e2e/collaboration-resource-offers-web.spec.ts:7` | 外部资源提供、采纳、独立拥有者授权 | 无实际 Provider 进程 |
| `e2e/collaboration-live-revoke-web.spec.ts:14` | 撤权与队列边界 | 只取得 permit，不能证明实际启动/终止 |

新增贯穿测试是必要的，不能把这些独立测试拼接成同一条任务的完整执行证明。

## 可复用入口

- `e2e/fixtures/collaboration-host-runtime.ts:407`：createHostActor，密码注册和实际 Cookie 登录。
- 同文件 `:176` enrollHostRuntime、`:218` startHostDaemon：托管注册、隔离目录、生产 poller/HostRunner、实际 Provider 回调。
- 同文件 `:652` sendHostComment：提及、预览、发送。
- 同文件 `:430` adoptHostTarget：仅适用于提供者有采纳权限且目标资源均由其拥有；跨拥有者路径须拆开。
- `e2e/fixtures/collaboration-handoff-chain.ts:44`：人的有向处理流程 UI 模式。
- `e2e/fixtures/collaboration-host-provider.go:45`：脚本化评论、小队活动和文件写入。
- `e2e/fixtures/collaboration-memory-provider.go:164`：实际 CLI 上下文读写；仅接受 release 回调。

## 当前工具可用性

只读检查观察到 Node 22.23.2、pnpm 10.28.2、Go 1.27.0 darwin/arm64、Docker 客户端/服务端 29.2.1、Playwright 1.58.2。Chromium 1208 的缓存可执行文件存在。PostgreSQL 客户端 18.6 位于 `/opt/homebrew/opt/libpq/bin/psql`，未在默认 PATH。

上述观察不证明数据库可连接、容量充足或浏览器可成功启动，执行阶段须验证。

## 环境陷阱

- `scripts/dev-env.sh:1380` 按 checkout 复用注册表中的环境，改 `--name` 不足以隔离。
- 同文件 `:1414` 会随端口 slot 改数据库名称，新 registry 也不自动证明最终数据库新建。
- `e2e/env.ts:5` 加载默认 dotenv，`e2e/fixtures.ts:14` 存在默认 DB fallback；测试进程必须显式绑定本任务配置。
- `hostEnvironment():83` 要求非默认 DB、loopback URL、部署 UUID、三个绝对二进制路径、`E2E_PASSWORD_AUTH=1` 及真实 renderer URL。
- `scripts/check.sh:61` 强制 legacy auth；不能直接作为 password/managed host 主线入口。
- `server/internal/daemon/collaboration_host_router_test.go:26` 提供编译后二进制的专用 poller 入口；`go test -c` 仅编译，不执行 TestMain。
- 所有共享全局 gate 的测试串行执行，保留原始 gate 状态且按逆序恢复嵌套 fixture。

## 证据边界

本次模拟 Provider 结果由测试控制。可验证账号授权、调度、进程生命周期、上下文传递、业务记录和 UI；不能证明商业模型自主决策、真实账号配额、第二台物理机器或其他操作系统。
