# Codex 产品适配与验收进度

## 当前结论

Codex 产品支持、完整模拟验收和独立审查已完成，本地集成提交为
`88f1cc8d4d809ed175d29a96ee2c99cd012bc36a`，独立工作区提交为
`429b1b6b4e8316aa943a0286b6b14c61b063298c`。当前分支的 4501 个服务端构建输入与
已验收字节一致。真实知识执行仍为 **0/15**，任务继续 `in_progress`，生产默认关闭。

用户已明确允许使用本机 Codex，并选择补齐完整 Codex 产品支持后真实验收。
当前阻塞是共享执行窗口：runtime 已明确落盘释放，通信恢复后 core 负责人明确
告知重型验收还需约 30–60 分钟，暂不移交新窗口；自然边界后通知。此前消息接口
HTTP 502 及 AC43/core 在途占用均保留记录。真实 registration/permit 草案仍为 false，
没有以时间经过或某一进程退出代替窗口移交。用户的本机 Codex 使用授权保持有效。

## 实现

- 普通 Codex 知识执行：真实线程 ID 正向 ACK 后发送原输入；精确恢复、配置身份、
  授权与撤权检查；禁止自动新建线程重放失败输入。
- 协作 Codex 知识执行：服务端与实际 Provider 一致，验证 ephemeral 线程，不保留
  可复用指针。Claude 原路径和非知识 Codex 恢复行为保留。
- 私有错误、未知状态与轮次 ID 脱敏；严格关联 turn/start 响应与后续通知。
- 验收框架使用原生双向 RPC、真实生产者/消费者、受限 CLI 读取及独立台账。
  物理启动前预留额度，启动事实、进程终态、完整样本持久化分开记录；前一项不完整
  不允许后续启动。15 次 measured 和 3 次 fake seed 分开计数。

复用现有授权、进程生命周期和配置隔离工具，无新依赖、数据库迁移或 UI 改动。
执行规范、操作指南和内置 Agent 指南同步更新。

## 验证

| 范围 | 结果 |
| --- | --- |
| Agent adapter | 新知识回归及相关旧 Codex 测试通过；normal/race、vet；A1/A2/A2-R 均有先失败后通过的回归证据 |
| Service | normal/race 各 16 个具名结果通过，0 失败/跳过 |
| Daemon / Host | normal/race 各 23 个具名结果通过，0 失败/跳过 |
| 完整 Codex fake normal03 | 45 具名通过，0 失败/跳过/非法 JSON；15 measured +3 seed +0 paid |
| 完整 Codex fake race01 | 同上；无数据竞争报告 |
| 构建和文档 | Go 构建、相关 vet、gofmt/diff 检查通过；内置 skill 模板测试通过 |
| 独立审查 | A adapter、B 集成、C 框架及最终真实准入包复核通过，真实启动等待独占窗口 |

两轮完整 fake 各 18 份样本、54 个构件哈希已独立复核，4501 个源文件、依赖与嵌入
资源及二进制/CLI 均绑定一致。最终 fake 与另一任务 quiet 窗口发生重叠，保留事实，
**不把它们表述为无竞争环境证明**；普通功能与竞态测试结果仍按原始输出记录。

此前 duplicate runtime 的 normal01 失败、编译期间资源变更拒绝、旧 Agent 清理时序
失败及复测均保留，未修改阈值。前端未改动，本轮未重复 TS 全套或浏览器全套。

## 真实执行准备与剩余条件

本机安装版 Codex `0.165.0`、用户配置模型 `gpt-6-astra`，使用原配置的自定义 Responses
服务。私有测试配置保留该 Provider 凭据，排除开发会话 hooks；全局配置未修改。
真实二进制选用同一 npm 安装包内的原生程序，哈希已核对。凭据不进入版本库或日志。

执行顺序 E0b1 → K1四次 → K2两次 → K3八次，总上限 15 次物理执行，每次最多
5 分钟，整体固定 90 分钟；首个不完整结果停止，不自动补测。三次前置 seed 保持模拟。
全源码、测试二进制、CLI、私有配置、真实程序、数据库、有效期与台账已整理为独立执行包。
当前 ledger 为空，真实输出目录不存在，启动许可尚未签发。

另一个任务的历史证据显示同一网关曾拒绝 `multica-agent-sdk`，提示只允许官方客户端。
其凭据与本轮配置不同，因此这仅是外部兼容性风险，不能判定本轮已经失败。维持真实
客户端身份，不改写 originator/User-Agent，不用 `codex exec` 桥接替代当前生产链路。

后继应读取最新窗口记录，取得明确移交并核对现场无并发重型/模型任务，再对已经审查的
配置和当前字节复核，签发单次许可并执行。结果完成后独立复核模型行为和实际输入，
更新父任务矩阵；发布、Windows/安装升级等仍由各自验收负责。

## 证据

- [最终 adapter 审查](research/codex-real-20261104/adapter-final-review.md)
- [集成审查](research/codex-real-20261104/integration-review.md)
- [框架修正复核](research/codex-real-20261104/harness-recheck.md)
- [完整 fake 结果](research/codex-real-20261104/harness-final-fake-results.json)
- [真实准入最终复核](research/codex-real-20261104/real-admission-final-review.md)
- [本地集成字节绑定](research/codex-real-20261104/code-commit-binding.json)
- [执行窗口状态](research/codex-real-20261104/real-window-claim.json)
- [只含摘要的真实执行草案](research/codex-real-20261104/real-execution-draft.json)
