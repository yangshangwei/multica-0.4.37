# RK1：新 runtime/知识版本的真实验收登记

状态：**已登记，不可运行**。调用上限为 0，未消耗任何 Provider 调用。机器可读登记见 `registration.json`，源码绑定见 `source-manifest.json`（由 `source_manifest.py` 生成）。

RK1 是独立账本，编号为 `RK1-<场景>-t<轮次>`，不复用、不填补原 E0–E6 槽位，也不借用原系列剩余的 22 次。

## 已固定

| 项目 | 登记值 |
| --- | --- |
| 源码 | HEAD `d53f1026f`；server tree `192ef85a0`。server 生产代码与知识合入 `8cb04e329` 相同，产品目录内只多了 `65cd0599b`、`629f0a079` 两个测试文件改动；无 overlay |
| 源码指纹 | 7,917 个跟踪文件，`source_id` 为 `15d3275b…88b49`。每次启动前重新生成；指纹变化即作废绑定并重验受影响范围 |
| 平台 | macOS 26.2 arm64，Go 1.27.0，Node 22.23.2，pnpm 10.28.2 |
| Provider | Claude CLI，原生安装符号链接指向 2.1.316（登记时只读链接，未执行 CLI）；运行时以框架的版本探测为准 |
| 模型 | 请求 `claude-opus-5-5`，thinking/tier 为 provider_default。CLI 设置把默认模型映射为 `claude-opus-5-5[1M]`；实际模型只能取自 Provider 输出 |
| 账户 | 推理网关 `sub.100xlabs.space`；不记录账号 ID 或凭据。本机所有 Claude 会话共用此网关 |
| 场景与顺序 | E0a/E0b → H1 → H2 → H3a/H3b → K1 → K2 → K3 → H5 → H6，各场景范围和轮次见 JSON |
| 停止规则 | 首个 runtime/环境/框架不完整结果停止后续全部轮次和场景；不替换样本、不加健康调用、不换模型、不延长超时、不自动重放业务输入。完整但错误的回答是有效样本，不触发停止；H5 canary 被当作指令执行则停下做阻塞审查 |

## 现有入口实际覆盖什么

- `TestHostProviderAccountSmoke`（`host_agentintegration_test.go:32`）能证明真实 CLI 启动、claim 协商参数被接受、CLI 读取 context、outcome/termination/proof 和 ledger。它的 claim 夹具 `hostTestClaim`（`host_runner_test.go:163`）不设置 `HandoffVersion` 或 `KnowledgeContext`，所以 `hostHandoff` 在 `host_handoff.go:44` 直接返回，`host_runner.go:195` 的 `--no-session-persistence` 分支也不会执行。只能用作 RK1-E0a 的 CLI/argv/生命周期部分。
- `TestHostContextExperiment` 只走旧的 `/context`、`/memory`、`/comments`，属于原系列。在新 HEAD 上运行它就是用旧 fixture 跑新代码，不计入 RK1。
- `knowledgeintegration` 组合测试有真实 server/DB/HTTP/WS/daemon，但 Provider 只能是测试可执行文件，没有真实 Provider 模式。可作为 RK1 框架的基础。

## 开始前必须补齐

1. **调用预算**：由用户分配。登记中给出两个参考值：最小集 14 次启动（E0a、E0b、H1、H3a、K1、K2），不含 K3 的完整集 29 次。K3 按实际接线的入口逐个计数。
2. **框架**：带 `agentintegration` 标签、先检查 `MULTICA_RUN_REAL_AGENT_SMOKE=1` 再查找可执行文件，服务端实际授予 handoff/recording/knowledge 版本；场景材料和预期答案冻结在框架里，答案能从记录的工具/HTTP 结果和 Provider stdin 追溯。
3. **假 Provider 证明与独立复核**：每个场景先用测试创建的可执行文件跑通，证明确实调用了 handoff/recording/recall/knowledge 接口，且不完整结果会停止后续；复核后把框架 SHA-256 写入本登记。
4. **H5 的第二个自控账号**：没有就把 H5 标为未运行，不用单账号代替。
5. **独占窗口**：本仓库没有编码、审查、数据库套件或迁移，原系列不在运行，其他会话也不在同一网关发模型请求。原系列 E2a trial 3 当前由会话 multica-0-4-37-54 持有。

人工页面、服务权限、fake stdin 与真实模型理解分别报告。Windows、安装包组合、多账号/额度溯源仍归父任务矩阵；技能层和生产知识开关（保持 false）不在本系列范围内。
