# Codex 真实验收续作方案

## 当前授权与基线

用户在当前会话明确允许使用本机 Codex，并要求按前述方案推进。执行上限按已说明的 E0b 1 + K1 4 + K2 2 + K3 8，共 15 次真实 Agent 执行登记；一次 Agent 执行可能包含多次模型请求。三个 K3 前置 seed 保持模拟执行。原 E 系列与 runtime25 的额度、源码及记录不变。

2026-11-04 已验收基线：产品共享 v16、审计子任务关闭、知识完整 fake normal/race 各 49 项通过，提交后 79 项绑定通过。源码锚点为 `5e6a4e496` / `da07e2e74`，本次调查 HEAD 为 `1a32ca36c`。旧真实执行包的 0/0 和 NOT_RUNNABLE 是当时事实，保留原件；本次授权单独登记，不把历史证据改写成 Codex 验收。

## 新发现的范围差异

当前产品只允许 built-in local Claude 消费 managed knowledge。Codex 的普通任务/Host 进程监督已经存在，但知识授权、精确会话和原生会话回执没有接入。将框架里的 claude 改名为 codex 不构成有效验收。

具体依据：

- `server/pkg/agent/knowledge_launch.go`：`KnowledgeLifecycleSupported` 与参数验证仅允许 Claude。
- `server/pkg/agent/launch_lifecycle.go`：知识生命周期再次限定 Claude。
- `server/pkg/agent/codex.go`：原生 thread/resume 失败及部分启动错误存在 fresh/retry 路径；知识执行必须保留授权身份，不自动重放业务输入。
- `server/internal/service/task_knowledge*.go`：claim/preparation/native-history 的 provider 判断需要逐项核对。
- `server/internal/daemon/collaborationrun/host_knowledge.go`、`host_runner.go`：协作知识使用 Claude 非持久化策略；Codex 必须有对应的实际线程策略证据。
- `server/cmd/server/rk1k_*_test.go` 与 tagged daemon driver：现有捕获、fixture、替身和场景绑定 Claude stream-json。

这属于新增 Codex 产品支持，范围大于原先的测试入口接线。账户使用授权已获得；用户已明确选择“补齐 Codex 产品支持，再完成真实验收”，本轮产品扩展获得授权。

## 推荐实施顺序与完成条件

1. **提供可审查的 Codex 适配范围。** 完成源码映射、安装版 CLI 协议核对和不含密钥的本机配置记录。更新旧计划的当前状态入口。此阶段无真实模型请求。
2. **产品适配。** 在现有边界加入 Codex invocation 指纹、参数与线程选择约束、每次物理启动授权、原生 thread ID 正向 ACK、精确 resume 与失败终止。复用生产 Codex app-server 后端；普通会话可受控续用，协作使用经过验证的非持久化线程。不借用 Claude 标识或凭据语义。
3. **跨层与模拟验收。** server/daemon/host 协商保持 provider 一致；覆盖未知参数、错误 thread ID、ACK 缺失、撤权、历史不可用、重试与启动数。新增测试仅运行测试自建替身。数据库测试使用新分配的专库；按改动范围运行 lint、类型/编译、普通/race 与独立复核。原 Claude 回归保留。
4. **真实执行包。** 使用用户指定的本机 Codex 配置，固定 CLI/模型/服务配置摘要、源码及测试材料，建立仅本系列的 durable 启动台账。密钥不进入命令行、日志或版本库。绑定独占执行窗口，防止并发的其他真实实验占用窗口。
5. **串行真实验收。** E0b → K1 → K2 → K3。每次实际启动占用一个额度；首个环境、框架或执行不完整结果立即停止。错误但完整的模型答案按模型失败记录。没有自动补测、换模型或额外健康请求。分别报告实际输入与模型行为；三个模拟 seed 不计成真实多 Agent 对话。
6. **整体验收与交付。** 独立核对真实结果与源码、更新任务和父级验收矩阵。生产默认关闭；发布、Windows、安装升级和其他系列的真实验收仍各自满足条件。

## 当前可确认的环境事实

- 本机 `codex-cli 0.165.0` 可执行。
- 当前本机配置模型为 `gpt-6-astra`，选用自定义 Responses provider，配置中存在 bearer credential。`codex login status` 显示未登录官方账户不能单独判定该自定义 provider 不可用。
- 已离线导出安装版 app-server JSON Schema：`thread/start` 支持 `ephemeral`，`thread/resume` 接受精确 `threadId`；原始文件摘要见 `research/codex-real-20261104/installed-protocol.json`。字段存在不等于运行时非持久化与授权行为已验证。
- 未发起模型连通性探测；账户服务是否接受实际请求尚未验证。
- 当前知识真实启动数为 0；生产知识注入默认 false。

## 排除项

不修改用户全局 Codex 配置，不扩展调用额度，不复用原实验台账，不增加依赖，不重开已归档审计子任务，不把 Codex 的独立问答替代生产知识链路验收。

## 参考

- [已完成的验收](verification-2026-11-04.md)
- [官方 Codex App Server 文档](https://learn.chatgpt.com/docs/app-server)
- [源码兼容性调查](research/codex-real-20261104/compatibility.md)

## 已确认的实施合同（独立复核 R1–R4）

- **R1 精确输入与线程**：物理启动授权 → thread/start 或精确 thread/resume → 验证真实 thread ID → 服务端同步正向 ACK → 知识 turn/start。缺失、变化或未 ACK 的线程不得接收知识输入、不得发布可复用指针。普通知识路径首版禁用自动进程重试和 resume-to-fresh fallback；保持既有非知识恢复。最终 turn 输入与已准备的 prompt 保持精确一致，continuity notice 必须在准备前定稿。
- **R2 私有结果**：复用 KnowledgeFailure / KnowledgeOutput，保留已发生的 usage/termination/private output；stderr、RPC 请求错误、异步 error 和退出错误不得将知识正文复制到通用日志/错误。普通诊断保持既有行为。
- **R3 Provider 合同**：新增 `ExecOptions.CodexEphemeral bool` 供协作设置，普通知识禁止 ephemeral。Codex 仅准入 built-in local、无 default/custom 额外参数的最小首版；模型/思考/service tier 通过现有强类型 opts 传递。配置指纹覆盖有效 argv/RPC、模型、service tier、MCP、Codex 账户/配置来源摘要及持久化策略；不在证据输出密钥。host 发送 ephemeral=true 并验证响应，缺失/不支持拒绝，禁止降级；保持既有 wire shape，新增 provider 字符串不自动修改协议版本。
- **R4 证据与次数**：双向原生 Codex RPC 记录与 authoritative turn/completed；底层进程启动前持久记账，逐次授权，计数失败立即拒绝。15 次 measured 与 3 次模拟 seed 分开；首个不完整结果停止。

## 分工与工作区

工作区 `/Volumes/artisan/code/2026/mc-knowledge-codex-20261104`，基线 `1a32ca36c`。主会话管理计划、规范、最终集成与真实执行台账。

- A：`server/pkg/agent/` Codex adapter、knowledge contract/lifecycle、相邻回归测试。独占共享 ExecOptions/指纹接口；先公布接口。
- B：`server/internal/service/` 知识协商/历史/准备，`server/internal/daemon/knowledge_context.go`，`server/internal/daemon/collaborationrun/host_knowledge.go`/`host_runner.go` 及相邻测试。消费 A 接口；不改 A 文件。
- C：`server/cmd/server/rk1k_*` 与 `server/internal/daemon/rk1k_driver_agentintegration.go`，独立 launcher/registration。先评估与设计，再消费 A/B 完成的接口。
- 独立 check：检查所有产品与 harness diff、模拟证据、源码与预算绑定。任何实际 Provider 仅由主会话启动。
