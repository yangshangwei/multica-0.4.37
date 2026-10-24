# 跨平台协作执行运行时支持

## Goal

让协作 AI 执行在产品行为上与普通 Multica 桌面端、CLI 和一般智能体执行保持一致：用户无需按操作系统选择不同的协作能力；Linux、Windows、macOS 运行时都能参与同一套 claim、permit、停止、结果记录、终止证明、重启恢复、附件/Git 准备和 Provider bundle 流程。

## Background / Confirmed facts

- 当前协作执行协议把 containment 固定为 `linux_pid_namespace_v1`：`server/pkg/protocol/collaboration.go:6`、`server/internal/service/collaboration_runtime_capabilities.go:126,282,493`、`server/internal/daemon/collaborationrun/client.go:78,121`、`server/internal/daemon/collaborationrun/runner.go:82`。
- 能力注册只接受 `OS=linux`、`Arch=amd64|arm64`，并要求 Linux PID namespace 和附件格式：`server/pkg/protocol/collaboration_preparation.go:276-281`。服务端还要求安装记录的 OS 与能力报告一致：`server/internal/service/collaboration_runtime_preparation_capabilities.go:97-103`。
- 当前 Linux controller 依赖 user/PID/mount/IPC/UTS namespace、tmpfs、proc、pivot root、`/dev/urandom` 设备绑定、`prctl` 权限收缩和 namespace init 的死亡证明：`server/internal/daemon/collaborationrun/controller_linux.go`。
- 非 Linux 构建目前把 controller 标记为 unsupported：`server/internal/daemon/collaborationrun/controller_unsupported.go`。macOS 只复用了 POSIX ledger/source-file 辅助代码，不能证明协作 containment；Windows 没有对应实现。
- Provider bundle 校验写死 ELF、Linux loader/library triplet 和 `runtime.GOARCH`：`server/internal/daemon/collaborationrun/controller.go:338-375`、`provider_bundle.go:302-370`。因此 macOS Mach-O、Windows PE、平台路径和动态依赖闭包尚未覆盖。
- 终止证明、容量占用和重启恢复依赖当前 process/namespace identity；现有普通进程树工具的 Windows Job Object 和 Unix process group 只能作为参考，不能直接替代协作执行要求的 containment proof：`server/internal/daemon/processtree/`、`.trellis/spec/server/collaboration-targets.md:158-164`。
- 现有端到端协作路由和 CLI/daemon 验收测试明确只在 Linux 执行，macOS/Windows containment 仍被规范定义为拒绝：`server/cmd/server/collaboration_*_routes_test.go`、`.trellis/spec/server/collaboration-preparation.md:101-110`。
- 普通 Multica 的安装、桌面端和一般 daemon 已覆盖 macOS、Windows、Linux；这只能证明普通执行跨平台，不能证明协作执行的跨平台安全边界。

## Requirements

### Product behavior

- 同一协作执行 API、状态机、UI 投影、CLI 业务命令和重试身份在三种操作系统上保持一致。
- OS/CPU/containment 只作为运行时内部能力协商和诊断信息，不成为用户需要选择的产品分支。
- 默认支持 Linux amd64/arm64、macOS amd64/arm64、Windows amd64/arm64；32 位 x86/ia32 不纳入本轮目标，除非规划阶段重新批准。
- 每个平台都必须在无法提供等价终止、凭据隔离、工作区隔离或恢复证明时 fail closed，不得降级为普通无隔离任务后继续声称“协作执行”。

### Compatibility and protocol

- 保留现有 run identity、request UUID、nonce ledger、claim/permit/outcome/proof 语义及数据库持久化边界。
- 将当前 Linux-specific containment 字段扩展为可校验的平台 containment capability；旧 Linux 客户端继续可用，服务端不能接受未知或未经注册的 containment。
- capability registration 必须声明 OS、arch、containment adapter/version、Provider bundle format/version 和实际 probe 结果；claim、permit、proof 必须绑定同一份冻结 capability。
- 服务端必须区分“平台不支持”“能力注册过期/变化”“运行中止但终止未证明”，并保留当前容量和更新 barrier 语义。

### Native runtime adapters

- Linux 保留现有 namespace adapter，抽取与平台无关的 Runner/ledger/proof 接口。
- macOS 提供独立的 process containment、工作区/临时目录隔离、凭据注入、子进程树终止和终止证明实现；必须明确使用受支持的 macOS 原生机制，不能把 `sandbox-exec` 或普通 PGID 直接当作已验证等价物。
- Windows 提供独立的 Job Object、进程创建/继承控制、工作区/临时目录隔离、凭据注入和终止证明实现；必须覆盖 Job Object 逃逸、父进程先退出、继承句柄和服务/桌面进程权限边界。
- Provider bundle 的可执行文件格式、架构、动态依赖和系统文件白名单按 ELF、Mach-O、PE/Windows loader 分别验证；禁止 PATH 发现、宿主 home/配置隐式泄漏和未经声明的 Provider fallback。
- 运行时探测应在注册前验证平台实际能力；探测失败时不参加协作候选，不影响普通 Multica 任务。

### Verification

- 为 Linux、macOS、Windows 各建立 native acceptance lane，覆盖单 Agent、squad leader/member/continuation、Stop/Resume、取消、Provider outcome、失联恢复、重启 proof recovery、容量一、附件/Git preparation 和 bundle 校验。
- 每个平台至少验证 direct child、double-fork/后台 descendant、关闭 stdio、leader 先退出、强制终止、重启后 retained ledger、错误/篡改 capability 和错误架构 executable。
- 不能用 Docker Linux、Wine、交叉编译或仅 mock 的 renderer 结果宣称 macOS/Windows 原生验收通过。

## Acceptance Criteria

- [ ] Linux amd64/arm64 现有协作流程回归通过，协议和数据库状态不发生无意变化。
- [ ] macOS amd64/arm64 与 Windows amd64/arm64 daemon 能完成 capability probe/register，并能被服务端作为协作候选；不支持的能力仍 fail closed。
- [ ] 三个平台都能完成一次真实单 Agent 和一次 squad（leader/member/continuation）执行，结果、评论/活动、容量释放和 proof 与 Linux 语义一致。
- [ ] 三个平台都能在 Stop、取消、父进程退出、Provider double-fork、网络中断和 daemon 重启后维持当前 unconfirmed/proof/recovery 语义，不提前释放槽位或自动 Resume。
- [ ] 三个平台的 legacy executable 和 prepared Provider bundle 都拒绝错误格式、错误 arch、动态依赖闭包不完整、路径替换、未声明设备/库和隐式凭据来源。
- [ ] 旧 Linux runtime/client 与新服务端兼容；未知 containment/version/OS/arch/adapter 被拒绝且不改变队列、grant、history、resource 或 control 状态。
- [ ] 文档、安装包/CLI 产物、CI matrix、原生 runner 和故障诊断清楚标识“普通执行跨平台”与“协作执行 capability 已通过原生验收”的区别。

## Out of scope / Deferred

- 不改变普通 Multica 任务的权限模型、Codex/Claude 等 Provider 自带沙箱默认值或桌面 UI 的普通运行时选择器。
- 不在本轮新增 Provider 类型、OAuth/home/session 账户适配器、MCP/Composio/skills 能力；先复用现有受限 adapter，并按平台扩展文件格式验证。
- 不承诺 Linux 以外平台可以复用 Linux namespace 的安全强度；平台 adapter 必须分别证明其 containment 和 proof 属性。
- 不把 32 位 x86/ia32、FreeBSD、Android、iOS 纳入默认支持矩阵。

## Key decision

- 本轮按**完整等价安全边界**规划：prepared bundle、附件/Git、终止证明、容量 barrier、重启恢复和 squad leader/member/continuation 都必须纳入三平台验收。这样首期交付范围较大，但不会出现“界面上跨平台、关键执行路径仍是 Linux 特供”的半成品状态。
