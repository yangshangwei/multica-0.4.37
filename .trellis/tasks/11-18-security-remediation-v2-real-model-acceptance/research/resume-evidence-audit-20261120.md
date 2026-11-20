# 续做审计：历史证据、工具限制与剩余准入

本轮从实际 HEAD `4c78c8b98958cb8a76f1c9a448b56e94285c36b3` 继续现有任务。保留分支 `codex/security-remediation-v2-cli-20261120` 在新 worktree `mc-secrem-v2-audit-20261120` 检出。主检出的其他会话改动未纳入；不复用已删除的旧 worktree 或旧 overlay 绝对路径。

本轮范围是历史平台证据审计、已有装置维护验证、任务记录同步。没有实现 C1 续接、调用真实模型、签发准入、恢复数据库或修改冻结 v2 正文。AC9、AC10 保持开放。机器可核对的摘要见 [resume-evidence-audit-20261120.json](resume-evidence-audit-20261120.json)，新增私有证据在 `/Volumes/artisan/code/2026/secrem-v2-resume-audit-20261120T065505Z/`。

## 重新核实的证据

子任务一三笔产品提交、`50045c12c` 集成提交和 `15e9e8d52` CLI 检查点都是当前集成分支的祖先。子任务一最终冻结 27 个产品路径与 `50045c12c` 及本轮 HEAD 的文件全部逐字节 hash 相符；实际集成回执、最终 service（RUN4239/PASS4220/FAIL0/SKIP19，Linux arm64 Go1.26.6、TZ=Asia/Shanghai）、handler、cmd/server、前端及 E2E 日志原件仍在，属于历史验证，没有本轮重跑声明。默认 UTC 的基线时间比较失败继续保留，不作跨时区通过声明。任务路径在主检出没有未提交改动。子任务一仍完成；父任务与真实验收子任务仍在进行。

| 对象 | 本轮核验 | 结论边界 |
| --- | --- | --- |
| 9 个私有数据库 dump | 原名称、字节数、sha256 全匹配；均为普通文件、0600；`pg_restore --list` 和全量离线解码均退出 0 | 能读取保存的数据库检查点，未验证新实例恢复 |
| C1 dump | 2 条 task、2 条 comment、1 条 issue；与原 `intake-after.json` 和排队快照一致 | leader failed、analyst queued，issue in_progress |
| D1 真实库 dump | task/comment/issue 均为 0 | 没有真实 D1 平台样本 |
| C1 原始结果 | intake-after、intake-result、code-verdict、HTTP 日志 4 个 hash 全匹配 | 原失败未被覆盖 |
| C1/D1 最终脚本 | 原 code-verdict 与仓库摘要相等，各 11 项平台条件为真 | evidence_type=fake；不替代真实模型 |
| 冻结夹具 | 16/16 文件与冻结 manifest 匹配 | 未更换输入或预期 |
| 后续源码副本 | P1 successor 31、live O1 26、C1 launch 33 个已保存文件 hash 全匹配 | 仅证明这些已列文件；首个失败 P1 缺 backend 原始字节的限制仍在 |
| 当前产品输入 | C1 manifest 所列 9 个产品源文件，包括两份冻结正文，与当前检出 hash 一致 | 不把整体新 HEAD 重新标成历史实际模型来源 |
| 旧运行资源 | 旧 worktree 不存在；未发现匹配旧任务的运行进程、容器或网络 | 本轮没有启动旧服务或恢复认证副本 |

C1 的分析师任务 `01a1f79a-473f-7e9e-9cb5-da884d33885b` 仍未领取：started_at/completed_at 为空，claim_generation=0。它的 trigger 是原委派评论 `01a1f79a-4320-75c4-b39f-f22db983e8e4`，delegated_from 指向失败队长 `01a1f792-4d93-76e1-92f8-0835aa3de77d`。委派正文、本人作者/source_task/revision1 和系统失败通知都与原平台快照相同。dump 内 856 条迁移记录，最高为 `868_collaboration_runtime_output`；这是保存状态，不是新环境的迁移或数据库身份证明。

P1/O1 的独立复核及原始用量重算见 [resume-p1-o1-audit-20261120.md](resume-p1-o1-audit-20261120.md)。已有维护的代码、历史探针和本轮控制重跑见 [resume-maintenance-audit-20261120.md](resume-maintenance-audit-20261120.md)。两者均不授权恢复真实运行。

## 原始工具拒绝的范围

从本地 Trellis session insight 定位到原会话 `01a1f5ff-1287-7003-98b8-16496086e2d9`，再读取原 JSONL：

- 2026-11-20 06:20:21.879Z，第 4738 行，发给 `cli_harness` 的请求是适配合法的 G1 前产品分析师分支，讨论恢复原排队任务或新装置样本，以及对应流程控制。
- 06:21:37.244Z，第 4759 行，该子智能体返回错误：**“This content was flagged for possible cybersecurity risk.”** 原回执同时指出，若授权安全工作需要更宽的网络安全保障范围，须先申请 [Daybreak access](https://platform.openai.com/settings/organization/status-and-access) 再重试。
- 原拒绝记录 sha256：`39d25b0a9d7881c16d3fb2fad0fe558e257fd8e739682ea0400bd6637e93bb5b`。完整请求/回执只保存在私有审计目录，仓库仅留范围与 hash。

这是子智能体执行面的内容筛查，不是 magpie 对官方 CLI 的访问拒绝，也不是预算耗尽。回执没有定位具体触发子步骤，不能自行缩小成“只拦某一句”；同样没有宣布禁止所有证据审计或 D1，本报告不扩大其范围。本轮没有新的 Daybreak 获准或工具方解除限制的记录，因此未重述、换代理、换模型或换通道尝试同一续接。

本轮完成的离线证据核验和既有维护控制属于用户明确允许的工作。D1 保持未准入，是因为冻结顺序为 P1→O1→C1→D1，原系列在 C1 装置不完整时已停止，且新环境与当前源码没有新绑定；不将它描述为已被同一筛查单独拒绝。

## 后续恢复的正式前置

1. **工具访问前置**：原被拒续接须有工具方正式访问或复核结果。原回执提供 Daybreak 申请入口；本会话没有可签发该访问的工具，也不能把普通用户授权、切换会话或供应商已接受 CLI 当作解除证明。是否批准由工具方决定。
2. **任务方案前置**：在获得适用访问之后，才处理仍未完成的合法可选分析分支。原 failed leader 与 queued analyst、原失败/用量账本完整保留；不强迫模型跳过可选步骤，不改变冻结预期或 v2 正文。需要变更模板时返回父任务走新版本。
3. **运行环境前置**：新建明确归属的隔离环境；从已核验 dump 恢复时先核对归属、完整迁移 ledger、实际 `current_database()` 与新配置。旧配置和准入不能直接复用，旧模型仓库的 Git 仍只能在隔离容器内读取。
4. **独立技术准入**：重新绑定当前源码、二进制、官方 CLI、镜像、完整 AGENTS/prompt、fixture、配置和场景；完成无模型全链及角色隔离验证，再由独立复核签发。leader/analysis 维护通过只覆盖装置局部。D1 还需要自己的准入。
5. **真实执行前置**：继续同账号、原模型、官方 `codex exec`，保持 1200 秒单次 watchdog、原始 mat_、受限审查员、角色独立仓库和具名人类 G1/G2。每次人工许可前读取实际审查/提交/测试证据并绑定 hash，逐次保留失败和用量。

## 未覆盖与本轮记录限制

没有恢复数据库，因此不声称新运行环境就绪。没有新增模型样本；官方 CLI 累计仍为 7 次启动、70 条 POST，SDK 2 次启动、7 条失败 POST 独立保留。首个 P1 源字节缺失不补造。

审计中三处本地工具假设错误已保留：父任务本来没有 design/implement；迁移版本是带名称字符串；源码副本采用 basename.sha256 文件名。分别核对目录和格式后修正读取方法，没有改历史数据或把失败命令记为通过。
