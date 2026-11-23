# AC43 本轮验收结果 — 2026-11-05

本轮四类业务场景中，三类通过，跨环境小队在 Claude 队长执行时被供应商安全策略拒绝。完整 AC43 保持未完成，父任务不归档。

用户已授权不限次数调用本地 Codex / Claude Code，明确排除网关用量和账单对账，并在普通 Codex app-server 被网关拒绝后指定直接使用 `codex exec`。本报告据此评价运行配置、业务身份及授权来源；没有验证独立计费账号或实际扣费。

| 场景 | 结果 | 实际证据 |
| --- | --- | --- |
| 两个目标分别使用 A / B 环境 | 通过 | Claude A 与真实 Codex exec B 分别绑定各自 installation/runtime/grant，完成服务端 outcome/proof |
| 同环境两个人提交 | 通过 | 两个真实指令作者保留，运行环境、授权和配置保持一致 |
| 普通处理人转交 | 通过 | 数据库处理人真实变化，后续执行继续使用原 runtime/installation/grant/epoch/config/invocation |
| Claude 队长 A → Codex 成员 B → 队长续作 | 阻挡 | 队长收到供应商 safeguards 拒绝；成员和续作未执行 |

最终 `real5` 有 4 次成功运行和 1 次失败队长运行，失败运行仍有 `host_exit_observed` 及相等的 authorized/sealed attempt。供应商拒绝消息 ID：`msg_3ce22a1112bc4bf1943e21068c595453`。未通过改写请求、替换模型等方式绕过该拒绝。

## 源码与执行边界

- 产品基线：`d862c5c4bb55d11074d9f4c51643871764e3a9bb`，独立 detached checkout；只有新增验收文件，未修改产品实现。
- 3 个测试自有人类账号、2 个独立登记的 installation/runtime；同一台 macOS、同一 OS 用户，不是跨机器或 OS 安全隔离证明。
- Claude Code 实际版本 `2.1.318`；成功 Codex exec 的保留会话元数据版本 `0.165.0`，originator 为 `codex_exec`。
- Codex 使用显式任务专用桥接脚本，真正调用官方 `codex exec --json`；桥接 logical handle 与实际 Codex thread ID 分开记录。read-only sandbox、无工具的 B 指令，不验证 Codex 工具调用或恢复会话。
- 原生 Multica Codex app-server 在 `real2` 被网关403拒绝（仅允许官方客户端）；这项失败不因 exec 路径通过而变为通过。

## 验证与失败保留

1. 两条本地 CLI 的直接连通性均通过。
2. `TestAC43FixtureContract` 验证真实注册、授权、转交及目录/ledger/Host.Probe 前置；最新执行 1 passed、0 failed/skipped。
3. Go 编译、gofmt、`go vet -tags=agentintegration ./cmd/server` 通过；Python 桥接 12 项无凭据测试通过，并经独立复核。
4. `real1`：测试 ledger 父目录缺失，Provider 未启动；已修复并保留失败。
5. `real2`：Claude A 成功，Codex 普通 app-server 被网关403拒绝。
6. `real3`：Claude baseline / mixed authors / transfer 共3次真实运行通过，明确记录为 partial。
7. `real4`：前三次 Claude 通过；Codex exec 桥误判被 CLI 截断路径的已知启动警告。已修复限定匹配并补正反例，原始失败保留。
8. `real5`：A、mixed、transfer、B 均通过；跨环境小队在 Claude 队长被供应商拒绝，完整测试 exit1。

完整源码 manifest 与运行摘要在本目录。私有原始日志、能力令牌、run ledger 和 provider 输出保留在 `environment.json` 所指的0700目录，未纳入版本控制。

## 最终身份核对与清理

产品源码、验收源码、Multica CLI 和 Claude 配置/可执行文件核对一致。Codex 全局配置和入口脚本在会话期间被外部操作变更，最终本地版本为 `0.166.0`，因此 **final environment unchanged=false**。成功运行的入口脚本启动时哈希与初始记录一致，真实 rollout 记录0.165.0，run-local provider endpoint/model/凭据指纹与初始一致；没有预先冻结底层 Rust 二进制哈希，不把最终0.166.0或完整工具环境宣称为已验证。

6 个本轮专属数据库最终均无遗留连接；数据库与证据保留以供追溯。未部署、未发布、未归档。

下一步只剩本轮跨环境小队的正常供应商执行条件，以及任务原有的主线交付事项。账单/额度对账按用户明确要求排除，不能再作为本轮调用预算阻挡。
