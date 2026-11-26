# 原生 Codex 工具隔离的无模型探针

这是本地进程与协议探针，不是模型验收。运行在已有 image digest `sha256:f3ba3b59ec5b66ed823735c612885ef11479914091e976c7df3410a17525608f`，每个容器 `--network=none`，没有挂载宿主账号目录或 credential，没有发送 `turn/start` 或模型请求。确实启动了 Codex help/schema/sandbox/app-server 本地进程；“真实模型 0 次”不等于“Codex 进程 0 次”。试验里的 thread/start 是无凭据的合成模型名、ephemeral thread，仅用于调用 MCP 方法。

## 事实与可行路线

1. 默认 drop-all-capabilities 容器内 bwrap/user namespace 失败（`native-sandbox-probe-20261120.json`）。以 UID1000 运行，保持 cap-dropALL/no-new-privileges/read-only rootfs，针对这个一次性容器设置 seccomp=unconfined 后，unprivileged user+net namespace 和 Codex `:workspace` sandbox 成功（nonroot/profile 两份记录）。未增加 capability、没有 privileged、没有改共享 Docker 或内核 sysctl。
2. `seccomp=unconfined` 移除了 Docker 默认 syscall filter，扩大内核攻击面；不是建议用于任意生产机器。角色隔离仍依赖容器 mount/user/network namespace、只读根目录、无 host mount/socket 和 Codex 自身 bwrap。最终选定 runtime/镜像必须重新验证。
3. 产品适配器 `server/pkg/agent/codex.go` 的 `thread/start` 依赖配置；`handleServerRequest` 会自动接受 command/file 以及 network/fileSystem 权限请求。单纯 task config 的 `approval_policy=never` 不足以证明不可扩大，必须叠加模型不能改的 root-owned/read-only managed requirements。
4. 只限制 `allowed_sandbox_modes=[read-only,workspace-write]` 时，native app-server command/exec 的普通 TCP 访问被拒，dangerFullAccess 被拒；但显式 workspaceWrite/networkAccess=true 能访问控制面 loopback。原始绕过证据在 `native-appserver-network-r4-20261120.json`，不能忽略。
5. 再加 managed `[experimental_network] enabled=false`，普通 command 和显式 networkAccess=true 都无法连接同容器里真实监听的控制面 loopback；dangerFullAccess 仍被 requirements 拒绝（r5/r6）。无网络父容器中的 loopback positive peer 确实存在；拒绝来自子 sandbox 边界，不是目标根本不存在。
6. 同一 native app-server 在这条严格策略下创建 ephemeral thread 后，`mcpServer/tool/call` 可以通过 stdio MCP server 访问控制面的合成 API，并携带合成原始 task 身份；超出固定 issue 的调用返回 isError，API 恰好收到一次请求。证据 `native-appserver-mcp-r6-20261120.json`。没有 turn/start。`synthetic_mcp_api.js` 只是假 API 控制，不冒充 Multica 业务接口。

可行路线已收敛为：每角色容器 + 不可写 managed requirements 强制 shell 无网络 + 受管 stdio MCP Multica broker。broker 应调用最终构建的真实 Multica CLI，使用该次 server-claimed task token，只允许当前 issue/允许命令，保留原始 API 结果/回执。模型执行普通代码测试时走原生离线 shell，操作平台时走 broker；两种工具的权限面必须分开。新增 broker 指引只能说明工具用法，不篡改 v2 gate 规则或代写结果。

## 不采用的探针路线

- 原生无网 sandbox 也拒绝 Unix socket connect（EPERM）。自定义 network profile 为允许 Unix socket 打开 network.enabled=true 后，TCP 控制面也能连通；探针明确失败，未采用（`native-boundary-unix-r4-20261120.json`）。
- FIFO 原型出现空流/JSON 结束错误，两轮均失败，没有作为平台通道使用。`native_boundary_probe.js` 是保留的诊断程序，不是可用真实 broker。
- 首个 requirements 只允许 workspace-write 被 Codex 拒绝，因为必须同时允许 read-only；r1/r2 timeout 的 owned 容器精确清理，日志保留。后续有效配置包含两个模式且禁止 network。
- 追加 curl/nsenter 多命令诊断出现三次有限超时（r7/r8 追加诊断、r9 回到最小程序，owned cleanup_confirmed=true），没有声称通过，也没有用它替代既有 socket/permission-widening 证明。r9 最小程序的后续重跑也超时，故保留 r6 的实际成功样本，不声称最终环境稳定性已验证。

## 下一步与准入

无需再请求实施许可。仍要完成：真实 task-token CLI broker、最终 API/daemon role fixture、最终配置哈希与隔离再验、平台全量 evidence oracle、用户给定的 model/account alias/次数/时间参数。当前准备版 guard 继续硬拒绝 real；本次原生机制证明不自动解除准入。

在最终镜像和实际任务上还需测试：managed config不可写、无额外 MCP/工具权限、网络/文件权限申请不能扩大策略、Python socket/curl/其他 shell 路径拒绝、peer role目录拒绝，broker正常操作与越权拒绝、token不会被升级为人类credential。现有 native probe 只证明已列出的机制，不能推导普遍“无法绕过”。
